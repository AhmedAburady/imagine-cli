package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	APIKey      string `json:"api_key"`
	GCPProject  string `json:"gcp_project,omitempty"`
	GCPLocation string `json:"gcp_location,omitempty"`
}

// DefaultConfigDir returns the default config directory
func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "banana")
}

// DefaultConfigPath returns the default config file path
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.json")
}

// Load reads the config from the default location
func Load() (*Config, error) {
	path := DefaultConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes the config to the default location
func Save(cfg *Config) error {
	dir := DefaultConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(DefaultConfigPath(), data, 0600)
}

// SaveAPIKey saves just the API key
func SaveAPIKey(key string) error {
	cfg, err := Load()
	if err != nil {
		cfg = &Config{}
	}
	cfg.APIKey = key
	return Save(cfg)
}

// GetAPIKey returns the API key from env vars or config file
// Priority: GEMINI_API_KEY > GOOGLE_API_KEY > config file
func GetAPIKey() string {
	// Check environment variables first
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		return key
	}

	// Fall back to config file
	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.APIKey
}

// GetGCPProject returns the GCP project from env vars or config file
// Priority: GOOGLE_CLOUD_PROJECT > GCLOUD_PROJECT > config file
func GetGCPProject() string {
	if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
		return project
	}
	if project := os.Getenv("GCLOUD_PROJECT"); project != "" {
		return project
	}

	cfg, err := Load()
	if err != nil {
		return ""
	}
	return cfg.GCPProject
}

// GetGCPLocation returns the GCP location from env vars or config file
// Priority: GOOGLE_CLOUD_LOCATION > config file > default "global"
func GetGCPLocation() string {
	if location := os.Getenv("GOOGLE_CLOUD_LOCATION"); location != "" {
		return location
	}

	cfg, err := Load()
	if err == nil && cfg.GCPLocation != "" {
		return cfg.GCPLocation
	}

	return "global" // Default location
}

// SaveGCPProject saves the GCP project to config
func SaveGCPProject(project string) error {
	cfg, err := Load()
	if err != nil {
		cfg = &Config{}
	}
	cfg.GCPProject = project
	return Save(cfg)
}

// SaveGCPLocation saves the GCP location to config
func SaveGCPLocation(location string) error {
	cfg, err := Load()
	if err != nil {
		cfg = &Config{}
	}
	cfg.GCPLocation = location
	return Save(cfg)
}
