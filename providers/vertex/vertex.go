// Package vertex implements the Provider interface for Gemini image models
// accessed via Google Vertex AI (GCP project + Application Default Credentials,
// not an API key). Supports grounding + thinking, same model lineup as the
// direct Gemini provider but under a different auth/transport.
package vertex

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// Provider is the Vertex AI implementation of providers.Provider.
type Provider struct {
	project     string
	location    string
	visionModel string
}

// New builds a Vertex provider. Reads gcp_project and location from the
// flat auth map.
func New(auth providers.Auth) (providers.Provider, error) {
	project := auth.Get("gcp_project")
	if project == "" {
		return nil, errors.New("vertex provider requires providers.vertex.gcp_project in ~/.config/imagine/config.yaml")
	}
	location := auth.Get("location")
	if location == "" {
		location = "global"
	}
	return &Provider{project: project, location: location, visionModel: auth.Get("vision_model")}, nil
}

// ConfigSchema declares the fields `imagine providers add vertex` collects.
// Vertex uses Application Default Credentials — no API key in config.
func (p *Provider) ConfigSchema() []providers.ConfigField {
	return []providers.ConfigField{
		{
			Key:         "gcp_project",
			Title:       "GCP Project",
			Description: "GCP project ID with the Vertex AI API enabled",
			Required:    true,
		},
		{
			Key:         "location",
			Title:       "Location",
			Description: "Vertex AI region (default: global)",
			Default:     "global",
		},
		{
			Key:         "vision_model",
			Title:       "Vision Model",
			Description: "Model for `imagine describe` (multimodal Gemini 3 variants)",
			Default:     DefaultVisionModel,
		},
	}
}

// Info mirrors Gemini's models (Vertex is Gemini under a different transport),
// with capability flags trimmed to what Vertex actually supports today:
// no image-search tool.
func (p *Provider) Info() providers.Info {
	return providers.Info{
		Name:         "vertex",
		DisplayName:  "Vertex AI (Gemini)",
		Summary:      "Gemini models via Google Vertex AI (GCP project + ADC auth)",
		DefaultModel: gemini.ModelPro,
		Models: []providers.ModelInfo{
			{
				ID:          gemini.ModelPro,
				Aliases:     []string{"pro"},
				Description: "Highest quality; no thinking flags.",
			},
			{
				ID:             gemini.ModelFlash,
				Aliases:        []string{"flash"},
				Description:    "Faster; supports --thinking.",
				SupportedFlags: []string{"thinking"},
			},
		},
		Capabilities: providers.Capabilities{
			Edit:        true,
			Grounding:   true,
			Thinking:    true,
			ImageSearch: false,
			MaxBatchN:   1,
			Sizes:       []string{"1K", "2K", "4K"},
		},
	}
}

// Generate runs one image generation via Vertex. MaxBatchN=1.
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	opts, ok := req.Options.(*Options)
	if !ok {
		return nil, fmt.Errorf("vertex: internal: expected *Options, got %T", req.Options)
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  p.project,
		Location: p.location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	// Vertex takes raw bytes — no base64 round-trip.
	parts := []*genai.Part{genai.NewPartFromText(req.Prompt)}
	for _, ref := range req.References {
		parts = append(parts, &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: ref.MimeType,
				Data:     ref.Data,
			},
		})
	}
	contents := []*genai.Content{{Parts: parts, Role: "user"}}

	genConfig := &genai.GenerateContentConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}
	// Guard: only set ImageConfig when at least one sub-field is populated.
	// Empty strings into the SDK may not be equivalent to omission.
	if opts.AspectRatio != "" || opts.Size != "" {
		imgCfg := &genai.ImageConfig{}
		if opts.AspectRatio != "" {
			imgCfg.AspectRatio = opts.AspectRatio
		}
		if opts.Size != "" {
			imgCfg.ImageSize = opts.Size
		}
		genConfig.ImageConfig = imgCfg
	}

	if opts.Grounding {
		genConfig.Tools = append(genConfig.Tools, &genai.Tool{GoogleSearch: &genai.GoogleSearch{}})
	}
	if opts.Thinking != "" {
		genConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevel(opts.Thinking),
		}
	}

	resp, err := client.Models.GenerateContent(ctx, opts.Model, contents, genConfig)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, pt := range c.Content.Parts {
			if pt.InlineData != nil && len(pt.InlineData.Data) > 0 {
				return &providers.Response{
					Images: []providers.GeneratedImage{{
						Data:     pt.InlineData.Data,
						MimeType: pt.InlineData.MIMEType,
					}},
				}, nil
			}
		}
	}

	for _, c := range resp.Candidates {
		if c.Content == nil {
			continue
		}
		for _, pt := range c.Content.Parts {
			if pt.Text != "" {
				return nil, fmt.Errorf("no image in response. API said: %s", pt.Text)
			}
		}
	}
	return nil, errors.New("no image in response")
}
