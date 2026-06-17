// Package api owns the orchestrator: it takes a resolved Provider, a Request,
// and orchestration-only parameters (output folder, filename rules, total
// count), fans out provider calls in parallel while respecting MaxBatchN,
// writes each image to disk, and returns a per-image result summary.
package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// Params holds orchestration-only settings — things no provider needs to know.
type Params struct {
	OutputFolder     string
	OutputFilename   string // -f
	NumImages        int    // total, across all batches
	PreserveFilename bool   // -r
	RefInputPath     string // original -i path, used by -r

	// MaxParallel caps in-flight provider calls within this run. 0
	// (default) means unlimited — every batch fires immediately, the
	// pre-existing behaviour. Ignored when Sem is non-nil.
	MaxParallel int

	// Sem, when non-nil, is a shared concurrency semaphore — the batch
	// runner creates one and passes the same channel into every entry's
	// orchestrator call so a single --max-parallel cap covers both the
	// per-entry and per-image axes. Single-shot leaves this nil and the
	// orchestrator builds a private sem from MaxParallel.
	Sem chan struct{}

	// Progress, when non-nil, receives each result in completion order and is closed when the run finishes.
	// Sends are synchronous: it MUST be buffered to >= NumImages or drained concurrently, else RunGeneration blocks.
	Progress chan<- GenerationResult

	// Metadata, when non-empty, is embedded into PNG output (--embed-metadata); a no-op for other formats.
	Metadata []images.TextTag
}

// GenerationResult is the outcome of a single image request/save.
type GenerationResult struct {
	Index     int
	ImageData []byte
	Filename  string
	Error     error
	Duration  time.Duration // wall time of the provider call that produced this image

	metadataEmbedded bool // whether saveOne actually embedded requested metadata
}

// GenerationOutput wraps the full run.
type GenerationOutput struct {
	Results      []GenerationResult
	OutputFolder string
	Elapsed      time.Duration

	// MetadataSkipped is true when Metadata was requested but a saved image wasn't PNG, so nothing was embedded.
	MetadataSkipped bool
}

// RunGeneration dispatches NumImages through the given Provider, batching at
// Info().Capabilities.MaxBatchN. Each batch runs in its own goroutine; each
// successful image is saved to disk using ResolveFilename's precedence rules.
// ctx cancels in-flight HTTP (Ctrl+C via fang).
func RunGeneration(ctx context.Context, provider providers.Provider, request providers.Request, params Params) GenerationOutput {
	startTime := time.Now()

	if params.Progress != nil {
		defer close(params.Progress)
	}

	if err := os.MkdirAll(params.OutputFolder, 0755); err != nil {
		return GenerationOutput{
			Results: []GenerationResult{{
				Index: 0,
				Error: fmt.Errorf("failed to create output folder: %v", err),
			}},
			OutputFolder: params.OutputFolder,
			Elapsed:      time.Since(startTime),
		}
	}

	// Plan batches: for providers with MaxBatchN=1 (Gemini/Vertex), this
	// yields NumImages batches of size 1. For MaxBatchN=10 (OpenAI), fewer
	// bigger batches.
	maxBatch := max(provider.Info().Capabilities.MaxBatchN, 1)
	var batchSizes []int
	remaining := params.NumImages
	for remaining > 0 {
		size := min(remaining, maxBatch)
		batchSizes = append(batchSizes, size)
		remaining -= size
	}

	var wg sync.WaitGroup
	resultsChan := make(chan GenerationResult, params.NumImages)

	// Sliding-window concurrency cap. Caller-supplied sem wins (batch
	// mode shares one across all entries); else build a private one
	// from MaxParallel. nil = unlimited, the original behaviour.
	sem := params.Sem
	if sem == nil && params.MaxParallel > 0 {
		sem = make(chan struct{}, params.MaxParallel)
	}

	globalIndex := 0
	for _, size := range batchSizes {
		startIndex := globalIndex
		globalIndex += size

		batchReq := request
		batchReq.N = size

		wg.Add(1)
		go func(startIndex, batchSize int, req providers.Request) {
			defer wg.Done()
			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					for i := range batchSize {
						resultsChan <- GenerationResult{Index: startIndex + i, Error: ctx.Err()}
					}
					return
				}
			}

			t0 := time.Now()
			resp, err := provider.Generate(ctx, req)
			dur := time.Since(t0)
			if err != nil {
				for i := range batchSize {
					resultsChan <- GenerationResult{Index: startIndex + i, Error: err, Duration: dur}
				}
				return
			}

			for i, asset := range resp.Assets {
				if i >= batchSize {
					// Provider returned more images than requested; ignore extras.
					break
				}
				res := GenerationResult{Index: startIndex + i, ImageData: asset.Data, Duration: dur}
				saveOne(&res, asset, params)
				resultsChan <- res
			}

			// If the provider returned fewer images than requested, fill the gap
			// so the per-image error surfaces to the user.
			for i := len(resp.Assets); i < batchSize; i++ {
				resultsChan <- GenerationResult{
					Index:    startIndex + i,
					Error:    fmt.Errorf("provider returned only %d of %d requested images", len(resp.Assets), batchSize),
					Duration: dur,
				}
			}
		}(startIndex, size, batchReq)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []GenerationResult
	metadataSkipped := false
	for r := range resultsChan {
		results = append(results, r)
		if len(params.Metadata) > 0 && r.Error == nil && !r.metadataEmbedded {
			metadataSkipped = true // metadata requested but EmbedPNGText no-op'd (non-PNG)
		}
		if params.Progress != nil {
			params.Progress <- r
		}
	}

	return GenerationOutput{
		Results:         results,
		OutputFolder:    params.OutputFolder,
		MetadataSkipped: metadataSkipped,
		Elapsed:         time.Since(startTime),
	}
}

// saveOne resolves the output filename (honouring -f, -r, and default rules),
// converts to JPEG when the extension requests it, and writes the file.
// Mutates res.Filename on success or res.Error on failure.
func saveOne(res *GenerationResult, asset providers.GeneratedAsset, params Params) {
	data := asset.Data
	filename := images.ResolveFilename(images.FilenameParams{
		Custom:       params.OutputFilename,
		Preserve:     params.PreserveFilename,
		RefInputPath: params.RefInputPath,
		Index:        res.Index,
		Total:        params.NumImages,
		AssetMime:    asset.MimeType,
	})

	// Post-processing keys off byte-level signals, not the asset's MIME label:
	// JPEG conversion runs only when the resolved filename is .jpg (video never
	// resolves to .jpg), and EmbedPNGText sniffs the PNG signature and no-ops on
	// non-PNG. So video/audio bytes pass through verbatim with no MIME gate.
	if images.HasJPEGExt(filename) {
		converted, err := images.ConvertToJPEG(data)
		if err != nil {
			res.Error = fmt.Errorf("failed to convert to JPEG: %v", err)
			return
		}
		data = converted
	}

	embedded := false
	if len(params.Metadata) > 0 {
		data, embedded = images.EmbedPNGText(data, params.Metadata) // no-op for non-PNG
	}

	outputFile := filepath.Join(params.OutputFolder, filename)
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		res.Error = fmt.Errorf("failed to save: %v", err)
		return
	}
	res.Filename = filename
	res.ImageData = data
	res.metadataEmbedded = embedded
}
