package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// imageStreamEvent is the subset of /v1/images SSE events we act on:
// image_generation.partial_image and image_generation.completed.
type imageStreamEvent struct {
	Type         string `json:"type"`
	B64JSON      string `json:"b64_json"`
	PartialIndex int    `json:"partial_image_index"`
	Error        *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// postStream POSTs to a /v1/images endpoint with stream=true and hands back the
// live body; the caller owns Close. Non-2xx is drained into a normal APIError.
func (p *Provider) postStream(ctx context.Context, url, contentType string, body io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "text/event-stream")
	if err := transport.Bearer(p.apiKey).Apply(req); err != nil {
		return nil, err
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	rc := newStallReader(resp.Body, stallTimeout)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(rc, 8<<10))
		_ = rc.Close()
		return nil, describeAPIError(transport.NewAPIError(resp.StatusCode, raw))
	}
	return rc, nil
}

// streamOne posts a streaming request and drains it into a Response.
func (p *Provider) streamOne(ctx context.Context, url, contentType string, body io.Reader,
	want, partials int, outMime string, onProgress func(providers.ProgressEvent)) (*providers.Response, error) {
	rc, err := p.postStream(ctx, url, contentType, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return streamImages(rc, want, partials, outMime, onProgress)
}

// streamImages consumes the SSE stream, forwarding each preview to onProgress
// and collecting every completed image. want is how many images were requested.
func streamImages(r io.Reader, want, partials int, outMime string, onProgress func(providers.ProgressEvent)) (*providers.Response, error) {
	var imgs []providers.GeneratedImage
	err := scanSSE(r, func(ev *imageStreamEvent) (bool, error) {
		if ev.Error != nil && ev.Error.Message != "" {
			return false, errors.New(ev.Error.Message)
		}
		switch ev.Type {
		case "image_generation.partial_image":
			if onProgress != nil {
				onProgress(providers.ProgressEvent{PartialIndex: ev.PartialIndex + 1, PartialTotal: partials})
			}
		case "image_generation.completed":
			if ev.B64JSON == "" {
				return false, nil
			}
			data, decErr := transport.DecodeB64(ev.B64JSON)
			if decErr != nil {
				return false, decErr
			}
			imgs = append(imgs, providers.GeneratedImage{Data: data, MimeType: outMime})
			return len(imgs) >= want, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if len(imgs) == 0 {
		return nil, errors.New("openai returned no images")
	}
	return &providers.Response{Images: imgs}, nil
}
