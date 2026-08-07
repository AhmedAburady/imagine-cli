package openai

// Subscription route: image generation through the Codex Responses endpoint
// (chatgpt.com/backend-api/codex/responses) driving the built-in
// image_generation tool, billed to the user's ChatGPT plan. The dedicated
// /images endpoints on that host are Cloudflare-gated for non-browser TLS
// clients (403 challenge); /responses, the official hot path, is reachable
// with the OAuth token — both verified empirically.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const (
	codexBaseURL  = "https://chatgpt.com/backend-api/codex"
	responsesPath = "/responses"

	// driverModel hosts the image_generation tool call; the image model
	// itself travels inside the tool object.
	driverModel = "gpt-5.5"

	// imageInstructions is the minimal system prompt the endpoint needs to
	// reliably invoke the tool (it misbehaves with none).
	imageInstructions = "You are an image generation assistant. Generate exactly one image for the user's request using the image_generation tool."
)

// stallTimeout aborts the SSE read only after this much continuous silence; it resets on every byte received.
const stallTimeout = 180 * time.Second

var errStalled = errors.New("stream stalled: no data for 180s (network hang or backend stall) — retry")

func (p *Provider) generateSubscription(ctx context.Context, opts *Options, req providers.Request) (*providers.Response, error) {
	access, account, err := p.ensureFreshToken(ctx)
	if err != nil {
		return nil, err
	}
	auth := &codexAuth{accessToken: access, accountID: account, sessionID: uuid.NewString()}

	partials := effectivePartials(opts.PartialImages, req.OnProgress)
	body := responsesBody{
		Model:        driverModel,
		Instructions: imageInstructions,
		Input:        []inputMessage{{Role: "user", Content: responsesContent(req.Prompt, req.References)}},
		Tools:        []imageTool{buildTool(opts, partials)},
		ToolChoice:   &toolChoice{Type: "image_generation"},
		Stream:       true,
	}
	return p.doResponses(ctx, auth, body, opts.OutputFormat, partials, req.OnProgress)
}

func responsesContent(text string, refs []images.Reference) []contentItem {
	content := []contentItem{{Type: "input_text", Text: text}}
	for _, ref := range refs {
		content = append(content, contentItem{Type: "input_image", ImageURL: dataURL(ref.MimeType, ref.Data)})
	}
	return content
}

// buildTool maps Options onto the image_generation tool, omitting anything left
// on its auto/default sentinel.
func buildTool(opts *Options, partials int) imageTool {
	t := imageTool{
		Type:          "image_generation",
		Model:         opts.Model,
		Size:          apiSize(opts.Size),
		Quality:       apiQuality(opts.Quality),
		OutputFormat:  opts.OutputFormat,
		Moderation:    opts.Moderation,
		Background:    opts.Background,
		PartialImages: partials,
	}
	if (opts.OutputFormat == "jpeg" || opts.OutputFormat == "webp") && opts.Compression > 0 && opts.Compression < 100 {
		t.OutputCompression = new(opts.Compression)
	}
	return t
}

// --- Request wire types ------------------------------------------------------

type responsesBody struct {
	Model        string          `json:"model"`
	Instructions string          `json:"instructions"`
	Input        []inputMessage  `json:"input"`
	Tools        []imageTool     `json:"tools,omitempty"`
	ToolChoice   *toolChoice     `json:"tool_choice,omitempty"`
	Reasoning    *reasoningParam `json:"reasoning,omitempty"`
	Stream       bool            `json:"stream"`
	Store        bool            `json:"store"`
}

type reasoningParam struct {
	Effort string `json:"effort,omitempty"`
}

