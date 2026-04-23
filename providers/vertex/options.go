package vertex

import "strings"

// Options is Vertex's private parameter struct. Vertex exposes the same
// models as Gemini but via the Vertex AI SDK — so flag names coincide with
// Gemini's (model, size, aspect-ratio, grounding, thinking). flagspec.Bind
// is idempotent by flag name, so both providers register their own structs
// without collision.
//
// Notably absent vs. Gemini: ImageSearch. The flag ownership gate relies on
// this struct's tagged fields being the source of truth for what Vertex
// accepts, so deliberately leaving the field off rejects `--image-search`
// when Vertex is active.
type Options struct {
	Model       string `flag:"model,m"        desc:"Model: pro, flash"              default:"pro" enum:"@models"`
	Size        string `flag:"size,s"         desc:"Image size: 1K, 2K, 4K"        default:"1K"  enum:"1K,2K,4K"`
	AspectRatio string `flag:"aspect-ratio,a" desc:"Aspect ratio (default: Auto)"`
	Grounding   bool   `flag:"grounding,g"    desc:"Enable Google Search grounding"`
	Thinking    string `flag:"thinking,t"     desc:"Thinking level: minimal, high (flash only)" enum:"MINIMAL,HIGH"`
}

// RequestLabel implements providers.RequestLabeler for status output —
// returns the canonical model ID, matching the legacy spinner behaviour.
func (o *Options) RequestLabel() string {
	return o.Model
}

// Normalize runs after flagspec's reflection-based population.
func (o *Options) Normalize() {
	o.AspectRatio = strings.TrimSpace(o.AspectRatio)
}
