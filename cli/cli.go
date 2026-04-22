// Package cli holds the generation-run glue: option struct, validation,
// the runtime runner (spinner + summary), and the first-run key prompt.
// Flag parsing, help text, and subcommand dispatch live in the commands
// package (cobra + fang).
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/paths"
)

// Options holds the parsed CLI options for the generate/edit flow. Cobra binds
// user flags directly onto the fields here in commands/root.go.
type Options struct {
	Prompt           string
	Output           string
	OutputFilename   string
	NumImages        int
	AspectRatio      string
	ImageSize        string
	Grounding        bool
	RefInputs        []string
	PreserveFilename bool
	UseVertex        bool
	Model            string
	ThinkingLevel    string
	ImageSearch      bool
}

var (
	validAspectRatios = map[string]bool{
		"":     true, // Auto (not included in request)
		"1:1":  true,
		"16:9": true,
		"9:16": true,
		"4:3":  true,
		"3:4":  true,
		"2:3":  true,
		"3:2":  true,
		"5:4":  true,
		"4:5":  true,
		"21:9": true,
	}
	validImageSizes = map[string]bool{
		"1K": true, "2K": true, "4K": true,
	}
	validModels = map[string]bool{
		"pro": true, "flash": true,
	}
	validThinkingLevels = map[string]bool{
		"minimal": true, "high": true,
	}
)

// Validate validates the CLI options.
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
	if !validAspectRatios[opts.AspectRatio] {
		return fmt.Errorf("invalid aspect ratio: %s", opts.AspectRatio)
	}
	if !validImageSizes[opts.ImageSize] {
		return fmt.Errorf("invalid image size: %s (valid: 1K, 2K, 4K)", opts.ImageSize)
	}
	if !validModels[opts.Model] {
		return fmt.Errorf("invalid model: %s (valid: pro, flash)", opts.Model)
	}
	if !validThinkingLevels[opts.ThinkingLevel] {
		return fmt.Errorf("invalid thinking level: %s (valid: minimal, high)", opts.ThinkingLevel)
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

	// -f and -r are mutually exclusive at the cobra layer too; keep this belt-and-braces.
	if opts.OutputFilename != "" && opts.PreserveFilename {
		return fmt.Errorf("-f and -r are mutually exclusive: use one or the other")
	}

	// -r requires exactly one -i pointing at a single file.
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

// Run executes the generate/edit flow end-to-end: load references, run the
// parallel generation, print per-image results and a summary. Returns a
// non-nil error when any image fails or setup fails; cobra/fang handles exit.
func Run(ctx context.Context, opts *Options, apiKey string) error {
	var refImages []images.Reference
	for _, ref := range opts.RefInputs {
		refs, err := images.Load(ref)
		if err != nil {
			return fmt.Errorf("failed to load references: %w", err)
		}
		refImages = append(refImages, refs...)
	}

	modelName := api.ModelPro
	if opts.Model == "flash" {
		modelName = api.ModelFlash
	}

	thinkingLevel := ""
	if opts.Model == "flash" {
		thinkingLevel = strings.ToUpper(opts.ThinkingLevel)
	}

	refInputPath := ""
	if len(opts.RefInputs) == 1 {
		refInputPath = opts.RefInputs[0]
	}

	cfg := &api.Config{
		OutputFolder:     opts.Output,
		OutputFilename:   opts.OutputFilename,
		NumImages:        opts.NumImages,
		Prompt:           opts.Prompt,
		APIKey:           apiKey,
		AspectRatio:      opts.AspectRatio,
		ImageSize:        opts.ImageSize,
		Grounding:        opts.Grounding,
		RefImages:        refImages,
		RefInputPath:     refInputPath,
		PreserveFilename: opts.PreserveFilename,
		UseVertex:        opts.UseVertex,
		Model:            modelName,
		ThinkingLevel:    thinkingLevel,
		ImageSearch:      opts.ImageSearch,
	}

	if err := os.MkdirAll(cfg.OutputFolder, 0755); err != nil {
		return fmt.Errorf("failed to create output folder: %w", err)
	}

	modeText := "Generating"
	if len(opts.RefInputs) > 0 {
		modeText = "Editing"
	}
	modeText += fmt.Sprintf(" (%s", opts.Model)
	if opts.UseVertex {
		modeText += ", Vertex AI"
	}
	modeText += ")"

	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Suffix = fmt.Sprintf(" %s %d image(s)...", modeText, opts.NumImages)
	_ = s.Color("magenta")
	s.Start()

	output := api.RunGeneration(ctx, cfg)
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

	outputPath := cfg.OutputFolder
	if !filepath.IsAbs(outputPath) {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
	}
	fmt.Printf("Output: %s\n", outputPath)

	if errorCount > 0 {
		return fmt.Errorf("%d image(s) failed", errorCount)
	}
	return nil
}
