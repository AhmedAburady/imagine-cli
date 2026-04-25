package batch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// --- IsBatchFile ------------------------------------------------------------

func TestIsBatchFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"batch.yaml", true},
		{"batch.YAML", true},
		{"batch.yml", true},
		{"batch.json", true},
		{"prompt.txt", false},
		{"noext", false},
		{"some.md", false},
	}
	for _, c := range cases {
		if got := IsBatchFile(c.path); got != c.want {
			t.Errorf("IsBatchFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// --- YAML map-form ----------------------------------------------------------

func TestLoadFile_YAMLMapForm_PreservesOrder(t *testing.T) {
	path := writeTempFile(t, "b.yaml", `
zeta:
  prompt: "z"
alpha:
  prompt: "a"
middle:
  prompt: "m"
`)
	spec, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	want := []string{"zeta", "alpha", "middle"}
	if len(spec.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(spec.Entries))
	}
	for i, w := range want {
		if spec.Entries[i].Key != w {
			t.Errorf("entry[%d].Key = %q, want %q (declaration order)", i, spec.Entries[i].Key, w)
		}
	}
}

func TestLoadFile_YAMLMapForm_RejectsDottedKey(t *testing.T) {
	path := writeTempFile(t, "b.yaml", `
"image1.png":
  prompt: "x"
`)
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected dotted-key rejection, got nil")
	}
	if !strings.Contains(err.Error(), "bare stem") {
		t.Errorf("error should explain bare-stem rule, got %v", err)
	}
}

// --- YAML list-form ---------------------------------------------------------

func TestLoadFile_YAMLListForm_LeavesKeyEmpty(t *testing.T) {
	path := writeTempFile(t, "b.yaml", `
- prompt: "first"
- prompt: "second"
`)
	spec, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(spec.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(spec.Entries))
	}
	for i, e := range spec.Entries {
		if e.Key != "" {
			t.Errorf("entry[%d].Key = %q, want empty (list-form)", i, e.Key)
		}
		if e.Index != i {
			t.Errorf("entry[%d].Index = %d, want %d", i, e.Index, i)
		}
	}
}

// --- JSON map-form ----------------------------------------------------------

func TestLoadFile_JSONMapForm_SortsKeysAlphabetically(t *testing.T) {
	path := writeTempFile(t, "b.json", `{
  "zeta":  {"prompt": "z"},
  "alpha": {"prompt": "a"},
  "middle":{"prompt": "m"}
}`)
	spec, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	want := []string{"alpha", "middle", "zeta"}
	for i, w := range want {
		if spec.Entries[i].Key != w {
			t.Errorf("entry[%d].Key = %q, want %q (json maps sorted)", i, spec.Entries[i].Key, w)
		}
	}
}

func TestLoadFile_JSONMapForm_RejectsDottedKey(t *testing.T) {
	path := writeTempFile(t, "b.json", `{"image1.png": {"prompt": "x"}}`)
	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "bare stem") {
		t.Errorf("expected dotted-key rejection, got %v", err)
	}
}

// --- JSON list-form ---------------------------------------------------------

func TestLoadFile_JSONListForm_LeavesKeyEmpty(t *testing.T) {
	path := writeTempFile(t, "b.json", `[
  {"prompt": "first"},
  {"prompt": "second"}
]`)
	spec, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(spec.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(spec.Entries))
	}
	for i, e := range spec.Entries {
		if e.Key != "" {
			t.Errorf("entry[%d].Key = %q, want empty", i, e.Key)
		}
	}
}

// --- Edge cases -------------------------------------------------------------

func TestLoadFile_EmptyYAML(t *testing.T) {
	path := writeTempFile(t, "b.yaml", "")
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error on empty file")
	}
}

func TestLoadFile_EmptyJSON(t *testing.T) {
	path := writeTempFile(t, "b.json", "")
	_, err := LoadFile(path)
	if err == nil {
		t.Fatal("expected error on empty file")
	}
}

// --- validateEntryKey (direct) ----------------------------------------------

func TestValidateEntryKey_AcceptsBareStems(t *testing.T) {
	for _, k := range []string{"hero", "hero_shot", "step-1", "a"} {
		if err := validateEntryKey(k); err != nil {
			t.Errorf("validateEntryKey(%q) = %v, want nil", k, err)
		}
	}
}

func TestValidateEntryKey_RejectsDotted(t *testing.T) {
	for _, k := range []string{"a.png", "a.b.c", ".hidden", "trailing."} {
		err := validateEntryKey(k)
		if err == nil || !strings.Contains(err.Error(), "bare stem") {
			t.Errorf("validateEntryKey(%q) = %v, want bare-stem error", k, err)
		}
	}
}

func TestLoadFile_UnknownExtension(t *testing.T) {
	path := writeTempFile(t, "b.txt", "anything")
	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "extension") {
		t.Errorf("expected extension rejection, got %v", err)
	}
}
