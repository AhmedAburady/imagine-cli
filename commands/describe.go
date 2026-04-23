package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

func newDescribeCmd() *cobra.Command {
	var (
		input            string
		output           string
		customPrompt     string
		additional       string
		model            string
		providerName     string
		jsonOutput       bool
		showInstructions bool
	)
	cmd := &cobra.Command{
		Use:           "describe",
		Short:         "Analyse images and produce a style description",
		Long:          "Analyse images with a vision model and return a text or structured-JSON description usable as a generation prompt.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showInstructions {
				return runShowInstructions(cmd, providerName)
			}
			// Bare invocation → show help and exit cleanly, same pattern
			// as bare `imagine`.
			if input == "" {
				return cmd.Help()
			}
			refs, err := images.Load(input)
			if err != nil {
				return fmt.Errorf("load references: %w", err)
			}
			if len(refs) == 0 {
				return errors.New("no readable images at the provided input path")
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			active, err := resolveDescriber(providerName, cfg)
			if err != nil {
				return err
			}

			auth := providers.Auth(cfg.Providers[active])
			bundle, _ := providers.Get(active)
			p, err := bundle.Factory(auth)
			if err != nil {
				return err
			}
			desc, ok := p.(providers.Describer)
			if !ok {
				return fmt.Errorf("provider %q doesn't support vision", active)
			}

			s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
			s.Suffix = fmt.Sprintf(" Analysing with %s...", active)
			_ = s.Color("magenta")
			s.Start()

			result, err := desc.Describe(cmd.Context(), providers.DescribeRequest{
				Images:           refs,
				CustomPrompt:     customPrompt,
				Additional:       additional,
				Model:            model,
				StructuredOutput: jsonOutput,
			})
			s.Stop()
			if err != nil {
				return err
			}

			rendered := renderDescription(result, jsonOutput)
			if output == "" {
				fmt.Fprintln(cmd.OutOrStdout(), rendered)
				return nil
			}
			if err := os.WriteFile(output, []byte(rendered+"\n"), 0o644); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  description saved to %s\n",
				successStyle.Render("✓"), boldStyle.Render(output))
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&input, "input", "i", "", "Image file or folder (required)")
	f.StringVarP(&output, "output", "o", "", "Output file path (default: stdout)")
	f.StringVarP(&customPrompt, "prompt", "p", "", "Custom instruction, replaces the default")
	f.StringVarP(&additional, "additional", "a", "", "Additional context, prepended to the default instruction")
	f.StringVarP(&model, "model", "m", "", "Override the provider's vision model for this invocation")
	f.StringVar(&providerName, "provider", "", "Override the describer provider for this invocation")
	f.BoolVar(&jsonOutput, "json", false, "Emit structured JSON (StyleAnalysis schema) instead of prose")
	f.BoolVar(&showInstructions, "show-instructions", false, "Print the built-in describe prompts for the active provider and exit")

	return cmd
}

func runShowInstructions(cmd *cobra.Command, providerName string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	active, err := resolveDescriber(providerName, cfg)
	if err != nil {
		return err
	}
	bundle, _ := providers.Get(active)
	p, err := bundle.Factory(providers.Auth(cfg.Providers[active]))
	if err != nil {
		return err
	}
	explainer, ok := p.(interface{ DefaultInstructions() (string, string) })
	if !ok {
		return fmt.Errorf("provider %q doesn't expose default instructions", active)
	}
	text, jsonOut := explainer.DefaultInstructions()

	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, titleStyle.Render(fmt.Sprintf("DESCRIBE INSTRUCTIONS · %s", active)))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s\n", dimStyle.Render("─── text mode (default) ───"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, indent(text, "  "))
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s\n", dimStyle.Render("─── --json mode ───"))
	fmt.Fprintln(out)
	fmt.Fprintln(out, indent(jsonOut, "  "))
	fmt.Fprintln(out)
	return nil
}

func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

// resolveDescriber picks the describer provider by precedence:
//
//	--provider X  →  X (must implement Describer)
//	vision_default_provider in config
//	default_provider in config
//	first describer-capable provider configured
func resolveDescriber(flagValue string, cfg *config.Config) (string, error) {
	describers := describerChoices(cfg)
	if len(describers) == 0 {
		return "", noDescribersError()
	}

	if flagValue != "" {
		if _, ok := providers.Get(flagValue); !ok {
			return "", fmt.Errorf("unknown provider %q (available: %v)", flagValue, providers.List())
		}
		if !supportsVision(flagValue) {
			return "", fmt.Errorf("provider %q doesn't support vision. describe-capable providers: %v", flagValue, describers)
		}
		if _, configured := cfg.Providers[flagValue]; !configured {
			return "", fmt.Errorf("provider %q isn't configured. add it with `imagine providers add %s`", flagValue, flagValue)
		}
		return flagValue, nil
	}

	for _, candidate := range []string{cfg.VisionDefaultProvider, cfg.DefaultProvider} {
		if candidate == "" {
			continue
		}
		if supportsVision(candidate) {
			if _, configured := cfg.Providers[candidate]; configured {
				return candidate, nil
			}
		}
	}
	return describers[0], nil
}

func renderDescription(d *providers.ImageDescription, jsonOut bool) string {
	if jsonOut && d.Structured != nil {
		out, _ := json.MarshalIndent(d.Structured, "", "  ")
		return string(out)
	}
	return d.Text
}
