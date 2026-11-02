package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const onePixel = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

// stubImagesAPI serves the /v1/images endpoints from an httptest server and
// returns an accessor for the last request it received, body re-readable.
func stubImagesAPI(t *testing.T) func() *http.Request {
	t.Helper()
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = r.Clone(context.Background())
		got.Body = io.NopCloser(bytes.NewReader(body))
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + onePixel + `"}]}`))
	}))
	t.Cleanup(srv.Close)
	prev := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = prev })
	return func() *http.Request { return got }
}

func TestGenerate_WireBodyCarriesModelAndQuality(t *testing.T) {
	last := stubImagesAPI(t)
	p := &Provider{method: methodAPIKey, apiKey: "sk-test"}
	opts := &Options{Model: "gpt-image-2.5-sunburst", Size: "auto", Quality: "max", OutputFormat: "png", Compression: 100}
	if _, err := p.Generate(context.Background(), providers.Request{Prompt: "a mug", N: 1, Options: opts}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	r := last()
	if r.URL.Path != generationsPath {
		t.Errorf("path = %q, want %s", r.URL.Path, generationsPath)
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["model"] != "gpt-image-2.5-sunburst" || body["quality"] != "max" {
		t.Errorf("body model/quality = %v/%v", body["model"], body["quality"])
	}
	if _, present := body["input_fidelity"]; present {
		t.Error("input_fidelity must not be sent")
	}
}

func TestEdit_MultipartCarriesModelAndQuality(t *testing.T) {
	last := stubImagesAPI(t)
	p := &Provider{method: methodAPIKey, apiKey: "sk-test"}
	opts := &Options{Model: "gpt-image-2.5-flare", Size: "1024x1024", Quality: "xhigh", OutputFormat: "png", Compression: 100}
	req := providers.Request{
		Prompt:     "make it winter",
		N:          1,
		Options:    opts,
		References: []images.Reference{{MimeType: "image/png", Data: []byte("png-bytes")}},
	}
	if _, err := p.Generate(context.Background(), req); err != nil {
		t.Fatalf("Generate (edit): %v", err)
	}
	r := last()
	if r.URL.Path != editsPath {
		t.Errorf("path = %q, want %s", r.URL.Path, editsPath)
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	if got := r.FormValue("model"); got != "gpt-image-2.5-flare" {
		t.Errorf("model = %q", got)
	}
	if got := r.FormValue("quality"); got != "xhigh" {
		t.Errorf("quality = %q", got)
	}
	if files := r.MultipartForm.File["image[]"]; len(files) != 1 {
		t.Errorf("image[] parts = %d, want 1", len(files))
	}
}
