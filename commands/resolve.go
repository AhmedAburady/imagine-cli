package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// resolveDefaultProviderForBatch is resolveProvider's batch-mode twin.
// Same precedence chain, but a missing provider is not an error —
// batch entries can set provider: themselves, so the per-entry resolver
// catches the truly missing case with better context. Returns an error
// only when --provider names something unknown.
func resolveDefaultProviderForBatch(flagValue string) (string, error) {
	if flagValue != "" {
		if _, ok := providers.Get(flagValue); !ok {
			return "", fmt.Errorf("unknown provider %q (available: %v)", flagValue, providers.List())
		}
		return flagValue, nil
	}
	if name := config.GetDefaultProvider(); name != "" {
		if _, ok := providers.Get(name); ok {
			return name, nil
		}
	}
	if cfg, err := config.Load(); err == nil {
		for _, candidate := range sortedProviderKeys(cfg.Providers) {
			if _, ok := providers.Get(candidate); ok {
				return candidate, nil
			}
		}
	}
	return "", nil
}

// resolveProvider returns the active provider name with precedence:
//
//	--provider flag → config.default_provider → first under providers: → error
func resolveProvider(flagValue string) (string, error) {
	name := flagValue
	if name == "" {
		name = config.GetDefaultProvider()
	}
	if name == "" {
		cfg, err := config.Load()
		if err == nil {
			for _, candidate := range sortedProviderKeys(cfg.Providers) {
				if _, ok := providers.Get(candidate); ok {
					name = candidate
					break
				}
			}
		}
	}
	if name == "" {
		return "", fmt.Errorf("no provider configured. Create %s with a providers: entry (see README for schema)", config.DefaultConfigPath())
	}
	if _, ok := providers.Get(name); !ok {
		return "", fmt.Errorf("unknown provider %q (available: %v)", name, providers.List())
	}
	return name, nil
}

// ProviderHintFromArgs resolves the best-effort provider for help rendering.
// Called from main() before fang.Execute so NewRootCmd can mark other
// providers' flags Hidden before fang sees the command.
//
// Order: --provider in argv → config.default_provider → first under providers:
// Returns "" when nothing is configured (help shows all flags).
func ProviderHintFromArgs(args []string) string {
	for i, a := range args {
		if a == "--provider" && i+1 < len(args) {
			name := args[i+1]
			if _, ok := providers.Get(name); ok {
				return name
			}
		} else if after, ok := strings.CutPrefix(a, "--provider="); ok {
			name := after
			if _, ok := providers.Get(name); ok {
				return name
			}
		}
	}
	if name := config.GetDefaultProvider(); name != "" {
		if _, ok := providers.Get(name); ok {
			return name
		}
	}
	cfg, err := config.Load()
	if err == nil {
		for _, candidate := range sortedProviderKeys(cfg.Providers) {
			if _, ok := providers.Get(candidate); ok {
				return candidate
			}
		}
	}
	return ""
}

// providerOrder returns registered provider names with `first` at position 0
// (if registered). Lets the active provider's BindFlags win the help
// description of shared flags (-m, -s).
func providerOrder(first string) []string {
	all := providers.List()
	if first == "" {
		return all
	}
	if _, ok := providers.Get(first); !ok {
		return all
	}
	ordered := []string{first}
	for _, n := range all {
		if n != first {
			ordered = append(ordered, n)
		}
	}
	return ordered
}

// sortedProviderKeys returns config's provider names alphabetically.
func sortedProviderKeys(m map[string]config.ProviderConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
