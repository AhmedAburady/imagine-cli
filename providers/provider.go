// Package providers defines the Provider abstraction, request/response shapes,
// and the registry into which concrete providers (Gemini, Vertex, OpenAI, …)
// self-register via init().
package providers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/AhmedAburady/imagine-cli/internal/images"
)

// ResolveModel translates a raw user-supplied model string (alias or full ID)
// into the canonical ID declared in Info.Models. Empty input returns the
// provider's DefaultModel.
func (i Info) ResolveModel(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return i.DefaultModel, nil
	}
	var accepted []string
	for _, m := range i.Models {
		if m.ID == raw {
			return m.ID, nil
		}
		if slices.Contains(m.Aliases, raw) {
			return m.ID, nil
		}
		accepted = append(accepted, m.ID)
		accepted = append(accepted, m.Aliases...)
	}
	return "", fmt.Errorf("unknown model %q for provider %q (accepted: %v)", raw, i.Name, accepted)
}

// Info is the static metadata a provider advertises to the CLI: its name,
// supported models, and capability flags. Built once per provider.
type Info struct {
	Name         string
	DisplayName  string
	Summary      string
	DefaultModel string
	Models       []ModelInfo
	Capabilities Capabilities
}

// ModelInfo describes one model a provider exposes. Aliases are CLI-friendly
// shorthands ("pro" → "gemini-3-pro-image-preview"). SupportedFlags is the
// subset of optional flags this model honours (e.g. Gemini flash supports
// thinking and image-search; pro does not).
type ModelInfo struct {
	ID             string
	Aliases        []string
	Description    string
	SupportedFlags []string
}

// Capabilities tells the CLI what orchestration / validation rules apply.
type Capabilities struct {
	Edit        bool     // supports reference images
	Grounding   bool     // supports Google Search grounding
	Thinking    bool     // supports thinking level
	ImageSearch bool     // supports image-search grounding
	MaxBatchN   int      // images per single Generate call; 1 means orchestrator loops
	Sizes       []string // accepted values for -s
}

// Request is the per-batch input to a provider's Generate call.
// N ≤ Capabilities.MaxBatchN. Everything else the provider needs
// (model, size, aspect ratio, grounding, quality, …) travels in Options
// as a provider-private value: the Bundle.ReadFlags harvester produced it,
// and the provider's Generate is the only thing that type-asserts it.
// Typed providers use a *XOptions struct; legacy providers may still use
// map[string]any.
type Request struct {
	Prompt     string
	N          int
	References []images.Reference
	Options    any
}

// GeneratedImage is a single produced image: raw bytes + MIME type.
type GeneratedImage struct {
	Data     []byte
	MimeType string
}

// Response is one Generate call's output.
type Response struct {
	Images []GeneratedImage
}

// Provider is the interface the CLI uses to talk to any image backend.
// All providers take ctx for cancellation; orchestration lives outside.
type Provider interface {
	Info() Info
	Generate(ctx context.Context, req Request) (*Response, error)
}

// RequestLabeler is an optional interface a provider's Options type may
// implement to supply a short human-readable label (typically the resolved
// model alias or ID) for status output. When unset the CLI falls back to
// just the provider name.
type RequestLabeler interface {
	RequestLabel() string
}

// ResolvedModeler is an optional interface a provider's Options type may
// implement so the framework can read the canonical model ID after flag
// parsing — used by the model-level flag-support gate. Kept separate from
// RequestLabeler because that method is for display and could legitimately
// return a decorated string ("flash+grounding"); ResolvedModel must return
// the bare canonical ID that matches Info.Models[*].ID.
type ResolvedModeler interface {
	ResolvedModel() string
}

// ConfigField describes one configurable field for a provider — used by
// `imagine providers add` to render dynamic forms and synthesise the
// per-invocation flag set. The Key is the on-disk storage key
// (providers.<name>.<Key> in config.yaml) and — dashed — the CLI flag
// name (api_key → --api-key).
//
// Providers ship their schema as a slice attached to Bundle.ConfigSchema
// at registration time (see providers/registry.go). Doing it via the
// Bundle avoids instantiating the provider — the Factory legitimately
// rejects empty auth, which would prevent schema introspection during
// onboarding when no auth exists yet.
type ConfigField struct {
	Key         string // storage key; flag becomes --<Key-with-dashes>
	Title       string // e.g. "API Key", "GCP Project"
	Description string // one-line help shown in the form and in --help
	Secret      bool   // mask input (EchoModePassword) in interactive mode
	Required    bool
	Default     string // used as form default and for flag default
}

// Vision declares describe capability on the Bundle. Non-nil means the
// Provider also implements Describer.
type Vision struct {
	DefaultModel string
}

// Describer is implemented by providers that analyse images.
type Describer interface {
	Describe(ctx context.Context, req DescribeRequest) (*ImageDescription, error)
}

type DescribeRequest struct {
	Images           []images.Reference
	CustomPrompt     string
	Additional       string
	Model            string
	StructuredOutput bool
}

type ImageDescription struct {
	Text       string
	Structured *StyleAnalysis
}

type StyleAnalysis struct {
	StyleName    string   `json:"style_name"`
	Description  string   `json:"description"`
	StyleSummary string   `json:"style_summary"`
	Colors       []string `json:"colors"`
	Medium       string   `json:"medium"`
	Composition  string   `json:"composition,omitempty"`
	KeyElements  []string `json:"key_elements,omitempty"`
	Avoid        []string `json:"avoid,omitempty"`
}
