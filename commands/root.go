// Package commands contains all imagine CLI commands.
//
// Root is the generate/edit command (bare `imagine -p "..."`). Only truly
// provider-agnostic flags live on the root (`-p`, `-o`, `-f`, `-n`, `-i`,
// `-r`, `--provider`). Everything else — model, size, aspect ratio,
// grounding, quality, … — is declared by each provider's BindFlags and
// interpreted by its ReadFlags. Flag names can overlap between providers;
// the *active* provider defines the semantics for the current invocation.
//
// Code organisation:
//
//	root.go      — command construction (this file)
//	resolve.go   — provider resolution (hint from args, default chain, order)
//	validate.go  — common-flag map + ownership gate
//	help.go      — provider-aware flag visibility + Examples builder
//	run.go       — orchestrator call + reference loading
//	describe.go  — describe subcommand shim
//	version.go   — version subcommand
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// NewRootCmd builds the root cobra command. activeHint is the best-effort
// provider name to use for help-output flag visibility; pre-parsed from argv
// + config in main() because fang renders help before PreRunE fires.
func NewRootCmd(version, activeHint string) *cobra.Command {
	opts := &cli.Options{}
	var providerName string
	// providerOptions is populated in PreRunE from the active provider's
	// ReadFlags call and consumed by RunE. Opaque at this layer — only the
	// provider's Generate type-asserts it.
	var providerOptions any

	longDesc := `imagine is a CLI for generating and editing images through multiple AI providers.

Run bare with -p "<prompt>" to generate. Add -i <reference> to switch to edit mode.
Configuration lives in ~/.config/imagine/config.yaml (see README for the schema).`

	root := &cobra.Command{
		Use:           "imagine",
		Short:         "Generate and edit images via Gemini, Vertex, or OpenAI",
		Long:          longDesc,
		Example:       buildExamples(activeHint),
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

			active, err := resolveProvider(providerName)
			if err != nil {
				return err
			}
			bundle, _ := providers.Get(active)

			if err := enforceFlagSupport(cmd, bundle); err != nil {
				return err
			}

			providerOptions, err = bundle.ReadFlags(cmd)
			if err != nil {
				return err
			}

			if err := enforceModelSupport(cmd, bundle, providerOptions); err != nil {
				return err
			}

			return opts.Validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			active, _ := resolveProvider(providerName)
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
				Prompt:     opts.Prompt,
				References: refs,
				Options:    providerOptions,
			}

			params := api.Params{
				OutputFolder:     opts.Output,
				OutputFilename:   opts.OutputFilename,
				NumImages:        opts.NumImages,
				PreserveFilename: opts.PreserveFilename,
				RefInputPath:     refInputPathFor(opts),
			}

			return runGeneration(cmd.Context(), provider, req, params, opts, providerOptions)
		},
	}
	root.SetVersionTemplate("imagine {{.Version}}\n")

	f := root.Flags()
	f.StringVarP(&opts.Prompt, "prompt", "p", "", "Prompt text or path to prompt file")
	f.StringVarP(&opts.Output, "output", "o", ".", "Output folder")
	f.StringVarP(&opts.OutputFilename, "filename", "f", "", "Output filename (e.g. image.png); with -n >1 suffixes as image_1.png, image_2.png, ...")
	f.IntVarP(&opts.NumImages, "count", "n", 1, "Number of images to generate (1-20)")
	f.StringSliceVarP(&opts.RefInputs, "input", "i", nil, "Reference image/folder, repeatable (enables edit mode)")
	f.BoolVarP(&opts.PreserveFilename, "replace", "r", false, "Replace: use input filename for output (single file only)")
	f.StringVar(&providerName, "provider", "", "Override the active provider (else: config default, else: first under providers:)")

	// Attach each registered provider's private flags. BindFlags is idempotent,
	// so whichever provider registers a shared flag name (-m, -s) first wins
	// the help description — register the active provider first so its help
	// text surfaces when the user inspects --help.
	for _, name := range providerOrder(activeHint) {
		if b, ok := providers.Get(name); ok && b.BindFlags != nil {
			b.BindFlags(root)
		}
	}

	root.MarkFlagsMutuallyExclusive("filename", "replace")

	// Hide the other providers' flags so --help focuses on the active one.
	applyProviderFlagVisibility(root, activeHint)

	root.AddCommand(
		newDescribeCmd(),
		newVersionCmd(version),
		newProvidersCmd(),
	)

	return root
}
