package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// commonFlags are the truly provider-agnostic flags. Every other flag must
// be owned by at least one provider (via Bundle.SupportedFlags) or the
// ownership gate will refuse to classify it.
var commonFlags = map[string]bool{
	"prompt":   true,
	"output":   true,
	"filename": true,
	"count":    true,
	"input":    true,
	"replace":  true,
	"provider": true,
	"help":     true,
	"version":  true,
}

// enforceFlagSupport rejects provider-private flags the user set explicitly
// but that the active provider doesn't support. The error lists the
// providers that DO support the flag, so the fix is obvious.
func enforceFlagSupport(cmd *cobra.Command, active providers.Bundle) error {
	supported := make(map[string]bool, len(active.SupportedFlags))
	for _, f := range active.SupportedFlags {
		supported[f] = true
	}

	var rejected error
	cmd.Flags().Visit(func(fl *pflag.Flag) {
		if rejected != nil {
			return
		}
		if commonFlags[fl.Name] {
			return
		}
		if supported[fl.Name] {
			return
		}
		others := providers.ProvidersSupportingFlag(fl.Name)
		if len(others) == 0 {
			// Unknown flag (cobra would normally reject earlier).
			return
		}
		rejected = fmt.Errorf("--%s is not supported by provider %q (supported by: %v)", fl.Name, active.Info.Name, others)
	})
	return rejected
}
