// Package cli holds common-flag glue: the Options struct cobra binds the
// truly provider-agnostic flags onto, and provider-agnostic validation.
// Provider-specific flags (model, size, aspect ratio, quality, …) live
// inside each provider's BindFlags/ReadFlags.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/paths"
)

// Options holds the truly common CLI flags — same meaning for every
// provider. Everything provider-specific lives inside each provider's
// bundle and ends up in Request.Options.
type Options struct {
	Prompt           string
	Output           string
	OutputFilename   string
	NumImages        int
	RefInputs        []string
	PreserveFilename bool
}

// Validate runs provider-agnostic checks:
//   - -p is required (reading from a file if the value points at a path)
//   - tilde expansion in -o, -i
//   - -n is in range
//   - -i paths exist and contain supported images
//   - -f and -r are mutually exclusive (cobra also enforces)
//   - -r requires exactly one -i pointing at a single file.
func (opts *Options) Validate() error {
	if opts.Prompt == "" {
		return fmt.Errorf("prompt is required (-p flag)")
	}

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
