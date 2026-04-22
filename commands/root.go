// Package commands contains all imagine CLI commands.
//
// Root is the generate/edit command (bare `imagine -p "..."`). Providers
// (gemini, vertex, openai) register at init-time; the root command attaches
// every registered provider's private flags up front, then resolves the
// active provider in PreRunE and enforces capability/ownership rules.
package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

func sortedKeys(m map[string]config.ProviderConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// commonFlags is the set of flag names that are NOT provider-private. Used by
// the ownership check in PreRunE to skip common flags.
var commonFlags = map[string]bool{
	"prompt":       true,
	"output":       true,
	"filename":     true,
	"count":        true,
	"aspect-ratio": true,
	"size":         true,
	"input":        true,
	"replace":      true,
	"model":        true,
	"provider":     true,
	"help":         true,
	"version":      true,
}

// NewRootCmd builds the root cobra command. Generation runs as the root's
// RunE — `imagine -p "..."` generates directly; subcommands cover config,
// describe, and version.
func NewRootCmd(version string) *cobra.Command {
	opts := &cli.Options{}
	var providerName string

	root := &cobra.Command{
		Use:   "imagine",
		Short: "Generate and edit images via Gemini, Vertex, or OpenAI",
		Long: `imagine is a CLI for generating and editing images through multiple AI providers.

Run bare with -p "<prompt>" to generate. Add -i <reference> to switch to edit mode.
Configuration lives in ~/.config/imagine/config.yaml (see README for the schema).`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Positional args carry shell-glob residuals for -i *.png.
			opts.RefInputs = append(opts.RefInputs, args...)

			// Bare invocation with no prompt → help, not error.
			if opts.Prompt == "" {
				return cmd.Help()
			}

			active, err := resolveProvider(cmd, providerName)
			if err != nil {
				return err
			}
			bundle, _ := providers.Get(active)

			// Reject user-set flags the active provider doesn't honour.
			if err := enforceFlagSupport(cmd, bundle); err != nil {
				return err
			}

			// Resolve model alias via the active provider's Info.
			if err := resolveModel(opts, bundle.Info); err != nil {
				return err
			}

			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bare-invocation case already returned in PreRunE; if we got here,
			// opts.Prompt is set.
			active, _ := resolveProvider(cmd, providerName)
			bundle, _ := providers.Get(active)

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			auth := providers.Auth{
				APIKey:  cfg.ProviderAPIKey(active),
				Options: cfg.Providers[active].ProviderOptions,
			}

			provider, err := bundle.Factory(auth)
			if err != nil {
				return err
			}

			refs, err := loadReferences(opts.RefInputs)
			if err != nil {
				return err
			}

			req := providers.Request{
				Prompt:      opts.Prompt,
				Model:       opts.Model, // already resolved to canonical ID in PreRunE
				Size:        opts.ImageSize,
				AspectRatio: opts.AspectRatio,
				References:  refs,
				Options:     bundle.ReadFlags(cmd),
			}

			params := api.Params{
				OutputFolder:     opts.Output,
				OutputFilename:   opts.OutputFilename,
				NumImages:        opts.NumImages,
				PreserveFilename: opts.PreserveFilename,
				RefInputPath:     refInputPathFor(opts),
			}

			return runGeneration(cmd.Context(), provider, req, params, opts)
		},
	}
	root.SetVersionTemplate("imagine {{.Version}}\n")

	f := root.Flags()
	f.StringVarP(&opts.Prompt, "prompt", "p", "", "Prompt text or path to prompt file")
	f.StringVarP(&opts.Output, "output", "o", ".", "Output folder")
	f.StringVarP(&opts.OutputFilename, "filename", "f", "", "Output filename (e.g. image.png); with -n >1 suffixes as image_1.png, image_2.png, ...")
	f.IntVarP(&opts.NumImages, "count", "n", 1, "Number of images to generate (1-20)")
	f.StringVarP(&opts.AspectRatio, "aspect-ratio", "a", "", "Aspect ratio (default: Auto)")
	f.StringVarP(&opts.ImageSize, "size", "s", "1K", "Image size (provider-specific; Gemini/Vertex: 1K, 2K, 4K)")
	f.StringSliceVarP(&opts.RefInputs, "input", "i", nil, "Reference image/folder, repeatable (enables edit mode)")
	f.BoolVarP(&opts.PreserveFilename, "replace", "r", false, "Replace: use input filename for output (single file only)")
	f.StringVarP(&opts.Model, "model", "m", "", "Model (provider-specific). Omit to use the provider's default.")
	f.StringVar(&providerName, "provider", "", "Override the active provider (else: config default, else: first under providers:)")

	// Attach each registered provider's private flags. BindFlags is idempotent:
	// providers that share flags (gemini/vertex) attach the same flag once.
	for _, name := range providers.List() {
		if b, ok := providers.Get(name); ok && b.BindFlags != nil {
			b.BindFlags(root)
		}
	}

	root.MarkFlagsMutuallyExclusive("filename", "replace")

	root.AddCommand(
		newDescribeCmd(),
		newVersionCmd(version),
	)

	return root
}

