package api

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/AhmedAburady/banana-cli/config"
	"google.golang.org/genai"
)

// getVertexConfig returns project and location from env vars or config file
// Priority: env vars > config file
func getVertexConfig() (project, location string, err error) {
	project = config.GetGCPProject()
	if project == "" {
		return "", "", fmt.Errorf("GCP project is required for Vertex AI. Set GOOGLE_CLOUD_PROJECT env var or run: banana config set-project <PROJECT_ID>")
	}

	location = config.GetGCPLocation()
	return project, location, nil
}

// GenerateImageVertex performs a single image generation request using Vertex AI
func GenerateImageVertex(config *Config, index int) GenerationResult {
	ctx := context.Background()

	project, location, err := getVertexConfig()
	if err != nil {
		return GenerationResult{Index: index, Error: err}
	}

	// Create Vertex AI client using Application Default Credentials
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  project,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("failed to create Vertex AI client: %w", err)}
	}

	// Build content parts
	var parts []*genai.Part
	parts = append(parts, genai.NewPartFromText(config.Prompt))

	// Add reference images if in edit mode
	for _, refImg := range config.RefImages {
		if refImg.InlineData != nil {
			// Decode base64 image data
			imageData, err := base64.StdEncoding.DecodeString(refImg.InlineData.Data)
			if err != nil {
				return GenerationResult{Index: index, Error: fmt.Errorf("failed to decode reference image: %w", err)}
			}
			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: refImg.InlineData.MimeType,
					Data:     imageData,
				},
			})
		}
	}

	contents := []*genai.Content{
		{
			Parts: parts,
			Role:  "user",
		},
	}

	// Configure generation settings
	genConfig := &genai.GenerateContentConfig{
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}

	// Add image configuration if aspect ratio or size is specified
	if config.AspectRatio != "" || config.ImageSize != "" {
		imgConfig := &genai.ImageConfig{}
		if config.AspectRatio != "" {
			imgConfig.AspectRatio = config.AspectRatio
		}
		if config.ImageSize != "" {
			imgConfig.ImageSize = config.ImageSize
		}
		genConfig.ImageConfig = imgConfig
	}

	// Add tools (google search grounding)
	if config.Grounding {
		genConfig.Tools = append(genConfig.Tools, &genai.Tool{GoogleSearch: &genai.GoogleSearch{}})
	}

	// Add thinking config if specified
	if config.ThinkingLevel != "" {
		genConfig.ThinkingConfig = &genai.ThinkingConfig{
			ThinkingLevel: genai.ThinkingLevel(config.ThinkingLevel),
		}
	}

	// Call the API
	resp, err := client.Models.GenerateContent(ctx, config.Model, contents, genConfig)
	if err != nil {
		return GenerationResult{Index: index, Error: fmt.Errorf("generation failed: %w", err)}
	}

	// Extract image from response
	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && len(part.InlineData.Data) > 0 {
				return GenerationResult{Index: index, ImageData: part.InlineData.Data}
			}
		}
	}

	// No image found - check if there's text explaining why
	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				return GenerationResult{Index: index, Error: fmt.Errorf("no image in response. API said: %s", part.Text)}
			}
		}
	}

	return GenerationResult{Index: index, Error: fmt.Errorf("no image in response")}
}
