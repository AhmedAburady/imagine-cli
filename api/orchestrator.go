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
}

// GenerationResult is the outcome of a single image request/save.
type GenerationResult struct {
	Index     int
	ImageData []byte
	Filename  string
	Error     error
}

// GenerationOutput wraps the full run.
type GenerationOutput struct {
	Results      []GenerationResult
	OutputFolder string
	Elapsed      time.Duration
}

// RunGeneration dispatches NumImages through the given Provider, batching at
// Info().Capabilities.MaxBatchN. Each batch runs in its own goroutine; each
// successful image is saved to disk using ResolveFilename's precedence rules.
// ctx cancels in-flight HTTP (Ctrl+C via fang).
func RunGeneration(ctx context.Context, provider providers.Provider, request providers.Request, params Params) GenerationOutput {
	startTime := time.Now()

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

	globalIndex := 0
	for _, size := range batchSizes {
		startIndex := globalIndex
		globalIndex += size

		batchReq := request
		batchReq.N = size

		wg.Add(1)
		go func(startIndex, batchSize int, req providers.Request) {
			defer wg.Done()

			resp, err := provider.Generate(ctx, req)
			if err != nil {
				for i := range batchSize {
					resultsChan <- GenerationResult{Index: startIndex + i, Error: err}
				}
				return
			}

			for i, img := range resp.Images {
				if i >= batchSize {
					// Provider returned more images than requested; ignore extras.
					break
				}
				res := GenerationResult{Index: startIndex + i, ImageData: img.Data}
				saveOne(&res, img.Data, params)
				resultsChan <- res
			}

			// If the provider returned fewer images than requested, fill the gap
			// so the per-image error surfaces to the user.
			for i := len(resp.Images); i < batchSize; i++ {
				resultsChan <- GenerationResult{
					Index: startIndex + i,
					Error: fmt.Errorf("provider returned only %d of %d requested images", len(resp.Images), batchSize),
				}
			}
		}(startIndex, size, batchReq)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []GenerationResult
	for r := range resultsChan {
		results = append(results, r)
	}

	return GenerationOutput{
		Results:      results,
		OutputFolder: params.OutputFolder,
		Elapsed:      time.Since(startTime),
	}
}

// saveOne resolves the output filename (honouring -f, -r, and default rules),
// converts to JPEG when the extension requests it, and writes the file.
// Mutates res.Filename on success or res.Error on failure.
func saveOne(res *GenerationResult, data []byte, params Params) {
	filename := images.ResolveFilename(images.FilenameParams{
		Custom:       params.OutputFilename,
		Preserve:     params.PreserveFilename,
		RefInputPath: params.RefInputPath,
		Index:        res.Index,
		Total:        params.NumImages,
	})

	if images.HasJPEGExt(filename) {
		converted, err := images.ConvertToJPEG(data)
		if err != nil {
			res.Error = fmt.Errorf("failed to convert to JPEG: %v", err)
			return
		}
		data = converted
	}

	outputFile := filepath.Join(params.OutputFolder, filename)
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		res.Error = fmt.Errorf("failed to save: %v", err)
		return
	}
	res.Filename = filename
	res.ImageData = data
}
