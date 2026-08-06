// Package openai implements the Provider interface for OpenAI's GPT Image
// models. It supports two mutually-exclusive auth methods, selected at
// `providers add` time and recorded as auth_method: an API key billed via
// platform.openai.com (the /v1/images endpoints), or a ChatGPT subscription
// signed in via OAuth (the Codex Responses route). The route is chosen per
// Provider instance; everything below dispatches on p.method.
package openai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
	"sync"
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

// authMethod selects the credential type / transport for one Provider instance.
type authMethod string

const (
	methodAPIKey       authMethod = "api_key"
	methodSubscription authMethod = "subscription"
)

// httpClient serves the API-key path: one JSON blob, so a total request cap fits.
var httpClient = transport.NewClient(180 * time.Second)

// streamClient serves the subscription SSE path: bounded by connection-phase timeouts + the stall watchdog, no total cap.
var streamClient = transport.NewStreamingClient()

// Provider is the OpenAI Images implementation of providers.Provider. It holds
// the credentials for whichever auth method is active; mu/cached guard the
// subscription token store.
type Provider struct {
	method      authMethod
	apiKey      string // api_key method
	visionModel string
	authFile    string // subscription token store path

	mu     sync.Mutex
	cached *storedAuth
}

// New builds a Provider for the configured auth method. Construction does not
// require the subscription token (checked at call time so help/metadata work
// pre-sign-in), but the API-key method still fails fast on a missing key.
func New(auth providers.Auth) (providers.Provider, error) {
	p := &Provider{
		method:      resolveMethod(auth),
		apiKey:      auth.Get("api_key"),
		visionModel: auth.Get("vision_model"),
		authFile:    subscriptionAuthFile(auth.Get("auth_file")),
	}
	if p.method == methodAPIKey && p.apiKey == "" {
		return nil, errors.New("openai (api_key) requires an API key — run `imagine providers add openai`, or set providers.openai.api_key")
	}
	return p, nil
}

// resolveMethod reads auth_method, inferring it for configs written before that
// key existed: an api_key present → api_key; else an existing token store →
// subscription; else api_key (New then errors with guidance).
func resolveMethod(auth providers.Auth) authMethod {
	switch authMethod(auth.Get("auth_method")) {
	case methodSubscription:
		return methodSubscription
	case methodAPIKey:
		return methodAPIKey
	}
	if auth.Get("api_key") != "" {
		return methodAPIKey
	}
	if subscriptionConfigured(auth.Get("auth_file")) {
		return methodSubscription
	}
	return methodAPIKey
}

// ConfigSchema is the field set for the API-key method (reused as that method's
// AuthMethod.Fields in register.go).
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
			Description: "Model for `imagine describe` (default: gpt-5.5)",
			Default:     DefaultVisionModel,
		},
	}
}

// Info advertises OpenAI's models; gpt-image-2 is the only one not shut down.
// MaxBatchN is 10 on the API-key endpoint, 1 on the subscription Responses tool.
func (p *Provider) Info() providers.Info {
	maxBatch := 10
	if p.method == methodSubscription {
		maxBatch = 1
	}
	return providers.Info{
		Name:         "openai",
		DisplayName:  "OpenAI",
		Summary:      "OpenAI GPT Image models — API key or ChatGPT subscription",
		DefaultModel: defaultModel,
		Models: []providers.ModelInfo{
			{ID: defaultModel, Aliases: []string{"2"}, Description: "Flagship GPT Image model. Flexible sizes, high-fidelity inputs."},
		},
		Capabilities: providers.Capabilities{
			Edit:      true,
			MaxBatchN: maxBatch,
		},
	}
}

// Generate dispatches to the active method's transport.
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
	opts, ok := req.Options.(*Options)
	if !ok {
		return nil, fmt.Errorf("openai: internal: expected *Options, got %T", req.Options)
	}
	if p.method == methodSubscription {
		return p.generateSubscription(ctx, opts, req)
	}
	return p.generateAPIKey(ctx, opts, req)
}

// generateAPIKey calls /v1/images/generations or /v1/images/edits (edit mode
// when References are present).
func (p *Provider) generateAPIKey(ctx context.Context, opts *Options, req providers.Request) (*providers.Response, error) {
	if len(req.References) > 0 {
		return p.edit(ctx, editRequest{
			Model:        opts.Model,
			Prompt:       req.Prompt,
			N:            req.N,
			Size:         opts.Size,
			Quality:      opts.Quality,
			OutputFormat: opts.OutputFormat,
			Compression:  opts.Compression,
			Background:   opts.Background,
			References:   req.References,
		})
	}

	return p.generate(ctx, generateRequest{
		Model:        opts.Model,
		Prompt:       req.Prompt,
		N:            req.N,
		Size:         opts.Size,
		Quality:      opts.Quality,
		OutputFormat: opts.OutputFormat,
		Compression:  opts.Compression,
		Moderation:   opts.Moderation,
		Background:   opts.Background,
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
		body.OutputCompression = new(r.Compression)
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
