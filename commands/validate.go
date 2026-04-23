package commands

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// commonFlags are the truly provider-agnostic flags. Every other flag must
// be owned by at least one provider (via Bundle.SupportedFlags) or the
// ownership gate will refuse to classify it.
var commonFlags = map[string]bool{
	"prompt":   true,
	"output":   true,
	"filename": true,
	"count":    true,
	"input":    true,
	"replace":  true,
	"provider": true,
	"help":     true,
	"version":  true,
}

// enforceFlagSupport rejects provider-private flags the user set explicitly
// but that the active provider doesn't support. The error lists the
// providers that DO support the flag, so the fix is obvious.
func enforceFlagSupport(cmd *cobra.Command, active providers.Bundle) error {
	supported := make(map[string]bool, len(active.SupportedFlags))
	for _, f := range active.SupportedFlags {
		supported[f] = true
	}

	var rejected error
	cmd.Flags().Visit(func(fl *pflag.Flag) {
		if rejected != nil {
			return
		}
		if commonFlags[fl.Name] {
			return
		}
		if supported[fl.Name] {
			return
		}
		others := providers.ProvidersSupportingFlag(fl.Name)
		if len(others) == 0 {
			// Unknown flag (cobra would normally reject earlier).
			return
		}
		rejected = fmt.Errorf("--%s is not supported by provider %q (supported by: %v)", fl.Name, active.Info.Name, others)
	})
	return rejected
}

// enforceModelSupport runs *after* ReadFlags, once the resolved model ID is
// known. It rejects flags the active provider accepts generically but that
// the resolved model doesn't honour (e.g. --thinking under Gemini's pro
// model). Rule:
//
//	advanced = union of Info.Models[].SupportedFlags across the provider
//	user-set flag is rejected iff it's in advanced but not in the
//	  resolved model's SupportedFlags
//
// Flags outside the advanced set are unaffected — the bundle-level gate
// (enforceFlagSupport) already handled them. When no model declares any
// SupportedFlags (e.g. OpenAI today), the advanced set is empty and this
// function is a no-op. Providers without a ResolvedModeler implementation
// are skipped too — the check is opt-in.
func enforceModelSupport(cmd *cobra.Command, active providers.Bundle, providerOptions any) error {
	resolved := resolvedModelID(providerOptions)
	if resolved == "" {
		return nil // provider hasn't opted in; nothing to check
	}

	advanced := advancedFlagSet(active.Info)
	if len(advanced) == 0 {
		return nil // no model declares model-specific flags
	}

	var modelInfo *providers.ModelInfo
	for i, m := range active.Info.Models {
		if m.ID == resolved {
			modelInfo = &active.Info.Models[i]
			break
		}
	}
	if modelInfo == nil {
		return nil // resolved model not in catalogue — ResolveModel would have rejected earlier
	}

	allowed := make(map[string]bool, len(modelInfo.SupportedFlags))
	for _, f := range modelInfo.SupportedFlags {
		allowed[f] = true
	}

	var rejected error
	cmd.Flags().Visit(func(fl *pflag.Flag) {
		if rejected != nil {
			return
		}
		if !advanced[fl.Name] || allowed[fl.Name] {
			return
		}
		siblings := modelsSupportingFlag(active.Info, fl.Name)
		rejected = fmt.Errorf("--%s is not supported by model %q (supported by: %v)", fl.Name, resolved, siblings)
	})
	return rejected
}

// resolvedModelID extracts the canonical model ID from opaque Options.
// Prefers ResolvedModeler; falls back to the legacy map[string]any "model"
// key for providers still on the untyped interface. Returns "" when
// neither is available — callers treat that as opt-out.
func resolvedModelID(opts any) string {
	if r, ok := opts.(providers.ResolvedModeler); ok {
		return r.ResolvedModel()
	}
	if m, ok := opts.(map[string]any); ok {
		if s, _ := m["model"].(string); s != "" {
			return s
		}
	}
	return ""
}

// advancedFlagSet returns every flag name that appears in at least one
// ModelInfo.SupportedFlags — i.e. the flags that are model-gated rather
// than provider-gated.
func advancedFlagSet(info providers.Info) map[string]bool {
	out := make(map[string]bool)
	for _, m := range info.Models {
		for _, f := range m.SupportedFlags {
			out[f] = true
		}
	}
	return out
}

// modelsSupportingFlag returns the aliases or IDs of models in this
// provider whose SupportedFlags include flagName. Prefers the first alias
// (friendlier than a full canonical ID) and falls back to the ID.
func modelsSupportingFlag(info providers.Info, flagName string) []string {
	var out []string
	for _, m := range info.Models {
		for _, f := range m.SupportedFlags {
			if f != flagName {
				continue
			}
			if len(m.Aliases) > 0 {
				out = append(out, m.Aliases[0])
			} else {
				out = append(out, m.ID)
			}
			break
		}
	}
	sort.Strings(out)
	return out
}
