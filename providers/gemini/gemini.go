// Package gemini implements the Provider interface for Google's Gemini image
// models, using the public generativelanguage REST API (API key auth).
//
// For Gemini via Vertex AI (service-account auth, enterprise quota), see the
// sibling `providers/vertex` package.
package gemini

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// Canonical model IDs — the GA names, not the retired -preview ones.
const (
	ModelPro       = "gemini-3-pro-image"
	ModelFlash     = "gemini-nano-banana-2.1"
	ModelFlashLite = "gemini-3.1-flash-lite-image"

	// Stays v1beta: v1 rejects image search as a tool ("not enabled for api
	// version v1"), though it serves plain generation fine.
	baseURL = "https://generativelanguage.googleapis.com/v1beta/models/"
)

const legacyModelPro = "gemini-3-pro-image-preview"

// Sizes* are funcs, not vars, so the slices Vertex shares stay immutable.
// No current model renders 512; flash-lite narrows the wide set to 1K.
func Sizes() []string          { return []string{"1K", "2K", "4K"} }
func SizesFlashLite() []string { return []string{"1K"} }

// Exported so configs pinned to an older ID resolve the same on Vertex.
// The 3.1 Flash Image IDs follow Google's named replacement, Nano Banana 2.1.
func DeprecatedAliasesPro() []string { return []string{legacyModelPro} }
func DeprecatedAliasesFlash() []string {
	return []string{"gemini-3.1-flash-image", "gemini-3.1-flash-image-preview"}
}

// ThinkingLevels lists the --thinking values a model documents; nil means the
// model takes no thinking level at all (CheckModel reports that case).
func ThinkingLevels(model string) []string {
	switch model {
	case ModelFlash:
		return []string{"MINIMAL", "MEDIUM", "HIGH"}
	case ModelFlashLite:
		return []string{"MINIMAL", "HIGH"}
	}
	return nil
}

// CheckThinking turns an opaque API 400 into a local error naming the model.
func CheckThinking(model, level string) error {
	levels := ThinkingLevels(model)
	if level == "" || len(levels) == 0 || slices.Contains(levels, level) {
		return nil
	}
	return fmt.Errorf("--thinking %s is not supported by model %q (supported: %s)",
		strings.ToLower(level), model, strings.ToLower(strings.Join(levels, ", ")))
}

// Google's published per-model tables disagree with each other and with this.
func AspectRatios() []string {
	return []string{
		"1:1", "1:4", "1:8", "2:3", "3:2", "3:4", "4:1",
		"4:3", "4:5", "5:4", "8:1", "9:16", "16:9", "21:9",
	}
}

// httpClient is shared by all Provider instances via package-level init.
// transport.NewClient provides pooling defaults.
var httpClient = transport.NewClient(120 * time.Second)

// Provider is the Gemini direct-REST implementation of providers.Provider.
type Provider struct {
	apiKey      string
	visionModel string
	store       *bool // nil: the project's logging setting decides
}

// New builds a Gemini provider from auth. Errors when api_key is empty.
func New(auth providers.Auth) (providers.Provider, error) {
	key := auth.Get("api_key")
	if key == "" {
		return nil, errors.New("gemini provider requires providers.gemini.api_key in ~/.config/imagine/config.yaml")
	}
	store, err := parseStore(auth.Get("store"))
	if err != nil {
		return nil, err
	}
	return &Provider{apiKey: key, visionModel: auth.Get("vision_model"), store: store}, nil
}

// parseStore reads the optional store setting. Empty sends nothing, so the
// project's Logs and Datasets setting applies (new paid projects log
// generateContent requests by default since 2026-10-26).
func parseStore(raw string) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return nil, nil
	case "true":
		v := true
		return &v, nil
	case "false":
		v := false
		return &v, nil
	}
	return nil, fmt.Errorf("providers.gemini.store must be true or false, got %q", raw)
}

