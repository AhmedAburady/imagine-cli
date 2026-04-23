// Package config loads imagine's YAML configuration.
//
// Location: ~/.config/imagine/config.yaml (or config.yml — both are tried).
// Users edit this file directly for credentials. imagine mutates it only
// via explicit commands — `providers use / select / add` — and preserves
// comments, key order, and quoting when it does. See Save() and
// SaveProvider() for the narrow write paths.
//
// Schema:
//
//	default_provider: openai
//	providers:
//	  gemini:
//	    api_key: AIza...
//	  openai:
//	    api_key: sk-...
//	  vertex:
//	    gcp_project: my-project
//	    location: us-central1
//
// Older configs used `provider_options:` as a sub-map under Vertex — that
// shape is silently flattened on read, so existing users keep working.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk shape.
type Config struct {
	DefaultProvider       string                    `yaml:"default_provider,omitempty"`
	VisionDefaultProvider string                    `yaml:"vision_default_provider,omitempty"`
	Providers             map[string]ProviderConfig `yaml:"providers,omitempty"`
}

// ProviderConfig is the flat per-provider config — any key/value pair the
// provider's ConfigSchema declares. Common example: {"api_key": "..."}.
// Vertex: {"gcp_project": "...", "location": "..."}.
//
// UnmarshalYAML also accepts the legacy shape where extras lived under a
// `provider_options:` sub-map; those entries are flattened into the parent
// on load, so an old config keeps working verbatim until the next Save.
type ProviderConfig map[string]string

// UnmarshalYAML flattens the legacy `provider_options:` sub-map into the
// parent on read. Scalar fields pass through as-is.
func (pc *ProviderConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("provider config must be a mapping, got kind %d", node.Kind)
	}
	out := map[string]string{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		val := node.Content[i+1]

		// Legacy shape: flatten provider_options.* into the parent.
		if key == "provider_options" && val.Kind == yaml.MappingNode {
			for j := 0; j+1 < len(val.Content); j += 2 {
				k := val.Content[j].Value
				v := val.Content[j+1]
				if v.Kind != yaml.ScalarNode {
					return fmt.Errorf("provider_options.%s: expected scalar value", k)
				}
				out[k] = v.Value
			}
			continue
		}

		if val.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s: expected scalar value", key)
		}
		out[key] = val.Value
	}
	*pc = out
	return nil
}

// DefaultConfigDir returns the imagine config directory for the current OS.
//
// Unix (Linux, macOS, *BSD): ~/.config/imagine/
// Windows: %AppData%/imagine/
//
// Returns "" if the underlying home / config dir cannot be resolved.
func DefaultConfigDir() string {
	if runtime.GOOS == "windows" {
		base, err := os.UserConfigDir()
		if err != nil {
			return ""
		}
		return filepath.Join(base, "imagine")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "imagine")
}

// candidatePaths returns the config file paths tried in order.
func candidatePaths() []string {
	dir := DefaultConfigDir()
	return []string{
		filepath.Join(dir, "config.yaml"),
		filepath.Join(dir, "config.yml"),
	}
}

// DefaultConfigPath returns the canonical config path (config.yaml). Use this
// in user-facing error messages; Load() also accepts the .yml variant.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// Load reads the config. Tries config.yaml, then config.yml. Returns a
// zero-value *Config when neither exists (not an error).
func Load() (*Config, error) {
	for _, path := range candidatePaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		var cfg Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		if cfg.Providers == nil {
			cfg.Providers = map[string]ProviderConfig{}
		}
		return &cfg, nil
	}
	return &Config{Providers: map[string]ProviderConfig{}}, nil
}

// ProviderAPIKey returns providers.<name>.api_key.
func (c *Config) ProviderAPIKey(name string) string {
	if c == nil {
		return ""
	}
	return c.Providers[name]["api_key"]
}

// ProviderOption returns providers.<name>.<key>. Named "option" for
// backwards compatibility with the legacy provider_options.<key> access
// pattern — the storage is now flat but the getter interface is unchanged.
func (c *Config) ProviderOption(name, key string) string {
	if c == nil {
		return ""
	}
	return c.Providers[name][key]
}

// -- Back-compat getters for describe (out of scope for the refactor) --------

// GetGeminiAPIKey reads providers.gemini.api_key.
func GetGeminiAPIKey() string {
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.ProviderAPIKey("gemini")
}

// GetGCPProject reads providers.vertex.gcp_project (or legacy
// provider_options.gcp_project, flattened on load).
func GetGCPProject() string {
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.ProviderOption("vertex", "gcp_project")
}

// GetGCPLocation reads providers.vertex.location (default "global").
func GetGCPLocation() string {
	cfg, err := Load()
	if err == nil {
		if l := cfg.ProviderOption("vertex", "location"); l != "" {
			return l
		}
	}
	return "global"
}

// GetDefaultProvider reads default_provider.
func GetDefaultProvider() string {
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.DefaultProvider
}
