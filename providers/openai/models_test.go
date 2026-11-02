package openai_test

import (
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
	_ "github.com/AhmedAburady/imagine-cli/providers/all"
)

func openaiInfo(t *testing.T) providers.Info {
	t.Helper()
	b, ok := providers.Get("openai")
	if !ok {
		t.Fatal("openai provider is not registered")
	}
	return b.Info
}

func TestModels_AliasesResolve(t *testing.T) {
	info := openaiInfo(t)
	cases := map[string]string{
		"":                       "gpt-image-2.5-flare",
		"flare":                  "gpt-image-2.5-flare",
		"2.5":                    "gpt-image-2.5-flare",
		"gpt-image-2.5-flare":    "gpt-image-2.5-flare",
		"sunburst":               "gpt-image-2.5-sunburst",
		"gpt-image-2.5-sunburst": "gpt-image-2.5-sunburst",
		"image2":                 "gpt-image-2",
		"2":                      "gpt-image-2",
		"gpt-image-2":            "gpt-image-2",
	}
	for raw, want := range cases {
		got, err := info.ResolveModel(raw)
		if err != nil {
			t.Errorf("ResolveModel(%q): %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("ResolveModel(%q) = %q, want %q", raw, got, want)
		}
	}
	if info.DefaultModel != "gpt-image-2.5-flare" {
		t.Errorf("DefaultModel = %q, want gpt-image-2.5-flare", info.DefaultModel)
	}
}

// The gpt-image-1 family and chatgpt-image-latest shut down Oct 23 / Dec 1 2026.
func TestModels_ShutDownModelsRejected(t *testing.T) {
	info := openaiInfo(t)
	gone := []string{
		"gpt-image-1.5", "1.5",
		"gpt-image-1", "1",
		"gpt-image-1-mini", "mini", "1-mini",
		"chatgpt-image-latest", "latest",
		"dall-e-2", "dall-e-3",
	}
	for _, raw := range gone {
		if got, err := info.ResolveModel(raw); err == nil {
			t.Errorf("ResolveModel(%q) = %q, want an error", raw, got)
		}
	}
}
