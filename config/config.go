// Package config loads imagine's YAML configuration.
//
// Location: ~/.config/imagine/config.yaml (or config.yml — both are tried).
// Users edit this file directly; imagine does not write to it.
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
//	    provider_options:
//	      gcp_project: my-project
//	      location: us-central1
package config

import (
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk shape.
type Config struct {
	DefaultProvider string                    `yaml:"default_provider,omitempty"`
	Providers       map[string]ProviderConfig `yaml:"providers,omitempty"`
}

// ProviderConfig is per-provider config. APIKey is the common case; extras
// live under ProviderOptions as a free-form string map (e.g. Vertex's
// gcp_project and location).
type ProviderConfig struct {
	APIKey          string            `yaml:"api_key,omitempty"`
	ProviderOptions map[string]string `yaml:"provider_options,omitempty"`
}

// DefaultConfigDir returns the imagine config directory for the current OS.
//
// Unix (Linux, macOS, *BSD): ~/.config/imagine/
//   Uses the XDG convention most developer CLIs follow. macOS users get
//   ~/.config rather than ~/Library/Application Support/ because the latter
//   has a space in the path, is awkward to browse, and breaks dotfiles repos.
//
// Windows: %AppData%/imagine/
//   Via os.UserConfigDir(). Typical location: C:\Users\<name>\AppData\Roaming\imagine\
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
	return c.Providers[name].APIKey
}

// ProviderOption returns providers.<name>.provider_options.<key>.
func (c *Config) ProviderOption(name, key string) string {
	if c == nil {
		return ""
	}
	return c.Providers[name].ProviderOptions[key]
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

// GetGCPProject reads providers.vertex.provider_options.gcp_project.
func GetGCPProject() string {
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.ProviderOption("vertex", "gcp_project")
}

// GetGCPLocation reads providers.vertex.provider_options.location (default "global").
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
