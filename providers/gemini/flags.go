package gemini

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Flag names. Gemini and Vertex share these — BindFlags is idempotent.
const (
	flagModel       = "model"
	flagSize        = "size"
	flagAspectRatio = "aspect-ratio"
	flagGrounding   = "grounding"
	flagThinking    = "thinking"
	flagImageSearch = "image-search"
)

var ownedFlags = []string{
	flagModel, flagSize, flagAspectRatio, flagGrounding, flagThinking, flagImageSearch,
}

// validSizes is the Gemini/Vertex shorthand set.
var validSizes = map[string]bool{"1K": true, "2K": true, "4K": true}

// BindFlags attaches Gemini's flags to cmd. Idempotent — Vertex calls this
// too; the second call no-ops because each flag checks Lookup first.
func BindFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	if f.Lookup(flagModel) == nil {
		f.StringP(flagModel, "m", "", "Model: pro, flash (default: pro)")
	}
	if f.Lookup(flagSize) == nil {
		f.StringP(flagSize, "s", "", "Image size: 1K, 2K, 4K (default: 1K)")
	}
	if f.Lookup(flagAspectRatio) == nil {
		f.StringP(flagAspectRatio, "a", "", "Aspect ratio (default: Auto)")
	}
	if f.Lookup(flagGrounding) == nil {
		f.BoolP(flagGrounding, "g", false, "Enable Google Search grounding")
	}
	if f.Lookup(flagThinking) == nil {
		f.StringP(flagThinking, "t", "", "Thinking level: minimal, high (flash only)")
	}
	if f.Lookup(flagImageSearch) == nil {
		f.BoolP(flagImageSearch, "I", false, "Enable Image Search grounding (flash only)")
	}
}

// ReadFlags validates and resolves Gemini's flags, returning them in a form
// Generate consumes. Called once per run from the root command's PreRunE.
func ReadFlags(cmd *cobra.Command) (map[string]any, error) {
	out := map[string]any{}
	f := cmd.Flags()

	// Model (alias resolution via providers.Info)
	info := (&Provider{}).Info()
	rawModel, _ := f.GetString(flagModel)
	model, err := info.ResolveModel(rawModel)
	if err != nil {
		return nil, err
	}
	out["model"] = model

	// Size (default 1K, validate against {1K, 2K, 4K})
	size, _ := f.GetString(flagSize)
	if size == "" {
		size = "1K"
	}
	if !validSizes[size] {
		return nil, fmt.Errorf("invalid --size %q for gemini (valid: 1K, 2K, 4K)", size)
	}
	out["size"] = size

	// Aspect ratio (any string, empty = Auto)
	if ar, _ := f.GetString(flagAspectRatio); ar != "" {
		out["aspect_ratio"] = ar
	}

	// Grounding / ImageSearch (bools)
	if b, _ := f.GetBool(flagGrounding); b {
		out["grounding"] = true
	}
	if b, _ := f.GetBool(flagImageSearch); b {
		out["image_search"] = true
	}

	// Thinking level (flash-only, case-normalised to upper)
	if t, _ := f.GetString(flagThinking); t != "" {
		t = strings.ToUpper(strings.TrimSpace(t))
		if t != "MINIMAL" && t != "HIGH" {
			return nil, fmt.Errorf("invalid --thinking %q for gemini (valid: minimal, high)", strings.ToLower(t))
		}
		out["thinking"] = t
	}

	return out, nil
}

// OwnedFlags returns the flag names Gemini supports. Vertex calls this too
// and filters out the ones it doesn't honour (image-search).
func OwnedFlags() []string { return append([]string(nil), ownedFlags...) }
