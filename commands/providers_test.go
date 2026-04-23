package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/config"
	_ "github.com/AhmedAburady/imagine-cli/providers/all" // register gemini/openai/vertex for tests
)

// seedConfigFile redirects HOME/APPDATA to a tmp dir and writes contents
// as config.yaml there. Returns the file path.
func seedConfigFile(t *testing.T, contents string) string {
	t.Helper()
	tmp := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", tmp)
	} else {
		t.Setenv("HOME", tmp)
	}
	dir := config.DefaultConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// --- configuredAndRegistered ------------------------------------------------

func TestConfiguredAndRegistered_IntersectionSorted(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai":    {APIKey: "k"},
			"gemini":    {APIKey: "k"},
			"ghost":     {APIKey: "k"}, // not registered in this binary
			"phantom":   {APIKey: "k"}, // not registered
		},
	}
	got := configuredAndRegistered(cfg)
	want := []string{"gemini", "openai"} // alphabetical, no ghost/phantom
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestConfiguredAndRegistered_Nil(t *testing.T) {
	if got := configuredAndRegistered(nil); got != nil {
		t.Errorf("nil cfg: got %v, want nil", got)
	}
}

func TestConfiguredAndRegistered_NoProviders(t *testing.T) {
	cfg := &config.Config{Providers: map[string]config.ProviderConfig{}}
	if got := configuredAndRegistered(cfg); len(got) != 0 {
		t.Errorf("empty providers: got %v, want empty", got)
	}
}

// --- providers use ---------------------------------------------------------

func TestProvidersUse_ValidName_UpdatesConfig(t *testing.T) {
	path := seedConfigFile(t, `default_provider: gemini
providers:
  gemini:
    api_key: AIza-xxx
  openai:
    api_key: sk-xxx
`)

	cmd := newProvidersCmd()
	cmd.SetArgs([]string{"use", "openai"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Config file updated on disk
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "default_provider: openai") {
		t.Errorf("config file not updated:\n%s", got)
	}
	// Confirmation printed (bytes.Buffer is non-TTY → lipgloss strips ANSI)
	if !strings.Contains(buf.String(), "set to openai") {
		t.Errorf("output missing confirmation:\n%s", buf.String())
	}
}

func TestProvidersUse_UnknownName_ErrorsWithChoices(t *testing.T) {
	seedConfigFile(t, `default_provider: gemini
providers:
  gemini:
    api_key: AIza-xxx
  openai:
    api_key: sk-xxx
`)

	cmd := newProvidersCmd()
	cmd.SetArgs([]string{"use", "bogus"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	msg := err.Error()
	for _, needle := range []string{`Unknown provider "bogus"`, "- gemini", "- openai", "providers select"} {
		if !strings.Contains(msg, needle) {
			t.Errorf("error message missing %q:\n%s", needle, msg)
		}
	}
}

func TestProvidersUse_NoConfiguredProviders(t *testing.T) {
	seedConfigFile(t, `default_provider: ""
providers: {}
`)
	cmd := newProvidersCmd()
	cmd.SetArgs([]string{"use", "gemini"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty config, got nil")
	}
	if !strings.Contains(err.Error(), "no providers available") {
		t.Errorf("expected no-providers error, got: %v", err)
	}
}

func TestProvidersUse_IdempotentWhenAlreadyCurrent(t *testing.T) {
	path := seedConfigFile(t, `default_provider: gemini
providers:
  gemini:
    api_key: AIza-xxx
`)
	statBefore, _ := os.Stat(path)

	cmd := newProvidersCmd()
	cmd.SetArgs([]string{"use", "gemini"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	statAfter, _ := os.Stat(path)
	if !statBefore.ModTime().Equal(statAfter.ModTime()) {
		t.Error("file rewritten on idempotent `use`")
	}
	if !strings.Contains(buf.String(), "already gemini") {
		t.Errorf("output missing 'already' confirmation:\n%s", buf.String())
	}
}

// --- providers (bare) → show ----------------------------------------------

func TestProvidersBare_ListsConfigured(t *testing.T) {
	seedConfigFile(t, `default_provider: gemini
providers:
  gemini:
    api_key: AIza-xxx
  openai:
    api_key: sk-xxx
`)
	cmd := newProvidersCmd()
	cmd.SetArgs(nil) // bare
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()

	// Header, both provider names, ACTIVE + DEFAULT pills on gemini, and
	// a footer summary. bytes.Buffer is non-TTY so lipgloss strips ANSI —
	// assertions work on plain text.
	for _, needle := range []string{"PROVIDERS", "gemini", "openai", "ACTIVE", "DEFAULT", "2 configured"} {
		if !strings.Contains(out, needle) {
			t.Errorf("bare providers output missing %q:\n%s", needle, out)
		}
	}
	// No YAML echo anymore.
	for _, absent := range []string{"default_provider:", "providers:"} {
		if strings.Contains(out, absent) {
			t.Errorf("output still contains obsolete YAML-echo %q:\n%s", absent, out)
		}
	}
}
