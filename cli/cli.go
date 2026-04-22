// Package cli holds the generation-run glue: the Options struct that cobra
// binds common flags onto, provider-agnostic validation, and the first-run
// key prompt. Flag parsing lives in commands/ (cobra + fang). Provider-
// specific validation (sizes, models, capability gating) lives in each
// provider's Info() and the root command's PreRunE.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/paths"
)

// Options holds the common CLI options cobra binds directly.
// Provider-private flags (grounding, thinking, quality, …) are NOT here —
// they're declared by each provider's BindFlags and harvested via ReadFlags.
type Options struct {
	Prompt           string
	Output           string
	OutputFilename   string
	NumImages        int
	AspectRatio     string
	ImageSize        string
	RefInputs        []string
	PreserveFilename bool
	Model            string // raw user input; PreRunE resolves aliases via the active provider
}

// Validate runs provider-agnostic checks:
//   - -p is required (reading from a file if the value points at a path)
//   - tilde expansion in -o, -i
//   - -n is in range
//   - -i paths exist and contain supported images
//   - -f and -r are mutually exclusive (cobra also enforces)
//   - -r requires exactly one -i pointing at a single file.
//
// Provider-specific validation (model aliases, allowed sizes, capability
// gating for grounding/thinking) runs in commands/root.go's PreRunE.
func (opts *Options) Validate() error {
	if opts.Prompt == "" {
		return fmt.Errorf("prompt is required (-p flag)")
	}

	// If -p points at a readable file, slurp the prompt from it.
	promptPath := paths.ExpandTilde(opts.Prompt)
	if info, err := os.Stat(promptPath); err == nil && !info.IsDir() {
		data, err := os.ReadFile(promptPath)
		if err != nil {
			return fmt.Errorf("failed to read prompt file: %v", err)
		}
		opts.Prompt = strings.TrimSpace(string(data))
		if opts.Prompt == "" {
			return fmt.Errorf("prompt file is empty: %s", promptPath)
		}
	}

	opts.Output = paths.ExpandTilde(opts.Output)
	for i, ref := range opts.RefInputs {
		opts.RefInputs[i] = paths.ExpandTilde(ref)
	}

	if opts.NumImages < 1 || opts.NumImages > 20 {
		return fmt.Errorf("number of images must be between 1 and 20")
	}

	for _, ref := range opts.RefInputs {
		info, err := os.Stat(ref)
		if os.IsNotExist(err) {
			return fmt.Errorf("reference path does not exist: %s", ref)
		}
		if err != nil {
			return fmt.Errorf("cannot access reference path: %v", err)
		}
		if info.IsDir() {
			count, _ := images.CountInDir(ref)
			if count == 0 {
				return fmt.Errorf("no images found in reference directory: %s", ref)
			}
		} else if !images.IsSupported(ref) {
			return fmt.Errorf("unsupported image format: %s", ref)
		}
	}

	// Belt-and-braces; cobra's MarkFlagsMutuallyExclusive catches this too.
	if opts.OutputFilename != "" && opts.PreserveFilename {
		return fmt.Errorf("-f and -r are mutually exclusive")
	}
	if opts.PreserveFilename {
		if len(opts.RefInputs) == 0 {
			return fmt.Errorf("-r flag requires -i with an input image file")
		}
		if len(opts.RefInputs) > 1 {
			return fmt.Errorf("-r flag only works with a single input file, not multiple")
		}
		info, _ := os.Stat(opts.RefInputs[0])
		if info != nil && info.IsDir() {
			return fmt.Errorf("-r flag only works with a single input file, not a folder")
		}
	}

	return nil
}
