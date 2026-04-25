package commands

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// applyProviderFlagVisibility hides provider-private flags that don't belong
// to the active provider, so `--help` focuses on what the user will actually
// use. Common flags and the active provider's flags stay visible. Hidden
// flags still parse — the ownership gate in validate.go catches them.
func applyProviderFlagVisibility(cmd *cobra.Command, active string) {
	if active == "" {
		return // show everything when no provider is resolvable
	}
	bundle, ok := providers.Get(active)
	if !ok {
		return
	}
	supported := make(map[string]bool, len(bundle.SupportedFlags))
	for _, f := range bundle.SupportedFlags {
		supported[f] = true
	}
	cmd.Flags().VisitAll(func(fl *pflag.Flag) {
		if cli.IsCommonFlag(fl.Name) {
			fl.Hidden = false
			return
		}
		if supported[fl.Name] {
			fl.Hidden = false
			return
		}
		if len(providers.ProvidersSupportingFlag(fl.Name)) > 0 {
			fl.Hidden = true
		}
	})
}

// buildExamples renders the EXAMPLES block for `imagine --help`. Structure:
//
//  1. Active provider + summary (auto from Bundle.Info)
//  2. Model list (auto from Info.Models, default marked with *)
//  3. Provider-owned examples/sizes block from Bundle.Examples()
//
// The auto-generated parts live here so each new provider only has to
// supply Examples(); its Info already carries the model catalogue.
func buildExamples(active string) string {
	if active == "" {
		return ""
	}
	b, ok := providers.Get(active)
	if !ok {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("  ACTIVE PROVIDER: ")
	sb.WriteString(b.Info.Name)
	if b.Info.Summary != "" {
		sb.WriteString("  — ")
		sb.WriteString(b.Info.Summary)
	}
	sb.WriteString("\n")

	if len(b.Info.Models) > 0 {
		sb.WriteString("\n  MODELS:\n")
		for _, m := range b.Info.Models {
			marker := "  "
			if m.ID == b.Info.DefaultModel {
				marker = "* "
			}
			aliases := ""
			if len(m.Aliases) > 0 {
				aliases = " (aliases: " + strings.Join(m.Aliases, ", ") + ")"
			}
			sb.WriteString("    ")
			sb.WriteString(marker)
			sb.WriteString(m.ID)
			sb.WriteString(aliases)
			sb.WriteString("\n")
		}
	}

	if b.Examples != nil {
		if ex := b.Examples(); ex != "" {
			sb.WriteString("\n")
			sb.WriteString(ex)
		}
	}

	return sb.String()
}
