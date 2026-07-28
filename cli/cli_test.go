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
		Prompts:          []string{path},
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

	opts := &Options{Prompts: []string{path}, Output: ".", NumImages: 1}
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

	opts := &Options{Prompts: []string{path}, Output: ".", NumImages: 1}
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

// --- Multi -p concatenation ------------------------------------------------

// writePrompt writes a prompt file into dir and returns its path.
func writePrompt(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestValidate_ConcatenatesPartsInOrder(t *testing.T) {
	dir := t.TempDir()
	style := writePrompt(t, dir, "style.md", "# Style\nwatercolour\n")
	subject := writePrompt(t, dir, "subject.md", "a lighthouse")

	opts := &Options{Prompts: []string{style, "make it night time", subject}, Separator: SeparatorFlagDefault, Output: ".", NumImages: 1}
	if err := opts.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	want := "# Style\nwatercolour\n\nmake it night time\n\na lighthouse"
	if opts.Prompt != want {
		t.Errorf("Prompt:\ngot  %q\nwant %q", opts.Prompt, want)
	}
}

func TestValidate_CustomSeparator(t *testing.T) {
	opts := &Options{Prompts: []string{"a", "b"}, Separator: "---", Output: ".", NumImages: 1}
	if err := opts.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if want := "a\n---\nb"; opts.Prompt != want {
		t.Errorf("Prompt: got %q, want %q", opts.Prompt, want)
	}
}

func TestValidate_EmptySeparatorRejected(t *testing.T) {
	opts := &Options{Prompts: []string{"a", "b"}, Separator: "", Output: ".", NumImages: 1}
	err := opts.Validate()
	if err == nil || !strings.Contains(err.Error(), "separator cannot be empty") {
		t.Errorf("expected empty-separator rejection, got %v", err)
	}
}

func TestValidate_EmptyPartsIgnored(t *testing.T) {
	opts := &Options{Prompts: []string{"", "a cat", ""}, Output: ".", NumImages: 1}
	if err := opts.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if opts.Prompt != "a cat" {
		t.Errorf("Prompt: got %q, want %q", opts.Prompt, "a cat")
	}
}

func TestValidate_AllPartsEmptyIsMissingPrompt(t *testing.T) {
	opts := &Options{Prompts: []string{"", ""}, Output: ".", NumImages: 1}
	err := opts.Validate()
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Errorf("expected missing-prompt error, got %v", err)
	}
}

func TestValidate_MissingPromptFilePartFails(t *testing.T) {
	dir := t.TempDir()
	empty := writePrompt(t, dir, "empty.md", "   \n")

	opts := &Options{Prompts: []string{"a cat", empty}, Separator: SeparatorFlagDefault, Output: ".", NumImages: 1}
	err := opts.Validate()
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-prompt-file error, got %v", err)
	}
}

func TestValidate_BatchFileCannotBeConcatenated(t *testing.T) {
	dir := t.TempDir()
	batch := writePrompt(t, dir, "b.yaml", "hero:\n  prompt: x\n")

	opts := &Options{Prompts: []string{batch, "extra instruction"}, Output: ".", NumImages: 1}
	err := opts.Validate()
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("expected batch-concatenation rejection, got %v", err)
	}
}

// --- NormalizeSeparator ----------------------------------------------------

func TestNormalizeSeparator_RejectsEmpty(t *testing.T) {
	if _, err := NormalizeSeparator(""); err == nil {
		t.Error("expected an empty separator to be rejected")
	}
}

func TestNormalizeSeparator(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{SeparatorFlagDefault, "\n\n"},       // flag default is the escaped form
		{" ", " "},                           // a single space is the minimum
		{"---", "\n---\n"},                   // bare token → its own line
		{"### next ###", "\n### next ###\n"}, // internal spaces still a divider
		{" | ", " | "},                       // padded → verbatim, joins inline
		{`\n-- \n`, "\n-- \n"},               // explicit newlines → verbatim
		{"\n---\n", "\n---\n"},               // real newlines → verbatim
		{`\t`, "\t"},                         // tab escape
		{`a\\nb`, "\n" + `a\nb` + "\n"},      // escaped backslash stays literal
	}
	for _, c := range cases {
		got, err := NormalizeSeparator(c.raw)
		if err != nil {
			t.Errorf("NormalizeSeparator(%q): %v", c.raw, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeSeparator(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// --- IsCommonFlag ----------------------------------------------------------

func TestIsCommonFlag(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"prompt", true},
		{"separator", true},
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
