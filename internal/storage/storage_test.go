package storage

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AhmedAburady/imagine-cli/config"
)

// TestSignRequestAWSVector checks the SigV4 implementation against AWS's
// published "GET Object" header-based-auth example:
// https://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-header-based-auth.html
func TestSignRequestAWSVector(t *testing.T) {
	const (
		accessKey = "AKIAIOSFODNN7EXAMPLE"
		secretKey = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
		region    = "us-east-1"
		wantAuth  = "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request," +
			"SignedHeaders=host;range;x-amz-content-sha256;x-amz-date," +
			"Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	)

	req, err := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Range", "bytes=0-9")

	signTime := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	if err := signRequest(req, accessKey, secretKey, region, emptyPayloadHash, signTime); err != nil {
		t.Fatalf("signRequest: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != wantAuth {
		t.Errorf("Authorization mismatch:\n got: %s\nwant: %s", got, wantAuth)
	}
	if got := req.Header.Get("X-Amz-Date"); got != "20130524T000000Z" {
		t.Errorf("X-Amz-Date = %q, want 20130524T000000Z", got)
	}
}

// TestSignRequestServiceAWSVector checks the generic signer against the AWS
// SigV4 test-suite "get-vanilla" fixture (service "service", no payload
// header), proving the service parameterisation produces the published
// signature. Fixtures: .firecrawl/get-vanilla.{req,creq,sts,authz}.
func TestSignRequestServiceAWSVector(t *testing.T) {
	const (
		accessKey = "AKIDEXAMPLE"
		secretKey = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
		wantAuth  = "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20150830/us-east-1/service/aws4_request," +
			"SignedHeaders=host;x-amz-date," +
			"Signature=5fa00fa31553b73ebf1942676e86291e8372ff2a2260956d9b8aae1d763fbf31"
	)
	req, err := http.NewRequest(http.MethodGet, "https://example.amazonaws.com", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	signTime := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	if err := signRequestService(req, accessKey, secretKey, "us-east-1", "service", emptyPayloadHash, signTime); err != nil {
		t.Fatalf("signRequestService: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != wantAuth {
		t.Errorf("Authorization mismatch:\n got: %s\nwant: %s", got, wantAuth)
	}
}

// TestSignRequestDeterministic verifies identical inputs produce an identical
// signature (no clock/map-ordering leakage).
func TestSignRequestDeterministic(t *testing.T) {
	signTime := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	sign := func() string {
		req, _ := http.NewRequest(http.MethodGet, "https://example.amazonaws.com/a/b", nil)
		req.Header.Set("X-Custom", "v")
		if err := signRequest(req, "AKID", "secret", "ap-southeast-1", emptyPayloadHash, signTime); err != nil {
			t.Fatalf("signRequest: %v", err)
		}
		return req.Header.Get("Authorization")
	}
	if a, b := sign(), sign(); a != b {
		t.Errorf("signatures differ across identical inputs:\n%s\n%s", a, b)
	}
}

// TestSignRequestMissingCredentials verifies the signer refuses empty AK/SK
// rather than producing a bogus signature.
func TestSignRequestMissingCredentials(t *testing.T) {
	signTime := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	for _, tc := range []struct{ ak, sk string }{{"", "sk"}, {"ak", ""}, {"", ""}} {
		req, _ := http.NewRequest(http.MethodGet, "https://example.amazonaws.com", nil)
		if err := signRequest(req, tc.ak, tc.sk, "us-east-1", emptyPayloadHash, signTime); err == nil {
			t.Errorf("expected error for ak=%q sk=%q, got nil", tc.ak, tc.sk)
		}
	}
}

// TestSignRequestDefaultRegion verifies an empty region signs under us-east-1
// (rather than an empty credential-scope segment).
func TestSignRequestDefaultRegion(t *testing.T) {
	signTime := time.Date(2015, 8, 30, 12, 36, 0, 0, time.UTC)
	req, _ := http.NewRequest(http.MethodGet, "https://example.amazonaws.com", nil)
	if err := signRequest(req, "AKID", "secret", "", emptyPayloadHash, signTime); err != nil {
		t.Fatalf("signRequest: %v", err)
	}
	if !strings.Contains(req.Header.Get("Authorization"), "/us-east-1/s3/aws4_request") {
		t.Errorf("empty region did not default to us-east-1: %s", req.Header.Get("Authorization"))
	}
}

func TestURIEncodePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "/"},
		{"/test.txt", "/test.txt"},
		{"/imagine-refs/abc.mp4", "/imagine-refs/abc.mp4"},
		{"/b/my refs/x.png", "/b/my%20refs/x.png"}, // space encoded exactly once
		{"/b/a+b/c.png", "/b/a%2Bb/c.png"},         // '+' is reserved, not unreserved
	}
	for _, tt := range tests {
		if got := uriEncodePath(tt.in); got != tt.want {
			t.Errorf("uriEncodePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestObjectKey(t *testing.T) {
	sc := &config.StorageConfig{PathPrefix: "imagine-refs/"}
	data := []byte("hello world")
	// sha256("hello world") is well-known.
	const wantSum = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if got := objectKey(sc, data, "video/mp4"); got != "imagine-refs/"+wantSum+".mp4" {
		t.Errorf("objectKey = %q", got)
	}
	// No prefix, image MIME → bare hash + ext.
	sc2 := &config.StorageConfig{}
	if got := objectKey(sc2, data, "image/png"); got != wantSum+".png" {
		t.Errorf("objectKey (no prefix) = %q", got)
	}
}

func TestObjectURL(t *testing.T) {
	tests := []struct {
		name   string
		sc     *config.StorageConfig
		key    string
		public bool
		want   string
	}{
		{
			name:   "virtual-host PUT URL (default, TOS)",
			sc:     &config.StorageConfig{Endpoint: "https://tos-ap-southeast-1.bytepluses.com", Bucket: "my-bucket"},
			key:    "imagine-refs/abc.mp4",
			public: false,
			want:   "https://my-bucket.tos-ap-southeast-1.bytepluses.com/imagine-refs/abc.mp4",
		},
		{
			name:   "path-style when path_style:true (MinIO)",
			sc:     &config.StorageConfig{Endpoint: "https://tos-ap-southeast-1.bytepluses.com", Bucket: "my-bucket", PathStyle: true},
			key:    "imagine-refs/abc.mp4",
			public: false,
			want:   "https://tos-ap-southeast-1.bytepluses.com/my-bucket/imagine-refs/abc.mp4",
		},
		{
			name:   "virtual-host trailing slash on endpoint trimmed",
			sc:     &config.StorageConfig{Endpoint: "https://tos.example.com/", Bucket: "b"},
			key:    "k.png",
			public: false,
			want:   "https://b.tos.example.com/k.png",
		},
		{
			name:   "public_url_base override used for read",
			sc:     &config.StorageConfig{Endpoint: "https://tos.example.com", Bucket: "b", PublicURLBase: "https://cdn.example.com/"},
			key:    "k.png",
			public: true,
			want:   "https://cdn.example.com/k.png",
		},
		{
			name:   "public_url_base ignored for PUT (virtual-host)",
			sc:     &config.StorageConfig{Endpoint: "https://tos.example.com", Bucket: "b", PublicURLBase: "https://cdn.example.com"},
			key:    "k.png",
			public: false,
			want:   "https://b.tos.example.com/k.png",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := objectURL(tt.sc, tt.key, tt.public); got != tt.want {
				t.Errorf("objectURL = %q, want %q", got, tt.want)
			}
		})
	}
}
