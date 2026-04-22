package vertex

import (
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// BindFlags delegates to Gemini's BindFlags. It's idempotent, so it's safe
// even when Gemini is also registered.
func BindFlags(cmd *cobra.Command) { gemini.BindFlags(cmd) }

// ReadFlags delegates to Gemini's ReadFlags, then validates the resolved
// model against Vertex's model list (Vertex mirrors Gemini's IDs so this
// just works — but ResolveModel must be called with Vertex's Info).
func ReadFlags(cmd *cobra.Command) (map[string]any, error) {
	info := (&Provider{}).Info()
	// Read raw model input and resolve via Vertex's Info (in case aliases diverge).
	raw, _ := cmd.Flags().GetString("model")
	model, err := info.ResolveModel(raw)
	if err != nil {
		return nil, err
	}
	// Borrow Gemini's harvester for everything else.
	out, err := gemini.ReadFlags(cmd)
	if err != nil {
		return nil, err
	}
	out["model"] = model // override with Vertex-validated model
	delete(out, "image_search") // not supported even if the flag was set
	return out, nil
}

// supportedFlags returns Vertex's subset (no image-search).
func supportedFlags() []string {
	return []string{"model", "size", "aspect-ratio", "grounding", "thinking"}
}
