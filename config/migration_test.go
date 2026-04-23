package config_test

import (
	"os"
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/config"
)

// TestLoad_FlattensLegacyProviderOptions verifies that an older-shape
// config with `provider_options:` under Vertex loads as flat fields,
// preserving on-disk contents (migration is silent at read time).
func TestLoad_FlattensLegacyProviderOptions(t *testing.T) {
	path := setupTempConfigDir(t)
	legacy := `default_provider: vertex

providers:
  vertex:
    provider_options:
      gcp_project: my-project
      location: us-central1
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	v := cfg.Providers["vertex"]
	if v["gcp_project"] != "my-project" {
		t.Errorf("gcp_project: got %q, want my-project (flattened from provider_options)", v["gcp_project"])
	}
	if v["location"] != "us-central1" {
		t.Errorf("location: got %q, want us-central1", v["location"])
	}
	// No nested key should remain in the parsed object.
	if _, stillNested := v["provider_options"]; stillNested {
		t.Error("provider_options key should not survive parsing")
	}

	// Convenience getters reflect the flattened values.
	if got := cfg.ProviderOption("vertex", "gcp_project"); got != "my-project" {
		t.Errorf("ProviderOption: got %q, want my-project", got)
	}
}

func TestLoad_AcceptsFlatShape(t *testing.T) {
	path := setupTempConfigDir(t)
	flat := `default_provider: vertex

providers:
  vertex:
    gcp_project: flat-project
    location: global
`
	if err := os.WriteFile(path, []byte(flat), 0o600); err != nil {
		t.Fatalf("write flat config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Providers["vertex"]["gcp_project"] != "flat-project" {
		t.Errorf("flat gcp_project not read: %+v", cfg.Providers["vertex"])
	}
}

func TestSaveProviderFields_CreatesInitialConfig(t *testing.T) {
	_ = setupTempConfigDir(t) // dir exists, no file yet

	err := config.SaveProviderFields("gemini", map[string]string{"api_key": "AIza-new"})
	if err != nil {
		t.Fatalf("SaveProviderFields: %v", err)
	}

	data, err := os.ReadFile(config.DefaultConfigPath())
	if err != nil {
		t.Fatalf("read created config: %v", err)
	}
	got := string(data)
	for _, needle := range []string{"default_provider: gemini", "providers:", "gemini:", "api_key: AIza-new"} {
		if !strings.Contains(got, needle) {
			t.Errorf("created config missing %q:\n%s", needle, got)
		}
	}
}

func TestSaveProviderFields_MigratesLegacyShape(t *testing.T) {
	path := setupTempConfigDir(t)
	legacy := `default_provider: vertex

providers:
  vertex:
    provider_options:
      gcp_project: old-project
      location: us-east1
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	err := config.SaveProviderFields("vertex", map[string]string{
		"gcp_project": "old-project",
		"location":    "global",
	})
	if err != nil {
		t.Fatalf("SaveProviderFields: %v", err)
	}

	got, _ := os.ReadFile(path)
	gotStr := string(got)
	// Legacy sub-map is gone; flat fields are in their place.
	if strings.Contains(gotStr, "provider_options:") {
		t.Errorf("provider_options should have been stripped:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "gcp_project: old-project") {
		t.Errorf("gcp_project not flat:\n%s", gotStr)
	}
	if !strings.Contains(gotStr, "location: global") {
		t.Errorf("location not flat or not updated:\n%s", gotStr)
	}
}

func TestSaveProviderFields_PreservesOtherProviders(t *testing.T) {
	path := setupTempConfigDir(t)
	original := `default_provider: gemini

providers:
  gemini:
    api_key: AIza-original
  openai:
    api_key: sk-keep-me
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := config.SaveProviderFields("vertex", map[string]string{
		"gcp_project": "new-proj",
	})
	if err != nil {
		t.Fatalf("SaveProviderFields: %v", err)
	}

	got, _ := os.ReadFile(path)
	gotStr := string(got)
	// Existing providers untouched.
	for _, needle := range []string{"AIza-original", "sk-keep-me"} {
		if !strings.Contains(gotStr, needle) {
			t.Errorf("unrelated provider field lost: missing %q\n%s", needle, gotStr)
		}
	}
	// New provider added.
	if !strings.Contains(gotStr, "gcp_project: new-proj") {
		t.Errorf("new provider not added:\n%s", gotStr)
	}
}
