package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
)

// Model name constants. Phase 4 moves these into providers/gemini.
const (
	ModelPro   = "gemini-3-pro-image-preview"
	ModelFlash = "gemini-3.1-flash-image-preview"

	geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models/"
)

// GeminiURL builds the full API endpoint for a given model name.
func GeminiURL(model string) string {
	return geminiBaseURL + model + ":generateContent"
}

// Shared HTTP client with connection pooling. 120s keeps long prompts alive.
var httpClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// Gemini request/response structures.

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

// GeminiError represents an API error response.
type GeminiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// Config holds the configuration for image generation. Shared between Gemini
// direct and Vertex callers; Phase 4 splits this into a Request struct passed
// to each provider.
type Config struct {
	OutputFolder     string
	OutputFilename   string // -f flag; suffixed _N for multiple images
	NumImages        int
	Prompt           string
	APIKey           string
	AspectRatio      string
	ImageSize        string
	Grounding        bool
	RefImages        []images.Reference
	RefInputPath     string // original -i path for -r flag
	PreserveFilename bool
	UseVertex        bool
	Model            string // full model name (e.g. ModelPro)
	ThinkingLevel    string // "MINIMAL" or "HIGH"; empty = omit
	ImageSearch      bool
}

// GenerateImage performs a single Gemini image generation request.
// ctx cancels in-flight HTTP.
func GenerateImage(ctx context.Context, config *Config, index int) GenerationResult {
	// Gemini wants base64-encoded bytes inline. Encode references on the fly.
	parts := []Part{{Text: config.Prompt}}
	for _, ref := range config.RefImages {
		parts = append(parts, Part{
			InlineData: &InlineData{
				MimeType: ref.MimeType,
				Data:     base64.StdEncoding.EncodeToString(ref.Data),
			},
		})
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

	if config.ThinkingLevel != "" {
		request.GenerationConfig.ThinkingConfig = &ThinkingConfig{
			ThinkingLevel: config.ThinkingLevel,
		}
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to marshal request: %v", err)}
	}

	url := fmt.Sprintf("%s?key=%s", GeminiURL(config.Model), config.APIKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
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
		var errResp struct {
			Error GeminiError `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
			msg := errResp.Error.Message
			if len(msg) > 100 {
				msg = msg[:97] + "..."
			}
			return GenerationResult{Index: index, Error: fmt.Errorf("%s", msg)}
		}
		return GenerationResult{Index: index, Error: fmt.Errorf("API error (status %d)", resp.StatusCode)}
	}

	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to parse response: %v", err)}
	}

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

	// No image: surface any text response for debugging.
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
