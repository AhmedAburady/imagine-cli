package gemini

import (
	"strings"

	"github.com/spf13/cobra"
)

// Names are declared once so BindFlags / ReadFlags / OwnedFlags stay in sync.
const (
	flagGrounding   = "grounding"
	flagThinking    = "thinking"
	flagImageSearch = "image-search"
)

// ownedFlags lists the flag names Gemini (and Vertex, which mirrors Gemini)
// claim exclusively. When the active provider is something else (OpenAI in
// Phase 5), setting any of these triggers a "flag not valid for provider X"
// error in the root command's PreRunE.
var ownedFlags = []string{flagGrounding, flagThinking, flagImageSearch}

// BindFlags attaches Gemini-specific flags to cmd. Called once at command
// construction by each registered provider; cobra sees every flag up front.
func BindFlags(cmd *cobra.Command) {
	f := cmd.Flags()
	// Register only if not already present — Vertex's BindFlags reuses this
	// same set so we must be idempotent when both providers wire up.
	if f.Lookup(flagGrounding) == nil {
		f.BoolP(flagGrounding, "g", false, "Enable grounding with Google Search")
	}
	if f.Lookup(flagThinking) == nil {
		f.StringP(flagThinking, "t", "minimal", "Thinking level: minimal, high (flash only)")
	}
	if f.Lookup(flagImageSearch) == nil {
		f.BoolP(flagImageSearch, "I", false, "Enable Image Search grounding (flash only)")
	}
}

// ReadFlags harvests Gemini-specific flag values into a Request.Options map.
// Only includes flags the user actually set (Changed()) OR flags with
// non-default semantic values, so downstream capability gating can detect
// unset-vs-explicit-default.
func ReadFlags(cmd *cobra.Command) map[string]any {
	out := map[string]any{}
	f := cmd.Flags()

	if b, err := f.GetBool(flagGrounding); err == nil && b {
		out[flagGrounding] = true
	}
	if b, err := f.GetBool(flagImageSearch); err == nil && b {
		out["image-search"] = true
	}
	// Thinking is only meaningful with flash; the caller (orchestrator) already
	// gates on model. Normalise to upper-case (what the API wants).
	if s, err := f.GetString(flagThinking); err == nil && s != "" {
		out["thinking"] = strings.ToUpper(s)
	}
	return out
}

// OwnedFlags returns the flags Gemini owns (shared with Vertex).
func OwnedFlags() []string {
	return append([]string(nil), ownedFlags...)
}
