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

// MediaKind classifies the output (and reference) media a provider handles.
type MediaKind int

const (
	KindImage MediaKind = iota // zero value = image
	KindVideo
	KindAudio
)

// Class maps a MediaKind to its top-level MIME class string.
func (k MediaKind) Class() string {
	switch k {
	case KindVideo:
		return "video"
	case KindAudio:
		return "audio"
	default:
		return "image"
	}
}

// RefClasses returns the accepted reference MIME classes; empty RefKinds means image-only.
func (c Capabilities) RefClasses() []string {
	if len(c.RefKinds) == 0 {
		return []string{"image"}
	}
	out := make([]string, len(c.RefKinds))
	for i, k := range c.RefKinds {
		out[i] = k.Class()
	}
	return out
}

// Capabilities tells the CLI what orchestration / validation rules apply.
type Capabilities struct {
	Edit        bool        // supports reference images
	Grounding   bool        // supports Google Search grounding
	Thinking    bool        // supports thinking level
	ImageSearch bool        // supports image-search grounding
	MaxBatchN   int         // images per single Generate call; 1 means orchestrator loops
	Sizes       []string    // accepted values for -s
	MediaKind   MediaKind   // output kind; zero value = image
	RefKinds    []MediaKind // accepted reference classes; nil = image-only
	MaxN        int         // per-provider cap on -n; 0 = use the global limit
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

// GeneratedAsset is a single produced asset: raw bytes + MIME type.
type GeneratedAsset struct {
	Data     []byte
	MimeType string
}

// GeneratedImage is a back-compat alias for GeneratedAsset.
type GeneratedImage = GeneratedAsset

// Response is one Generate call's output.
type Response struct {
	Assets []GeneratedAsset
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

	// Efforts are the reasoning/thinking levels this describer accepts, in
	// display order; empty means the provider exposes no effort control.
	Efforts []string

	// DefaultEffort is applied when --effort is omitted; must be in Efforts.
	DefaultEffort string
}

// NormalizeEffortForModel validates against the default model's set; a custom model gets an explicit effort passed through (API validates) and no forced default.
func (v *Vision) NormalizeEffortForModel(effort, model string) (string, error) {
	if model != "" && model != v.DefaultModel {
		return strings.ToLower(strings.TrimSpace(effort)), nil
	}
	return v.NormalizeEffort(effort)
}

// NormalizeEffort lowercases effort, applies DefaultEffort when empty, and
// validates against Efforts — one validation path shared by every describer.
func (v *Vision) NormalizeEffort(effort string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(effort))
	if e == "" {
		e = v.DefaultEffort
	}
	if e == "" || len(v.Efforts) == 0 {
		return e, nil
	}
	if !slices.Contains(v.Efforts, e) {
		return "", fmt.Errorf("unsupported effort %q (valid: %s)", effort, strings.Join(v.Efforts, ", "))
	}
	return e, nil
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
	Effort           string // reasoning/thinking effort; "" = provider default
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

// PickInstruction composes the final prompt for a describe call.
// CustomPrompt replaces the provider's default entirely; Additional
// prepends a "CRITICAL USER CONTEXT" preamble. Shared across providers so
// instruction-composition logic stays consistent — only the default
// prompt texts are per-provider.
func PickInstruction(req DescribeRequest, textDefault, jsonDefault string) string {
	base := textDefault
	if req.StructuredOutput {
		base = jsonDefault
	}
	if req.CustomPrompt != "" {
		base = req.CustomPrompt
	}
	if req.Additional != "" {
		return "CRITICAL USER CONTEXT - You MUST incorporate this into your analysis:\n" + req.Additional + "\n\n" + base
	}
	return base
}
