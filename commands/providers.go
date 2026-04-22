package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// newProvidersCmd builds the `imagine providers` command tree. Subcommands
// are discovery/query actions (not help) — `show` lists what's configured.
func newProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Inspect configured and built-in providers",
	}
	cmd.AddCommand(newProvidersShowCmd())
	return cmd
}

func newProvidersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "List configured providers (marking the default and the active one)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			active, _ := resolveProvider("") // ignore error — we still want to print

			out := cmd.OutOrStdout()

			// Default provider line
			if cfg.DefaultProvider == "" {
				fmt.Fprintln(out, "default_provider: (not set — falls back to first configured)")
			} else {
				fmt.Fprintf(out, "default_provider: %s\n", cfg.DefaultProvider)
			}
			fmt.Fprintln(out)

			if len(cfg.Providers) == 0 {
				fmt.Fprintf(out, "No providers configured. Add a providers: entry to %s (see README).\n", config.DefaultConfigPath())
				return nil
			}

			fmt.Fprintln(out, "providers:")
			for _, name := range sortedProviderKeys(cfg.Providers) {
				pc := cfg.Providers[name]
				markers := []string{}
				if name == active {
					markers = append(markers, "active")
				}
				if name == cfg.DefaultProvider {
					markers = append(markers, "default")
				}
				if _, registered := providers.Get(name); !registered {
					markers = append(markers, "unknown: not built into this binary")
				}
				label := name
				if len(markers) > 0 {
					label += "  [" + joinMarkers(markers) + "]"
				}
				fmt.Fprintf(out, "  %s\n", label)

				if pc.APIKey != "" {
					fmt.Fprintf(out, "    api_key: %s\n", maskKey(pc.APIKey))
				}
				if len(pc.ProviderOptions) > 0 {
					fmt.Fprintln(out, "    provider_options:")
					for k, v := range pc.ProviderOptions {
						fmt.Fprintf(out, "      %s: %s\n", k, v)
					}
				}
			}
			return nil
		},
	}
}

func joinMarkers(markers []string) string {
	s := ""
	for i, m := range markers {
		if i > 0 {
			s += ", "
		}
		s += m
	}
	return s
}

func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) > 12 {
		return key[:8] + "..." + key[len(key)-4:]
	}
	return "***"
}
