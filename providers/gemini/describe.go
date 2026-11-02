package gemini

import (
	"context"

	"google.golang.org/genai"

	"github.com/AhmedAburady/imagine-cli/internal/gvision"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// DefaultVisionModel is GA on both the Gemini API and Vertex (the Vertex
// provider reuses it). Thinking levels: low, medium, high; "minimal" errors.
// gemini-4-argon, the flagship, stays reachable with -m.
const DefaultVisionModel = "gemini-3.8-flash"

func (p *Provider) Describe(ctx context.Context, req providers.DescribeRequest) (*providers.ImageDescription, error) {
	model := req.Model
	if model == "" {
		model = p.visionModel
	}
	if model == "" {
		model = DefaultVisionModel
	}
	return gvision.Describe(ctx, &genai.ClientConfig{APIKey: p.apiKey}, model, req, p.store)
}

func (p *Provider) DefaultInstructions() (text, json string) {
	return gvision.TextInstruction, gvision.JSONInstruction
}
