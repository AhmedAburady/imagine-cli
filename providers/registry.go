package providers

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// Bundle is the CLI-facing wiring for a provider: how to construct it,
// attach its provider-private flags to a cobra command, declare which flags
// it honours, and harvest them into a Request.Options map at run time.
type Bundle struct {
	// Factory builds a Provider from the resolved Auth. Returns an error if a
	// required credential is missing.
	Factory func(Auth) (Provider, error)

	// BindFlags registers the provider's private flags on the given cobra
	// command. Must be idempotent: providers that share flags (Gemini ↔
	// Vertex both use --grounding / --thinking) each call BindFlags, and the
	// second call must no-op.
	BindFlags func(cmd *cobra.Command)

	// SupportedFlags lists the provider-private flag names this provider
	// honours. When the active provider is X and the user explicitly set a
	// flag Y that isn't in X's SupportedFlags (and Y isn't a common flag),
	// the CLI rejects the call in PreRunE. Common flags (prompt, output,
	// n, …) are NOT listed here.
	SupportedFlags []string

	// ReadFlags harvests flag values from the cobra command into an opaque
	// options value that ends up in Request.Options. The provider decodes it
	// inside Generate (typically a *XOptions struct, or a legacy
	// map[string]any). Returns an error when a provider-specific flag value
	// is invalid (unknown model, out-of-range size, …).
	ReadFlags func(cmd *cobra.Command) (any, error)

	// Info mirrors Provider.Info so the registry can answer queries without
	// constructing the provider.
	Info Info

	// Examples returns the provider-specific block rendered under the
	// EXAMPLES section of `imagine --help`. The provider owns this text so
	// adding a new provider requires no edits to commands/. Empty → no
	// EXAMPLES section.
	Examples func() string
}

var registry = map[string]Bundle{}

// Register adds a provider to the registry. Usually called from init() in the
// provider's own package so the CLI picks it up via a single blank import.
func Register(name string, b Bundle) {
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("providers: duplicate registration for %q", name))
	}
	registry[name] = b
}

// Get returns the Bundle registered under name.
func Get(name string) (Bundle, bool) {
	b, ok := registry[name]
	return b, ok
}

// List returns all registered provider names, sorted alphabetically.
func List() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ProvidersSupportingFlag returns the names of providers whose
// SupportedFlags include flagName. Used by the CLI to format a helpful
// "--X is not valid for provider 'Y' (used by: Z)" error message.
func ProvidersSupportingFlag(flagName string) []string {
	var out []string
	for name, b := range registry {
		for _, f := range b.SupportedFlags {
			if f == flagName {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}
