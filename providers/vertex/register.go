package vertex

import (
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// init self-registers the Vertex provider. Flag binding/parsing goes through
// providers/flagspec against Vertex's own Options struct — the shared flag
// names with Gemini are safe because flagspec.Bind is idempotent by name.
func init() {
	info := (&Provider{}).Info()
	providers.Register("vertex", providers.Bundle{
		Factory: New,
		BindFlags: func(cmd *cobra.Command) {
			flagspec.Bind(cmd, Options{})
		},
		ReadFlags: func(cmd *cobra.Command) (any, error) {
			return flagspec.Read(cmd, Options{}, info)
		},
		SupportedFlags: flagspec.FieldNames(Options{}),
		Examples:       gemini.Examples, // Vertex reuses Gemini's examples
		Info:           info,
		ConfigSchema:   (&Provider{}).ConfigSchema(),
		Vision:         &providers.Vision{DefaultModel: DefaultVisionModel},
	})
}
