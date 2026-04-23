package gemini

import (
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// init self-registers the Gemini provider. Consumed by cmd/imagine/main.go's
// blank-import, which is the only thing that triggers this side effect.
//
// Flag binding/parsing is delegated to providers/flagspec, which reflects
// the tags on Options. Adding a new flag means adding a field — no edits
// to this file.
func init() {
	info := (&Provider{}).Info()
	providers.Register("gemini", providers.Bundle{
		Factory: New,
		BindFlags: func(cmd *cobra.Command) {
			// flagspec.Bind is idempotent by flag name — safe alongside Vertex.
			// Panics on malformed tags (programmer error at init time).
			flagspec.Bind(cmd, Options{})
		},
		ReadFlags: func(cmd *cobra.Command) (any, error) {
			return flagspec.Read(cmd, Options{}, info)
		},
		SupportedFlags: flagspec.FieldNames(Options{}),
		Examples:       Examples,
		Info:           info,
		ConfigSchema:   (&Provider{}).ConfigSchema(),
	})
}