type inputMessage struct {
	Role    string        `json:"role"`
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type imageTool struct {
	Type              string `json:"type"`
	Model             string `json:"model,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	Background        string `json:"background,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
	PartialImages     int    `json:"partial_images,omitempty"`
}

type toolChoice struct {
	Type string `json:"type"`
}

// --- SSE response handling ---------------------------------------------------

// sseEvent is the subset of each streamed event we inspect: the terminal image
// item (base64 result), assistant text (describe), and error envelopes.
type sseEvent struct {
	Type              string `json:"type"`
	Text              string `json:"text"` // response.output_text.done
	PartialImageIndex int    `json:"partial_image_index"`
	Item              *struct {
		Type         string `json:"type"`
		Status       string `json:"status"`
		Result       string `json:"result"`
		OutputFormat string `json:"output_format"`
	} `json:"item"`
	Response *struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		IncompleteDetails *struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
	} `json:"response"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// postResponses POSTs a Responses request and returns the live response on
// success; the caller owns Body.Close(). A non-2xx is drained, closed, and
// turned into a readable error.
func (p *Provider) postResponses(ctx context.Context, auth *codexAuth, body responsesBody) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, codexBaseURL+responsesPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := auth.Apply(req); err != nil {
		return nil, err
	}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	resp.Body = newStallReader(resp.Body, stallTimeout)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("openai subscription error (status %d): %s", resp.StatusCode, summarizeError(resp.StatusCode, snippet))
	}
	return resp, nil
}

// doResponses scans the SSE stream for the rendered image. wantFormat is the
// MIME fallback when the server doesn't echo an output_format.
func (p *Provider) doResponses(ctx context.Context, auth *codexAuth, body responsesBody, wantFormat string,
	partials int, onProgress func(providers.ProgressEvent)) (*providers.Response, error) {
	resp, err := p.postResponses(ctx, auth, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return parseImageStream(resp.Body, wantFormat, partials, onProgress)
}

// stallReader wraps a streaming body, closing it (which unblocks the read) after its idle duration of silence.
// Read records activity timestamps rather than resetting the timer, so a Read that races the
// timer's expiry can't be wrongly reported as a stall — the timer func re-checks last activity.
type stallReader struct {
	rc       io.ReadCloser
	idle     time.Duration
	timer    *time.Timer
	lastRead atomic.Int64 // unix-nanos of the most recent Read that returned bytes
	stalled  atomic.Bool
}

func newStallReader(rc io.ReadCloser, idle time.Duration) *stallReader {
	sr := &stallReader{rc: rc, idle: idle}
	sr.lastRead.Store(time.Now().UnixNano())
	sr.timer = time.AfterFunc(idle, sr.onTimer)
	return sr
}

func (sr *stallReader) onTimer() {
	last := sr.lastRead.Load()
	if idle := time.Duration(time.Now().UnixNano() - last); idle < sr.idle {
		sr.timer.Reset(sr.idle - idle) // activity since scheduling; re-check after the remaining window
		return
	}
	// Commit the close only if no Read updated lastRead since we sampled it; a
	// byte racing into this window makes the CAS fail, so we reschedule rather
	// than terminate a stream that just came back to life.
	if !sr.lastRead.CompareAndSwap(last, last) {
		sr.timer.Reset(sr.idle)
		return
	}
	sr.stalled.Store(true)
	_ = sr.rc.Close()
}

func (sr *stallReader) Read(p []byte) (int, error) {
	n, err := sr.rc.Read(p)
	if n > 0 {
		sr.lastRead.Store(time.Now().UnixNano())
	}
	if err != nil && sr.stalled.Load() {
		return n, errStalled
	}
	return n, err
}

func (sr *stallReader) Close() error {
	sr.timer.Stop()
	return sr.rc.Close()
}

// scanSSE invokes fn for every decoded `data:` event. Data lines can be ~1MB of
// base64, so bufio.Reader.ReadString is used rather than Scanner's capped
// tokens. E is inferred from fn; the Responses and /v1/images event shapes differ.
func scanSSE[E any](r io.Reader, fn func(ev *E) (stop bool, err error)) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if data, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "data: "); ok && data != "[DONE]" {
			var ev E
			if json.Unmarshal([]byte(data), &ev) == nil {
				stop, ferr := fn(&ev)
				if ferr != nil {
					return ferr
				}
				if stop {
					return nil
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stream: %w", err)
		}
	}
}

func parseImageStream(r io.Reader, wantFormat string, partials int, onProgress func(providers.ProgressEvent)) (*providers.Response, error) {
	var out *providers.Response
	err := scanSSE(r, func(ev *sseEvent) (bool, error) {
		if msg := eventError(ev); msg != "" {
			return false, fmt.Errorf("openai subscription: %s", msg)
		}
		if ev.Type == "response.image_generation_call.partial_image" {
			if onProgress != nil {
				onProgress(providers.ProgressEvent{PartialIndex: ev.PartialImageIndex + 1, PartialTotal: partials})
			}
			return false, nil
		}
		if ev.Type == "response.output_item.done" && ev.Item != nil &&
			ev.Item.Type == "image_generation_call" && ev.Item.Result != "" {
			raw, derr := transport.DecodeB64(ev.Item.Result)
			if derr != nil {
				return false, derr
			}
			format := ev.Item.OutputFormat
			if format == "" {
				format = wantFormat
			}
			out = &providers.Response{Images: []providers.GeneratedImage{{Data: raw, MimeType: mimeTypeFor(format)}}}
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, errors.New("openai subscription returned no image")
	}
	return out, nil
}

func eventError(ev *sseEvent) string {
	if ev.Error != nil && ev.Error.Message != "" {
		return ev.Error.Message
	}
	if ev.Response != nil {
		if ev.Response.Error != nil && ev.Response.Error.Message != "" {
			return ev.Response.Error.Message
		}
		// A moderation/safety stop arrives as response.incomplete, not a
		// "failed" event — surface its reason instead of a generic "no image".
		if ev.Response.IncompleteDetails != nil && ev.Response.IncompleteDetails.Reason != "" {
			return "image generation stopped: " + ev.Response.IncompleteDetails.Reason
		}
	}
	return ""
}

// summarizeError pulls a message from a non-2xx body, with a Cloudflare-aware
// hint (a challenge here usually means the session expired).
func summarizeError(status int, raw []byte) string {
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(raw, &env) == nil {
		if env.Error.Message != "" {
			return env.Error.Message
		}
		if env.Detail != "" {
			return env.Detail
		}
	}
	if status == 403 && bytes.Contains(bytes.ToLower(raw), []byte("cloudflare")) {
		return "blocked by Cloudflare — your session may have expired; re-run `imagine providers add openai`"
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 200 {
		s = s[:197] + "..."
	}
	return s
}

func apiSize(size string) string {
	if size == "auto" {
		return ""
	}
	return size
}

func apiQuality(q string) string {
	if q == "auto" {
		return ""
	}
	return q
}

// codexAuth sets the headers the Codex Responses endpoint expects: bearer token,
// workspace account id, streaming/beta markers, a per-request session id, and
// the client identity we authenticated as. Implements transport.Auth.
type codexAuth struct {
	accessToken string
	accountID   string
	sessionID   string
}

func (a *codexAuth) Apply(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.accessToken)
	if a.accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", a.accountID)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
	req.Header.Set("originator", originator)
	req.Header.Set("User-Agent", userAgent)
	if a.sessionID != "" {
		req.Header.Set("session_id", a.sessionID)
	}
	return nil
}
