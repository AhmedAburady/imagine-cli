package gemini

import "strings"

// Options is Gemini's private parameter struct. flagspec reflects the tags
// at registration to bind Cobra flags, and again at PreRunE to populate this
// struct from the parsed flag values. Generate type-asserts Request.Options
// to *Options — no string keys, no silent zero-value fallbacks.
//
// Shared flag names (model, size, aspect-ratio, grounding, thinking) are
// registered idempotently alongside Vertex's own Options; the active
// provider registers first, so its desc+default text wins in `--help`.
type Options struct {
	Model       string `flag:"model,m"        desc:"Model: pro, flash"                        default:"pro" enum:"@models"`
	Size        string `flag:"size,s"         desc:"Image size: 1K, 2K, 4K"                  default:"1K"  enum:"1K,2K,4K"`
	AspectRatio string `flag:"aspect-ratio,a" desc:"Aspect ratio (default: Auto)"`
	Grounding   bool   `flag:"grounding,g"    desc:"Enable Google Search grounding"`
	Thinking    string `flag:"thinking,t"     desc:"Thinking level: minimal, high (flash only)" enum:"MINIMAL,HIGH"`
	ImageSearch bool   `flag:"image-search,I" desc:"Enable Image Search grounding (flash only)"`
}

// RequestLabel implements providers.RequestLabeler for status output.
// Returns the canonical model ID (matching the historical spinner output
// where the map-based options stored the resolved model under "model").
func (o *Options) RequestLabel() string {
	return o.Model
}

// Normalize runs after flagspec's reflection-based population. Kept as a
// defensive hook; strings are already trimmed by flagspec, and enum fields
// (Thinking) are canonicalised to uppercase by the enum tag itself.
func (o *Options) Normalize() {
	o.AspectRatio = strings.TrimSpace(o.AspectRatio)
}
