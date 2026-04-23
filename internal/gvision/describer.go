// Package gvision provides the shared Gemini-based describe implementation
// consumed by both the gemini (direct REST) and vertex (ADC) providers.
package gvision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// Describe runs an ADK agent against the given genai ClientConfig and
// returns an ImageDescription.
func Describe(ctx context.Context, cc *genai.ClientConfig, model string, req providers.DescribeRequest) (*providers.ImageDescription, error) {
	if len(req.Images) == 0 {
		return nil, fmt.Errorf("no images provided")
	}

	llm, err := gemini.NewModel(ctx, model, cc)
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}

	instruction := pickInstruction(req)

	cfg := llmagent.Config{
		Name:        "style_analyzer",
		Model:       llm,
		Description: "Analyses images and extracts style descriptions",
		Instruction: instruction,
	}
	if req.StructuredOutput {
		cfg.OutputSchema = styleSchema()
	} else {
		cfg.GenerateContentConfig = &genai.GenerateContentConfig{
			ThinkingConfig: &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelHigh},
		}
	}

	ag, err := llmagent.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	sess := session.InMemoryService()
	if _, err := sess.Create(ctx, &session.CreateRequest{
		AppName: "imagine-describe", UserID: "cli-user", SessionID: "describe-session",
	}); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        "imagine-describe",
		Agent:          ag,
		SessionService: sess,
	})
	if err != nil {
		return nil, fmt.Errorf("create runner: %w", err)
	}

	parts := toParts(req.Images)
	parts = append(parts, genai.NewPartFromText("Analyze the style of the provided image(s)."))

	var text string
	events := 0
	for event, err := range r.Run(ctx, "cli-user", "describe-session", &genai.Content{Role: "user", Parts: parts}, agent.RunConfig{}) {
		events++
		if err != nil {
			return nil, fmt.Errorf("agent run (after %d events): %w", events, err)
		}
		if event.IsFinalResponse() && event.Content != nil {
			text = firstText(event.Content.Parts)
			if text != "" {
				break
			}
		}
	}
	if text == "" {
		return nil, fmt.Errorf("no description generated (received %d events)", events)
	}

	return parse(text, req.StructuredOutput), nil
}

func toParts(refs []images.Reference) []*genai.Part {
	out := make([]*genai.Part, 0, len(refs))
	for _, r := range refs {
		out = append(out, genai.NewPartFromBytes(r.Data, r.MimeType))
	}
	return out
}

func firstText(parts []*genai.Part) string {
	for _, p := range parts {
		if p != nil && p.Text != "" {
			return p.Text
		}
	}
	return ""
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

func parse(text string, structured bool) *providers.ImageDescription {
	if !structured {
		return &providers.ImageDescription{Text: text}
	}
	var s providers.StyleAnalysis
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &s); err != nil {
		return &providers.ImageDescription{Text: text}
	}
	return &providers.ImageDescription{Structured: &s}
}

func styleSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"style_name":    {Type: genai.TypeString, Description: "Style name. EXTRACT properly if provided, otherwise identify. No special characters, numbers, or underscores."},
			"description":   {Type: genai.TypeString, Description: "Detailed style description"},
			"style_summary": {Type: genai.TypeString, Description: "One-line style classification"},
			"colors":        {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "Hex color codes"},
			"medium":        {Type: genai.TypeString, Description: "Art medium (digital, vector, watercolor, ...)"},
			"composition":   {Type: genai.TypeString, Description: "Layout and arrangement"},
			"key_elements":  {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "Key visual elements that define this style"},
			"avoid":         {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "Elements to avoid when recreating"},
		},
		Required: []string{"style_name", "description", "style_summary", "colors", "medium"},
	}
}

// TextInstruction / JSONInstruction are the built-in prompts used when the
// caller doesn't pass a custom one. Exported so `imagine describe
// --show-instructions` can display them.
const TextInstruction = `You are an expert image style analyst. Analyze the provided image(s) and create a detailed description that captures the visual style.

When multiple images are provided, identify the UNIFIED style elements across all images - treat them as style references to extract a cohesive style description.

Your description should be detailed enough to be used as a prompt to recreate this exact style. Focus on what you actually SEE:

- Visual characteristics (colors, shapes, patterns, textures)
- Art style if applicable (illustration, photography, 3D render, vector art, etc.)
- Mood and atmosphere
- Distinctive visual elements
- Composition and layout characteristics

Be specific and descriptive. The description you write will be used directly as a generation prompt, so make it actionable and clear.

IMPORTANT: Only describe what is actually present in the image. Don't invent elements. If something like lighting or perspective isn't relevant (e.g., flat vector art), don't mention it.

Output only the description text, no preamble.`

const JSONInstruction = `Analyze the image style. Respond ONLY with a JSON object matching the schema. Be concise and specific.`
