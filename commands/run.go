package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/batch"
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

// requestLabel extracts a short status-line label from opaque provider
// options. Prefers providers.RequestLabeler; falls back to the legacy
// map[string]any "model" key for providers still on the map interface.
// Returns "" when nothing usable is available.
func requestLabel(opts any) string {
	if l, ok := opts.(providers.RequestLabeler); ok {
		return l.RequestLabel()
	}
	if m, ok := opts.(map[string]any); ok {
		if s, _ := m["model"].(string); s != "" {
			return s
		}
	}
	return ""
}

// runBatch loads a batch file, resolves every entry against CLI defaults,
// and dispatches them in parallel. Validation is exhaustive — all errors
// across all entries surface in one error before any HTTP call.
func runBatch(cmd *cobra.Command, opts *cli.Options, providerName string) error {
	spec, err := batch.LoadFile(opts.Prompt)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	defaultProv, err := resolveDefaultProviderForBatch(providerName)
	if err != nil {
		return err
	}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      opts,
		Cmd:             cmd,
		Config:          cfg,
		DefaultProvider: defaultProv,
	})
	if err != nil {
		return err
	}
	return batch.Run(cmd.Context(), resolved)
}

// runGeneration wraps the orchestrator in a spinner and prints per-image results.
// Returns a non-nil error when any image fails or setup fails; fang uses the
// return value to decide the process exit code.
func runGeneration(ctx context.Context, provider providers.Provider, req providers.Request, params api.Params, opts *cli.Options, providerOpts any) error {
	modeText := "Generating"
	if len(opts.RefInputs) > 0 {
		modeText = "Editing"
	}
	modeText += fmt.Sprintf(" (%s", provider.Info().Name)
	if label := requestLabel(providerOpts); label != "" && label != provider.Info().DefaultModel {
		modeText += ", " + label
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
