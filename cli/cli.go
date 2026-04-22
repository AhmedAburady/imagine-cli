package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	"github.com/AhmedAburady/banana-cli/api"
	"github.com/AhmedAburady/banana-cli/config"
	"golang.org/x/term"
)

// version is set at build time via ldflags
var version = "dev"

// GetVersion returns the version from ldflags or go build info
func GetVersion() string {
	if version != "" && version != "dev" {
		return version
	}

	// Get version from go install build info
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return "dev"
}

// stringSlice implements flag.Value to allow repeating a flag (e.g. -i a.png -i b.png)
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// Options holds CLI configuration
type Options struct {
	Prompt           string
	Output           string
	OutputFilename   string   // -f flag, custom output filename
	NumImages        int
	AspectRatio      string
	ImageSize        string
	Grounding        bool
	RefInputs        []string // -i flag(s), triggers edit mode if set
	PreserveFilename bool     // -r flag, preserve input filename for output (replace)
	UseVertex        bool     // -vertex flag, use Vertex AI instead of Gemini API
	Model            string   // -m flag, "pro" or "flash"
	ThinkingLevel    string   // -t flag, "minimal" or "high"
	ImageSearch      bool     // -is flag, enable image search grounding
	Help             bool
	Version          bool
}

// Valid aspect ratios and image sizes
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

// ParseFlags parses CLI flags and returns options and whether CLI mode is active
func ParseFlags() (*Options, bool) {
	opts := &Options{}

	flag.StringVar(&opts.Prompt, "p", "", "Prompt text (required for CLI mode)")
	flag.StringVar(&opts.Output, "o", ".", "Output folder")
	flag.StringVar(&opts.OutputFilename, "f", "", "Output filename (suffixed _N for multiple images, e.g. image.png)")
	flag.IntVar(&opts.NumImages, "n", 1, "Number of images to generate (1-20)")
	flag.StringVar(&opts.AspectRatio, "ar", "", "Aspect ratio (default: Auto)")
	flag.StringVar(&opts.ImageSize, "s", "1K", "Image size: 1K, 2K, 4K")
	flag.BoolVar(&opts.Grounding, "g", false, "Enable grounding with Google Search")
	flag.Var((*stringSlice)(&opts.RefInputs), "i", "Reference image/folder, repeatable (enables edit mode)")
	flag.BoolVar(&opts.PreserveFilename, "r", false, "Replace: use input filename for output (single file only)")
	flag.BoolVar(&opts.UseVertex, "vertex", false, "Use Vertex AI instead of Gemini API (requires gcloud auth)")
	flag.StringVar(&opts.Model, "m", "pro", "Model: pro, flash")
	flag.StringVar(&opts.ThinkingLevel, "t", "minimal", "Thinking level: minimal, high")
	flag.BoolVar(&opts.ImageSearch, "is", false, "Enable Image Search grounding")
	flag.BoolVar(&opts.Help, "help", false, "Show help message")
	flag.BoolVar(&opts.Version, "version", false, "Show version")
	flag.BoolVar(&opts.Version, "v", false, "Show version")

	flag.Parse()

	// When -i is last and uses a glob (e.g. -i *.png), the shell expands it.
	// The flag captures the first file; remaining files land in flag.Args().
	if len(opts.RefInputs) > 0 && len(flag.Args()) > 0 {
		opts.RefInputs = append(opts.RefInputs, flag.Args()...)
	}

	// CLI mode is active if prompt is provided
	cliMode := opts.Prompt != ""

	return opts, cliMode
}

// PrintVersion prints the version
func PrintVersion() {
	fmt.Printf("banana version %s\n", GetVersion())
}