// ConfigSchema declares the fields `imagine providers add gemini` collects.
func (p *Provider) ConfigSchema() []providers.ConfigField {
	return []providers.ConfigField{
		{
			Key:         "api_key",
			Title:       "API Key",
			Description: "Gemini API key from Google AI Studio (aistudio.google.com/app/apikey)",
			Secret:      true,
			Required:    true,
		},
		{
			Key:         "vision_model",
			Title:       "Vision Model",
			Description: "Model for `imagine describe` (multimodal Gemini 3 variants)",
			Default:     DefaultVisionModel,
		},
		{
			Key:         "store",
			Title:       "Store Requests",
			Description: "false keeps requests out of Gemini API logs; empty follows the project setting",
		},
	}
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
				ID:                ModelPro,
				Aliases:           []string{"pro"},
				DeprecatedAliases: DeprecatedAliasesPro(),
				Description:       "Highest quality; no thinking / image-search flags.",
				SupportedFlags:    []string{"grounding"},
			},
			{
				ID:                ModelFlash,
				Aliases:           []string{"flash"},
				DeprecatedAliases: DeprecatedAliasesFlash(),
				Description:       "Nano Banana 2.1: fast and low-cost; supports --thinking (minimal, medium, high) and --image-search.",
				SupportedFlags:    []string{"grounding", "thinking", "image-search"},
			},
			{
				ID:             ModelFlashLite,
				Aliases:        []string{"flash-lite", "lite"},
				Description:    "Fastest and cheapest; 1K only, no grounding.",
				SupportedFlags: []string{"thinking"},
				Sizes:          SizesFlashLite(),
			},
		},
		Capabilities: providers.Capabilities{
			Edit:         true,
			Grounding:    true,
			Thinking:     true,
			ImageSearch:  true,
			MaxBatchN:    1,
			Sizes:        Sizes(),
			AspectRatios: AspectRatios(),
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
		Store: p.store,
	}
	if gs := buildSearchTool(opts.Grounding, opts.ImageSearch); gs != nil {
		body.Tools = append(body.Tools, tool{GoogleSearch: gs})
	}
	if opts.Thinking != "" {
		body.GenerationConfig.ThinkingConfig = &thinkingConfig{ThinkingLevel: opts.Thinking}
	}

	url := baseURL + opts.Model + ":generateContent"
	parsed, err := transport.PostJSON[geminiResponse](ctx, httpClient, url, transport.QueryKey("key", p.apiKey), body)
	if err != nil {
		return nil, err
	}

	// Extract the first image from candidates[].content.parts[].inlineData.
	for _, c := range parsed.Candidates {
		for _, pt := range c.Content.Parts {
			if pt.InlineData != nil && strings.HasPrefix(pt.InlineData.MimeType, "image/") {
				data, err := transport.DecodeB64(pt.InlineData.Data)
				if err != nil {
					return nil, err
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

// buildSearchTool nests image search inside google_search per the current docs.
// Grounding alone stays a bare googleSearch, which is what it has always sent.
func buildSearchTool(grounding, images bool) *googleSearch {
	if !images {
		if !grounding {
			return nil
		}
		return &googleSearch{}
	}
	st := &searchTypes{ImageSearch: &struct{}{}}
	if grounding {
		st.WebSearch = &struct{}{}
	}
	return &googleSearch{SearchTypes: st}
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

type searchTypes struct {
	WebSearch   *struct{} `json:"webSearch,omitempty"`
	ImageSearch *struct{} `json:"imageSearch,omitempty"`
}

type googleSearch struct {
	SearchTypes *searchTypes `json:"searchTypes,omitempty"`
}

type tool struct {
	GoogleSearch *googleSearch `json:"googleSearch,omitempty"`
}

type geminiRequest struct {
	Contents         []content        `json:"contents"`
	GenerationConfig generationConfig `json:"generationConfig"`
	Tools            []tool           `json:"tools,omitempty"`
	Store            *bool            `json:"store,omitempty"`
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
