package openai

import (
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// xhigh and max exist on the gpt-image-2.5 models only; the flag enum admits
// them and the per-model table decides.
func TestQualityMatrix(t *testing.T) {
	info := (&Provider{}).Info()
	cases := []struct {
		model, quality string
		ok             bool
	}{
		{"flare", "low", true},
		{"flare", "high", true},
		{"flare", "xhigh", true},
		{"flare", "max", true},
		{"sunburst", "max", true},
		{"sunburst", "auto", true},
		{"image2", "high", true},
		{"image2", "auto", true},
		{"image2", "xhigh", false},
		{"image2", "max", false},
		{"", "max", true},
		{"flare", "ultra", false},
	}
	for _, c := range cases {
		_, err := flagspec.Parse(Options{}, map[string]any{"model": c.model, "quality": c.quality}, info)
		if c.ok && err != nil {
			t.Errorf("model %q quality %q: unexpected error: %v", c.model, c.quality, err)
		}
		if !c.ok && err == nil {
			t.Errorf("model %q quality %q: expected an error", c.model, c.quality)
		}
	}
}

func TestQualityError_NamesSupportedLevels(t *testing.T) {
	err := validateQuality("gpt-image-2", "max")
	if err == nil || !strings.Contains(err.Error(), "low, medium, high, auto") {
		t.Fatalf("error should list gpt-image-2's levels, got: %v", err)
	}
}
