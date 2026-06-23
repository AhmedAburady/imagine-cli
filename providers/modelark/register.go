package modelark

import (
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// init self-registers the modelark provider. Consumed by providers/all's
// blank-import. Flag binding/parsing is delegated to providers/flagspec.
//
// RequireStorage marks modelark as needing the shared S3 storage brick — the
// only framework-level distinction from a plain provider like fal.
func init() {
	info := (&Provider{}).Info()
	providers.Register("modelark", providers.Bundle{
		Factory: New,
		BindFlags: func(cmd *cobra.Command) {
			flagspec.Bind(cmd, Options{})
		},
		ReadFlags: func(cmd *cobra.Command) (any, error) {
			return flagspec.Read(cmd, Options{}, info)
		},
		ParseOptions: func(values map[string]any, _ providers.Common) (any, error) {
			return flagspec.Parse(Options{}, values, info)
		},
		SupportedFlags: flagspec.FieldNames(Options{}),
		Examples:       Examples,
		Info:           info,
		ConfigSchema:   (&Provider{}).ConfigSchema(),
		RequireStorage: true,
	})
}
