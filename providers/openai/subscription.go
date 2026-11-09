package openai

// Subscription route, billed to the user's ChatGPT plan. Images go through
// the Codex image endpoints (chatgpt.com/backend-api/codex/images/generations
// and /images/edits), the route Codex itself uses. The Responses
// image_generation tool stopped working on this host in November 2026: the
// backend now forwards the driver's reasoning effort as an image "complexity"
// that every image model rejects. Describe still uses /responses.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// codexBaseURL is a var so tests can point the sign-in route at an httptest server.
var codexBaseURL = "https://chatgpt.com/backend-api/codex"

const (
	responsesPath = "/responses"

	// driverModel is the default describe model on sign-in. Codex retired gpt-5.5 for ChatGPT
	// sign-in on 2026-10-14 and named a replacement per plan: gpt-6.1-sol on
	// Plus, Pro, Business, Enterprise and Edu, gpt-6-luna on Free and Go.
	driverModel     = "gpt-6.1-sol"
	driverModelLite = "gpt-6-luna"
)

// retiredOnSignIn are models Codex no longer serves to ChatGPT sign-in
// (still available with an API key). A config that saved one of them as
// vision_model falls back to the plan's model on this route.
var retiredOnSignIn = []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex-spark"}

// stallTimeout aborts the SSE read only after this much continuous silence; it resets on every byte received.
const stallTimeout = 180 * time.Second

var errStalled = errors.New("stream stalled: no data for 180s (network hang or backend stall) — retry")

func (p *Provider) generateSubscription(ctx context.Context, opts *Options, req providers.Request) (*providers.Response, error) {
	if q := emptyToAuto(strings.ToLower(opts.Quality)); !slices.Contains(signInQualities, q) {
		return nil, fmt.Errorf("--quality %s needs an API key; ChatGPT sign-in supports %s", opts.Quality, strings.Join(signInQualities, ", "))
	}
	access, account, err := p.ensureFreshToken(ctx)
	if err != nil {
		return nil, err
	}
	path, body := signInImageRequest(opts, req)
	resp, err := transport.PostJSON[generationsResponse](ctx, imagesClient, codexBaseURL+path, &codexAuth{accessToken: access, accountID: account}, body)
	if err != nil {
		return nil, subscriptionAPIError(err)
	}
	return decodeImages(resp, mimeTypeFor(resp.OutputFormat))
}

// signInQualities are the quality levels the sign-in image endpoints accept;
// anything else (xhigh, max) is silently downgraded server-side, so it is
// rejected here instead.
var signInQualities = []string{"low", "medium", "high", "auto"}

// signInImageBody is the Codex images request, the same shape Codex itself
// sends. The backend currently picks the model and the output size itself
// (steered by the prompt) and always returns PNG; model and size are still
// sent so the request states what the user asked for.
type signInImageBody struct {
	Images     []signInImageRef `json:"images,omitempty"` // edits only
	Prompt     string           `json:"prompt"`
	Model      string           `json:"model"`
	Background string           `json:"background,omitempty"`
	Quality    string           `json:"quality,omitempty"`
	Size       string           `json:"size,omitempty"`
}

type signInImageRef struct {
	ImageURL string `json:"image_url"`
}

// signInImageRequest picks /images/edits when references are attached, else
// /images/generations, and builds the matching body.
func signInImageRequest(opts *Options, req providers.Request) (path string, body signInImageBody) {
	body = signInImageBody{
		Prompt:     req.Prompt,
		Model:      opts.Model,
		Background: opts.Background,
		Quality:    emptyToAuto(strings.ToLower(opts.Quality)),
		Size:       emptyToAuto(opts.Size),
	}
	if len(req.References) == 0 {
		return generationsPath, body
	}
	for _, ref := range req.References {
		body.Images = append(body.Images, signInImageRef{ImageURL: dataURL(ref.MimeType, ref.Data)})
	}
	return editsPath, body
}

// subscriptionAPIError labels a non-2xx from the images endpoints, falling
// back to summarizeError for bodies without an OpenAI error envelope
// (Codex's {"detail": ...} and Cloudflare pages).
func subscriptionAPIError(err error) error {
	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("openai subscription: %w", err)
	}
	if apiErr.Message == "" {
		return fmt.Errorf("openai subscription error (status %d): %s", apiErr.StatusCode, summarizeError(apiErr.StatusCode, apiErr.Raw))
	}
	return fmt.Errorf("openai subscription error (status %d): %w", apiErr.StatusCode, describeAPIError(err))
}

// driverFor picks the sign-in driver model for the account's plan. The plan
// is read from the id_token first (where Codex reads it), then the access
// token; an unreadable plan gets the paid-plan model.
func driverFor(idToken, accessToken string) string {
	plan := planType(idToken)
	if plan == "" {
		plan = planType(accessToken)
	}
	switch plan {
	case "free", "go":
		return driverModelLite
	}
	return driverModel
}

// planType reads chatgpt_plan_type from a token's auth claim ("" if absent).
func planType(token string) string {
	claims, err := jwtClaims(token)
	if err != nil {
		return ""
	}
	auth, _ := claims[authClaim].(map[string]any)
	plan, _ := auth["chatgpt_plan_type"].(string)
	return strings.ToLower(strings.TrimSpace(plan))
}

func responsesContent(text string, refs []images.Reference) []contentItem {
	content := []contentItem{{Type: "input_text", Text: text}}
	for _, ref := range refs {
		content = append(content, contentItem{Type: "input_image", ImageURL: dataURL(ref.MimeType, ref.Data)})
	}
	return content
}

// --- Request wire types ------------------------------------------------------

type responsesBody struct {
	Model        string          `json:"model"`
	Instructions string          `json:"instructions"`
	Input        []inputMessage  `json:"input"`
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

// --- SSE response handling ---------------------------------------------------

// sseEvent is the subset of each streamed event describe inspects: assistant
// text and error envelopes.
type sseEvent struct {
	Type     string `json:"type"`
	Text     string `json:"text"` // response.output_text.done
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
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("OpenAI-Beta", "responses=experimental")
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
// tokens. fn returns (stop, err); shared by the image and text parsers.
func scanSSE(r io.Reader, fn func(ev *sseEvent) (stop bool, err error)) error {
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if data, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "data: "); ok && data != "[DONE]" {
			var ev sseEvent
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

// codexAuth sets the headers every Codex endpoint expects: bearer token,
// workspace account id, the client identity we authenticated as, and (for
// describe's Responses calls) a per-request session id. Implements
// transport.Auth.
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
	req.Header.Set("originator", originator)
	req.Header.Set("User-Agent", userAgent)
	if a.sessionID != "" {
		req.Header.Set("session_id", a.sessionID)
	}
	return nil
}
