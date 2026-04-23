// Package openai implements the Provider interface for OpenAI's GPT Image
// models, using the /v1/images endpoints (API key auth).
package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/internal/transport"
	"github.com/AhmedAburady/imagine-cli/providers"
)

const (
	baseURL         = "https://api.openai.com/v1"
	defaultModel    = "gpt-image-2"
	generationsPath = "/images/generations"
	editsPath       = "/images/edits"
)

// httpClient uses a longer timeout than Gemini — OpenAI docs note that
// complex prompts may take up to 2 minutes.
var httpClient = transport.NewClient(180 * time.Second)

// Provider is the OpenAI Images implementation of providers.Provider.
type Provider struct {
	apiKey      string
	visionModel string
}

// New builds an OpenAI provider from auth.
func New(auth providers.Auth) (providers.Provider, error) {
	key := auth.Get("api_key")
	if key == "" {
		return nil, errors.New("openai provider requires providers.openai.api_key in ~/.config/imagine/config.yaml")
	}
	return &Provider{apiKey: key, visionModel: auth.Get("vision_model")}, nil
}

// ConfigSchema declares the fields `imagine providers add openai` collects.
func (p *Provider) ConfigSchema() []providers.ConfigField {
	return []providers.ConfigField{
		{
			Key:         "api_key",
			Title:       "API Key",
			Description: "OpenAI API key from platform.openai.com",
			Secret:      true,
			Required:    true,
		},
		{
			Key:         "vision_model",
			Title:       "Vision Model",
			Description: "Model for `imagine describe` (any GPT-5.4 variant)",
			Default:     DefaultVisionModel,
		},
	}
}

// Info advertises OpenAI's models + capabilities.
func (p *Provider) Info() providers.Info {
	return providers.Info{
		Name:         "openai",
		DisplayName:  "OpenAI",
		Summary:      "OpenAI GPT Image models via api.openai.com",
		DefaultModel: defaultModel,
		Models: []providers.ModelInfo{
			{ID: "gpt-image-2", Aliases: []string{"2"}, Description: "Flagship GPT Image model. Flexible sizes, high-fidelity inputs."},
			{ID: "gpt-image-1.5", Aliases: []string{"1.5"}, Description: "Previous flagship; stable."},
			{ID: "gpt-image-1", Aliases: []string{"1"}, Description: "First generation."},
			{ID: "gpt-image-1-mini", Aliases: []string{"mini", "1-mini"}, Description: "Fastest, cheapest."},
			{ID: "chatgpt-image-latest", Aliases: []string{"latest"}, Description: "ChatGPT-variant latest."},
		},
		Capabilities: providers.Capabilities{
			Edit:        true,
			Grounding:   false,
			Thinking:    false,
			ImageSearch: false,
			MaxBatchN:   10, // /v1/images supports up to 10 per call
		},
	}
}

// Generate calls /v1/images/generations (pure generate) or /v1/images/edits
// (when References are present).
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	opts, ok := req.Options.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("openai: internal: expected map[string]any options, got %T", req.Options)
	}
	model, _ := opts["model"].(string)
	size, _ := opts["size"].(string)
	quality, _ := opts["quality"].(string)
	outputFormat, _ := opts["output_format"].(string)
	moderation, _ := opts["moderation"].(string)
	background, _ := opts["background"].(string)
	compression, _ := opts["compression"].(int)

	// Edit mode when references are present.
	if len(req.References) > 0 {
		return p.edit(ctx, editRequest{
			Model:        model,
			Prompt:       req.Prompt,
			N:            req.N,
			Size:         size,
			Quality:      quality,
			OutputFormat: outputFormat,
			Compression:  compression,
			Background:   background,
			References:   req.References,
		})
	}

	return p.generate(ctx, generateRequest{
		Model:        model,
		Prompt:       req.Prompt,
		N:            req.N,
		Size:         size,
		Quality:      quality,
		OutputFormat: outputFormat,
		Compression:  compression,
		Moderation:   moderation,
		Background:   background,
	})
}

// -- Generate (JSON) ----------------------------------------------------------

type generateRequest struct {
	Model        string
	Prompt       string
	N            int
	Size         string
	Quality      string
	OutputFormat string
	Compression  int
	Moderation   string
	Background   string
}

type generationsBody struct {
	Model             string `json:"model"`
	Prompt            string `json:"prompt"`
	N                 int    `json:"n,omitempty"`
	Size              string `json:"size,omitempty"`
	Quality           string `json:"quality,omitempty"`
	OutputFormat      string `json:"output_format,omitempty"`
	OutputCompression *int   `json:"output_compression,omitempty"`
	Moderation        string `json:"moderation,omitempty"`
	Background        string `json:"background,omitempty"`
}

type generationsResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

func (p *Provider) generate(ctx context.Context, r generateRequest) (*providers.Response, error) {
	body := generationsBody{
		Model:        r.Model,
		Prompt:       r.Prompt,
		N:            r.N,
		Size:         r.Size,
		Quality:      emptyToAuto(r.Quality),
		OutputFormat: r.OutputFormat,
		Moderation:   r.Moderation,
		Background:   r.Background,
	}
	if (r.OutputFormat == "jpeg" || r.OutputFormat == "webp") && r.Compression > 0 && r.Compression < 100 {
		c := r.Compression
		body.OutputCompression = &c
	}

	resp, err := transport.PostJSON[generationsResponse](ctx, httpClient, baseURL+generationsPath, transport.Bearer(p.apiKey), body)
	if err != nil {
		return nil, err
	}
	return decodeImages(resp, mimeTypeFor(r.OutputFormat))
}

// -- Edit (multipart) ---------------------------------------------------------

type editRequest struct {
	Model        string
	Prompt       string
	N            int
	Size         string
	Quality      string
	OutputFormat string
	Compression  int
	Background   string
	References   []images.Reference
}

func (p *Provider) edit(ctx context.Context, r editRequest) (*providers.Response, error) {
	// Edit endpoint constraint: size must be one of 1024x1024, 1536x1024,
	// 1024x1536, auto. The flag layer maps 1K etc. to dimensions; reject
	// anything else client-side.
	switch r.Size {
	case "", "auto", "1024x1024", "1536x1024", "1024x1536":
		// ok
	default:
		return nil, fmt.Errorf("openai edit endpoint only accepts size 1024x1024, 1536x1024, 1024x1536, or auto (got %q)", r.Size)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	write := func(name, value string) error {
		if value == "" {
			return nil
		}
		return w.WriteField(name, value)
	}

	if err := write("model", r.Model); err != nil {
		return nil, err
	}
	if err := write("prompt", r.Prompt); err != nil {
		return nil, err
	}
	if r.N > 0 {
		if err := write("n", fmt.Sprintf("%d", r.N)); err != nil {
			return nil, err
		}
	}
	if err := write("size", r.Size); err != nil {
		return nil, err
	}
	if err := write("quality", emptyToAuto(r.Quality)); err != nil {
		return nil, err
	}
	if err := write("output_format", r.OutputFormat); err != nil {
		return nil, err
	}
	if err := write("background", r.Background); err != nil {
		return nil, err
	}
	if (r.OutputFormat == "jpeg" || r.OutputFormat == "webp") && r.Compression > 0 && r.Compression < 100 {
		if err := write("output_compression", fmt.Sprintf("%d", r.Compression)); err != nil {
			return nil, err
		}
	}

	for i, ref := range r.References {
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="ref%d%s"`, i, extForMime(ref.MimeType)))
		partHeader.Set("Content-Type", ref.MimeType)
		fw, err := w.CreatePart(partHeader)
		if err != nil {
			return nil, fmt.Errorf("failed to create multipart part: %w", err)
		}
		if _, err := fw.Write(ref.Data); err != nil {
			return nil, fmt.Errorf("failed to write reference bytes: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize multipart: %w", err)
	}

	resp, err := transport.PostMultipart[generationsResponse](ctx, httpClient, baseURL+editsPath, transport.Bearer(p.apiKey), &buf, w.FormDataContentType())
	if err != nil {
		return nil, err
	}
	return decodeImages(resp, mimeTypeFor(r.OutputFormat))
}

// -- Shared ------------------------------------------------------------------

// decodeImages unpacks /v1/images responses (generations + edits share the
// same data[].b64_json shape). outMime is applied to every emitted image.
func decodeImages(parsed *generationsResponse, outMime string) (*providers.Response, error) {
	imgs := make([]providers.GeneratedImage, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		if d.B64JSON == "" {
			continue
		}
		data, err := transport.DecodeB64(d.B64JSON)
		if err != nil {
			return nil, err
		}
		imgs = append(imgs, providers.GeneratedImage{Data: data, MimeType: outMime})
	}
	if len(imgs) == 0 {
		return nil, errors.New("openai returned no images")
	}
	return &providers.Response{Images: imgs}, nil
}

func emptyToAuto(s string) string {
	if s == "" {
		return "auto"
	}
	return s
}

func mimeTypeFor(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "image/jpeg"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func extForMime(m string) string {
	switch m {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
}
