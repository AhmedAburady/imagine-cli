package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

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

// runGeneration wraps the orchestrator in a spinner and prints per-image results.
// Returns a non-nil error when any image fails or setup fails; fang uses the
// return value to decide the process exit code.
func runGeneration(ctx context.Context, provider providers.Provider, req providers.Request, params api.Params, opts *cli.Options, providerOpts map[string]any) error {
	modeText := "Generating"
	if len(opts.RefInputs) > 0 {
		modeText = "Editing"
	}
	modeText += fmt.Sprintf(" (%s", provider.Info().Name)
	if m, _ := providerOpts["model"].(string); m != "" && m != provider.Info().DefaultModel {
		modeText += ", " + m
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
