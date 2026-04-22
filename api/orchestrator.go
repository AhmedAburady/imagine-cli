// Package api owns the orchestrator that runs N parallel generation requests,
// and the concrete provider clients (Gemini direct, Vertex). In Phase 4 the
// provider clients move into providers/ and this package owns just the
// orchestrator + shared types.
package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
)

// GenerationResult is the outcome of a single image request/save.
type GenerationResult struct {
	Index     int
	ImageData []byte
	Filename  string
	Error     error
}

// GenerationOutput wraps the full run: per-image results, the output folder,
// and wall-clock elapsed time.
type GenerationOutput struct {
	Results      []GenerationResult
	OutputFolder string
	Elapsed      time.Duration
}

// RunGeneration fans N requests out in parallel, saves each successful image
// to disk using ResolveFilename's precedence rules, and returns the collected
// results. Provider dispatch (Gemini direct vs Vertex) happens inline here —
// Phase 4 replaces the bool-branch with a Provider interface.
//
// ctx cancels in-flight HTTP (Ctrl+C via fang).
func RunGeneration(ctx context.Context, config *Config) GenerationOutput {
	startTime := time.Now()

	if err := os.MkdirAll(config.OutputFolder, 0755); err != nil {
		return GenerationOutput{
			Results: []GenerationResult{{
				Index: 0,
				Error: fmt.Errorf("failed to create output folder: %v", err),
			}},
			OutputFolder: config.OutputFolder,
			Elapsed:      time.Since(startTime),
		}
	}

	var wg sync.WaitGroup
	resultsChan := make(chan GenerationResult, config.NumImages)

	for i := 0; i < config.NumImages; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			var result GenerationResult
			if config.UseVertex {
				result = GenerateImageVertex(ctx, config, index)
			} else {
				result = GenerateImage(ctx, config, index)
			}

			if result.Error == nil && result.ImageData != nil {
				filename := images.ResolveFilename(images.FilenameParams{
					Custom:       config.OutputFilename,
					Preserve:     config.PreserveFilename,
					RefInputPath: config.RefInputPath,
					Index:        result.Index,
					Total:        config.NumImages,
				})

				imageData := result.ImageData
				if images.HasJPEGExt(filename) {
					converted, err := images.ConvertToJPEG(imageData)
					if err != nil {
						result.Error = fmt.Errorf("failed to convert to JPEG: %v", err)
						resultsChan <- result
						return
					}
					imageData = converted
				}

				outputFile := filepath.Join(config.OutputFolder, filename)
				if err := os.WriteFile(outputFile, imageData, 0644); err != nil {
					result.Error = fmt.Errorf("failed to save: %v", err)
				} else {
					result.Filename = filename
				}
			}

			resultsChan <- result
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []GenerationResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return GenerationOutput{
		Results:      results,
		OutputFolder: config.OutputFolder,
		Elapsed:      time.Since(startTime),
	}
}
