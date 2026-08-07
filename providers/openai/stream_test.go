package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

func flagspecParse(t *testing.T, info providers.Info, values map[string]any) (any, error) {
	t.Helper()
	return flagspec.Parse(Options{}, values, info)
}

func sse(lines ...string) *strings.Reader {
	return strings.NewReader(strings.Join(lines, "\n") + "\n")
}

func TestStreamImages_PreviewsThenImage(t *testing.T) {
	var seen []providers.ProgressEvent
	out, err := streamImages(sse(
		`data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"Zm9v"}`,
		`data: {"type":"image_generation.partial_image","partial_image_index":1,"b64_json":"Zm9v"}`,
		`data: {"type":"image_generation.completed","b64_json":"YmFy"}`,
		`data: [DONE]`,
	), 1, 2, "image/png", func(ev providers.ProgressEvent) { seen = append(seen, ev) })
	if err != nil {
		t.Fatalf("streamImages: %v", err)
	}
	if len(out.Images) != 1 || string(out.Images[0].Data) != "bar" {
		t.Errorf("want the completed image, got %+v", out.Images)
	}
	// partial_image_index is 0-based on the wire, 1-based to the user.
	want := []providers.ProgressEvent{{PartialIndex: 1, PartialTotal: 2}, {PartialIndex: 2, PartialTotal: 2}}
	if len(seen) != len(want) {
		t.Fatalf("got %d previews, want %d: %+v", len(seen), len(want), seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("preview %d = %+v, want %+v", i, seen[i], want[i])
		}
	}
}

func TestStreamImages_CollectsEveryImageInABatch(t *testing.T) {
	out, err := streamImages(sse(
		`data: {"type":"image_generation.completed","b64_json":"Zm9v"}`,
		`data: {"type":"image_generation.completed","b64_json":"YmFy"}`,
		`data: [DONE]`,
	), 2, 0, "image/png", nil)
	if err != nil {
		t.Fatalf("streamImages: %v", err)
	}
	if len(out.Images) != 2 {
		t.Fatalf("got %d images, want 2", len(out.Images))
	}
}

func TestStreamImages_NilProgressIsSafe(t *testing.T) {
	if _, err := streamImages(sse(
		`data: {"type":"image_generation.partial_image","partial_image_index":0,"b64_json":"Zm9v"}`,
		`data: {"type":"image_generation.completed","b64_json":"YmFy"}`,
	), 1, 1, "image/png", nil); err != nil {
		t.Fatalf("streamImages with nil onProgress: %v", err)
	}
}

func TestStreamImages_Errors(t *testing.T) {
	if _, err := streamImages(sse(`data: {"type":"error","error":{"message":"blocked"}}`),
		1, 0, "image/png", nil); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected the streamed error surfaced, got %v", err)
	}
	if _, err := streamImages(sse(`data: {"type":"image_generation.partial_image","partial_image_index":0}`),
		1, 1, "image/png", nil); err == nil {
		t.Error("expected an error when the stream ends with no completed image")
	}
}

// Previews cost ~100 output tokens each, so a run with no listener (piped
// output, batch mode) must not request them.
func TestEffectivePartials_ZeroWithoutListener(t *testing.T) {
	if got := effectivePartials(3, nil); got != 0 {
		t.Errorf("no listener should suppress previews, got %d", got)
	}
	noop := func(providers.ProgressEvent) {}
	if got := effectivePartials(3, noop); got != 3 {
		t.Errorf("with a listener the count should pass through, got %d", got)
	}
	if got := effectivePartials(0, noop); got != 0 {
		t.Errorf("flag off should stay off, got %d", got)
	}
}

// The default path must stay on the non-streaming call.
func TestGenerationsBody_OmitsStreamWhenOff(t *testing.T) {
	raw, err := json.Marshal(generationsBody{Model: "gpt-image-2", Prompt: "x"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, unwanted := range []string{"stream", "partial_images"} {
		if strings.Contains(string(raw), unwanted) {
			t.Errorf("body should omit %q when previews are off: %s", unwanted, raw)
		}
	}
}

func TestPartialImagesFlagRange(t *testing.T) {
	info := (&Provider{}).Info()
	for _, n := range []any{0, 1, 3} {
		if _, err := flagspecParse(t, info, map[string]any{"partial-images": n}); err != nil {
			t.Errorf("partial-images=%v should be accepted: %v", n, err)
		}
	}
	for _, n := range []any{-1, 4} {
		if _, err := flagspecParse(t, info, map[string]any{"partial-images": n}); err == nil {
			t.Errorf("partial-images=%v should be rejected", n)
		}
	}
}
