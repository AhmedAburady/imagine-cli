package commands

import (
	"errors"
	"fmt"
	"maps"
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

	use := name
	args := cobra.NoArgs
	if len(bundle.AuthMethods) > 0 {
		// Optional positional selects an auth method: `login` for the
		// interactive sign-in, or a method key.
		args = cobra.MaximumNArgs(1)
		if hasInteractiveMethod(bundle) {
			use = name + " [login]"
		}
	}

	cmd := &cobra.Command{
		Use:           use,
		Short:         short,
		Args:          args,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAddForProvider(cmd, name, schema, args)
		},
	}

	if len(bundle.AuthMethods) > 0 {
		registerFieldFlags(cmd, unionFields(bundle.AuthMethods))
	} else {
		registerFieldFlags(cmd, schema)
	}
	return cmd
}

func registerFieldFlags(cmd *cobra.Command, fields []providers.ConfigField) {
	for _, f := range fields {
		desc := f.Description
		if f.Required {
			desc += "  (required)"
		} else if f.Default != "" {
			desc += fmt.Sprintf("  (default: %s)", f.Default)
		}
		if cmd.Flags().Lookup(toFlag(f.Key)) == nil {
			cmd.Flags().String(toFlag(f.Key), "", desc)
		}
	}
}

// unionFields flattens every method's fields, de-duplicated by key, for flag
// registration only (persistence uses each method's own fields). A key shared
// by multiple methods is emitted once with its method-specific default dropped,
// so `--help` doesn't show one method's default as if it applied to all.
func unionFields(methods []providers.AuthMethod) []providers.ConfigField {
	count := map[string]int{}
	for _, m := range methods {
		for _, f := range m.Fields {
			count[f.Key]++
		}
	}
	seen := map[string]bool{}
	var out []providers.ConfigField
	for _, m := range methods {
		for _, f := range m.Fields {
			if seen[f.Key] {
				continue
			}
			seen[f.Key] = true
			if count[f.Key] > 1 {
				f.Default = ""
			}
			out = append(out, f)
		}
	}
	return out
}

func hasInteractiveMethod(b providers.Bundle) bool {
	for _, m := range b.AuthMethods {
		if m.Login != nil {
			return true
		}
	}
	return false
}

func methodKeys(b providers.Bundle) []string {
	keys := make([]string, len(b.AuthMethods))
	for i, m := range b.AuthMethods {
		keys[i] = m.Key
	}
	return keys
}

// runAddForProvider dispatches onboarding: multi-auth providers go through the
// auth-method selection; the rest use their ConfigSchema field form.
func runAddForProvider(cmd *cobra.Command, name string, schema []providers.ConfigField, args []string) error {
	if bundle, ok := providers.Get(name); ok && len(bundle.AuthMethods) > 0 {
		return runMultiAuthAdd(cmd, name, bundle, args)
	}
	if len(schema) == 0 {
		return fmt.Errorf("provider %q declares no configuration fields (nothing to add)", name)
	}
	collected, err := collectFields(cmd, schema)
	if err != nil {
		return err
	}
	if collected == nil {
		return nil // user cancelled
	}
	return persistAndReport(cmd, name, collected)
}

// runMultiAuthAdd selects an auth method, runs its onboarding (an interactive
// login and/or a field form), and persists auth_method alongside the gathered
// fields. Field collection runs for both kinds so an explicit flag (e.g.
// --vision-model) is honoured even on the login path.
func runMultiAuthAdd(cmd *cobra.Command, name string, bundle providers.Bundle, args []string) error {
	method, err := selectAuthMethod(cmd, bundle, args)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", bulletDim, dimStyle.Render("cancelled"))
			return nil
		}
		return err
	}

	if method.Login != nil {
		if err := method.Login(cmd.Context(), cmd.OutOrStdout()); err != nil {
			return err
		}
	}

	collected, err := collectFields(cmd, method.Fields)
	if err != nil {
		return err
	}
	if collected == nil {
		return nil // user cancelled
	}
	fields := map[string]string{"auth_method": method.Key}
	maps.Copy(fields, collected)
	return persistAndReport(cmd, name, fields)
}

