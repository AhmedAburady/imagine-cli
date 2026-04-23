package openai

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

const (
	flagModel       = "model"
	flagSize        = "size"
	flagQuality     = "quality"
	flagCompression = "compression"
	flagModeration  = "moderation"
	flagBackground  = "background"
	// OpenAI reads -f (common flag) to auto-infer output_format.
	flagFilename = "filename"
)

var ownedFlags = []string{
	flagModel, flagSize, flagQuality, flagCompression, flagModeration, flagBackground,
}

// BindFlags registers OpenAI-specific flags. Idempotent with Gemini's
// BindFlags — both declare `model` and `size`; whoever registers first wins
// the help text, but the values flow through cobra the same way.
func BindFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	if f.Lookup(flagModel) == nil {
		f.StringP(flagModel, "m", "", "Model: gpt-image-2, 1.5, 1, mini, latest (default: gpt-image-2)")
	}
	if f.Lookup(flagSize) == nil {
		f.StringP(flagSize, "s", "", "Image size: 1K, 2K, 4K, auto, or WxH (default: auto)")
	}
	if f.Lookup(flagQuality) == nil {
		f.StringP(flagQuality, "q", "", "Rendering quality: low, medium, high, auto (default: auto)")
	}
	if f.Lookup(flagCompression) == nil {
		f.Int(flagCompression, 100, "Compression 0-100 (jpeg/webp only; 100=best quality)")
	}
	if f.Lookup(flagModeration) == nil {
		f.String(flagModeration, "", "Content moderation: auto, low (default: auto)")
	}
	if f.Lookup(flagBackground) == nil {
		f.String(flagBackground, "", "Background: auto, opaque, transparent (default: auto)")
	}
}

// ReadFlags resolves, validates, and packages all OpenAI flags into an
// opaque map carried through Request.Options. The provider's Generate
// type-asserts back to map[string]any. OpenAI keeps the legacy map layout
// because its validation (raw WxH sizes, --background transparent vs.
// gpt-image-2 conflict) doesn't fit the declarative flagspec tags cleanly
// — a deliberate demonstration that flagspec is opt-in.
func ReadFlags(cmd *cobra.Command) (any, error) {
	out := map[string]any{}
	f := cmd.Flags()

	// Model
	info := (&Provider{}).Info()
	rawModel, _ := f.GetString(flagModel)
	model, err := info.ResolveModel(rawModel)
	if err != nil {
		return nil, err
	}
	out["model"] = model

	// Size — accept 1K/2K/4K shorthand, raw WxH, auto, or empty (→ auto)
	rawSize, _ := f.GetString(flagSize)
	size, err := resolveSize(rawSize)
	if err != nil {
		return nil, err
	}
	out["size"] = size

	// Quality
	q, _ := f.GetString(flagQuality)
	q = strings.ToLower(strings.TrimSpace(q))
	switch q {
	case "", "auto":
		out["quality"] = "auto"
	case "low", "medium", "high":
		out["quality"] = q
	default:
		return nil, fmt.Errorf("invalid --quality %q (valid: low, medium, high, auto)", q)
	}

	// Output format — auto-inferred from -f extension, default png
	filename, _ := f.GetString(flagFilename)
	outputFormat := outputFormatFromFilename(filename)
	out["output_format"] = outputFormat

	// Compression (jpeg/webp only)
	comp, _ := f.GetInt(flagCompression)
	if comp < 0 || comp > 100 {
		return nil, fmt.Errorf("--compression must be 0-100 (got %d)", comp)
	}
	out["compression"] = comp

	// Moderation
	mod, _ := f.GetString(flagModeration)
	mod = strings.ToLower(strings.TrimSpace(mod))
	switch mod {
	case "", "auto":
		// don't set — API default
	case "low":
		out["moderation"] = "low"
	default:
		return nil, fmt.Errorf("invalid --moderation %q (valid: auto, low)", mod)
	}

	// Background
	bg, _ := f.GetString(flagBackground)
	bg = strings.ToLower(strings.TrimSpace(bg))
	switch bg {
	case "", "auto":
		// don't set — API default
	case "opaque":
		out["background"] = "opaque"
	case "transparent":
		if outputFormat == "jpeg" {
			return nil, fmt.Errorf("--background transparent requires PNG or WebP output (use -f file.png or -f file.webp)")
		}
		if model == "gpt-image-2" {
			return nil, fmt.Errorf("--background transparent is not supported by gpt-image-2; pick a different model (e.g. -m 1.5)")
		}
		out["background"] = "transparent"
	default:
		return nil, fmt.Errorf("invalid --background %q (valid: auto, opaque, transparent)", bg)
	}

	return out, nil
}

// OwnedFlags lists the flag names OpenAI supports (for the ownership gate).
func OwnedFlags() []string { return append([]string(nil), ownedFlags...) }

// resolveSize turns user input into a dimension string the API accepts.
// Shorthand (1K/2K/4K) maps to landscape/square presets. Raw WxH is
// passed through. Empty or "auto" becomes "auto".
func resolveSize(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "auto") {
		return "auto", nil
	}
	switch strings.ToUpper(s) {
	case "1K":
		return "1024x1024", nil
	case "2K":
		return "2048x2048", nil
	case "4K":
		return "3840x2160", nil
	}
	// Accept raw WxH — API validates dimensional constraints.
	if isDimensionString(s) {
		return s, nil
	}
	return "", fmt.Errorf("invalid --size %q (use 1K, 2K, 4K, auto, or WxH e.g. 1536x1024)", s)
}

// isDimensionString checks for a simple "WxH" pattern with positive integers.
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
