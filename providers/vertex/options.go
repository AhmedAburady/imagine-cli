package vertex

import (
	"strings"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// Options is Vertex's private parameter struct. Vertex exposes the same
// models as Gemini but via the Vertex AI SDK, so flag names coincide with
// Gemini's (model, size, aspect-ratio, grounding, thinking, image-search).
// flagspec.Bind is idempotent by flag name, so both providers register their
// own structs without collision.
type Options struct {
	Model       string `flag:"model,m"        desc:"Model: pro, flash, flash-lite"                       default:"pro" enum:"@models"`
	Size        string `flag:"size,s"         desc:"Image size: 1K, 2K, 4K (flash-lite: 1K only)" default:"1K" enum:"1K,2K,4K"`
	AspectRatio string `flag:"aspect-ratio,a" desc:"Aspect ratio: 14 options, see ASPECT RATIOS (default: Auto)"`
	Grounding   bool   `flag:"grounding,g"    desc:"Enable Google Search grounding (not on flash-lite)"`
	Thinking    string `flag:"thinking,t"     desc:"Thinking level: minimal, medium, high (medium: flash only; not on pro)" enum:"MINIMAL,MEDIUM,HIGH"`
	ImageSearch bool   `flag:"image-search,I" desc:"Enable Image Search grounding (flash only)"`
}

// RequestLabel implements providers.RequestLabeler for status output —
// returns the canonical model ID, matching the legacy spinner behaviour.
func (o *Options) RequestLabel() string {
	return o.Model
}

// ResolvedModel implements providers.ResolvedModeler so the model-level
// flag-support gate can look up per-model SupportedFlags.
func (o *Options) ResolvedModel() string {
	return o.Model
}

// Normalize runs after flagspec's reflection-based population.
func (o *Options) Normalize() {
	o.AspectRatio = strings.TrimSpace(o.AspectRatio)
}

// Validate runs on the CLI and batch paths alike.
func (o *Options) Validate(info providers.Info) error {
	if err := info.CheckSize(o.Model, o.Size); err != nil {
		return err
	}
	if err := gemini.CheckThinking(o.Model, o.Thinking); err != nil {
		return err
	}
	return info.CheckAspectRatio(o.AspectRatio)
}
