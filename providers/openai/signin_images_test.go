package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// stubSignInImages serves the Codex image endpoints, answering with status and
// body, and returns an accessor for the last request (body re-readable).
func stubSignInImages(t *testing.T, status int, body string) func() *http.Request {
	t.Helper()
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		got = r.Clone(context.Background())
		got.Body = io.NopCloser(bytes.NewReader(raw))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := codexBaseURL
	codexBaseURL = srv.URL
	t.Cleanup(func() { codexBaseURL = prev })
	return func() *http.Request { return got }
}

// signedInProvider returns a subscription Provider holding a token that does
// not need a refresh.
func signedInProvider(t *testing.T) *Provider {
	t.Helper()
	sa := &storedAuth{}
	sa.Tokens.AccessToken = makeJWT(t, map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
	sa.Tokens.AccountID = "acct-1"
	return &Provider{method: methodSubscription, authFile: writeTempStore(t), cached: sa}
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

const signInPNG = `{"created":1,"output_format":"png","data":[{"b64_json":"` + onePixel + `"}]}`

func TestSignIn_GenerateUsesImagesEndpoint(t *testing.T) {
	last := stubSignInImages(t, http.StatusOK, signInPNG)
	opts := &Options{Model: "gpt-image-2.5-flare", Size: "1536x1024", Quality: "high", OutputFormat: "jpeg", Compression: 80, Moderation: "low", Background: "opaque"}
	out, err := signedInProvider(t).Generate(context.Background(), providers.Request{Prompt: "a mug", N: 1, Options: opts})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(out.Images) != 1 || out.Images[0].MimeType != "image/png" {
		t.Fatalf("images = %+v, want one PNG (the server's output_format, not -f)", out.Images)
	}
	r := last()
	if r.URL.Path != generationsPath {
		t.Errorf("path = %q, want %s", r.URL.Path, generationsPath)
	}
	if got := r.Header.Get("ChatGPT-Account-Id"); got != "acct-1" {
		t.Errorf("account header = %q", got)
	}
	if got := r.Header.Get("Accept"); got == "text/event-stream" {
		t.Error("image requests are plain JSON, not SSE")
	}
	body := decodeBody(t, r)
	want := map[string]any{"prompt": "a mug", "model": "gpt-image-2.5-flare", "quality": "high", "size": "1536x1024", "background": "opaque"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%s] = %v, want %v", k, body[k], v)
		}
	}
	for _, k := range []string{"tools", "complexity", "images", "n", "output_format", "output_compression", "moderation"} {
		if _, present := body[k]; present {
			t.Errorf("body must not carry %q: %v", k, body)
		}
	}
}

func TestSignIn_EditSendsReferencesAsDataURLs(t *testing.T) {
	last := stubSignInImages(t, http.StatusOK, signInPNG)
	opts := &Options{Model: "gpt-image-2.5-flare", Size: "auto", Quality: "auto", OutputFormat: "png"}
	refs := []images.Reference{{MimeType: "image/png", Data: []byte("foo")}, {MimeType: "image/jpeg", Data: []byte("bar")}}
	if _, err := signedInProvider(t).Generate(context.Background(), providers.Request{Prompt: "add a hat", N: 1, Options: opts, References: refs}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	r := last()
	if r.URL.Path != editsPath {
		t.Errorf("path = %q, want %s", r.URL.Path, editsPath)
	}
	body := decodeBody(t, r)
	imgs, _ := body["images"].([]any)
	if len(imgs) != 2 {
		t.Fatalf("images = %v, want 2", body["images"])
	}
	first, _ := imgs[0].(map[string]any)
	second, _ := imgs[1].(map[string]any)
	if first["image_url"] != "data:image/png;base64,Zm9v" || second["image_url"] != "data:image/jpeg;base64,YmFy" {
		t.Errorf("image refs = %v", imgs)
	}
	if body["quality"] != "auto" || body["size"] != "auto" {
		t.Errorf("quality/size = %v/%v, want auto/auto", body["quality"], body["size"])
	}
}

func TestSignIn_RejectsQualityTheEndpointDowngrades(t *testing.T) {
	for _, q := range []string{"xhigh", "max"} {
		opts := &Options{Model: "gpt-image-2.5-sunburst", Size: "auto", Quality: q, OutputFormat: "png"}
		_, err := (&Provider{method: methodSubscription}).Generate(context.Background(), providers.Request{Prompt: "x", N: 1, Options: opts})
		if err == nil || !strings.Contains(err.Error(), "needs an API key") {
			t.Errorf("quality %s: err = %v, want an API-key error", q, err)
		}
	}
}

func TestSignIn_ErrorsAreLabelled(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		want   string
	}{
		"openai envelope": {400, `{"error":{"message":"Invalid value: 'x'.","code":"invalid_value"}}`, "openai subscription error (status 400): Invalid value: 'x'."},
		"codex detail":    {403, `{"detail":"Not allowed on this plan."}`, "openai subscription error (status 403): Not allowed on this plan."},
		"cloudflare":      {403, `<html>Attention Required! | Cloudflare</html>`, "blocked by Cloudflare"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stubSignInImages(t, c.status, c.body)
			opts := &Options{Model: "gpt-image-2.5-flare", Size: "auto", Quality: "auto", OutputFormat: "png"}
			_, err := signedInProvider(t).Generate(context.Background(), providers.Request{Prompt: "x", N: 1, Options: opts})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want it to contain %q", err, c.want)
			}
		})
	}
}