// resolveProvider returns the active provider name with precedence:
//
//	--provider flag → config.default_provider → first under providers: → error
func resolveProvider(cmd *cobra.Command, flagValue string) (string, error) {
	name := flagValue
	if name == "" {
		name = config.GetDefaultProvider()
	}
	if name == "" {
		// Fall back to the first provider configured under `providers:` in config.
		cfg, err := config.Load()
		if err == nil {
			for _, candidate := range sortedKeys(cfg.Providers) {
				if _, ok := providers.Get(candidate); ok {
					name = candidate
					break
				}
			}
		}
	}
	if name == "" {
		return "", fmt.Errorf("no provider configured. Create %s with a providers: entry (see README for schema)", config.DefaultConfigPath())
	}
	if _, ok := providers.Get(name); !ok {
		return "", fmt.Errorf("unknown provider %q (available: %v)", name, providers.List())
	}
	return name, nil
}

// enforceFlagSupport rejects provider-private flags the user set explicitly
// but that the active provider doesn't support. Common flags (prompt,
// output, etc.) are always allowed.
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
			// Unknown flag (shouldn't happen; cobra would've rejected earlier).
			return
		}
		rejected = fmt.Errorf("--%s is not supported by provider %q (supported by: %v)", fl.Name, active.Info.Name, others)
	})
	return rejected
}

// resolveModel translates user input ("pro", "flash", or a full ID) into the
// active provider's canonical model ID. Empty input falls back to the
// provider's default model.
func resolveModel(opts *cli.Options, info providers.Info) error {
	raw := strings.TrimSpace(opts.Model)
	if raw == "" {
		opts.Model = info.DefaultModel
		return nil
	}

	for _, m := range info.Models {
		if m.ID == raw {
			opts.Model = m.ID
			return nil
		}
		for _, alias := range m.Aliases {
			if alias == raw {
				opts.Model = m.ID
				return nil
			}
		}
	}

	// Build a helpful error with accepted aliases + IDs.
	var accepted []string
	for _, m := range info.Models {
		accepted = append(accepted, m.ID)
		accepted = append(accepted, m.Aliases...)
	}
	return fmt.Errorf("unknown model %q for provider %q (accepted: %v)", raw, info.Name, accepted)
}

// loadReferences walks the user-provided -i paths and returns the full set of
// reference images. Errors out on the first unreadable or unsupported entry.
func loadReferences(refInputs []string) ([]images.Reference, error) {
	var refs []images.Reference
	for _, ref := range refInputs {
		loaded, err := images.Load(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to load references: %w", err)
		}
		refs = append(refs, loaded...)
	}
	return refs, nil
}

// refInputPathFor returns the original input path to feed ResolveFilename's
// -r rule. Only non-empty when exactly one -i was provided.
func refInputPathFor(opts *cli.Options) string {
	if len(opts.RefInputs) == 1 {
		return opts.RefInputs[0]
	}
	return ""
}

// runGeneration calls the orchestrator, wraps it in a spinner, and prints the
// per-image summary.
func runGeneration(ctx context.Context, provider providers.Provider, req providers.Request, params api.Params, opts *cli.Options) error {
	modeText := "Generating"
	if len(opts.RefInputs) > 0 {
		modeText = "Editing"
	}
	modeText += fmt.Sprintf(" (%s", provider.Info().Name)
	if opts.Model != "" && opts.Model != provider.Info().DefaultModel {
		modeText += ", " + opts.Model
	}
	modeText += ")"

	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s %d image(s)...", modeText, params.NumImages)
	_ = s.Color("magenta")
	s.Start()

	output := api.RunGeneration(ctx, provider, req, params)
	s.Stop()

	fmt.Println()
	successCount := 0
	errorCount := 0
	for _, r := range output.Results {
		if r.Error != nil {
			fmt.Printf("\033[31m✗\033[0m Image %d: %v\n", r.Index+1, r.Error)
			errorCount++
		} else {
			fmt.Printf("\033[32m✓\033[0m %s\n", r.Filename)
			successCount++
		}
	}

	fmt.Println()
	fmt.Printf("Done: %d success, %d failed (%.1fs)\n", successCount, errorCount, output.Elapsed.Seconds())

	outputPath := params.OutputFolder
	if !filepath.IsAbs(outputPath) {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
	}
	fmt.Printf("Output: %s\n", outputPath)

	if errorCount > 0 {
		return fmt.Errorf("%d image(s) failed", errorCount)
	}
	_ = os.Stdout.Sync()
	return nil
}
