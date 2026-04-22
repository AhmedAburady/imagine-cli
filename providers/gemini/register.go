package gemini

import (
	"github.com/AhmedAburady/imagine-cli/providers"
)

// init self-registers the Gemini provider. Consumed by cmd/imagine/main.go's
// blank-import, which is the only thing that triggers this side effect.
func init() {
	info := (&Provider{}).Info()
	providers.Register("gemini", providers.Bundle{
		Factory:        New,
		BindFlags:      BindFlags,
		ReadFlags:      ReadFlags,
		SupportedFlags: OwnedFlags(),
		Info:           info,
	})
}