// Validate validates the CLI options
func (opts *Options) Validate() error {
	// Prompt is required for CLI mode
	if opts.Prompt == "" {
		return fmt.Errorf("prompt is required (-p flag)")
	}

	// Check if prompt is a file path - if file exists, read prompt from it
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

	// Expand tilde in paths
	opts.Output = api.ExpandTilde(opts.Output)
	for i, ref := range opts.RefInputs {
		opts.RefInputs[i] = api.ExpandTilde(ref)
	}

	// Validate number of images
	if opts.NumImages < 1 || opts.NumImages > 20 {
		return fmt.Errorf("number of images must be between 1 and 20")
	}

	// Validate aspect ratio
	if !validAspectRatios[opts.AspectRatio] {
		return fmt.Errorf("invalid aspect ratio: %s", opts.AspectRatio)
	}

	// Validate image size
	if !validImageSizes[opts.ImageSize] {
		return fmt.Errorf("invalid image size: %s (valid: 1K, 2K, 4K)", opts.ImageSize)
	}

	// Validate model
	if !validModels[opts.Model] {
		return fmt.Errorf("invalid model: %s (valid: pro, flash)", opts.Model)
	}

	// Validate thinking level
	if !validThinkingLevels[opts.ThinkingLevel] {
		return fmt.Errorf("invalid thinking level: %s (valid: minimal, high)", opts.ThinkingLevel)
	}

	// Validate reference inputs if provided (edit mode)
	for _, ref := range opts.RefInputs {
		info, err := os.Stat(ref)
		if os.IsNotExist(err) {
			return fmt.Errorf("reference path does not exist: %s", ref)
		}
		if err != nil {
			return fmt.Errorf("cannot access reference path: %v", err)
		}

		if info.IsDir() {
			count, _ := api.FindImagesInDir(ref)
			if count == 0 {
				return fmt.Errorf("no images found in reference directory: %s", ref)
			}
		} else if !api.IsSupportedImage(ref) {
			return fmt.Errorf("unsupported image format: %s", ref)
		}
	}

	// -f and -r are mutually exclusive
	if opts.OutputFilename != "" && opts.PreserveFilename {
		return fmt.Errorf("-f and -r are mutually exclusive: use one or the other")
	}

	// -r requires exactly one -i with a single file (not a folder)
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

// Run executes CLI mode with terminal spinner
func Run(opts *Options, apiKey string) {
	// Validate options
	if err := opts.Validate(); err != nil {
		fmt.Printf("\033[31mError:\033[0m %v\n", err)
		os.Exit(1)
	}

	// Load reference images if in edit mode
	var refImages []api.Part
	for _, ref := range opts.RefInputs {
		parts, err := api.LoadReferences(ref)
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to load references: %v\n", err)
			os.Exit(1)
		}
		refImages = append(refImages, parts...)
	}

	// Resolve model name
	modelName := api.ModelPro
	if opts.Model == "flash" {
		modelName = api.ModelFlash
	}

	// Thinking config is only supported on flash model
	thinkingLevel := ""
	if opts.Model == "flash" {
		thinkingLevel = strings.ToUpper(opts.ThinkingLevel)
	}

	// RefInputPath is used by -r for filename preservation (single file only)
	refInputPath := ""
	if len(opts.RefInputs) == 1 {
		refInputPath = opts.RefInputs[0]
	}

	// Create config
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

	// Ensure output folder exists
	if err := os.MkdirAll(cfg.OutputFolder, 0755); err != nil {
		fmt.Printf("\033[31mError:\033[0m Failed to create output folder: %v\n", err)
		os.Exit(1)
	}

	// Start spinner (CharSet 14 = braille dots)
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
	s.Color("magenta")
	s.Start()

	// Run generation
	output := api.RunGeneration(cfg)

	// Stop spinner
	s.Stop()

	// Print results
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

	// Show output path as absolute if it was relative
	outputPath := cfg.OutputFolder
	if !filepath.IsAbs(outputPath) {
		if abs, err := filepath.Abs(outputPath); err == nil {
			outputPath = abs
		}
	}
	fmt.Printf("Output: %s\n", outputPath)

	if errorCount > 0 {
		os.Exit(1)
	}
}

// PrintHelp prints the usage help message
func PrintHelp() {
	help := `
BANANA CLI - Gemini AI Image Generator

Usage:
  banana                        Open interactive TUI
  banana [flags]                Generate/edit images from command line
  banana describe [flags]       Describe/analyze image style using AI
  banana config <command>       Manage configuration

Generate/Edit Flags:
  -p string    Prompt text or path to prompt file (required for CLI mode)
  -o string    Output folder (default ".")
  -f string    Output filename (e.g. image.png); with -n >1, saves as image_1.png, image_2.png, ...
  -n int       Number of images (default 1)
  -ar string   Aspect ratio (default: Auto)
  -s string    Image size: 1K, 2K, 4K (default "1K")
  -m string    Model: pro, flash (default "pro")
  -t string    Thinking level: minimal, high (default "minimal")
  -g           Enable grounding with Google Search
  -is          Enable grounding with Image Search
  -i string    Reference image/folder, repeatable (put last when using globs)
  -r           Replace: use input filename for output (single file only)
  -vertex      Use Vertex AI instead of Gemini API (requires gcloud auth)
  --version    Show version
  --help       Show this help message

Describe Flags:
  -i string    Input image or folder (required)
  -o string    Output file path (default: stdout)
  -p string    Custom prompt (overrides default instruction)
  -a string    Additional instructions (prepended to default)
  -json        Output as structured JSON format

Config Commands:
  banana config set-key <KEY>       Save your Gemini API key
  banana config set-project <ID>    Save your GCP project ID (for -vertex)
  banana config set-location <LOC>  Save your GCP location (default: global)
  banana config show                Show current configuration
  banana config path                Show config file location

Examples:
  banana -p "a sunset over mountains" -n 3
  banana -p "a sunset" -o ~/Documents -f sunset.png
  banana -p "a sunset" -o ~/Documents -f sunset.png -n 5  # saves sunset_1.png ... sunset_5.png
  banana -p prompt.txt -n 3                          # load prompt from file
  banana -p "a sunset" -m flash                      # use flash model
  banana -p "a sunset" -m flash -t high              # flash with high thinking
  banana -p "a cat in supreme hoodie" -is             # with image search
  banana -i ./photo.png -p "make it cartoon style"
  banana -i a.png -i b.png -p "merge these styles"   # multiple reference images
  banana -p "add rain" -s 2K -i *.png                # glob (put -i last)
  banana -i ./photo.png -p "make it cartoon" -r      # output keeps name: photo.png
  banana -p "a futuristic city" -g -ar 16:9 -s 2K
  banana -i ./images/ -p "add rain effect" -n 2 -o ./output
  banana describe -i photo.jpg                       # analyze image style
  banana describe -i ./styles/ -o style.json         # analyze folder of images
`
	fmt.Print(help)
}

