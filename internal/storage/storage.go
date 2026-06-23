// Package storage is a generic, S3-compatible upload brick. Any provider that
// must turn a local reference into a public URL (because the upstream API
// fetches references server-side and rejects Base64) uploads through it. The
// brick is backend-agnostic: it speaks raw SigV4 against any S3-compatible
// endpoint (BytePlus TOS, MinIO, Cloudflare R2, Wasabi, AWS S3) with zero AWS
// SDK dependencies.
//
// Configuration lives in the top-level `storage:` section of config.yaml (see
// config.StorageConfig). The RequireStorage gate (providers.Bundle) ensures a
// provider that needs the brick fails fast with a helpful message when it is
// unconfigured, before any HTTP call.
package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/transport"
)

// httpClient signs and PUTs reference bytes and runs the public-read check;
// the 180s ceiling covers a slow large-video upload. It's a plain net/http
// client behind transport's Client, as the transport package explicitly
// permits for signed/no-JSON requests.
var httpClient = transport.NewClient(180 * time.Second)

// nowFunc returns the signing time; overridable in tests. Production uses the
// real clock.
var nowFunc = func() time.Time { return time.Now().UTC() }

// Configured reports whether a usable storage: section exists. Cheap: it loads
// the config file but does not resolve secrets (no ${ENV}/op:// round-trip),
// so the RequireStorage gate can call it without triggering a 1Password
// prompt. "Usable" = endpoint, bucket, and both credentials present.
func Configured() bool {
	cfg, err := config.Load()
	if err != nil || cfg == nil || cfg.Storage == nil {
		return false
	}
	s := cfg.Storage
	return s.Endpoint != "" && s.Bucket != "" && s.AccessKey != "" && s.SecretKey != ""
}

