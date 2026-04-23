// Package transport provides shared HTTP primitives for image-generation
// providers. It handles the standard-issue plumbing — connection pooling,
// auth injection, request construction, status-code checks, JSON-error
// extraction, base64 decoding — so each provider's code stays focused on
// what's genuinely provider-specific (wire types, endpoint paths, response
// shape).
//
// Opt-in: a provider that needs exotic behaviour can drop to raw
// net/http at any time; nothing in this package is a requirement. Vertex,
// for example, uses the genai SDK and never touches transport.
package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with provider-friendly connection pooling
// defaults. Embed *http.Client so callers still have the full surface.
type Client struct {
	*http.Client
}

// NewClient returns a Client with sensible pool settings. Cancellation is
// handled via context on each request; timeout is a per-request ceiling.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// --- Auth injectors --------------------------------------------------------

// Auth applies credentials to an outgoing request. Implementations below
// cover the two patterns the shipped providers use (Bearer token header,
// query-param API key). Providers with exotic auth can implement this
// interface themselves.
type Auth interface {
	Apply(*http.Request) error
}

// Bearer sets "Authorization: Bearer <token>". Used by OpenAI.
func Bearer(token string) Auth { return &bearerAuth{token: token} }

type bearerAuth struct{ token string }

func (a *bearerAuth) Apply(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

// QueryKey appends ?<param>=<value> to the URL. Used by Gemini's REST API.
func QueryKey(param, value string) Auth { return &queryKeyAuth{param: param, value: value} }

type queryKeyAuth struct{ param, value string }

func (a *queryKeyAuth) Apply(req *http.Request) error {
	q := req.URL.Query()
	q.Set(a.param, a.value)
	req.URL.RawQuery = q.Encode()
	return nil
}

// NoAuth applies no credentials. Useful for local endpoints or providers
// that handle auth through other means (e.g. ambient credentials).
func NoAuth() Auth { return &noAuth{} }

type noAuth struct{}

func (n *noAuth) Apply(*http.Request) error { return nil }

// --- Request helpers -------------------------------------------------------

// PostJSON marshals body as JSON, POSTs to url with auth applied, and
// decodes the response into *Resp on 2xx. Non-2xx yields *APIError with
// StatusCode and (if parseable) the server's error.message.
func PostJSON[Resp any](ctx context.Context, c *Client, url string, auth Auth, body any) (*Resp, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return doAndDecode[Resp](c, req, auth)
}

// PostMultipart POSTs a caller-built multipart body. The caller produces
// both the body reader and the full Content-Type header (typically
// multipart.Writer.FormDataContentType()). Decodes 2xx into *Resp.
func PostMultipart[Resp any](ctx context.Context, c *Client, url string, auth Auth, body io.Reader, contentType string) (*Resp, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	return doAndDecode[Resp](c, req, auth)
}

func doAndDecode[Resp any](c *Client, req *http.Request, auth Auth) (*Resp, error) {
	if err := auth.Apply(req); err != nil {
		return nil, fmt.Errorf("failed to apply auth: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, raw)
	}

	var out Resp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &out, nil
}

// --- Errors ----------------------------------------------------------------

// APIError is a structured non-2xx response. Providers can errors.As into
// *APIError when they need to inspect StatusCode (e.g. to retry on 429).
type APIError struct {
	StatusCode int
	Message    string
}

// Error satisfies the error interface. When Message is empty (unparseable
// error body) the caller still gets the status code.
func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error (status %d)", e.StatusCode)
}

// errBodyMaxLen is the ceiling on the extracted error.message string.
// Unified across providers — Gemini used 100 and OpenAI 200 before;
// 200 loses nothing and gives Gemini users slightly fuller errors.
const errBodyMaxLen = 200

func parseAPIError(status int, raw []byte) *APIError {
	// Both Gemini and OpenAI use {"error": {"message": "..."}}. If a future
	// provider uses a different shape, it can still surface a useful error
	// via APIError.StatusCode (the message will be empty).
	var errResp struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &errResp); err == nil && errResp.Error.Message != "" {
		msg := errResp.Error.Message
		if len(msg) > errBodyMaxLen {
			msg = msg[:errBodyMaxLen-3] + "..."
		}
		return &APIError{StatusCode: status, Message: msg}
	}
	return &APIError{StatusCode: status}
}

// --- Image helpers ---------------------------------------------------------

// DecodeB64 decodes a base64-encoded image payload. Wraps the stdlib call
// with a provider-friendly error message.
func DecodeB64(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	return data, nil
}
