package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const (
	DefaultVisionModel = "gpt-5.4-mini"
	chatCompletionsPath = "/chat/completions"
)

func (p *Provider) Describe(ctx context.Context, req providers.DescribeRequest) (*providers.ImageDescription, error) {
	if len(req.Images) == 0 {
		return nil, errors.New("no images provided")
	}

	model := req.Model
	if model == "" {
		model = p.visionModel
	}
	if model == "" {
		model = DefaultVisionModel
	}

	body := chatRequest{
		Model:    model,
		Messages: []chatMessage{{Role: "user", Content: buildContent(req)}},
	}
	if req.StructuredOutput {
		body.ResponseFormat = &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchemaWrapper{
				Name:   "style_analysis",
				Strict: true,
				Schema: styleSchema(),
			},
		}
	}

	resp, err := transport.PostJSON[chatResponse](ctx, httpClient, baseURL+chatCompletionsPath, transport.Bearer(p.apiKey), body)
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, errors.New("openai returned no choices")
	}
	text := strings.TrimSpace(resp.Choices[0].Message.Content)
	if text == "" {
		return nil, errors.New("openai returned empty description")
	}

	if !req.StructuredOutput {
		return &providers.ImageDescription{Text: text}, nil
	}
	var s providers.StyleAnalysis
	if err := json.Unmarshal([]byte(text), &s); err != nil {
		return &providers.ImageDescription{Text: text}, nil
	}
	return &providers.ImageDescription{Structured: &s}, nil
}

func (p *Provider) DefaultInstructions() (text, json string) {
	return TextInstruction, JSONInstruction
}

func buildContent(req providers.DescribeRequest) []contentPart {
	instruction := pickInstruction(req)
	parts := []contentPart{{Type: "text", Text: instruction}}
	for _, ref := range req.Images {
		parts = append(parts, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: dataURL(ref.MimeType, ref.Data)},
		})
	}
	return parts
}

func dataURL(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func pickInstruction(req providers.DescribeRequest) string {
	base := TextInstruction
	if req.StructuredOutput {
		base = JSONInstruction
	}
	if req.CustomPrompt != "" {
		base = req.CustomPrompt
	}
	if req.Additional != "" {
		return "CRITICAL USER CONTEXT - You MUST incorporate this into your analysis:\n" + req.Additional + "\n\n" + base
	}
	return base
}

// TextInstruction / JSONInstruction are the built-in prompts used when
// the caller doesn't pass a custom one. Exported so `imagine describe
// --show-instructions` can display them.
const TextInstruction = `You are an expert image style analyst. Analyze the provided image(s) and create a detailed description that captures the visual style.

When multiple images are provided, identify the UNIFIED style elements across all images.

Focus on what you actually SEE: colors, shapes, patterns, textures, art style, mood, distinctive visual elements, composition. Output only the description, no preamble.`

const JSONInstruction = `Analyze the image style. Respond with a JSON object matching the provided schema. Be concise and specific.`

func styleSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"style_name":    map[string]any{"type": "string"},
			"description":   map[string]any{"type": "string"},
			"style_summary": map[string]any{"type": "string"},
			"colors":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"medium":        map[string]any{"type": "string"},
			"composition":   map[string]any{"type": "string"},
			"key_elements":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"avoid":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required":             []string{"style_name", "description", "style_summary", "colors", "medium", "composition", "key_elements", "avoid"},
		"additionalProperties": false,
	}
}

// --- wire types -------------------------------------------------------------

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type responseFormat struct {
	Type       string             `json:"type"`
	JSONSchema *jsonSchemaWrapper `json:"json_schema,omitempty"`
}

type jsonSchemaWrapper struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
