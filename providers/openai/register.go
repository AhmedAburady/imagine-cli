package openai

import (
	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// init self-registers the OpenAI provider. Consumed by cmd/imagine/main.go's
// blank-import.
//
// Flag binding/parsing is delegated to providers/flagspec, which reflects
// the tags on Options. Adding a new flag means adding a field — no edits
// to this file.
func init() {
	info := (&Provider{}).Info()
	providers.Register("openai", providers.Bundle{
		Factory: New,
		BindFlags: func(cmd *cobra.Command) {
			// flagspec.Bind is idempotent by flag name — safe alongside
			// other providers that share names.
			flagspec.Bind(cmd, Options{})
		},
		ReadFlags: func(cmd *cobra.Command) (any, error) {
			optsAny, err := flagspec.Read(cmd, Options{}, info)
			if err != nil {
				return nil, err
			}
			o := optsAny.(*Options)
			// OutputFormat comes from the common -f filename's extension.
			filename, _ := cmd.Flags().GetString("filename")
			o.OutputFormat = outputFormatFromFilename(filename)
			return o, o.validateOutput()
		},
		ParseOptions: func(values map[string]any, common providers.Common) (any, error) {
			optsAny, err := flagspec.Parse(Options{}, values, info)
			if err != nil {
				return nil, err
			}
			o := optsAny.(*Options)
			o.OutputFormat = outputFormatFromFilename(common.Filename)
			return o, o.validateOutput()
		},
		SupportedFlags: flagspec.FieldNames(Options{}),
		Examples:       Examples,
		Info:           info,
		ConfigSchema:   (&Provider{}).ConfigSchema(),
		Vision: &providers.Vision{
			DefaultModel:  DefaultVisionModel,
			Efforts:       []string{"low", "medium", "high", "xhigh"}, // gpt-6.1-sol via Chat Completions; no "none", "minimal" or "max"
			DefaultEffort: "medium",
		},
		AuthMethods: []providers.AuthMethod{
			{
				Key:    "api_key",
				Title:  "OpenAI API key",
				Fields: (&Provider{}).ConfigSchema(),
			},
			{
				Key:   "subscription",
				Title: "ChatGPT plan (Codex sign-in)",
				// vision_model is declared here too (not just on api_key) because
				// describe reads it on this route as well; declaring it keeps the
				// resolver from skipping it as an "api_key-only" field.
				Fields: []providers.ConfigField{{
					Key:         "vision_model",
					Title:       "Vision Model",
					// No Default: the sign-in default follows the ChatGPT plan
					// (driverFor), so a fixed value here would be wrong for some.
					Description: "Model for `imagine describe` (default: picked by ChatGPT plan)",
				}},
				Login: subscriptionLogin,
			},
		},
	})
}
