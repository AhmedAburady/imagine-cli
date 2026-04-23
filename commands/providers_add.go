package commands

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// newProvidersAddCmd builds the `imagine providers add` command tree.
//
// Each registered provider gets its own sub-sub-command so help text is
// accurate per-provider — `providers add gemini --help` shows Gemini's
// flags only, not the union across all providers. Adding a provider to
// the registry automatically surfaces a new `add <name>` sub-command with
// no edits here.
//
// Dual-mode behaviour (see Docs/adding-a-provider.md):
//   - all required fields supplied via flags → non-interactive, headless-friendly
//   - any required field missing + stdin is a TTY → huh wizard prompts
//     only for the missing fields (flag-provided values pre-fill the form)
//   - any required field missing + non-TTY → error with exact flag names,
//     so agents and CI scripts get deterministic, actionable output
func newProvidersAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <provider>",
		Short: "Register a provider's credentials (interactive or via flags)",
		Long: "Register a provider's credentials (interactive or via flags).\n\n" +
			"Run `imagine providers add <provider>` for a specific provider's\n" +
			"flags and behaviour. Examples: `providers add gemini`, `providers add vertex`.",
	}
	// Fan out: one sub-sub-command per registered provider. Fixes the
	// flag-collision problem (--api-key means different things for
	// gemini vs openai; --gcp-project only applies to vertex).
	//
	// Snapshot of providers.List() at command-tree build time. Every
	// built-in provider registers via init() (see providers/all), so by
	// the time main() calls NewRootCmd the list is complete. Runtime
	// registration after this point is not supported.
	for _, name := range providers.List() {
		cmd.AddCommand(newProvidersAddForCmd(name))
	}
	return cmd
}

// newProvidersAddForCmd builds a sub-sub-command dedicated to one
// provider. Its flag set is exactly that provider's ConfigSchema —
// no pollution from other providers' fields.
func newProvidersAddForCmd(name string) *cobra.Command {
	schema := providerSchema(name)
	bundle, _ := providers.Get(name)

	short := fmt.Sprintf("Register credentials for %s", name)
	if bundle.Info.DisplayName != "" {
		short = fmt.Sprintf("Register credentials for %s (%s)", name, bundle.Info.DisplayName)
	}

	cmd := &cobra.Command{
		Use:           name,
		Short:         short,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAddForProvider(cmd, name, schema)
		},
	}

	for _, f := range schema {
		flagName := toFlag(f.Key)
		desc := f.Description
		if f.Required {
			desc = desc + "  (required)"
		} else if f.Default != "" {
			desc = desc + fmt.Sprintf("  (default: %s)", f.Default)
		}
		cmd.Flags().String(flagName, "", desc)
	}
	return cmd
}

// runAddForProvider is the shared happy-path logic for every
// `add <provider>` sub-command.
func runAddForProvider(cmd *cobra.Command, name string, schema []providers.ConfigField) error {
	if len(schema) == 0 {
		return fmt.Errorf("provider %q declares no configuration fields (nothing to add)", name)
	}

	// Partition: flag-provided vs missing. Optional fields with a
	// non-empty Default auto-resolve without prompting.
	collected := map[string]string{}
	var missing []providers.ConfigField
	for _, f := range schema {
		flagName := toFlag(f.Key)
		val, _ := cmd.Flags().GetString(flagName)
		if val != "" {
			collected[f.Key] = val
			continue
		}
		if f.Default != "" && !f.Required {
			collected[f.Key] = f.Default
			continue
		}
		missing = append(missing, f)
	}

	missingRequired := filterRequired(missing)
	if len(missingRequired) > 0 {
		if !stdinIsTerminal() {
			return missingFlagsError(name, missingRequired)
		}
		if err := wizardFill(schema, collected); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n",
					bulletDim, dimStyle.Render("cancelled"))
				return nil
			}
			return err
		}
	}

	for _, f := range schema {
		if f.Required && collected[f.Key] == "" {
			return fmt.Errorf("required field %q is empty", f.Key)
		}
	}

	if err := config.SaveProviderFields(name, collected); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s  %s added to %s\n",
		successStyle.Render("✓"),
		boldStyle.Render(name),
		dimStyle.Render(config.DefaultConfigPath()))
	fmt.Fprintf(out, "  %s  %s\n\n",
		bulletDim,
		dimStyle.Render(fmt.Sprintf("run `imagine providers use %s` to make it the default", name)))
	return nil
}

// wizardFill mutates `collected` in place with values gathered from an
// interactive huh form. Only fields not already in `collected` are asked
// for; flag-prefilled fields are respected.
func wizardFill(schema []providers.ConfigField, collected map[string]string) error {
	values := make(map[string]*string, len(schema))
	var inputs []huh.Field
	for i := range schema {
		f := schema[i]
		if _, already := collected[f.Key]; already {
			continue
		}
		initial := f.Default
		values[f.Key] = &initial

		input := huh.NewInput().
			Title(f.Title).
			Description(f.Description).
			Value(values[f.Key])
		if f.Secret {
			input = input.EchoMode(huh.EchoModePassword)
		}
		if f.Required {
			input = input.Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("required")
				}
				return nil
			})
		}
		inputs = append(inputs, input)
	}

	if len(inputs) == 0 {
		return nil
	}

	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(
		key.WithKeys("ctrl+c", "esc"),
		key.WithHelp("esc", "cancel"),
	)

	form := huh.NewForm(huh.NewGroup(inputs...)).
		WithKeyMap(km).
		WithShowHelp(true).
		WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	for k, v := range values {
		if *v != "" {
			collected[k] = strings.TrimSpace(*v)
		}
	}
	return nil
}

// providerSchema returns the provider's ConfigSchema from its Bundle,
// falling back to a single required api_key field when unset.
func providerSchema(name string) []providers.ConfigField {
	b, ok := providers.Get(name)
	if !ok {
		return nil
	}
	if len(b.ConfigSchema) > 0 {
		return b.ConfigSchema
	}
	return defaultSchema()
}

func defaultSchema() []providers.ConfigField {
	return []providers.ConfigField{
		{Key: "api_key", Title: "API Key", Description: "Provider API key", Secret: true, Required: true},
	}
}

func filterRequired(fields []providers.ConfigField) []providers.ConfigField {
	var out []providers.ConfigField
	for _, f := range fields {
		if f.Required {
			out = append(out, f)
		}
	}
	return out
}

func missingFlagsError(name string, missing []providers.ConfigField) error {
	var b strings.Builder
	fmt.Fprintf(&b, "missing required flags for %q:\n", name)
	for _, f := range missing {
		fmt.Fprintf(&b, "  --%s  (%s)\n", toFlag(f.Key), f.Title)
	}
	b.WriteString("Run this command from a terminal to use the interactive wizard instead.")
	return errors.New(b.String())
}

// toFlag converts a schema key (underscore_case) to a CLI flag name
// (dash-case): api_key → api-key, gcp_project → gcp-project.
func toFlag(key string) string {
	return strings.ReplaceAll(key, "_", "-")
}

func stdinIsTerminal() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())
}
