// Package providers defines the Provider abstraction, request/response shapes,
// and the registry into which concrete providers (Gemini, Vertex, OpenAI, …)
// self-register via init().
package providers

import (
	"context"

	"github.com/AhmedAburady/imagine-cli/internal/images"
)

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
// N ≤ Capabilities.MaxBatchN.
type Request struct {
	Prompt      string
	N           int
	Model       string // resolved provider-specific model ID (aliases already expanded)
	Size        string
	AspectRatio string
	References  []images.Reference
	Options     map[string]any // provider-specific parsed flags (grounding, thinking, quality, …)
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
