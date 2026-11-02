package openai_test

import (
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/openai"
)

// Transparent needs an alpha channel, so the output format decides, not the model.
func TestBackgroundTransparent(t *testing.T) {
	b, ok := providers.Get("openai")
	if !ok {
		t.Fatal("openai Bundle not registered")
	}
	cases := []struct {
		model, filename string
		ok              bool
	}{
		{"flare", "out.png", true},
		{"sunburst", "out.webp", true},
		{"image2", "out.png", true},
		{"flare", "out.jpg", false},
		{"flare", "out.jpeg", false},
	}
	for _, c := range cases {
		got, err := b.ParseOptions(
			map[string]any{"model": c.model, "background": "transparent"},
			providers.Common{Filename: c.filename},
		)
		if c.ok && err != nil {
			t.Errorf("%s -f %s: unexpected error: %v", c.model, c.filename, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s -f %s: expected an error", c.model, c.filename)
		}
		if c.ok && got.(*openai.Options).Background != "transparent" {
			t.Errorf("%s -f %s: background not passed through", c.model, c.filename)
		}
	}
}
