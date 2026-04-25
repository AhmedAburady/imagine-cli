package providers

import (
	"fmt"
	"sort"
)

// Gate is the single source of truth for "is this flag valid here?"
// validation. Both the single-shot CLI path and the batch-file path
// drive the same rules — they only differ in how they collect the set
// of "user-set" flags (cmd.Flags().Visit vs the merged entry map).
//
// Adding a new provider requires no changes here: the rules read from
// Bundle.SupportedFlags and Info.Models, which providers populate
// themselves at registration time.
//
// All Check* functions return a slice of errors so the batch path can
// collect every problem in one go and surface them together. Single-
// shot callers pick the first.

// CheckBundle rejects set flag names that the active provider doesn't
// claim. setNames must be the long flag names the user has explicitly
// set, with common flags already filtered out by the caller.
func CheckBundle(setNames []string, active Bundle) []error {
	supported := make(map[string]bool, len(active.SupportedFlags))
	for _, f := range active.SupportedFlags {
		supported[f] = true
	}
	var errs []error
	for _, name := range setNames {
		if supported[name] {
			continue
		}
		others := ProvidersSupportingFlag(name)
		if len(others) == 0 {
			// Unknown to every provider — cobra would normally reject
			// earlier, so this branch is defensive.
			continue
		}
		errs = append(errs, fmt.Errorf("--%s is not supported by provider %q (supported by: %v)", name, active.Info.Name, others))
	}
	return errs
}

// CheckModel rejects set flag names that the active provider accepts at
// the bundle level but the resolved model itself doesn't honour (e.g.
// gemini accepts --thinking, but only flash supports it; pro does not).
// Pass providerOpts produced by ReadFlags or ParseOptions; the resolved
// model is read via the optional ResolvedModeler interface. Returns no
// errors if the provider hasn't opted into model-level gating.
func CheckModel(setNames []string, active Bundle, providerOpts any) []error {
	resolved := ResolvedModelID(providerOpts)
	if resolved == "" {
		return nil
	}
	advanced := AdvancedFlagSet(active.Info)
	if len(advanced) == 0 {
		return nil
	}

	var modelInfo *ModelInfo
	for i, m := range active.Info.Models {
		if m.ID == resolved {
			modelInfo = &active.Info.Models[i]
			break
		}
	}
	if modelInfo == nil {
		return nil
	}

	allowed := make(map[string]bool, len(modelInfo.SupportedFlags))
	for _, f := range modelInfo.SupportedFlags {
		allowed[f] = true
	}

	var errs []error
	for _, name := range setNames {
		if !advanced[name] || allowed[name] {
			continue
		}
		siblings := ModelsSupportingFlag(active.Info, name)
		errs = append(errs, fmt.Errorf("--%s is not supported by model %q (supported by: %v)", name, resolved, siblings))
	}
	return errs
}

// CheckClaimedSomewhere rejects set flag names that no provided bundle
// claims. The batch path uses this across all entries' resolved
// providers — a CLI flag the user set must apply to at least one entry
// for it to make sense.
func CheckClaimedSomewhere(setNames []string, bundles []Bundle) []error {
	if len(bundles) == 0 {
		return nil
	}
	claimed := map[string]bool{}
	for _, b := range bundles {
		for _, f := range b.SupportedFlags {
			claimed[f] = true
		}
	}
	var errs []error
	for _, name := range setNames {
		if claimed[name] {
			continue
		}
		others := ProvidersSupportingFlag(name)
		errs = append(errs, fmt.Errorf("--%s is not supported by any provider used in this batch (supported by: %v)", name, others))
	}
	// Stable order so error messages don't churn between runs.
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errs
}

// ResolvedModelID extracts the canonical model ID from opaque Options.
// Prefers ResolvedModeler; falls back to the legacy map[string]any
// "model" key for providers still on the untyped interface. Returns ""
// when neither is available — callers treat that as opt-out.
func ResolvedModelID(opts any) string {
	if r, ok := opts.(ResolvedModeler); ok {
		return r.ResolvedModel()
	}
	if m, ok := opts.(map[string]any); ok {
		if s, _ := m["model"].(string); s != "" {
			return s
		}
	}
	return ""
}

// AdvancedFlagSet returns every flag name that appears in at least one
// ModelInfo.SupportedFlags — i.e. the flags that are model-gated rather
// than provider-gated. When empty, no model declares anything special
// and CheckModel is a no-op.
func AdvancedFlagSet(info Info) map[string]bool {
	out := make(map[string]bool)
	for _, m := range info.Models {
		for _, f := range m.SupportedFlags {
			out[f] = true
		}
	}
	return out
}

// ModelsSupportingFlag returns the aliases or IDs of models whose
// SupportedFlags include flagName. Prefers first alias (friendlier than
// a canonical ID), falls back to the ID. Sorted for stable error text.
func ModelsSupportingFlag(info Info, flagName string) []string {
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

