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
	colAccent  = lipgloss.Color("#FF5F87")
	colActive  = lipgloss.Color("#04B575")
	colDefault = lipgloss.Color("#00B2C7")
	colVision  = lipgloss.Color("#B57FF5")
	colUnknown = lipgloss.Color("#EF476F")
	colDim     = lipgloss.Color("#6C7086")
	colInk     = lipgloss.Color("#1A1A1A")

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent).PaddingLeft(2)
	dimStyle     = lipgloss.NewStyle().Foreground(colDim)
	boldStyle    = lipgloss.NewStyle().Bold(true)
	successStyle = lipgloss.NewStyle().Bold(true).Foreground(colActive)

	pillBase    = lipgloss.NewStyle().Foreground(colInk).Bold(true).Padding(0, 1)
	activePill  = pillBase.Background(colActive).Render("ACTIVE")
	defaultPill = pillBase.Background(colDefault).Render("DEFAULT")
	visionPill  = pillBase.Background(colVision).Render("VISION")
	unknownPill = pillBase.Background(colUnknown).Render("NOT BUILT-IN")

	bulletActive = lipgloss.NewStyle().Foreground(colActive).Render("●")
	bulletDim    = dimStyle.Render("·")
)

// --- Command tree ----------------------------------------------------------

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
		Short: "List configured providers (marking the default, active, and vision defaults)",
		RunE:  runProvidersShow,
	}
}

// --- Listing ---------------------------------------------------------------

func runProvidersShow(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	active, _ := resolveProvider("")
	visionDefault := cfg.VisionDefaultProvider

	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, titleStyle.Render("PROVIDERS"))
	fmt.Fprintln(out)

	if len(cfg.Providers) == 0 {
		fmt.Fprintf(out, "  %s\n\n", dimStyle.Render("No providers configured. Edit "+config.DefaultConfigPath()))
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

	// First pass: compute per-row pills string and the widest one across
	// all rows so the capability badges line up in a single column.
	pillStrs := make([]string, len(names))
	maxPillWidth := 0
	for i, name := range names {
		_, registered := providers.Get(name)
		var pills []string
		if name == active {
			pills = append(pills, activePill)
		}
		if name == cfg.DefaultProvider {
			pills = append(pills, defaultPill)
		}
		if visionDefault != "" && name == visionDefault && visionDefault != cfg.DefaultProvider {
			pills = append(pills, visionPill)
		}
		if !registered {
			pills = append(pills, unknownPill)
		}
		pillStrs[i] = strings.Join(pills, " ")
		if w := lipgloss.Width(pillStrs[i]); w > maxPillWidth {
			maxPillWidth = w
		}
	}

	for i, name := range names {
		b, registered := providers.Get(name)
		bullet := bulletDim
		if name == active {
			bullet = bulletActive
		}

		pills := pillStrs[i]
		pad := maxPillWidth - lipgloss.Width(pills)
		if pad < 0 {
			pad = 0
		}

		caps := dimStyle.Render(capabilityBadges(b, registered))

		var row strings.Builder
		row.WriteString("  ")
		row.WriteString(bullet)
		row.WriteString("  ")
		row.WriteString(nameStyle.Render(name))
		if maxPillWidth > 0 {
			row.WriteString("  ")
			row.WriteString(pills)
			row.WriteString(strings.Repeat(" ", pad))
		}
		if caps != "" {
			row.WriteString("   ")
			row.WriteString(caps)
		}
		fmt.Fprintln(out, row.String())
	}

	fmt.Fprintln(out)
	footer := fmt.Sprintf("%d configured  ·  %s", len(names), config.DefaultConfigPath())
	fmt.Fprintf(out, "  %s\n\n", dimStyle.Render(footer))
	return nil
}

func capabilityBadges(b providers.Bundle, registered bool) string {
	if !registered {
		return ""
	}
	caps := []string{"generate"}
	if b.Vision != nil {
		caps = append(caps, "describe")
	}
	return strings.Join(caps, "  ")
}

// --- Use (non-interactive) -------------------------------------------------

func newProvidersUseCmd() *cobra.Command {
	var vision bool
	cmd := &cobra.Command{
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
			if vision {
				if !supportsVision(name) {
					return noVisionError(name, describerChoices(cfg))
				}
				return applyVisionDefault(cmd, cfg, name)
			}
			return applyDefault(cmd, cfg, name)
		},
	}
	cmd.Flags().BoolVar(&vision, "vision", false, "Set vision_default_provider (describe) instead of default_provider")
	return cmd
}

// --- Select (interactive) --------------------------------------------------

