package vertex

import (
	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// init self-registers the Vertex provider. Vertex shares flag definitions with
// Gemini (both back into the same model family); the Registry's BindFlags
// step is idempotent (see gemini.BindFlags) so both packages can attach.
func init() {
	info := (&Provider{}).Info()
	// Vertex honours the same provider-private flags as Gemini (grounding,
	// thinking) minus image-search — filter to match Capabilities.
	supported := []string{"grounding", "thinking"}
	providers.Register("vertex", providers.Bundle{
		Factory:        New,
		BindFlags:      gemini.BindFlags,
		ReadFlags:      gemini.ReadFlags,
		SupportedFlags: supported,
		Info:           info,
	})
}
