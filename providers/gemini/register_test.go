package gemini_test

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// Exercises the full Bundle.BindFlags → Parse → Bundle.ReadFlags → *Options
// pipeline end-to-end. This is the wiring that the runtime CLI hits; if it
// breaks, every Gemini invocation is broken.
func TestBundle_ParsesAndResolvesAllFlags(t *testing.T) {
	b, ok := providers.Get("gemini")
	if !ok {
		t.Fatal("gemini Bundle not registered")
	}

	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	b.BindFlags(cmd)

	args := []string{
		"--model", "flash",
		"--size", "2K",
		"--aspect-ratio", "16:9",
		"--grounding",
		"--thinking", "high",
		"--image-search",
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := b.ReadFlags(cmd)
	if err != nil {
		t.Fatalf("ReadFlags: %v", err)
	}
	opts, ok := raw.(*gemini.Options)
	if !ok {
		t.Fatalf("ReadFlags returned %T, want *gemini.Options", raw)
	}

	if opts.Model != gemini.ModelFlash {
		t.Errorf("Model: got %q, want %q (alias 'flash' → canonical)", opts.Model, gemini.ModelFlash)
	}
	if opts.Size != "2K" {
		t.Errorf("Size: got %q, want 2K", opts.Size)
	}
	if opts.AspectRatio != "16:9" {
		t.Errorf("AspectRatio: got %q, want 16:9", opts.AspectRatio)
	}
	if !opts.Grounding {
		t.Error("Grounding: got false, want true")
	}
	if opts.Thinking != "HIGH" {
		t.Errorf("Thinking: got %q, want HIGH (canonicalised from 'high')", opts.Thinking)
	}
	if !opts.ImageSearch {
		t.Error("ImageSearch: got false, want true")
	}
}

func TestBundle_DefaultsMatchLegacyBehavior(t *testing.T) {
	b, _ := providers.Get("gemini")
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	b.BindFlags(cmd)

	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := b.ReadFlags(cmd)
	if err != nil {
		t.Fatalf("ReadFlags: %v", err)
	}
	opts := raw.(*gemini.Options)

	if opts.Model != gemini.ModelPro {
		t.Errorf("default Model: got %q, want %q", opts.Model, gemini.ModelPro)
	}
	if opts.Size != "1K" {
		t.Errorf("default Size: got %q, want 1K", opts.Size)
	}
	if opts.AspectRatio != "" {
		t.Errorf("default AspectRatio: got %q, want empty", opts.AspectRatio)
	}
	if opts.Grounding || opts.ImageSearch {
		t.Error("defaults for Grounding/ImageSearch should be false")
	}
	if opts.Thinking != "" {
		t.Errorf("default Thinking: got %q, want empty", opts.Thinking)
	}
}

func TestBundle_SupportedFlagsCoverAllDeclared(t *testing.T) {
	b, _ := providers.Get("gemini")
	want := map[string]bool{
		"model": true, "size": true, "aspect-ratio": true,
		"grounding": true, "thinking": true, "image-search": true,
	}
	if len(b.SupportedFlags) != len(want) {
		t.Errorf("SupportedFlags count: got %d, want %d: %v", len(b.SupportedFlags), len(want), b.SupportedFlags)
	}
	for _, f := range b.SupportedFlags {
		if !want[f] {
			t.Errorf("unexpected flag in SupportedFlags: %q", f)
		}
	}
}

func TestBundle_RequestLabelReturnsCanonicalModelID(t *testing.T) {
	opts := &gemini.Options{Model: gemini.ModelFlash}
	if got := opts.RequestLabel(); got != gemini.ModelFlash {
		t.Errorf("RequestLabel: got %q, want %q", got, gemini.ModelFlash)
	}
}

func TestBundle_InvalidThinkingRejected(t *testing.T) {
	b, _ := providers.Get("gemini")
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	b.BindFlags(cmd)
	cmd.SetArgs([]string{"--thinking", "extreme"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := b.ReadFlags(cmd); err == nil {
		t.Error("expected invalid --thinking to be rejected, got nil")
	}
}
