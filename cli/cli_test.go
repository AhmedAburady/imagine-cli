package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Batch detection + --replace rejection ---------------------------------

func TestValidate_BatchModeRejectsTopLevelReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(path, []byte("hero:\n  prompt: x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := &Options{
		Prompt:           path,
		Output:           ".",
		NumImages:        1,
		PreserveFilename: true, // top-level -r — must be rejected in batch mode
	}
	err := opts.Validate()
	if err == nil {
		t.Fatal("expected --replace rejection in batch mode")
	}
	if !strings.Contains(err.Error(), "replace") || !strings.Contains(err.Error(), "batch mode") {
		t.Errorf("error should explain --replace + batch mode rule, got %v", err)
	}
}

func TestValidate_BatchModeSetsIsBatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(path, []byte("hero:\n  prompt: x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := &Options{Prompt: path, Output: ".", NumImages: 1}
	if err := opts.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !opts.IsBatch {
		t.Error("Validate should have set IsBatch=true for .yaml file")
	}
}

func TestValidate_PlainPromptFileReadAsText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.txt")
	if err := os.WriteFile(path, []byte("  a sunset  \n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	opts := &Options{Prompt: path, Output: ".", NumImages: 1}
	if err := opts.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if opts.IsBatch {
		t.Error("plain .txt should not flip IsBatch")
	}
	if opts.Prompt != "a sunset" {
		t.Errorf("Prompt: got %q, want trimmed file contents", opts.Prompt)
	}
}

// --- IsCommonFlag ----------------------------------------------------------

func TestIsCommonFlag(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"prompt", true},
		{"output", true},
		{"count", true},
		{"replace", true},
		{"provider", true},
		{"help", true},
		{"version", true},
		{"model", false},
		{"size", false},
		{"thinking", false},
		{"bogus", false},
	}
	for _, c := range cases {
		if got := IsCommonFlag(c.name); got != c.want {
			t.Errorf("IsCommonFlag(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// --- IsBatchPath -----------------------------------------------------------

func TestIsBatchPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"a.yaml", true},
		{"a.YAML", true},
		{"a.yml", true},
		{"a.json", true},
		{"a.txt", false},
		{"a", false},
		{"a.md", false},
	}
	for _, c := range cases {
		if got := IsBatchPath(c.path); got != c.want {
			t.Errorf("IsBatchPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
