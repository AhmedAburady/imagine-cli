package vertex

import (
	"context"

	"google.golang.org/genai"

	"github.com/AhmedAburady/imagine-cli/internal/gvision"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const DefaultVisionModel = "gemini-3-flash-preview"

func (p *Provider) Describe(ctx context.Context, req providers.DescribeRequest) (*providers.ImageDescription, error) {
	model := req.Model
	if model == "" {
		model = p.visionModel
	}
	if model == "" {
		model = DefaultVisionModel
	}
	return gvision.Describe(ctx, &genai.ClientConfig{
		Project:  p.project,
		Location: p.location,
		Backend:  genai.BackendVertexAI,
	}, model, req)
}

func (p *Provider) DefaultInstructions() (text, json string) {
	return gvision.TextInstruction, gvision.JSONInstruction
}