func newProvidersSelectCmd() *cobra.Command {
	var vision bool
	cmd := &cobra.Command{
		Use:           "select",
		Short:         "Interactively pick the default provider",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			var choices []string
			var title, kind string
			var currentDefault string
			if vision {
				choices = describerChoices(cfg)
				title = "Select vision default provider"
				kind = "vision_default_provider"
				currentDefault = cfg.VisionDefaultProvider
			} else {
				choices = configuredAndRegistered(cfg)
				title = "Select default provider"
				kind = "default_provider"
				currentDefault = cfg.DefaultProvider
			}

			switch len(choices) {
			case 0:
				if vision {
					return noDescribersError()
				}
				return noProvidersError()
			case 1:
				fmt.Fprintf(cmd.OutOrStdout(),
					"  %s  Only one %s-eligible provider available (%s). Nothing to pick.\n",
					bulletDim, kind, boldStyle.Render(choices[0]))
				return nil
			}

			opts := make([]huh.Option[string], 0, len(choices))
			for _, n := range choices {
				opts = append(opts, huh.NewOption(n, n))
			}

			chosen := currentDefault
			if !slices.Contains(choices, chosen) {
				chosen = choices[0]
			}

			km := huh.NewDefaultKeyMap()
			km.Quit = key.NewBinding(
				key.WithKeys("q", "esc", "ctrl+c"),
				key.WithHelp("q/esc", "quit"),
			)

			sel := huh.NewSelect[string]().Title(title).Options(opts...).Value(&chosen)
			form := huh.NewForm(huh.NewGroup(sel)).
				WithKeyMap(km).
				WithShowHelp(true).
				WithTheme(huh.ThemeCharm())

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", bulletDim, dimStyle.Render("cancelled"))
					return nil
				}
				return err
			}

			if vision {
				return applyVisionDefault(cmd, cfg, chosen)
			}
			return applyDefault(cmd, cfg, chosen)
		},
	}
	cmd.Flags().BoolVar(&vision, "vision", false, "Pick the vision_default_provider (describe) instead of default_provider")
	return cmd
}

// --- Apply + helpers -------------------------------------------------------

func applyDefault(cmd *cobra.Command, cfg *config.Config, name string) error {
	out := cmd.OutOrStdout()
	if cfg.DefaultProvider == name {
		fmt.Fprintf(out, "  %s  default_provider is already %s\n", bulletDim, boldStyle.Render(name))
		return nil
	}
	cfg.DefaultProvider = name
	if err := saveOrExplain(cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %s  default_provider set to %s\n", successStyle.Render("✓"), boldStyle.Render(name))
	return nil
}

func applyVisionDefault(cmd *cobra.Command, cfg *config.Config, name string) error {
	out := cmd.OutOrStdout()
	if cfg.VisionDefaultProvider == name {
		fmt.Fprintf(out, "  %s  vision_default_provider is already %s\n", bulletDim, boldStyle.Render(name))
		return nil
	}
	cfg.VisionDefaultProvider = name
	if err := saveOrExplain(cfg); err != nil {
		return err
	}
	fmt.Fprintf(out, "  %s  vision_default_provider set to %s\n", successStyle.Render("✓"), boldStyle.Render(name))
	return nil
}

func saveOrExplain(cfg *config.Config) error {
	if err := config.Save(cfg); err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			return fmt.Errorf("no config file at %s — add a provider first with `imagine providers add <name>`", config.DefaultConfigPath())
		}
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// configuredAndRegistered returns provider names present in config AND
// built into this binary, sorted.
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

// describerChoices narrows configuredAndRegistered to providers whose
// Bundle.Vision is non-nil.
func describerChoices(cfg *config.Config) []string {
	var out []string
	for _, name := range configuredAndRegistered(cfg) {
		if supportsVision(name) {
			out = append(out, name)
		}
	}
	return out
}

func supportsVision(name string) bool {
	b, ok := providers.Get(name)
	return ok && b.Vision != nil
}

func noProvidersError() error {
	return fmt.Errorf("no providers available. Add at least one with `imagine providers add <name>`")
}

func noDescribersError() error {
	return fmt.Errorf("no describe-capable providers configured. Add one with `imagine providers add <%s>`",
		strings.Join(providers.ProvidersSupportingDescribe(), "|"))
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

func noVisionError(name string, describers []string) error {
	if len(describers) == 0 {
		return fmt.Errorf("provider %q doesn't support vision, and no describe-capable provider is configured. Add one with `imagine providers add <%s>`",
			name, strings.Join(providers.ProvidersSupportingDescribe(), "|"))
	}
	return fmt.Errorf("provider %q doesn't support vision. Describe-capable providers: %v", name, describers)
}
