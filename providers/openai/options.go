package openai

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// Options is OpenAI's private parameter struct. flagspec reflects the tags
// at registration to bind Cobra flags, and again at PreRunE (or per-batch-
// entry parse) to populate this struct from input. Generate type-asserts
// Request.Options to *Options — no string keys, no silent zero-value
// fallbacks.
//
// Shared flag names (model, size, quality) register idempotently alongside
// other providers' Options; the active provider registers first, so its
// desc text wins in `--help`.
type Options struct {
	Model       string `flag:"model,m"     desc:"Model: gpt-image-2, 1.5, 1, mini, latest (default: gpt-image-2)" enum:"@models"`
	Size        string `flag:"size,s"      desc:"Image size: 1K, 2K, 4K, auto, or WxH (default: auto)"`
	Quality     string `flag:"quality,q"   desc:"Rendering quality: low, medium, high, auto (default: auto)" enum:"auto,low,medium,high"`
	Compression int    `flag:"compression" desc:"Compression 0-100 (jpeg/webp only; 100=best quality)" default:"100" range:"0:100"`
	Moderation  string `flag:"moderation"  desc:"Content moderation: auto, low (default: auto)" enum:"auto,low"`
	Background  string `flag:"background"  desc:"Background: auto, opaque, transparent (default: auto)" enum:"auto,opaque,transparent"`

	// OutputFormat is derived from the -f filename's extension by the
	// caller (CLI ReadFlags closure or batch runner) before Generate.
	// Not a CLI flag.
	OutputFormat string `flag:"-"`
}

// RequestLabel implements providers.RequestLabeler for status output.
func (o *Options) RequestLabel() string {
	return o.Model
}

// ResolvedModel implements providers.ResolvedModeler so the model-level
// flag-support gate can look up per-model SupportedFlags.
func (o *Options) ResolvedModel() string {
	return o.Model
}

// Normalize canonicalises size shorthand and maps `auto` → "" for fields
// where the OpenAI API treats absence as the default (moderation,
// background). Runs after flagspec's per-field enum/range checks.
func (o *Options) Normalize() {
	o.Size = canonicalSize(o.Size)
	if strings.EqualFold(o.Moderation, "auto") {
		o.Moderation = ""
	}
	if strings.EqualFold(o.Background, "auto") {
		o.Background = ""
	}
}

// Validate enforces field-level rules not expressible as enum/range tags.
// Cross-field rules involving OutputFormat live in finalizeOptions, run
// by the caller after OutputFormat is set from the -f filename.
func (o *Options) Validate(_ providers.Info) error {
	if o.Size != "auto" && !isDimensionString(o.Size) {
		return fmt.Errorf("invalid --size %q (use 1K, 2K, 4K, auto, or WxH e.g. 1536x1024)", o.Size)
	}
	return nil
}

// finalizeOptions runs cross-field rules that depend on OutputFormat
// (which is derived from the common -f filename, not the provider's own
// flags). Caller sets OutputFormat first, then invokes this.
func finalizeOptions(o *Options) error {
	if o.Background == "transparent" {
		if o.OutputFormat == "jpeg" {
			return fmt.Errorf("--background transparent requires PNG or WebP output (use -f file.png or -f file.webp)")
		}
		if o.Model == "gpt-image-2" {
			return fmt.Errorf("--background transparent is not supported by gpt-image-2; pick a different model (e.g. -m 1.5)")
		}
	}
	return nil
}

// canonicalSize maps shorthand (1K/2K/4K) and auto to API-accepted
// strings. Unknown values pass through for Validate to reject.
func canonicalSize(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "auto") {
		return "auto"
	}
	switch strings.ToUpper(s) {
	case "1K":
		return "1024x1024"
	case "2K":
		return "2048x2048"
	case "4K":
		return "3840x2160"
	}
	return strings.ToLower(s)
}

// isDimensionString matches a simple "WxH" pattern with positive integers.
func isDimensionString(s string) bool {
	parts := strings.Split(strings.ToLower(s), "x")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// outputFormatFromFilename inspects -f's extension. Defaults to png.
func outputFormatFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".webp":
		return "webp"
	default:
		return "png"
	}
}
