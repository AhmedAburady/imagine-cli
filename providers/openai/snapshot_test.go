package openai

import (
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// A dated ID is a pin: it must reach the API unchanged and still be
// validated as its base model.
func TestSnapshots_ResolveToThemselves(t *testing.T) {
	info := (&Provider{}).Info()
	for _, spec := range modelSpecs {
		for _, snap := range spec.Snapshots {
			got, err := flagspec.Parse(Options{}, map[string]any{"model": snap}, info)
			if err != nil {
				t.Fatalf("-m %s: %v", snap, err)
			}
			if m := got.(*Options).Model; m != snap {
				t.Errorf("-m %s resolved to %q, want the snapshot itself", snap, m)
			}
		}
	}
}

func TestSnapshots_ValidatedAsBaseModel(t *testing.T) {
	info := (&Provider{}).Info()
	cases := []struct {
		model, quality string
		ok             bool
	}{
		{"gpt-image-2.5-flare-2026-09-08", "max", true},
		{"gpt-image-2.5-sunburst-2026-09-08", "xhigh", true},
		{"gpt-image-2-2026-04-21", "high", true},
		{"gpt-image-2-2026-04-21", "xhigh", false},
	}
	for _, c := range cases {
		_, err := flagspec.Parse(Options{}, map[string]any{"model": c.model, "quality": c.quality}, info)
		if c.ok != (err == nil) {
			t.Errorf("-m %s -q %s: ok=%v, err=%v", c.model, c.quality, c.ok, err)
		}
	}
}
