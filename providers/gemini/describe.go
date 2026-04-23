package gemini

import (
	"context"

	"google.golang.org/genai"

	"github.com/AhmedAburady/imagine-cli/internal/gvision"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// "gemini-pro-latest" points at gemini-3.1-pro-preview latest pro model from google
const DefaultVisionModel = "gemini-pro-latest"

func (p *Provider) Describe(ctx context.Context, req providers.DescribeRequest) (*providers.ImageDescription, error) {
	model := req.Model
	if model == "" {
		model = p.visionModel
	}
	if model == "" {
		model = DefaultVisionModel
	}
	return gvision.Describe(ctx, &genai.ClientConfig{APIKey: p.apiKey}, model, req)
}

func (p *Provider) DefaultInstructions() (text, json string) {
	return gvision.TextInstruction, gvision.JSONInstruction
}
