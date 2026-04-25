package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// enforceFlagSupport rejects provider-private flags the user set
// explicitly but that the active provider doesn't claim. Thin adapter
// over providers.CheckBundle — the rule lives in providers/gate.go so
// the batch path applies the exact same logic.
func enforceFlagSupport(cmd *cobra.Command, active providers.Bundle) error {
	set := collectExplicitProviderFlags(cmd)
	if errs := providers.CheckBundle(set, active); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// enforceModelSupport runs *after* ReadFlags, once the resolved model
// ID is known. Thin adapter over providers.CheckModel.
func enforceModelSupport(cmd *cobra.Command, active providers.Bundle, providerOptions any) error {
	set := collectExplicitProviderFlags(cmd)
	if errs := providers.CheckModel(set, active, providerOptions); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// collectExplicitProviderFlags returns the long names of provider-
// private flags the user explicitly set on the CLI invocation. Common
// flags are filtered out — they have no opinion from the gate.
func collectExplicitProviderFlags(cmd *cobra.Command) []string {
	var out []string
	cmd.Flags().Visit(func(fl *pflag.Flag) {
		if cli.IsCommonFlag(fl.Name) {
			return
		}
		out = append(out, fl.Name)
	})
	return out
}
