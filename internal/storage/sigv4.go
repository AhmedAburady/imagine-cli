package storage

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

// AWS Signature Version 4 signing, implemented in pure stdlib (crypto/hmac +
// crypto/sha256). This is the only net-new primitive the storage brick needs —
// the AWS SDK would pull dozens of transitive packages for one signed request.
// The service is a parameter so the signer stays generic; the brick signs for
// "s3", correct for every S3-compatible backend (BytePlus TOS, MinIO, R2,
// Wasabi, AWS S3 itself).

const (
	sigV4Algorithm = "AWS4-HMAC-SHA256"
	sigV4Service   = "s3"
	defaultRegion  = "us-east-1"
	// emptyPayloadHash is sha256("") — the documented hash for an empty body.
	emptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// signRequest signs req in place for the "s3" service, adding the x-amz-date,
// x-amz-content-sha256, and Authorization headers. Thin S3 wrapper over
// signRequestService.
func signRequest(req *http.Request, accessKey, secretKey, region, payloadHash string, t time.Time) error {
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	return signRequestService(req, accessKey, secretKey, region, sigV4Service, payloadHash, t)
}

// signRequestService signs req in place with SigV4 for an arbitrary service,
// adding the x-amz-date and Authorization headers. payloadHash is the
// lowercase hex sha256 of the body (use hashPayload). t is the signing time;
// pass time.Now().UTC() in production (tests pin it to match AWS vectors).
// Empty region defaults to us-east-1. Returns an error on missing credentials.
//
// Every header already present on req is signed (sorted, lowercased), plus the
// host derived from the URL — the standard, vector-matching behaviour. Callers
// add service-specific signed headers (e.g. x-amz-content-sha256 for S3)
// before calling.
func signRequestService(req *http.Request, accessKey, secretKey, region, service, payloadHash string, t time.Time) error {
	if accessKey == "" || secretKey == "" {
		return errors.New("storage: missing access key or secret key")
	}
	if region == "" {
		region = defaultRegion
	}
	t = t.UTC()
	amzDate := t.Format("20060102T150405Z")
	dateStamp := t.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)

	host := req.URL.Host
	if req.Host != "" {
		host = req.Host
	}

	// Canonical headers: every request header plus host, lowercased and
	// sorted by name. Values are whitespace-trimmed.
	headers := map[string]string{"host": host}
	for name, vals := range req.Header {
		headers[strings.ToLower(name)] = strings.TrimSpace(strings.Join(vals, ","))
	}
	names := make([]string, 0, len(headers))
	for n := range headers {
		names = append(names, n)
	}
	sort.Strings(names)

	var canonHeaders strings.Builder
	for _, n := range names {
		canonHeaders.WriteString(n)
		canonHeaders.WriteByte(':')
		canonHeaders.WriteString(headers[n])
		canonHeaders.WriteByte('\n')
	}
	signedHeaders := strings.Join(names, ";")

	canonicalRequest := strings.Join([]string{
		req.Method,
		uriEncodePath(req.URL.Path), // encode the DECODED path once (S3 single-encoding)
		req.URL.RawQuery,            // our requests carry no query string
		canonHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := strings.Join([]string{dateStamp, region, service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		sigV4Algorithm,
		amzDate,
		credentialScope,
		hashHex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(secretKey, dateStamp, region, service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	auth := sigV4Algorithm +
		" Credential=" + accessKey + "/" + credentialScope +
		",SignedHeaders=" + signedHeaders +
		",Signature=" + signature
	req.Header.Set("Authorization", auth)
	return nil
}

// deriveSigningKey computes the SigV4 signing key chain.
func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// hashPayload returns the lowercase hex sha256 of body, or the empty-body
// constant for nil/empty input.
func hashPayload(body []byte) string {
	if len(body) == 0 {
		return emptyPayloadHash
	}
	return hashHex(body)
}

// uriEncodePath percent-encodes a DECODED URL path for the SigV4 canonical
// request. It is given req.URL.Path (decoded), not EscapedPath, so it encodes
// exactly once — re-encoding an already-escaped path would turn "%20" into
// "%2520" and break the signature. It keeps RFC-3986 unreserved characters and
// '/' literal and percent-encodes everything else, matching S3's
// single-encoding rule and the form Go's URL.RequestURI() puts on the wire.
func uriEncodePath(path string) string {
	if path == "" {
		return "/"
	}
	var b strings.Builder
	for _, c := range []byte(path) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '/':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			const hexUpper = "0123456789ABCDEF"
			b.WriteByte(hexUpper[c>>4])
			b.WriteByte(hexUpper[c&0xf])
		}
	}
	return b.String()
}
