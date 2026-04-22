// Package commands contains all imagine CLI commands.
package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
)

// NewRootCmd builds the root cobra command. Generation runs as the root's
// RunE — `imagine -p "..."` generates directly; subcommands cover config,
// describe, and version. Presence of `-i` on the root flips the generation
// path into edit mode (same contract as banana-cli).
func NewRootCmd(version string) *cobra.Command {
	opts := &cli.Options{}

	root := &cobra.Command{
		Use:   "imagine",
		Short: "Generate and edit images via Gemini, Vertex, or OpenAI",
		Long: `imagine is a CLI for generating and editing images through multiple AI providers.

Run bare with -p "<prompt>" to generate. Add -i <reference> to switch to edit mode.
Config, describe, and version live as subcommands.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Positional args carry shell-glob residuals: when users type
		// `-i *.png`, the shell expands — the first file lands in opts.RefInputs
		// via the flag, and the remainder lands in args. Preserve that.
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.RefInputs = append(opts.RefInputs, args...)

			// Bare invocation with no prompt → print help, don't error.
			if opts.Prompt == "" {
				return cmd.Help()
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			apiKey := config.GetAPIKey()
			if apiKey == "" {
				apiKey = cli.PromptForAPIKey()
			}
			return cli.Run(cmd.Context(), opts, apiKey)
		},
	}
	// `-v` and `--version` print just the raw version (no "imagine version" prefix).
	root.SetVersionTemplate("imagine {{.Version}}\n")

	flags := root.Flags()
	flags.StringVarP(&opts.Prompt, "prompt", "p", "", "Prompt text or path to prompt file")
	flags.StringVarP(&opts.Output, "output", "o", ".", "Output folder")
	flags.StringVarP(&opts.OutputFilename, "filename", "f", "", "Output filename (e.g. image.png); with -n >1 suffixes as image_1.png, image_2.png, ...")
	flags.IntVarP(&opts.NumImages, "count", "n", 1, "Number of images to generate (1-20)")
	flags.StringVar(&opts.AspectRatio, "aspect-ratio", "", "Aspect ratio (default: Auto)")
	flags.StringVarP(&opts.ImageSize, "size", "s", "1K", "Image size: 1K, 2K, 4K")
	flags.BoolVarP(&opts.Grounding, "grounding", "g", false, "Enable grounding with Google Search (Gemini)")
	flags.StringSliceVarP(&opts.RefInputs, "input", "i", nil, "Reference image/folder, repeatable (enables edit mode)")
	flags.BoolVarP(&opts.PreserveFilename, "replace", "r", false, "Replace: use input filename for output (single file only)")
	flags.BoolVar(&opts.UseVertex, "vertex", false, "Use Vertex AI instead of Gemini API (requires gcloud auth)")
	flags.StringVarP(&opts.Model, "model", "m", "pro", "Model: pro, flash")
	flags.StringVarP(&opts.ThinkingLevel, "thinking", "t", "minimal", "Thinking level: minimal, high (flash only)")
	flags.BoolVar(&opts.ImageSearch, "image-search", false, "Enable Image Search grounding (flash only)")

	root.MarkFlagsMutuallyExclusive("filename", "replace")

	root.AddCommand(
		newConfigCmd(),
		newDescribeCmd(),
		newVersionCmd(version),
	)

	// Suppress cobra's default error wording for unknown flag; pass through cleanly.
	root.SetErrPrefix(redBold("Error:"))

	return root
}

func redBold(s string) string {
	return fmt.Sprintf("\033[1;31m%s\033[0m", s)
}
