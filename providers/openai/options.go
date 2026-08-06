package openai

import (
	"fmt"
	"path/filepath"
	"strconv"
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
	Model       string `flag:"model,m"     desc:"Model: gpt-image-2 (default)" enum:"@models"`
	Size        string `flag:"size,s"      desc:"Image size: 1K, 2K, 4K, auto, or WxH (default: auto)"`
	Quality     string `flag:"quality,q"   desc:"Rendering quality: low, medium, high, auto (default: auto)" enum:"auto,low,medium,high" default:"auto"`
	Compression int    `flag:"compression" desc:"Compression 0-100 (jpeg/webp only; 100=best quality)" default:"100" range:"0:100"`
	Moderation  string `flag:"moderation"  desc:"Content moderation: auto, low (default: auto)" enum:"auto,low"`
	Background  string `flag:"background"  desc:"Background: auto, opaque (default: auto)" enum:"auto,opaque"`

	// PartialImages streams preview frames while rendering; each costs ~100 output tokens.
	PartialImages int `flag:"partial-images" desc:"Stream 1-3 previews while rendering (0 = off)" range:"0:3"`

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
func (o *Options) Validate(_ providers.Info) error {
	return validateImageSize(o.Size)
}

// gpt-image-2's documented size envelope, applied to generations and edits
// alike now that it is the only model.
const (
	sizeEdgeMultiple = 16
	sizeMaxEdge      = 3840
	sizeMaxRatio     = 3
	sizeMinPixels    = 655_360
	sizeMaxPixels    = 8_294_400
)

func validateImageSize(size string) error {
	if size == "" || size == "auto" {
		return nil
	}
	w, h, ok := parseDimensions(size)
	if !ok {
		return fmt.Errorf("invalid --size %q (use 1K, 2K, 4K, auto, or WxH e.g. 1536x1024)", size)
	}
	long, short, pixels := max(w, h), min(w, h), w*h
	switch {
	case w%sizeEdgeMultiple != 0 || h%sizeEdgeMultiple != 0:
		return fmt.Errorf("invalid --size %q: both edges must be multiples of %d", size, sizeEdgeMultiple)
	case long > sizeMaxEdge:
		return fmt.Errorf("invalid --size %q: longest edge must be at most %dpx", size, sizeMaxEdge)
	case long > short*sizeMaxRatio:
		return fmt.Errorf("invalid --size %q: aspect ratio must be within 1:%d to %d:1", size, sizeMaxRatio, sizeMaxRatio)
	case pixels < sizeMinPixels || pixels > sizeMaxPixels:
		return fmt.Errorf("invalid --size %q: total pixels must be between %d and %d (got %d)", size, sizeMinPixels, sizeMaxPixels, pixels)
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

// parseDimensions splits a "WxH" string into its two positive edges.
func parseDimensions(s string) (int, int, bool) {
	wRaw, hRaw, found := strings.Cut(strings.ToLower(s), "x")
	if !found {
		return 0, 0, false
	}
	w, wErr := atoiPositive(wRaw)
	h, hErr := atoiPositive(hRaw)
	if wErr != nil || hErr != nil {
		return 0, 0, false
	}
	return w, h, true
}

// atoiPositive rejects signs, blanks and overflow that strconv.Atoi tolerates.
func atoiPositive(s string) (int, error) {
	if s == "" || strings.ContainsAny(s, "+-") {
		return 0, strconv.ErrSyntax
	}
	return strconv.Atoi(s)
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
