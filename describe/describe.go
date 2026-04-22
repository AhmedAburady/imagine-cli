package describe

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"google.golang.org/genai"

	"github.com/AhmedAburady/banana-cli/api"
	"github.com/AhmedAburady/banana-cli/config"
)

// Options holds CLI configuration for describe command
type Options struct {
	Input      string // -i flag: image or folder path (required)
	Output     string // -o flag: output file (optional, default stdout)
	Prompt     string // -p flag: custom prompt (overrides default)
	Additional string // -a flag: additional instructions (appended to default)
	JSONOutput bool   // -json flag: output as JSON (default: text)
	UseVertex  bool   // -vertex flag: use Vertex AI instead of Gemini API
	Help       bool   // --help flag
}

// ParseFlags parses describe subcommand flags
func ParseFlags(args []string) (*Options, error) {
	opts := &Options{}

	fs := flag.NewFlagSet("describe", flag.ContinueOnError)
	fs.StringVar(&opts.Input, "i", "", "Input image or folder (required)")
	fs.StringVar(&opts.Output, "o", "", "Output file path (default: stdout)")
	fs.StringVar(&opts.Prompt, "p", "", "Custom prompt (overrides default instruction)")
	fs.StringVar(&opts.Additional, "a", "", "Additional instructions (appended to default)")
	fs.BoolVar(&opts.JSONOutput, "json", false, "Output as JSON format")
	fs.BoolVar(&opts.UseVertex, "vertex", false, "Use Vertex AI instead of Gemini API")
	fs.BoolVar(&opts.Help, "help", false, "Show help message")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return opts, nil
}

// Validate validates the describe options
func (opts *Options) Validate() error {
	// Input is required
	if opts.Input == "" {
		return fmt.Errorf("input is required (-i flag)")
	}

	// Expand tilde in paths
	opts.Input = api.ExpandTilde(opts.Input)
	opts.Output = api.ExpandTilde(opts.Output)

	// Check if input exists
	info, err := os.Stat(opts.Input)
	if os.IsNotExist(err) {
		return fmt.Errorf("input path does not exist: %s", opts.Input)
	}
	if err != nil {
		return fmt.Errorf("cannot access input path: %v", err)
	}

	// Validate input
	if info.IsDir() {
		count, _ := api.FindImagesInDir(opts.Input)
		if count == 0 {
			return fmt.Errorf("no images found in directory: %s", opts.Input)
		}
	} else if !api.IsSupportedImage(opts.Input) {
		return fmt.Errorf("unsupported image format: %s", opts.Input)
	}

	// Check if prompt is a file path - if file exists, read prompt from it
	if opts.Prompt != "" {
		promptPath := api.ExpandTilde(opts.Prompt)
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
	}

	return nil
}

// PrintHelp prints the help message for describe command
func PrintHelp() {
	help := `
BANANA DESCRIBE - AI-Powered Image Style Analysis

Usage:
  banana describe -i <image-or-folder> [flags]

Flags:
  -i string    Input image or folder with style reference images (required)
  -o string    Output file path (default: stdout)
  -p string    Custom prompt (completely overrides default instruction)
  -a string    Additional instructions (prepended to default instruction)
  -json        Output as structured JSON (default: plain text)
  -vertex      Use Vertex AI instead of Gemini API (requires gcloud auth)
  --help       Show this help message

Output Modes:
  Plain Text   A detailed style description ready to use as a generation prompt

  JSON (-json) Structured style guide with:
               • style_name, description, style_summary
               • colors (hex codes)
               • medium, composition
               • key_elements, avoid

Examples:
  banana describe -i photo.jpg                         # Plain text description
  banana describe -i ./styles/ -json                   # JSON style guide
  banana describe -i image.png -json -o style.json    # Save JSON to file
  banana describe -i refs/ -a "2D flat vector art"    # Add style context
  banana describe -i img.jpg -p "Focus on colors"     # Custom prompt
`
	fmt.Print(help)
}

// loadImageParts loads images and converts them to genai.Part format
func loadImageParts(inputPath string) ([]*genai.Part, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return loadImagesFromDir(inputPath)
	}
	return loadSingleImage(inputPath)
}

// imageLoadResult holds the result of loading a single image
type imageLoadResult struct {
	index int
	part  *genai.Part
	err   error
}

