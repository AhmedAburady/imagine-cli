package describe

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AhmedAburady/banana-cli/config"
	"google.golang.org/genai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
)

// StyleAnalysis represents the structured output for -json flag
type StyleAnalysis struct {
	StyleName    string   `json:"style_name"`             // Identified or provided style name
	Description  string   `json:"description"`            // Detailed style description
	StyleSummary string   `json:"style_summary"`          // One-line classification
	Colors       []string `json:"colors"`                 // Hex color codes
	Medium       string   `json:"medium"`                 // Art medium
	Composition  string   `json:"composition,omitempty"`  // Layout style
	KeyElements  []string `json:"key_elements,omitempty"` // Key visual elements
	Avoid        []string `json:"avoid,omitempty"`        // Elements to avoid
}

// DescriptionResult holds the output from the describe agent
type DescriptionResult struct {
	Text     string         // Plain text output
	Analysis *StyleAnalysis // Structured output (when -json)
	IsJSON   bool
}

// DescribeAgent wraps the ADK agent for image description
type DescribeAgent struct {
	apiKey    string
	useVertex bool
}

// NewDescribeAgent creates a new ADK-powered describe agent
func NewDescribeAgent(ctx context.Context, apiKey string, useVertex bool) (*DescribeAgent, error) {
	return &DescribeAgent{
		apiKey:    apiKey,
		useVertex: useVertex,
	}, nil
}

// defaultTextInstruction returns instruction for plain text output
func defaultTextInstruction() string {
	return `You are an expert image style analyst. Analyze the provided image(s) and create a detailed description that captures the visual style.

When multiple images are provided, identify the UNIFIED style elements across all images - treat them as style references to extract a cohesive style description.

Your description should be detailed enough that it can be used as a prompt to recreate this exact style. Focus on what you actually SEE in the image(s):

- Visual characteristics (colors, shapes, patterns, textures)
- Art style if applicable (illustration, photography, 3D render, vector art, etc.)
- Mood and atmosphere
- Any distinctive visual elements
- Composition and layout characteristics

Be specific and descriptive. The description you write will be used directly as a generation prompt, so make it actionable and clear.

IMPORTANT: Only describe what is actually present in the image. Don't make up elements that aren't there. If something like lighting or perspective isn't relevant (e.g., flat vector art), don't mention it.

Output only the description text, nothing else. No preamble, no "Here is the description:", just the description itself.`
}

// jsonOutputInstruction returns instruction for structured JSON output
func jsonOutputInstruction() string {
	return `Analyze the image style. Respond ONLY with a JSON object matching the schema. Be concise and specific.`
}

// createOutputSchema builds the JSON schema for structured output
func createOutputSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"style_name":    {Type: genai.TypeString, Description: "Identified or extracted style name for the references. EXTRACT it properly if provided with no special characters or Identify from the reference images NO SPECIAL CHARACTERS NO NUMBERS NO UNDERSCORES"},
			"description":   {Type: genai.TypeString, Description: "Detailed style description"},
			"style_summary": {Type: genai.TypeString, Description: "One-line style classification"},
			"colors":        {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "Hex color codes from the image"},
			"medium":        {Type: genai.TypeString, Description: "Art medium (digital, vector, watercolor, etc.)"},
			"composition":   {Type: genai.TypeString, Description: "Layout and arrangement style"},
			"key_elements":  {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "Key visual elements that define this style"},
			"avoid":         {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}, Description: "Elements to avoid when recreating"},
		},
		Required: []string{"style_name", "description", "style_summary", "colors", "medium"},
	}
}

// extractTextFromParts extracts text content from genai.Part slice
func extractTextFromParts(parts []*genai.Part) string {
	for _, part := range parts {
		if part == nil {
			continue
		}
		if part.Text != "" {
			return part.Text
		}
	}
	return ""
}

