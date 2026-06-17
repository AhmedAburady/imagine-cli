package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
func loadReferences(refInputs []string, accept []string) ([]images.Reference, error) {
	var refs []images.Reference
	for _, ref := range refInputs {
		loaded, err := images.Load(ref, accept...)
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
	defaultProv, err := resolveDefaultProviderForBatch(providerName, cfg)
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
	return batch.Run(cmd.Context(), resolved, opts.MaxParallel)
}

// runGeneration wraps the orchestrator in a spinner and prints per-image results.
// Returns a non-nil error when any image fails or setup fails; fang uses the
// return value to decide the process exit code.
func runGeneration(ctx context.Context, provider providers.Provider, req providers.Request, params api.Params, opts *cli.Options, providerOpts any) error {
	isVideo := provider.Info().Capabilities.MediaKind == providers.KindVideo
	// noun labels the produced asset in user-facing output (table, failures).
	noun := "image"
	if isVideo {
		noun = "video"
	}
	modeText := "Generating"
	if len(opts.RefInputs) > 0 {
		modeText = "Editing"
	}
	if isVideo {
		modeText += " video"
	}
	modeText += fmt.Sprintf(" (%s", provider.Info().Name)
	if label := requestLabel(providerOpts); label != "" && label != provider.Info().DefaultModel {
		modeText += ", " + label
	}
	modeText += ")"

	model := requestLabel(providerOpts)
	if model == "" {
		model = provider.Info().DefaultModel
	}

	// --embed-metadata is image-specific (PNG text chunks); skip it for video.
	if opts.EmbedMetadata && !isVideo {
		params.Metadata = images.MetadataTags(req.Prompt, model, provider.Info().Name, opts.RefInputs)
	}

	output, aborted := runWithProgress(ctx, modeText, provider, req, &params)

	successCount := 0
	errorCount := 0
	for _, r := range output.Results {
		if r.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	outputPath := params.OutputFolder
	if !filepath.IsAbs(outputPath) {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
	}

	fmt.Println()
	if aborted {
		fmt.Println(abortBlock(successCount, params.NumImages, fmt.Sprintf("%.1fs", output.Elapsed.Seconds()), outputPath))
		_ = os.Stdout.Sync()
		os.Exit(130) // SIGINT convention: graceful summary, but non-zero for scripts/CI
	}

	if output.MetadataSkipped && !isVideo {
		fmt.Println(warnLine("--embed-metadata supports PNG only; metadata was not embedded for this output format"))
	}
	fmt.Println(resultsTable(model, noun, output.Results, outputPath))

	if errorCount > 0 {
		return fmt.Errorf("%d %s(s) failed", errorCount, noun)
	}
	_ = os.Stdout.Sync()
	return nil
}