// selectAuthMethod resolves which auth method to onboard, in order: an explicit
// positional (`login` → the interactive method, or a method key); a field
// method whose flag was supplied (e.g. --api-key); an interactive chooser on a
// TTY; otherwise an actionable error.
func selectAuthMethod(cmd *cobra.Command, bundle providers.Bundle, args []string) (providers.AuthMethod, error) {
	if len(args) == 1 {
		return matchPositional(bundle, args[0])
	}
	if m, ok := methodFromFlags(cmd, bundle); ok {
		return m, nil
	}
	if stdinIsTerminal() {
		return chooseAuthMethod(bundle.AuthMethods)
	}
	return providers.AuthMethod{}, fmt.Errorf("specify how to authenticate: pass a credential flag (e.g. --api-key <key>), or run `imagine providers add %s login` to sign in", bundle.Info.Name)
}

// matchPositional resolves the `login` keyword (the interactive method) or a
// literal method key.
func matchPositional(bundle providers.Bundle, arg string) (providers.AuthMethod, error) {
	if arg == "login" {
		for _, m := range bundle.AuthMethods {
			if m.Login != nil {
				return m, nil
			}
		}
		return providers.AuthMethod{}, fmt.Errorf("%q has no interactive sign-in", bundle.Info.Name)
	}
	for _, m := range bundle.AuthMethods {
		if m.Key == arg {
			return m, nil
		}
	}
	return providers.AuthMethod{}, fmt.Errorf("unknown auth method %q (choices: %s, or `login`)", arg, strings.Join(methodKeys(bundle), ", "))
}

// methodFromFlags picks the field method whose required field flag was set
// (e.g. --api-key selects the api_key method).
func methodFromFlags(cmd *cobra.Command, bundle providers.Bundle) (providers.AuthMethod, bool) {
	for _, m := range bundle.AuthMethods {
		for _, f := range m.Fields {
			if !f.Required {
				continue
			}
			if v, _ := cmd.Flags().GetString(toFlag(f.Key)); v != "" {
				return m, true
			}
		}
	}
	return providers.AuthMethod{}, false
}

func chooseAuthMethod(methods []providers.AuthMethod) (providers.AuthMethod, error) {
	opts := make([]huh.Option[string], len(methods))
	for i, m := range methods {
		label := m.Title
		if m.Description != "" {
			label += " — " + m.Description
		}
		opts[i] = huh.NewOption(label, m.Key)
	}
	var picked string
	form := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("How do you want to authenticate?").Options(opts...).Value(&picked),
	)).WithTheme(huh.ThemeCharm())
	if err := form.Run(); err != nil {
		return providers.AuthMethod{}, err
	}
	for _, m := range methods {
		if m.Key == picked {
			return m, nil
		}
	}
	return providers.AuthMethod{}, errors.New("no auth method selected")
}

// collectFields gathers values for `fields` from flags, falling back to an
// interactive wizard for missing required fields on a TTY. Returns a nil map
// (no error) when the user cancels the wizard.
func collectFields(cmd *cobra.Command, fields []providers.ConfigField) (map[string]string, error) {
	collected := map[string]string{}
	var missing []providers.ConfigField
	for _, f := range fields {
		val, _ := cmd.Flags().GetString(toFlag(f.Key))
		switch {
		case val != "":
			collected[f.Key] = val
		case f.Default != "" && !f.Required:
			collected[f.Key] = f.Default
		default:
			missing = append(missing, f)
		}
	}

	if len(filterRequired(missing)) > 0 {
		if !stdinIsTerminal() {
			return nil, missingFlagsError(cmd.Name(), filterRequired(missing))
		}
		if err := wizardFill(fields, collected); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", bulletDim, dimStyle.Render("cancelled"))
				return nil, nil
			}
			return nil, err
		}
	}
	for _, f := range fields {
		if f.Required && collected[f.Key] == "" {
			return nil, fmt.Errorf("required field %q is empty", f.Key)
		}
	}
	return collected, nil
}

// persistAndReport writes the stanza and prints the success block.
func persistAndReport(cmd *cobra.Command, name string, fields map[string]string) error {
	if err := config.SaveProviderFields(name, fields); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "\n  %s  %s added to %s\n",
		successStyle.Render("✓"), boldStyle.Render(name), dimStyle.Render(config.DefaultConfigPath()))
	fmt.Fprintf(out, "  %s  %s\n\n",
		bulletDim, dimStyle.Render(fmt.Sprintf("run `imagine providers use %s` to make it the default", name)))
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