// DescribeImages analyzes images using ADK agent
func (a *DescribeAgent) DescribeImages(ctx context.Context, imageParts []*genai.Part, customPrompt string, additional string, jsonOutput bool) (*DescriptionResult, error) {
	var clientConfig *genai.ClientConfig
	if a.useVertex {
		project := config.GetGCPProject()
		if project == "" {
			return nil, fmt.Errorf("GCP project is required for Vertex AI. Set GOOGLE_CLOUD_PROJECT env var or run: banana config set-project <PROJECT_ID>")
		}
		location := config.GetGCPLocation()
		clientConfig = &genai.ClientConfig{
			Project:  project,
			Location: location,
			Backend:  genai.BackendVertexAI,
		}
	} else {
		clientConfig = &genai.ClientConfig{
			APIKey: a.apiKey,
		}
	}

	llmModel, err := gemini.NewModel(ctx, "gemini-3-pro-preview", clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	// Determine instruction: -p overrides default, -a prepends context to default
	var instruction string
	if customPrompt != "" {
		// -p flag: completely override default instruction
		instruction = customPrompt
	} else if jsonOutput {
		instruction = jsonOutputInstruction()
	} else {
		instruction = defaultTextInstruction()
	}

	// -a flag: prepend additional instructions so they take priority
	if additional != "" {
		instruction = "CRITICAL USER CONTEXT - You MUST incorporate this into your analysis:\n" + additional + "\n\n" + instruction
	}

	// Create agent config
	agentConfig := llmagent.Config{
		Name:        "style_analyzer",
		Model:       llmModel,
		Description: "Analyzes images and extracts style descriptions",
		Instruction: instruction,
	}

	if jsonOutput {
		// JSON mode: use ADK's OutputSchema (disables tools, enforces JSON)
		agentConfig.OutputSchema = createOutputSchema()
	} else {
		// Text mode: use thinking for better analysis
		agentConfig.GenerateContentConfig = &genai.GenerateContentConfig{
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingLevel: genai.ThinkingLevelHigh,
			},
		}
	}

	describeAgent, err := llmagent.New(agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	sessionService := session.InMemoryService()

	// Create session before running
	_, err = sessionService.Create(ctx, &session.CreateRequest{
		AppName:   "banana-describe",
		UserID:    "cli-user",
		SessionID: "describe-session",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	r, err := runner.New(runner.Config{
		AppName:        "banana-describe",
		Agent:          describeAgent,
		SessionService: sessionService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	parts := make([]*genai.Part, 0, len(imageParts)+1)
	parts = append(parts, imageParts...)
	parts = append(parts, genai.NewPartFromText("Analyze the style of the provided image(s)."))

	userContent := &genai.Content{
		Role:  "user",
		Parts: parts,
	}

	var resultText string
	eventCount := 0
	for event, err := range r.Run(ctx, "cli-user", "describe-session", userContent, agent.RunConfig{}) {
		eventCount++
		if err != nil {
			return nil, fmt.Errorf("agent run error (after %d events): %w", eventCount, err)
		}
		if event.IsFinalResponse() && event.Content != nil {
			resultText = extractTextFromParts(event.Content.Parts)
			if resultText != "" {
				break
			}
		}
	}

	if resultText == "" {
		if eventCount == 0 {
			return nil, fmt.Errorf("no response from API - connection may have failed")
		}
		return nil, fmt.Errorf("no description generated (received %d events but no final response)", eventCount)
	}

	return parseResult(resultText, jsonOutput)
}

// parseResult parses the model output into DescriptionResult
func parseResult(text string, isStructured bool) (*DescriptionResult, error) {
	result := &DescriptionResult{IsJSON: isStructured}

	if isStructured {
		var analysis StyleAnalysis
		if err := json.Unmarshal([]byte(text), &analysis); err != nil {
			// If JSON parsing fails, return as plain text
			result.Text = text
			result.IsJSON = false
		} else {
			result.Analysis = &analysis
		}
	} else {
		result.Text = text
	}

	return result, nil
}

// FormatOutput returns the result formatted for display
func (r *DescriptionResult) FormatOutput() string {
	if r.IsJSON && r.Analysis != nil {
		data, _ := json.MarshalIndent(r.Analysis, "", "  ")
		return string(data)
	}
	return r.Text
}
