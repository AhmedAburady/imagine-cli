package openai

// Subscription-route describe: send the image(s) + analysis instruction over
// the Responses stream (no image tool) and read back the model's text.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/AhmedAburady/imagine-cli/providers"
)

const describeSystem = "You are an expert image style analyst. Follow the user's instruction exactly and output only what it asks for, with no preamble."

func (p *Provider) describeSubscription(ctx context.Context, req providers.DescribeRequest) (*providers.ImageDescription, error) {
	access, account, err := p.ensureFreshToken(ctx)
	if err != nil {
		return nil, err
	}
	auth := &codexAuth{accessToken: access, accountID: account, sessionID: uuid.NewString()}

	model := req.Model
	if model == "" {
		model = p.visionModel
	}
	if model == "" {
		model = DefaultVisionModel
	}

	instruction := providers.PickInstruction(req, TextInstruction, JSONInstruction)
	body := responsesBody{
		Model:        model,
		Instructions: describeSystem,
		Input:        []inputMessage{{Role: "user", Content: responsesContent(instruction, req.Images)}},
		Stream:       true,
	}

	resp, err := p.postResponses(ctx, auth, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	text, err := parseTextStream(resp.Body)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("openai subscription returned an empty description")
	}

	if !req.StructuredOutput {
		return &providers.ImageDescription{Text: text}, nil
	}
	// Structured mode requests JSON via the prompt (no enforced schema on this
	// route); fall back to raw text if the model didn't comply.
	var s providers.StyleAnalysis
	if json.Unmarshal([]byte(stripCodeFence(text)), &s) == nil && s.StyleName != "" {
		return &providers.ImageDescription{Structured: &s}, nil
	}
	return &providers.ImageDescription{Text: text}, nil
}

// parseTextStream returns the assistant text from the terminal
// response.output_text.done event; error events short-circuit.
func parseTextStream(r io.Reader) (string, error) {
	var text string
	err := scanSSE(r, func(ev *sseEvent) (bool, error) {
		if msg := eventError(ev); msg != "" {
			return false, errors.New(msg)
		}
		if ev.Type == "response.output_text.done" && ev.Text != "" {
			text = ev.Text
		}
		return false, nil
	})
	return text, err
}

// stripCodeFence removes a ```json ... ``` wrapper the model sometimes adds.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
