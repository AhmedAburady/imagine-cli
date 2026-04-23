package commands

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// --- Visual palette --------------------------------------------------------

var (
	colAccent  = lipgloss.Color("#FF5F87") // titles, chevrons
	colActive  = lipgloss.Color("#04B575") // bright green — running now
	colDefault = lipgloss.Color("#00B2C7") // cyan — config default
	colUnknown = lipgloss.Color("#EF476F") // red-pink — not built in
	colDim     = lipgloss.Color("#6C7086") // secondary text
	colInk     = lipgloss.Color("#1A1A1A") // dark foreground on pill bg

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent).PaddingLeft(2)
	dimStyle     = lipgloss.NewStyle().Foreground(colDim)
	boldStyle    = lipgloss.NewStyle().Bold(true)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(colActive)

	pillBase    = lipgloss.NewStyle().Foreground(colInk).Bold(true).Padding(0, 1)
	activePill  = pillBase.Background(colActive).Render("ACTIVE")
	defaultPill = pillBase.Background(colDefault).Render("DEFAULT")
	unknownPill = pillBase.Background(colUnknown).Render("NOT BUILT-IN")

	bulletActive = lipgloss.NewStyle().Foreground(colActive).Render("●")
	bulletDim    = dimStyle.Render("·")
)

// --- Command tree ----------------------------------------------------------

// newProvidersCmd builds the `imagine providers` command tree:
//
//	imagine providers          — styled listing (same as `show`)
//	imagine providers show     — explicit listing
//	imagine providers use X    — set default_provider to X
//	imagine providers select   — interactive picker for default_provider
func newProvidersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Inspect and configure providers",
		RunE:  runProvidersShow,
	}
	cmd.AddCommand(
		newProvidersShowCmd(),
		newProvidersUseCmd(),
		newProvidersSelectCmd(),
		newProvidersAddCmd(),
	)
	return cmd
}

func newProvidersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "List configured providers (marking the default and the active one)",
		RunE:  runProvidersShow,
	}
}

// --- Listing rendering -----------------------------------------------------

func runProvidersShow(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	active, _ := resolveProvider("")

	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, titleStyle.Render("PROVIDERS"))
	fmt.Fprintln(out)

	if len(cfg.Providers) == 0 {
		fmt.Fprintf(out, "  %s\n", dimStyle.Render("No providers configured. Edit "+config.DefaultConfigPath()))
		fmt.Fprintln(out)
		return nil
	}

	names := sortedProviderKeys(cfg.Providers)
	maxName := 0
	for _, n := range names {
		if len(n) > maxName {
			maxName = len(n)
		}
	}

	nameStyle := boldStyle.Width(maxName)
	for _, name := range names {
		_, registered := providers.Get(name)
		bullet := bulletDim
		if name == active {
			bullet = bulletActive
		}

		var pills []string
		if name == active {
			pills = append(pills, activePill)
		}
		if name == cfg.DefaultProvider {
			pills = append(pills, defaultPill)
		}
		if !registered {
			pills = append(pills, unknownPill)
		}

		row := strings.Builder{}
		row.WriteString("  ")
		row.WriteString(bullet)
		row.WriteString("  ")
		row.WriteString(nameStyle.Render(name))
		if len(pills) > 0 {
			row.WriteString("  ")
			row.WriteString(strings.Join(pills, " "))
		}
		fmt.Fprintln(out, row.String())
	}

	fmt.Fprintln(out)
	footer := fmt.Sprintf("%d configured  ·  %s", len(names), config.DefaultConfigPath())
	fmt.Fprintf(out, "  %s\n", dimStyle.Render(footer))
	fmt.Fprintln(out)
	return nil
}

// --- Use (non-interactive) -------------------------------------------------

func newProvidersUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "use <name>",
		Short:         "Set the default provider (writes config.yaml)",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			choices := configuredAndRegistered(cfg)
			if len(choices) == 0 {
				return noProvidersError()
			}
			if !slices.Contains(choices, name) {
				return unknownProviderError(name, choices)
			}
			return applyDefault(cmd, cfg, name)
		},
	}
}

// --- Select (interactive) --------------------------------------------------

func newProvidersSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "select",
		Short:         "Interactively pick the default provider",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			choices := configuredAndRegistered(cfg)
			switch len(choices) {
			case 0:
				return noProvidersError()
			case 1:
				fmt.Fprintf(cmd.OutOrStdout(),
					"  %s  Only one provider available (%s). Nothing to pick.\n",
					bulletDim, boldStyle.Render(choices[0]))
				return nil
			}

			opts := make([]huh.Option[string], 0, len(choices))
			for _, n := range choices {
				opts = append(opts, huh.NewOption(n, n))
			}

			chosen := cfg.DefaultProvider
			if !slices.Contains(choices, chosen) {
				chosen = choices[0]
			}

			// Extend the default quit keymap so q and esc also abort.
			// Select.WithKeyMap only scopes to k.Select; custom Quit
			// must go on the Form.
			km := huh.NewDefaultKeyMap()
			km.Quit = key.NewBinding(
				key.WithKeys("q", "esc", "ctrl+c"),
				key.WithHelp("q/esc", "quit"),
			)

			sel := huh.NewSelect[string]().
				Title("Select default provider").
				Options(opts...).
				Value(&chosen)

			form := huh.NewForm(huh.NewGroup(sel)).
				WithKeyMap(km).
				WithShowHelp(true).
				WithTheme(huh.ThemeCharm())

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n",
						bulletDim, dimStyle.Render("cancelled"))
					return nil
				}
				return err
			}

			return applyDefault(cmd, cfg, chosen)
		},
	}
}

// --- Apply + helpers -------------------------------------------------------

func applyDefault(cmd *cobra.Command, cfg *config.Config, name string) error {
	out := cmd.OutOrStdout()
	if cfg.DefaultProvider == name {
		fmt.Fprintf(out, "  %s  default_provider is already %s\n",
			bulletDim, boldStyle.Render(name))
		return nil
	}
	cfg.DefaultProvider = name
	if err := config.Save(cfg); err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			return fmt.Errorf("no config file at %s — create it with at least one provider entry before setting a default", config.DefaultConfigPath())
		}
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(out, "  %s  default_provider set to %s\n",
		successStyle.Render("✓"), boldStyle.Render(name))
	return nil
}

// configuredAndRegistered returns provider names present under providers:
// in config AND compiled into this binary. A config entry for a provider
// not built in ("unknown") is not a valid choice.
func configuredAndRegistered(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	var out []string
	for name := range cfg.Providers {
		if _, ok := providers.Get(name); ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func noProvidersError() error {
	return fmt.Errorf("no providers available. Add at least one under providers: in %s, then retry", config.DefaultConfigPath())
}

func unknownProviderError(name string, choices []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Unknown provider %q. Configured and built-in:\n", name)
	for _, c := range choices {
		fmt.Fprintf(&b, "  - %s\n", c)
	}
	b.WriteString("Run `imagine providers select` for an interactive picker.")
	return errors.New(b.String())
}