// HandleConfigCommand handles the config subcommand
func HandleConfigCommand(args []string) bool {
	if len(args) < 2 || args[0] != "config" {
		return false
	}

	switch args[1] {
	case "set-key":
		if len(args) < 3 {
			fmt.Println("Usage: banana config set-key <API_KEY>")
			os.Exit(1)
		}
		if err := config.SaveAPIKey(args[2]); err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to save API key: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\033[32m✓\033[0m API key saved successfully")
		fmt.Printf("  Location: %s\n", config.DefaultConfigPath())

	case "set-project":
		if len(args) < 3 {
			fmt.Println("Usage: banana config set-project <GCP_PROJECT_ID>")
			os.Exit(1)
		}
		if err := config.SaveGCPProject(args[2]); err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to save GCP project: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\033[32m✓\033[0m GCP project saved successfully")
		fmt.Printf("  Project: %s\n", args[2])

	case "set-location":
		if len(args) < 3 {
			fmt.Println("Usage: banana config set-location <GCP_LOCATION>")
			os.Exit(1)
		}
		if err := config.SaveGCPLocation(args[2]); err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to save GCP location: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\033[32m✓\033[0m GCP location saved successfully")
		fmt.Printf("  Location: %s\n", args[2])

	case "show":
		cfg, err := config.Load()
		if err != nil {
			fmt.Printf("\033[31mError:\033[0m Failed to load config: %v\n", err)
			os.Exit(1)
		}
		// API Key
		if cfg.APIKey == "" {
			fmt.Println("API Key: (not set)")
		} else {
			key := cfg.APIKey
			masked := key
			if len(key) > 12 {
				masked = key[:8] + "..." + key[len(key)-4:]
			}
			fmt.Printf("API Key: %s\n", masked)
		}
		// GCP Project
		if cfg.GCPProject == "" {
			fmt.Println("GCP Project: (not set)")
		} else {
			fmt.Printf("GCP Project: %s\n", cfg.GCPProject)
		}
		// GCP Location
		if cfg.GCPLocation == "" {
			fmt.Println("GCP Location: (not set, defaults to 'global')")
		} else {
			fmt.Printf("GCP Location: %s\n", cfg.GCPLocation)
		}

	case "path":
		fmt.Println(config.DefaultConfigPath())

	default:
		fmt.Printf("Unknown config command: %s\n", args[1])
		fmt.Println("Available commands: set-key, set-project, set-location, show, path")
		os.Exit(1)
	}

	return true
}

// PromptForAPIKey prompts the user to enter their API key
func PromptForAPIKey() string {
	fmt.Println("\033[33mNo API key found.\033[0m")
	fmt.Println()
	fmt.Println("Get your free API key from: https://aistudio.google.com/app/apikey")
	fmt.Println()
	fmt.Print("Enter your Gemini API key: ")

	// Read password without echoing to terminal
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // Add newline after hidden input
	if err != nil {
		fmt.Printf("\033[31mError:\033[0m Failed to read input: %v\n", err)
		os.Exit(1)
	}

	key := strings.TrimSpace(string(keyBytes))
	if key == "" {
		fmt.Println("\033[31mError:\033[0m API key cannot be empty")
		os.Exit(1)
	}

	// Save the key
	if err := config.SaveAPIKey(key); err != nil {
		fmt.Printf("\033[31mError:\033[0m Failed to save API key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\033[32m✓\033[0m API key saved successfully")
	fmt.Println()

	return key
}
