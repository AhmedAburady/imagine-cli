package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/config"
)

// setupTempConfigDir redirects DefaultConfigDir to a temp path by pointing
// HOME (Unix) or APPDATA (Windows) at a freshly-created tmp dir. Returns
// the config file path.
func setupTempConfigDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", tmp)
	} else {
		t.Setenv("HOME", tmp)
	}
	dir := config.DefaultConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	return filepath.Join(dir, "config.yaml")
}

func TestSave_PreservesCommentsAndOrder(t *testing.T) {
	path := setupTempConfigDir(t)
	original := `# My imagine config
default_provider: gemini  # switched 2026-01-15

providers:
  # Primary provider
  gemini:
    api_key: AIza-original-key
  openai:
    api_key: sk-original-key
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.DefaultProvider = "openai"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after save: %v", err)
	}
	gotStr := string(got)

	// Comments preserved
	for _, needle := range []string{
		"# My imagine config",
		"# switched 2026-01-15",
		"# Primary provider",
	} {
		if !strings.Contains(gotStr, needle) {
			t.Errorf("comment not preserved: %q\n--- file ---\n%s", needle, gotStr)
		}
	}

	// default_provider updated
	if !strings.Contains(gotStr, "default_provider: openai") {
		t.Errorf("default_provider not updated to openai:\n%s", gotStr)
	}
	if strings.Contains(gotStr, "default_provider: gemini") {
		t.Errorf("old default_provider value still present:\n%s", gotStr)
	}

	// Untouched scalars preserved
	for _, needle := range []string{"AIza-original-key", "sk-original-key"} {
		if !strings.Contains(gotStr, needle) {
			t.Errorf("api_key mutated or removed: missing %q\n%s", needle, gotStr)
		}
	}

	// Re-load round-trips
	cfg2, err := config.Load()
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if cfg2.DefaultProvider != "openai" {
		t.Errorf("reloaded default_provider: got %q, want openai", cfg2.DefaultProvider)
	}
}

func TestSave_InsertsDefaultProviderWhenAbsent(t *testing.T) {
	path := setupTempConfigDir(t)
	original := `providers:
  gemini:
    api_key: AIza-xxx
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	cfg, _ := config.Load()
	cfg.DefaultProvider = "gemini"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, _ := os.ReadFile(path)
	gotStr := string(got)
	if !strings.Contains(gotStr, "default_provider: gemini") {
		t.Errorf("default_provider not inserted:\n%s", gotStr)
	}

	// default_provider should precede providers: in the file (the
	// implementation prepends).
	defIdx := strings.Index(gotStr, "default_provider:")
	provIdx := strings.Index(gotStr, "providers:")
	if defIdx < 0 || provIdx < 0 || defIdx > provIdx {
		t.Errorf("default_provider should appear before providers:\n%s", gotStr)
	}
}

func TestSave_NoOpWhenValueUnchanged(t *testing.T) {
	path := setupTempConfigDir(t)
	original := `default_provider: gemini
providers:
  gemini:
    api_key: AIza-xxx
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}
	statBefore, _ := os.Stat(path)

	cfg, _ := config.Load()
	cfg.DefaultProvider = "gemini" // unchanged
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	statAfter, _ := os.Stat(path)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Error("Save rewrote file even though value was unchanged")
	}

	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file bytes changed on no-op Save:\nwant:\n%s\ngot:\n%s", original, got)
	}
}

func TestSave_ErrNoConfigWhenFileMissing(t *testing.T) {
	_ = setupTempConfigDir(t) // creates dir but no file
	cfg := &config.Config{DefaultProvider: "gemini"}
	err := config.Save(cfg)
	if !errors.Is(err, config.ErrNoConfig) {
		t.Errorf("expected ErrNoConfig, got %v", err)
	}
}

func TestSave_LeavesNoTempFiles(t *testing.T) {
	path := setupTempConfigDir(t)
	original := `default_provider: gemini
providers:
  gemini:
    api_key: AIza-xxx
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	cfg, _ := config.Load()
	cfg.DefaultProvider = "openai"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	dir := filepath.Dir(path)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".imagine-config-") {
			t.Errorf("temp file left behind after success: %s", e.Name())
		}
	}
}

func TestSave_AcceptsConfigYmlVariant(t *testing.T) {
	// Load() tries .yaml first, then .yml. When only .yml exists, Save
	// should write to .yml (not create a new .yaml).
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", tmp)
	}
	dir := config.DefaultConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ymlPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(ymlPath, []byte("default_provider: gemini\nproviders:\n  gemini:\n    api_key: k\n"), 0o600); err != nil {
		t.Fatalf("write yml: %v", err)
	}

	cfg, _ := config.Load()
	cfg.DefaultProvider = "openai"
	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// yaml variant must NOT have been created
	if _, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil {
		t.Error("Save created config.yaml instead of updating config.yml")
	}
	got, _ := os.ReadFile(ymlPath)
	if !strings.Contains(string(got), "default_provider: openai") {
		t.Errorf(".yml not updated:\n%s", got)
	}
}