// Get loads and fully resolves the storage config (${ENV}/op://). ctx cancels
// in-flight op reads. Returns an error when storage is unconfigured so callers
// get an actionable message rather than a nil dereference.
func Get(ctx context.Context) (*config.StorageConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	sc, err := cfg.ResolveStorage(ctx)
	if err != nil {
		return nil, err
	}
	if sc == nil {
		return nil, fmt.Errorf("no storage configured — run `imagine storage set` first")
	}
	if err := validateConfig(sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func validateConfig(sc *config.StorageConfig) error {
	var missing []string
	if sc.Endpoint == "" {
		missing = append(missing, "endpoint")
	}
	if sc.Bucket == "" {
		missing = append(missing, "bucket")
	}
	if sc.AccessKey == "" {
		missing = append(missing, "access_key")
	}
	if sc.SecretKey == "" {
		missing = append(missing, "secret_key")
	}
	if len(missing) > 0 {
		return fmt.Errorf("storage config incomplete (missing: %s) — run `imagine storage set`", strings.Join(missing, ", "))
	}
	return nil
}

// uploads memoizes uploads by content hash so identical reference bytes upload
// once across concurrent Generate calls (-n>1) and reused provider instances
// (batch mode) — the reusable home for the dedup pattern providers would
// otherwise hand-roll.
var (
	uploadsMu sync.Mutex
	uploads   = map[string]*uploadResult{}
)

type uploadResult struct {
	url  string
	err  error
	done chan struct{}
}

// Upload resolves the storage config (one ${ENV}/op:// round-trip) and PUTs
// data. Convenience wrapper for callers that upload a single object; callers
// uploading several references should resolve once with Get and call
// UploadWith to avoid re-reading config and re-resolving secrets per object.
func Upload(ctx context.Context, data []byte, contentType string) (string, error) {
	sc, err := Get(ctx)
	if err != nil {
		return "", err
	}
	return UploadWith(ctx, sc, data, contentType)
}

// UploadWith PUTs data to the bucket described by an already-resolved config
// and returns its public URL. The object key is content-addressed:
// path_prefix + sha256(data) + ext, so an identical re-upload is an idempotent
// re-PUT and concurrent callers for the same bytes upload once (failures are
// evicted so a later call may retry). Taking the resolved config as a
// parameter lets a provider resolve credentials once per generation and reuse
// them across every reference — no repeated config reads or 1Password prompts.
func UploadWith(ctx context.Context, sc *config.StorageConfig, data []byte, contentType string) (string, error) {
	key := objectKey(sc, data, contentType)

	// The dedup key includes the destination so a config change (different
	// bucket/endpoint/prefix) never returns a URL for the wrong target.
	dedupKey := sc.Endpoint + "\x00" + sc.Bucket + "\x00" + key

	uploadsMu.Lock()
	if r, ok := uploads[dedupKey]; ok {
		uploadsMu.Unlock()
		<-r.done
		return r.url, r.err
	}
	r := &uploadResult{done: make(chan struct{})}
	uploads[dedupKey] = r
	uploadsMu.Unlock()

	r.url, r.err = putObject(ctx, sc, key, data, contentType)
	if r.err != nil {
		uploadsMu.Lock()
		delete(uploads, dedupKey)
		uploadsMu.Unlock()
	}
	close(r.done)
	return r.url, r.err
}

// objectKey builds the content-addressed key: {path_prefix}{sha256}{ext}.
func objectKey(sc *config.StorageConfig, data []byte, contentType string) string {
	sum := sha256.Sum256(data)
	return sc.PathPrefix + hex.EncodeToString(sum[:]) + images.ExtForMime(contentType)
}

// putObject signs and PUTs one object, returning its public URL.
func putObject(ctx context.Context, sc *config.StorageConfig, key string, data []byte, contentType string) (string, error) {
	putURL := objectURL(sc, key, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("storage: build PUT: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if err := signRequest(req, sc.AccessKey, sc.SecretKey, sc.Region, hashPayload(data), nowFunc()); err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("storage: PUT %s: %w", key, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return "", fmt.Errorf("storage: PUT %s failed (status %d): %s", key, resp.StatusCode, bytes.TrimSpace(snippet))
	}
	return objectURL(sc, key, true), nil
}

// objectURL builds the URL for an object key. When public is true and a
// public_url_base override is set, that base is used (CDN/custom domain).
// Otherwise addressing is virtual-host style (https://{bucket}.{host}/{key}) —
// BytePlus TOS supports only virtual-host, not path-style. path_style:true
// selects path style for backends that require it (e.g. MinIO).
func objectURL(sc *config.StorageConfig, key string, public bool) string {
	if public && sc.PublicURLBase != "" {
		return strings.TrimRight(sc.PublicURLBase, "/") + "/" + key
	}
	if sc.PathStyle {
		return strings.TrimRight(sc.Endpoint, "/") + "/" + sc.Bucket + "/" + key
	}
	return virtualHostBase(sc.Endpoint, sc.Bucket) + "/" + key
}

// virtualHostBase rewrites an endpoint into its virtual-host form by prefixing
// the bucket as a subdomain: "https://tos-…bytepluses.com" + "b" →
// "https://b.tos-…bytepluses.com". The scheme (if any) is preserved.
func virtualHostBase(endpoint, bucket string) string {
	e := strings.TrimRight(endpoint, "/")
	if i := strings.Index(e, "://"); i >= 0 {
		return e[:i+3] + bucket + "." + e[i+3:]
	}
	return bucket + "." + e
}

// Delete removes one object by key (best-effort cleanup). Errors are returned
// but callers typically ignore them.
func Delete(ctx context.Context, sc *config.StorageConfig, key string) error {
	delURL := objectURL(sc, key, false)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
	if err != nil {
		return err
	}
	if err := signRequest(req, sc.AccessKey, sc.SecretKey, sc.Region, emptyPayloadHash, nowFunc()); err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// Test round-trips a marker object to prove the configured bucket is writable
// and the produced URL is publicly readable, then deletes the marker. This is
// exactly the check that prevents the opaque server-side "access denied" a
// provider hits when the upstream API fetches an unreadable reference URL.
func Test(ctx context.Context, sc *config.StorageConfig) error {
	if err := validateConfig(sc); err != nil {
		return err
	}
	marker := []byte("imagine storage test " + uuid.NewString())
	key := sc.PathPrefix + "imagine-test-" + uuid.NewString() + ".txt"

	publicURL, err := putObject(ctx, sc, key, marker, "text/plain")
	if err != nil {
		return err
	}
	// Best-effort cleanup regardless of the read result.
	defer func() { _ = Delete(ctx, sc, key) }()

	got, err := transport.GetBytes(ctx, httpClient, publicURL, transport.NoAuth())
	if err != nil {
		return fmt.Errorf("anonymous read of %s failed — the bucket must be public-read; use a dedicated public bucket for imagine: %w", publicURL, err)
	}
	if !bytes.Equal(got, marker) {
		return fmt.Errorf("public read of %s returned unexpected content", publicURL)
	}
	return nil
}
