package gemini_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// readFlags drives the real Bundle wiring so these tests hit the same path a
// CLI invocation does, including the Validate hook flagspec calls at the end.
func readFlags(t *testing.T, args ...string) (any, error) {
	t.Helper()
	b, ok := providers.Get("gemini")
	if !ok {
		t.Fatal("gemini Bundle not registered")
	}
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	b.BindFlags(cmd)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return b.ReadFlags(cmd)
}

func TestFlashLite_AliasesResolveToCanonicalID(t *testing.T) {
	for _, alias := range []string{"flash-lite", "lite", gemini.ModelFlashLite} {
		raw, err := readFlags(t, "--model", alias)
		if err != nil {
			t.Fatalf("--model %s: %v", alias, err)
		}
		if got := raw.(*gemini.Options).Model; got != gemini.ModelFlashLite {
			t.Errorf("--model %s resolved to %q, want %q", alias, got, gemini.ModelFlashLite)
		}
	}
}

func TestFlashLite_DefaultSizeAccepted(t *testing.T) {
	if _, err := readFlags(t, "--model", "flash-lite"); err != nil {
		t.Errorf("flash-lite at the default 1K should parse cleanly: %v", err)
	}
}

// 2K/4K stay valid for flash-lite's siblings, so the rejection has to come from
// the per-model set rather than the enum tag.
func TestFlashLite_RejectsSizesAboveOneK(t *testing.T) {
	for _, size := range []string{"2K", "4K"} {
		_, err := readFlags(t, "--model", "flash-lite", "--size", size)
		if err == nil {
			t.Errorf("--size %s on flash-lite should be rejected", size)
			continue
		}
		if !strings.Contains(err.Error(), gemini.ModelFlashLite) {
			t.Errorf("error should name the model, got %q", err)
		}
	}
}

func TestFlashLite_SiblingsStillReachFourK(t *testing.T) {
	for _, model := range []string{"pro", "flash"} {
		if _, err := readFlags(t, "--model", model, "--size", "4K"); err != nil {
			t.Errorf("--model %s --size 4K should be accepted: %v", model, err)
		}
	}
}

// Grounding became model-gated once flash-lite joined without Google Search
// support, so the check runs in providers.CheckModel, not the flag layer.
func TestFlashLite_GroundingGatedByModel(t *testing.T) {
	b, _ := providers.Get("gemini")

	errs := providers.CheckModel([]string{"grounding"}, b, &gemini.Options{Model: gemini.ModelFlashLite})
	if len(errs) != 1 {
		t.Fatalf("expected --grounding on flash-lite to be rejected, got %v", errs)
	}
	for _, model := range []string{gemini.ModelPro, gemini.ModelFlash} {
		if errs := providers.CheckModel([]string{"grounding"}, b, &gemini.Options{Model: model}); len(errs) != 0 {
			t.Errorf("--grounding on %s should be accepted, got %v", model, errs)
		}
	}
}

func TestFlashLite_ThinkingAllowedImageSearchNot(t *testing.T) {
	b, _ := providers.Get("gemini")
	opts := &gemini.Options{Model: gemini.ModelFlashLite}

	if errs := providers.CheckModel([]string{"thinking"}, b, opts); len(errs) != 0 {
		t.Errorf("flash-lite supports thinking (minimal and high), got %v", errs)
	}
	if errs := providers.CheckModel([]string{"image-search"}, b, opts); len(errs) != 1 {
		t.Errorf("expected --image-search on flash-lite to be rejected, got %v", errs)
	}
}

// Google's published per-model tables disagree with each other and with the
// API, which accepts all 14 for every model. Pinned here so a doc-driven "fix"
// that narrows the set has to fail a test first.
func TestAspectRatios_MatchesAPIAcceptedSet(t *testing.T) {
	want := []string{
		"1:1", "1:4", "1:8", "2:3", "3:2", "3:4", "4:1",
		"4:3", "4:5", "5:4", "8:1", "9:16", "16:9", "21:9",
	}
	got := gemini.AspectRatios()
	if len(got) != len(want) {
		t.Fatalf("got %d ratios, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ratio %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAspectRatios_AcceptedOnEveryModel(t *testing.T) {
	for _, model := range []string{"pro", "flash", "flash-lite"} {
		for _, ar := range gemini.AspectRatios() {
			if _, err := readFlags(t, "--model", model, "--aspect-ratio", ar); err != nil {
				t.Errorf("-m %s -a %s should be accepted: %v", model, ar, err)
			}
		}
	}
}

func TestAspectRatios_RejectsUnsupportedRatio(t *testing.T) {
	// 9:21 appears in Google's Vertex model card but the API rejects it.
	for _, ar := range []string{"9:21", "16;9", "3:1", "square"} {
		if _, err := readFlags(t, "--aspect-ratio", ar); err == nil {
			t.Errorf("--aspect-ratio %s should be rejected", ar)
		}
	}
}

func TestAspectRatios_EmptyStaysAuto(t *testing.T) {
	raw, err := readFlags(t)
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if got := raw.(*gemini.Options).AspectRatio; got != "" {
		t.Errorf("default AspectRatio: got %q, want empty (auto)", got)
	}
}

// The GA rename dropped the -preview suffix. Configs and scripts pinned the old
// spellings, so they must keep resolving forward.
func TestLegacyPreviewIDs_ResolveToStableIDs(t *testing.T) {
	cases := map[string]string{
		"gemini-3-pro-image-preview":     gemini.ModelPro,
		"gemini-3.1-flash-image-preview": gemini.ModelFlash,
	}
	for legacy, want := range cases {
		raw, err := readFlags(t, "--model", legacy)
		if err != nil {
			t.Errorf("--model %s: %v", legacy, err)
			continue
		}
		if got := raw.(*gemini.Options).Model; got != want {
			t.Errorf("--model %s resolved to %q, want %q", legacy, got, want)
		}
	}
}

func TestStableIDs_HaveNoPreviewSuffix(t *testing.T) {
	for _, id := range []string{gemini.ModelPro, gemini.ModelFlash, gemini.ModelFlashLite} {
		if strings.HasSuffix(id, "-preview") {
			t.Errorf("canonical model ID %q is still a preview ID", id)
		}
	}
}
