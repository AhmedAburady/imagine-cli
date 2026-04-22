package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder for image.Decode
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// Model name constants
	ModelPro   = "gemini-3-pro-image-preview"
	ModelFlash = "gemini-3.1-flash-image-preview"

	geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"
)

// GeminiURL builds the full API endpoint for a given model name.
func GeminiURL(model string) string {
	return geminiBaseURL + model + ":generateContent"
}

// Shared HTTP client with connection pooling and timeouts
var httpClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// Gemini API structures
type InlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type Part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inline_data,omitempty"`
}

type Content struct {
	Parts []Part `json:"parts"`
}

type ImageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize"`
}

type GenerationConfig struct {
	ResponseModalities []string        `json:"responseModalities"`
	ImageConfig        ImageConfig     `json:"imageConfig"`
	ThinkingConfig     *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

type GoogleSearch struct{}

type ImageSearch struct{}

type Tool struct {
	GoogleSearch *GoogleSearch `json:"googleSearch,omitempty"`
	ImageSearch  *ImageSearch  `json:"imageSearch,omitempty"`
}

type ThinkingConfig struct {
	ThinkingLevel string `json:"thinkingLevel"`
}

type GeminiRequest struct {
	Contents         []Content        `json:"contents"`
	GenerationConfig GenerationConfig `json:"generationConfig"`
	Tools            []Tool           `json:"tools,omitempty"`
}

// Response structures
type ResponseInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type ResponsePart struct {
	Text       string              `json:"text,omitempty"`
	InlineData *ResponseInlineData `json:"inlineData,omitempty"`
}

type ResponseContent struct {
	Parts []ResponsePart `json:"parts"`
	Role  string         `json:"role"`
}

type Candidate struct {
	Content ResponseContent `json:"content"`
}

type GeminiResponse struct {
	Candidates []Candidate  `json:"candidates"`
	Error      *GeminiError `json:"error,omitempty"`
}

// GeminiError represents API error response
type GeminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// GenerationResult holds the result of an image generation attempt
type GenerationResult struct {
	Index     int
	ImageData []byte
	Filename  string
	Error     error
}

// Config holds the configuration for image generation
type Config struct {
	OutputFolder     string
	OutputFilename   string // Custom output filename (-f flag); suffixed _N for multiple images
	NumImages        int
	Prompt           string
	APIKey           string
	AspectRatio      string
	ImageSize        string
	Grounding        bool
	RefImages        []Part
	RefInputPath     string // Original input path for -r flag
	PreserveFilename bool   // Whether to preserve input filename for output
	UseVertex        bool   // Use Vertex AI instead of Gemini API
	Model            string // Full model name (e.g. ModelPro, ModelFlash)
	ThinkingLevel    string // "MINIMAL" or "HIGH" (empty = omit from request)
	ImageSearch      bool   // Enable image search grounding tool
}

// Supported image extensions and their MIME types
var supportedExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// ExpandTilde expands ~ to the user's home directory
func ExpandTilde(path string) string {
	if path == "~" {
		if usr, err := user.Current(); err == nil {
			return usr.HomeDir
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if usr, err := user.Current(); err == nil {
			return filepath.Join(usr.HomeDir, path[2:])
		}
	}
	return path
}

// IsSupportedImage checks if a file has a supported image extension
func IsSupportedImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := supportedExts[ext]
	return ok
}

// GetImageMimeType returns the MIME type for a supported image extension
func GetImageMimeType(ext string) (string, bool) {
	mimeType, ok := supportedExts[strings.ToLower(ext)]
	return mimeType, ok
}

// LoadReferences loads images from either:
// - A directory (all images directly in that folder)
// - A single image file path
func LoadReferences(path string) ([]Part, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return loadImagesFromDir(path)
	}
	return loadSingleImage(path)
}

// imageLoadResult holds the result of loading a single image
type imageLoadResult struct {
	index int
	part  Part
	err   error
}

// loadImagesFromDir loads all supported images from a directory in parallel
func loadImagesFromDir(dirPath string) ([]Part, error) {
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
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		mimeType, ok := supportedExts[ext]
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

			encoded := base64.StdEncoding.EncodeToString(data)
			resultsChan <- imageLoadResult{
				index: img.index,
				part: Part{
					InlineData: &InlineData{
						MimeType: img.mimeType,
						Data:     encoded,
					},
				},
			}
		}(img)
	}

	// Close channel when done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results and preserve order
	results := make([]Part, len(imageEntries))
	for result := range resultsChan {
		if result.err != nil {
			return nil, result.err
		}
		results[result.index] = result.part
	}

	return results, nil
}

// loadSingleImage loads a single image file
func loadSingleImage(filePath string) ([]Part, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType, ok := supportedExts[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %v", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return []Part{{
		InlineData: &InlineData{
			MimeType: mimeType,
			Data:     encoded,
		},
	}}, nil
}

// FindImagesInDir returns the count of images in a directory
func FindImagesInDir(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := supportedExts[ext]; ok {
			count++
		}
	}
	return count, nil
}

