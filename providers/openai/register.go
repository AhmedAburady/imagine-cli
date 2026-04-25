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
			// OutputFormat is derived from the common -f filename's
			// extension. ReadFlags reaches into the cobra FlagSet
			// rather than introducing a new abstraction for one
			// shared field.
			filename, _ := cmd.Flags().GetString("filename")
			o.OutputFormat = outputFormatFromFilename(filename)
			if err := finalizeOptions(o); err != nil {
				return nil, err
			}
			return o, nil
		},
		ParseOptions: func(values map[string]any, common providers.Common) (any, error) {
			optsAny, err := flagspec.Parse(Options{}, values, info)
			if err != nil {
				return nil, err
			}
			o := optsAny.(*Options)
			o.OutputFormat = outputFormatFromFilename(common.Filename)
			if err := finalizeOptions(o); err != nil {
				return nil, err
			}
			return o, nil
		},
		SupportedFlags: flagspec.FieldNames(Options{}),
		Examples:       Examples,
		Info:           info,
		ConfigSchema:   (&Provider{}).ConfigSchema(),
		Vision:         &providers.Vision{DefaultModel: DefaultVisionModel},
	})
}
