// Package gemini implements the Provider interface for Google's Gemini image
// models, using the public generativelanguage REST API (API key auth).
//
// For Gemini via Vertex AI (service-account auth, enterprise quota), see the
// sibling `providers/vertex` package.
package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// Canonical model IDs. The cobra flag accepts aliases "pro" / "flash"
// (declared in Info().Models[*].Aliases); the provider resolves those.
const (
	ModelPro   = "gemini-3-pro-image-preview"
	ModelFlash = "gemini-3.1-flash-image-preview"

	baseURL = "https://generativelanguage.googleapis.com/v1beta/models/"
)

// Shared HTTP client with connection pooling.
var httpClient = &http.Client{
	Timeout: 120 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// Provider is the Gemini direct-REST implementation of providers.Provider.
type Provider struct {
	apiKey string
}

// New builds a Gemini provider from auth. Errors when APIKey is empty.
func New(auth providers.Auth) (providers.Provider, error) {
	if auth.APIKey == "" {
		return nil, errors.New("gemini provider requires providers.gemini.api_key in ~/.config/imagine/config.yaml")
	}
	return &Provider{apiKey: auth.APIKey}, nil
}

// Info returns Gemini's static metadata.
func (p *Provider) Info() providers.Info {
	return providers.Info{
		Name:         "gemini",
		DisplayName:  "Google Gemini",
		Summary:      "Google Gemini image models via generativelanguage.googleapis.com",
		DefaultModel: ModelPro,
		Models: []providers.ModelInfo{
			{
				ID:          ModelPro,
				Aliases:     []string{"pro"},
				Description: "Highest quality; no thinking / image-search flags.",
			},
			{
				ID:             ModelFlash,
				Aliases:        []string{"flash"},
				Description:    "Faster; supports --thinking and --image-search.",
				SupportedFlags: []string{"thinking", "image-search"},
			},
		},
		Capabilities: providers.Capabilities{
			Edit:        true,
			Grounding:   true,
			Thinking:    true,
			ImageSearch: true,
			MaxBatchN:   1,
			Sizes:       []string{"1K", "2K", "4K"},
		},
	}
}

// Generate performs one Gemini API call for a single image (MaxBatchN=1).
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	opts, ok := req.Options.(*Options)
	if !ok {
		return nil, fmt.Errorf("gemini: internal: expected *Options, got %T", req.Options)
	}

	parts := []part{{Text: req.Prompt}}
	for _, ref := range req.References {
		parts = append(parts, part{
			InlineData: &inlineData{
				MimeType: ref.MimeType,
				Data:     base64.StdEncoding.EncodeToString(ref.Data),
			},
		})
	}

	body := geminiRequest{
		Contents: []content{{Parts: parts}},
		GenerationConfig: generationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			ImageConfig: imageConfig{
				AspectRatio: opts.AspectRatio,
				ImageSize:   opts.Size,
			},
		},
	}

	if opts.Grounding {
		body.Tools = append(body.Tools, tool{GoogleSearch: &googleSearch{}})
	}
	if opts.ImageSearch {
		body.Tools = append(body.Tools, tool{ImageSearch: &imageSearch{}})
	}
	if opts.Thinking != "" {
		body.GenerationConfig.ThinkingConfig = &thinkingConfig{ThinkingLevel: opts.Thinking}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s:generateContent?key=%s", baseURL, opts.Model, p.apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &errResp); err == nil && errResp.Error.Message != "" {
			msg := errResp.Error.Message
			if len(msg) > 100 {
				msg = msg[:97] + "..."
			}
			return nil, errors.New(msg)
		}
		return nil, fmt.Errorf("API error (status %d)", resp.StatusCode)
	}

	var parsed geminiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract image bytes.
	for _, c := range parsed.Candidates {
		for _, pt := range c.Content.Parts {
			if pt.InlineData != nil && strings.HasPrefix(pt.InlineData.MimeType, "image/") {
				data, err := base64.StdEncoding.DecodeString(pt.InlineData.Data)
				if err != nil {
					return nil, fmt.Errorf("failed to decode image: %w", err)
				}
				return &providers.Response{
					Images: []providers.GeneratedImage{{Data: data, MimeType: pt.InlineData.MimeType}},
				}, nil
			}
		}
	}

	// No image: surface any explanatory text the API returned.
	for _, c := range parsed.Candidates {
		for _, pt := range c.Content.Parts {
			if pt.Text != "" {
				return nil, fmt.Errorf("no image in response. API said: %s", pt.Text)
			}
		}
	}
	return nil, errors.New("no image in response")
}

// -- Wire types (private). -----------------------------------------------------

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type content struct {
	Parts []part `json:"parts"`
}

type imageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize"`
}

type generationConfig struct {
	ResponseModalities []string        `json:"responseModalities"`
	ImageConfig        imageConfig     `json:"imageConfig"`
	ThinkingConfig     *thinkingConfig `json:"thinkingConfig,omitempty"`
}

type thinkingConfig struct {
	ThinkingLevel string `json:"thinkingLevel"`
}

type googleSearch struct{}

type imageSearch struct{}

type tool struct {
	GoogleSearch *googleSearch `json:"googleSearch,omitempty"`
	ImageSearch  *imageSearch  `json:"imageSearch,omitempty"`
}

type geminiRequest struct {
	Contents         []content        `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
	Tools            []tool           `json:"tools,omitempty"`
}

type responseInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type responsePart struct {
	Text       string              `json:"text,omitempty"`
	InlineData *responseInlineData `json:"inlineData,omitempty"`
}

type responseContent struct {
	Parts []responsePart `json:"parts"`
	Role  string         `json:"role"`
}

type candidate struct {
	Content responseContent `json:"content"`
}

type geminiResponse struct {
	Candidates []candidate `json:"candidates"`
}
