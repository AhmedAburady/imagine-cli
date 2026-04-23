package transport_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/transport"
)

// --- Fixtures ---------------------------------------------------------------

type echoReq struct {
	Foo string `json:"foo"`
}

type echoResp struct {
	Got string `json:"got"`
}

func newServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// --- Auth -------------------------------------------------------------------

func TestBearer_SetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, `{"got":"ok"}`)
	})
	defer srv.Close()

	_, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.Bearer("sk-test"), echoReq{Foo: "x"})
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization header: got %q, want %q", gotAuth, "Bearer sk-test")
	}
}

func TestQueryKey_SetsURLParam(t *testing.T) {
	var gotKey string
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		_, _ = io.WriteString(w, `{"got":"ok"}`)
	})
	defer srv.Close()

	_, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.QueryKey("key", "AIza-xxx"), echoReq{Foo: "x"})
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if gotKey != "AIza-xxx" {
		t.Errorf("query key: got %q, want %q", gotKey, "AIza-xxx")
	}
}

func TestNoAuth_AppliesNothing(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("NoAuth should not set Authorization")
		}
		if r.URL.Query().Get("key") != "" {
			t.Error("NoAuth should not set query key")
		}
		_, _ = io.WriteString(w, `{"got":"ok"}`)
	})
	defer srv.Close()

	_, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.NoAuth(), echoReq{Foo: "x"})
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
}

// --- PostJSON ---------------------------------------------------------------

func TestPostJSON_RoundTrip(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type: got %q, want application/json", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"foo":"hello"`) {
			t.Errorf("body: got %s", body)
		}
		_, _ = io.WriteString(w, `{"got":"hello"}`)
	})
	defer srv.Close()

	resp, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.NoAuth(), echoReq{Foo: "hello"})
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if resp.Got != "hello" {
		t.Errorf("response: got %q, want hello", resp.Got)
	}
}

func TestPostJSON_ErrorWithMessage(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"bad prompt"}}`)
	})
	defer srv.Close()

	_, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.NoAuth(), echoReq{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 400 {
		t.Errorf("StatusCode: got %d, want 400", apiErr.StatusCode)
	}
	if apiErr.Message != "bad prompt" {
		t.Errorf("Message: got %q, want 'bad prompt'", apiErr.Message)
	}
}

func TestPostJSON_ErrorWithoutParseableBody(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `gibberish not-json`)
	})
	defer srv.Close()

	_, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.NoAuth(), echoReq{})
	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode: got %d, want 500", apiErr.StatusCode)
	}
	if apiErr.Message != "" {
		t.Errorf("Message should be empty when body isn't parseable: got %q", apiErr.Message)
	}
	// Error() falls back to status when Message is empty.
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Error() should mention status code, got %q", err.Error())
	}
}

func TestPostJSON_ErrorMessageTruncated(t *testing.T) {
	// Build a message longer than the 200-char ceiling to confirm truncation.
	long := strings.Repeat("x", 300)
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"`+long+`"}}`)
	})
	defer srv.Close()

	_, err := transport.PostJSON[echoResp](context.Background(), transport.NewClient(5*time.Second), srv.URL, transport.NoAuth(), echoReq{})
	var apiErr *transport.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if len(apiErr.Message) > 200 {
		t.Errorf("Message length: got %d, want <= 200", len(apiErr.Message))
	}
	if !strings.HasSuffix(apiErr.Message, "...") {
		t.Errorf("truncated message should end in '...', got %q", apiErr.Message)
	}
}

func TestPostJSON_RespectsContextCancellation(t *testing.T) {
	// Server that hangs; cancel the context quickly.
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := transport.PostJSON[echoResp](ctx, transport.NewClient(5*time.Second), srv.URL, transport.NoAuth(), echoReq{})
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in chain, got %v", err)
	}
}

// --- PostMultipart ----------------------------------------------------------

func TestPostMultipart_RoundTrip(t *testing.T) {
	srv := newServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			t.Errorf("Content-Type: got %q, want multipart/form-data prefix", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello-body") {
			t.Errorf("body didn't include payload: %s", body)
		}
		_, _ = io.WriteString(w, `{"got":"mp"}`)
	})
	defer srv.Close()

	resp, err := transport.PostMultipart[echoResp](
		context.Background(),
		transport.NewClient(5*time.Second),
		srv.URL,
		transport.NoAuth(),
		strings.NewReader("hello-body"),
		`multipart/form-data; boundary=xyz`,
	)
	if err != nil {
		t.Fatalf("PostMultipart: %v", err)
	}
	if resp.Got != "mp" {
		t.Errorf("response: got %q, want mp", resp.Got)
	}
}

// --- DecodeB64 --------------------------------------------------------------

func TestDecodeB64_RoundTrip(t *testing.T) {
	// "hello" base64-encoded is "aGVsbG8="
	data, err := transport.DecodeB64("aGVsbG8=")
	if err != nil {
		t.Fatalf("DecodeB64: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("decoded: got %q, want hello", data)
	}
}

func TestDecodeB64_Invalid(t *testing.T) {
	_, err := transport.DecodeB64("not valid base64!!!!")
	if err == nil {
		t.Error("expected error on invalid base64, got nil")
	}
}

// --- APIError.Error() -------------------------------------------------------

func TestAPIError_ErrorWithMessage(t *testing.T) {
	e := &transport.APIError{StatusCode: 400, Message: "bad input"}
	if e.Error() != "bad input" {
		t.Errorf("Error(): got %q, want 'bad input'", e.Error())
	}
}

func TestAPIError_ErrorWithoutMessage(t *testing.T) {
	e := &transport.APIError{StatusCode: 500}
	if !strings.Contains(e.Error(), "500") {
		t.Errorf("Error() should mention status, got %q", e.Error())
	}
}
