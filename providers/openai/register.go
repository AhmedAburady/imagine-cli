package openai

import (
	"github.com/AhmedAburady/imagine-cli/providers"
)

// init self-registers the OpenAI provider. Consumed by cmd/imagine/main.go's
// blank-import.
func init() {
	info := (&Provider{}).Info()
	providers.Register("openai", providers.Bundle{
		Factory:        New,
		BindFlags:      BindFlags,
		ReadFlags:      ReadFlags,
		SupportedFlags: OwnedFlags(),
		Examples:       Examples,
		Info:           info,
		ConfigSchema:   (&Provider{}).ConfigSchema(),
	})
}
