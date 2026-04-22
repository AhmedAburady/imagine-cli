package vertex

import (
	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

// init self-registers the Vertex provider.
func init() {
	info := (&Provider{}).Info()
	providers.Register("vertex", providers.Bundle{
		Factory:        New,
		BindFlags:      BindFlags,
		ReadFlags:      ReadFlags,
		SupportedFlags: supportedFlags(),
		Examples:       gemini.Examples, // vertex mirrors gemini examples
		Info:           info,
	})
}