// loadImagesFromDir loads all supported images from a directory in parallel
func loadImagesFromDir(dirPath string) ([]*genai.Part, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	// Filter to only supported images
	type imageEntry struct {
		index    int
		filePath string
		mimeType string
	}
	var imageEntries []imageEntry

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		mimeType, ok := api.GetImageMimeType(ext)
		if !ok {
			continue
		}
		imageEntries = append(imageEntries, imageEntry{
			index:    len(imageEntries),
			filePath: filepath.Join(dirPath, entry.Name()),
			mimeType: mimeType,
		})
	}

	if len(imageEntries) == 0 {
		return nil, nil
	}

	// Load and encode images in parallel
	var wg sync.WaitGroup
	resultsChan := make(chan imageLoadResult, len(imageEntries))

	for _, img := range imageEntries {
		wg.Add(1)
		go func(img imageEntry) {
			defer wg.Done()

			data, err := os.ReadFile(img.filePath)
			if err != nil {
				resultsChan <- imageLoadResult{
					index: img.index,
					err:   fmt.Errorf("failed to read %s: %v", filepath.Base(img.filePath), err),
				}
				return
			}

			resultsChan <- imageLoadResult{
				index: img.index,
				part:  genai.NewPartFromBytes(data, img.mimeType),
			}
		}(img)
	}

	// Close channel when done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and preserve order
	results := make([]*genai.Part, len(imageEntries))
	for result := range resultsChan {
		if result.err != nil {
			return nil, result.err
		}
		results[result.index] = result.part
	}

	return results, nil
}

// loadSingleImage loads a single image file
func loadSingleImage(filePath string) ([]*genai.Part, error) {
	ext := filepath.Ext(filePath)
	mimeType, ok := api.GetImageMimeType(ext)
	if !ok {
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %v", err)
	}

	return []*genai.Part{genai.NewPartFromBytes(data, mimeType)}, nil
}

// Run executes the describe command
func Run(opts *Options, apiKey string, useVertex bool) error {
	// Validate options
	if err := opts.Validate(); err != nil {
		return err
	}

	// Start spinner (CharSet 14 = braille dots)
	s := spinner.New(spinner.CharSets[14], 80*time.Millisecond)
	s.Suffix = " Analyzing image style..."
	s.Color("magenta")
	s.Start()

	// Load images
	imageParts, err := loadImageParts(opts.Input)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to load images: %v", err)
	}

	// Create agent and run analysis
	ctx := context.Background()
	agent, err := NewDescribeAgent(ctx, apiKey, useVertex)
	if err != nil {
		s.Stop()
		return fmt.Errorf("failed to create agent: %v", err)
	}

	// Run description
	result, err := agent.DescribeImages(ctx, imageParts, opts.Prompt, opts.Additional, opts.JSONOutput)
	if err != nil {
		s.Stop()
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Stop spinner
	s.Stop()

	// Format output
	output := result.FormatOutput()

	// Write output
	if opts.Output != "" {
		if err := os.WriteFile(opts.Output, []byte(output+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %v", err)
		}
		fmt.Printf("\033[32m✓\033[0m Description saved to: %s\n", opts.Output)
	} else {
		// Print to stdout
		fmt.Println(output)
	}

	return nil
}

// HandleDescribeCommand handles the describe subcommand
func HandleDescribeCommand(args []string) {
	opts, err := ParseFlags(args)
	if err != nil {
		fmt.Printf("\033[31mError:\033[0m %v\n", err)
		PrintHelp()
		os.Exit(1)
	}

	if opts.Help || opts.Input == "" {
		PrintHelp()
		if opts.Input == "" && !opts.Help {
			os.Exit(1)
		}
		return
	}

	// Get API key
	apiKey := config.GetAPIKey()
	if apiKey == "" {
		fmt.Println("\033[33mNo API key found.\033[0m")
		fmt.Println()
		fmt.Println("Get your free API key from: https://aistudio.google.com/app/apikey")
		fmt.Println("Then set it with: banana config set-key <YOUR_API_KEY>")
		fmt.Println("Or set GEMINI_API_KEY environment variable")
		os.Exit(1)
	}

	if err := Run(opts, apiKey, opts.UseVertex); err != nil {
		fmt.Printf("\033[31mError:\033[0m %v\n", err)
		os.Exit(1)
	}
}
