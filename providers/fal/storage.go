package fal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/transport"
)

// falClient serves CDN uploads + the queue API (submit/poll/fetch). The 180s
// ceiling covers slow ref uploads; per-poll cancellation rides on context.
var falClient = transport.NewClient(180 * time.Second)

// downloadClient fetches the finished video. A large 1080p mp4 over a slow link
// can outrun the 180s upload ceiling, so the download leg has no whole-request
// deadline (cancellation rides on context); the streaming client still bounds
// each pre-body phase so a stalled connection fails fast.
var downloadClient = transport.NewStreamingClient()

// initiateBody asks fal's CDN to allocate a slot and hand back a signed PUT URL.
type initiateBody struct {
	ContentType string `json:"content_type"`
	FileName    string `json:"file_name"`
}

// initiateResp carries the public file URL and the signed URL we PUT bytes to.
type initiateResp struct {
	FileURL   string `json:"file_url"`
	UploadURL string `json:"upload_url"`
}

// uploadToFalStorage uploads one local reference to fal's CDN (single-file
// path — all seedance refs are <90 MB) and returns its public file URL for use
// as image_url / image_urls[i] / video_urls[i] / audio_urls[i]. It runs the
// two-step protocol: initiate (auth'd) → PUT raw bytes to the signed URL.
func uploadToFalStorage(ctx context.Context, apiKey string, data []byte, mime, name string) (string, error) {
	resp, err := transport.PostJSON[initiateResp](
		ctx, falClient,
		"https://rest.fal.ai/storage/upload/initiate?storage_type=fal-cdn-v3",
		transport.Key(apiKey),
		initiateBody{ContentType: mime, FileName: name},
	)
	if err != nil {
		return "", fmt.Errorf("fal: initiate CDN upload for %q: %w", name, err)
	}

	// The signed URL carries its own credentials — no Authorization header.
	if err := putBytes(ctx, resp.UploadURL, mime, data); err != nil {
		return "", fmt.Errorf("fal: upload %q to CDN: %w", name, err)
	}
	return resp.FileURL, nil
}

// putBytes PUTs raw bytes to a signed URL with only a Content-Type header. This
// is the one fal flow transport's helpers don't cover (no auth, no JSON), so we
// drop to net/http as the transport package explicitly permits.
func putBytes(ctx context.Context, url, contentType string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := falClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("CDN PUT failed (status %d): %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	return nil
}