// GenerationOutput holds the complete output of a generation run
type GenerationOutput struct {
	Results      []GenerationResult
	OutputFolder string
	Elapsed      time.Duration
}

// convertToJPEG decodes image bytes (any format) and re-encodes them as JPEG at quality 95.
func convertToJPEG(imgData []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// RunGeneration performs parallel image generation and saves results
func RunGeneration(config *Config) GenerationOutput {
	startTime := time.Now()

	// Ensure output folder exists
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

	// Generate images in parallel
	var wg sync.WaitGroup
	resultsChan := make(chan GenerationResult, config.NumImages)

	for i := 0; i < config.NumImages; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			var result GenerationResult
			if config.UseVertex {
				result = GenerateImageVertex(config, index)
			} else {
				result = GenerateImage(config, index)
			}

			// Save if successful
			if result.Error == nil && result.ImageData != nil {
				var filename string
				if config.OutputFilename != "" {
					// Honour the user's extension; fall back to .png for anything unsupported
					rawExt := strings.ToLower(filepath.Ext(config.OutputFilename))
					stem := strings.TrimSuffix(config.OutputFilename, filepath.Ext(config.OutputFilename))
					ext := ".png"
					if rawExt == ".jpg" || rawExt == ".jpeg" {
						ext = ".jpg"
					}
					if config.NumImages > 1 {
						filename = fmt.Sprintf("%s_%d%s", stem, result.Index+1, ext)
					} else {
						filename = stem + ext
					}
				} else if config.PreserveFilename && config.RefInputPath != "" {
					// Use the input filename with .png extension
					baseName := filepath.Base(config.RefInputPath)
					ext := filepath.Ext(baseName)
					nameWithoutExt := strings.TrimSuffix(baseName, ext)
					if config.NumImages > 1 {
						filename = fmt.Sprintf("%s_%d.png", nameWithoutExt, result.Index+1)
					} else {
						filename = nameWithoutExt + ".png"
					}
				} else {
					filename = fmt.Sprintf("generated_%d_%s.png", result.Index+1, time.Now().Format("20060102_150405"))
				}

				// Convert to JPEG inside the goroutine to keep parallelism intact
				imageData := result.ImageData
				if ext := strings.ToLower(filepath.Ext(filename)); ext == ".jpg" || ext == ".jpeg" {
					converted, err := convertToJPEG(imageData)
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

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
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

// GenerateImage performs a single image generation request
func GenerateImage(config *Config, index int) GenerationResult {
	// Build request parts: text prompt only for generate, text + images for edit
	var parts []Part
	parts = append(parts, Part{Text: config.Prompt})

	// Only add reference images if they exist (edit mode)
	if len(config.RefImages) > 0 {
		parts = append(parts, config.RefImages...)
	}

	request := GeminiRequest{
		Contents: []Content{{Parts: parts}},
		GenerationConfig: GenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			ImageConfig: ImageConfig{
				AspectRatio: config.AspectRatio,
				ImageSize:   config.ImageSize,
			},
		},
	}

	// Add tools (google search, image search)
	var tools []Tool
	if config.Grounding {
		tools = append(tools, Tool{GoogleSearch: &GoogleSearch{}})
	}
	if config.ImageSearch {
		tools = append(tools, Tool{ImageSearch: &ImageSearch{}})
	}
	if len(tools) > 0 {
		request.Tools = tools
	}

	// Add thinking config if specified
	if config.ThinkingLevel != "" {
		request.GenerationConfig.ThinkingConfig = &ThinkingConfig{
			ThinkingLevel: config.ThinkingLevel,
		}
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to marshal request: %v", err)}
	}

	// Build URL with model and API key
	url := fmt.Sprintf("%s?key=%s", GeminiURL(config.Model), config.APIKey)

	// Make request using shared client with connection pooling
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("request failed: %v", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to read response: %v", err)}
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response for cleaner message
		var errResp struct {
			Error GeminiError `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			msg := errResp.Error.Message
			// Truncate long messages
			if len(msg) > 100 {
				msg = msg[:97] + "..."
			}
			return GenerationResult{Index: index, Error: fmt.Errorf("%s", msg)}
		}
		return GenerationResult{Index: index, Error: fmt.Errorf("API error (status %d)", resp.StatusCode)}
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to parse response: %v", err)}
	}

	// Extract image from response
	for _, candidate := range geminiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && strings.HasPrefix(part.InlineData.MimeType, "image/") {
				imageData, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
				if err != nil {
					return GenerationResult{Index: index, Error: fmt.Errorf("failed to decode image: %v", err)}
				}
				return GenerationResult{Index: index, ImageData: imageData}
			}
		}
	}

	// No image found - check if there's text explaining why
	var textResponse string
	for _, candidate := range geminiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				textResponse = part.Text
			}
		}
	}
	if textResponse != "" {
		return GenerationResult{Index: index, Error: fmt.Errorf("no image in response. API said: %s", textResponse)}
	}

	return GenerationResult{Index: index, Error: fmt.Errorf("no image in response")}
}
