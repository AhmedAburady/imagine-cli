package vertex_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
	"github.com/AhmedAburady/imagine-cli/providers/vertex"
)

// readFlags drives the real Bundle wiring, which is where the per-model size
// rule is enforced via the Validate hook.
func readFlags(t *testing.T, args ...string) (any, error) {
	t.Helper()
	b, ok := providers.Get("vertex")
	if !ok {
		t.Fatal("vertex Bundle not registered")
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
		if got := raw.(*vertex.Options).Model; got != gemini.ModelFlashLite {
			t.Errorf("--model %s resolved to %q, want %q", alias, got, gemini.ModelFlashLite)
		}
	}
}

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

func TestFlashLite_DefaultSizeAccepted(t *testing.T) {
	if _, err := readFlags(t, "--model", "flash-lite"); err != nil {
		t.Errorf("flash-lite at the default 1K should parse cleanly: %v", err)
	}
}

// Vertex reports grounding as unsupported for flash-lite too; --image-search it
// never offered at all, so that flag isn't on vertex.Options.
func TestFlashLite_GroundingGatedByModel(t *testing.T) {
	b, _ := providers.Get("vertex")

	errs := providers.CheckModel([]string{"grounding"}, b, &vertex.Options{Model: gemini.ModelFlashLite})
	if len(errs) != 1 {
		t.Fatalf("expected --grounding on flash-lite to be rejected, got %v", errs)
	}
	for _, model := range []string{gemini.ModelPro, gemini.ModelFlash} {
		if errs := providers.CheckModel([]string{"grounding"}, b, &vertex.Options{Model: model}); len(errs) != 0 {
			t.Errorf("--grounding on %s should be accepted, got %v", model, errs)
		}
	}
}

func TestFlashLite_ThinkingAllowed(t *testing.T) {
	b, _ := providers.Get("vertex")
	opts := &vertex.Options{Model: gemini.ModelFlashLite}
	if errs := providers.CheckModel([]string{"thinking"}, b, opts); len(errs) != 0 {
		t.Errorf("flash-lite supports thinking (minimal and high), got %v", errs)
	}
}
