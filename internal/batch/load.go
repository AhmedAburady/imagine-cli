// Package batch loads, resolves, and runs imagine batch files (YAML/JSON
// describing multiple jobs in one invocation).
//
// Pipeline:
//
//	LoadFile  → *Spec       (raw entries, format-detected by extension)
//	Resolve   → []Resolved  (CLI defaults + entry overrides, validated)
//	Run       → exit-code   (parallel api.RunGeneration per entry)
//
// Each phase fails fast and exhaustively at validation boundaries so
// users see every problem in one error rather than chasing them one at
// a time.
package batch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Entry is one job's raw values from the batch file. Common-flag and
// provider-private fields aren't separated here — that happens during
// Resolve once the entry's effective provider is known.
type Entry struct {
	// Key is the entry's identity in the file. Map-form: the YAML/JSON
	// key (e.g. "hero_shot"). List-form: empty string.
	Key string

	// Index is the entry's 0-based position for ordering and error
	// messages.
	Index int

	// Raw is every key/value pair under this entry, as decoded by the
	// YAML or JSON parser. Values are scalars (string/int/float64/bool)
	// or, where flagspec accepts coercion, lenient near-equivalents.
	Raw map[string]any
}

// Spec is the parsed batch file.
type Spec struct {
	Path    string
	Entries []Entry
}

// IsBatchFile reports whether path's extension marks it as a batch file
// (.yaml/.yml/.json). The CLI uses this to choose between today's
// plain-prompt-file path and the new batch path.
func IsBatchFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".json":
		return true
	}
	return false
}

// LoadFile reads path and parses it. Caller has already verified the
// extension via IsBatchFile.
func LoadFile(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	var entries []Entry
	switch ext {
	case ".yaml", ".yml":
		entries, err = parseYAML(data)
	case ".json":
		entries, err = parseJSON(data)
	default:
		return nil, fmt.Errorf("%s: extension %q not recognised (use .yaml, .yml, or .json)", path, ext)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s: no entries found", path)
	}
	return &Spec{Path: path, Entries: entries}, nil
}

// --- YAML -------------------------------------------------------------------

func parseYAML(data []byte) ([]Entry, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("yaml: %w", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("yaml: empty document")
	}
	return entriesFromYAMLNode(root.Content[0])
}

func entriesFromYAMLNode(n *yaml.Node) ([]Entry, error) {
	switch n.Kind {
	case yaml.MappingNode:
		return yamlMapEntries(n)
	case yaml.SequenceNode:
		return yamlSeqEntries(n)
	default:
		return nil, fmt.Errorf("root must be a map (entry-name keys) or a list (- prefix)")
	}
}

func yamlMapEntries(n *yaml.Node) ([]Entry, error) {
	entries := make([]Entry, 0, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 {
		keyNode := n.Content[i]
		valNode := n.Content[i+1]
		if err := validateEntryKey(keyNode.Value); err != nil {
			return nil, err
		}
		raw := map[string]any{}
		if err := valNode.Decode(&raw); err != nil {
			return nil, fmt.Errorf("entry %q: %w", keyNode.Value, err)
		}
		entries = append(entries, Entry{
			Key:   keyNode.Value,
			Index: i / 2,
			Raw:   raw,
		})
	}
	return entries, nil
}

// validateEntryKey rejects keys that look like filenames-with-extension
// (e.g. "image1.png:") because the extension is inferred from `-f` or
// defaults to .png — embedding it in the key produces stems like
// "image1.png_1.png" with `count > 1`. Bare stems only.
func validateEntryKey(key string) error {
	if strings.Contains(key, ".") {
		return fmt.Errorf("entry %q: key must be a bare stem; extension is inferred from -f or defaults to .png", key)
	}
	return nil
}

func yamlSeqEntries(n *yaml.Node) ([]Entry, error) {
	entries := make([]Entry, 0, len(n.Content))
	for i, item := range n.Content {
		raw := map[string]any{}
		if err := item.Decode(&raw); err != nil {
			return nil, fmt.Errorf("entry [%d]: %w", i, err)
		}
		// List-form entries have no name. Filename falls through to
		// the existing timestamp default in internal/images/naming.go,
		// matching single-shot behaviour.
		entries = append(entries, Entry{
			Key:   "",
			Index: i,
			Raw:   raw,
		})
	}
	return entries, nil
}

// --- JSON -------------------------------------------------------------------

// parseJSON accepts a top-level object (map-form) or array (list-form).
// Map-form keys are sorted alphabetically because Go's encoding/json
// decodes objects into unordered maps; users wanting deterministic
// ordering should use list-form (or YAML map-form, which preserves
// declaration order).
func parseJSON(data []byte) ([]Entry, error) {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("json: empty file")
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]any
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		entries := make([]Entry, 0, len(arr))
		for i, raw := range arr {
			entries = append(entries, Entry{Key: "", Index: i, Raw: raw})
		}
		return entries, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var m map[string]map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			if err := validateEntryKey(k); err != nil {
				return nil, err
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		entries := make([]Entry, 0, len(m))
		for i, k := range keys {
			entries = append(entries, Entry{Key: k, Index: i, Raw: m[k]})
		}
		return entries, nil
	}
	return nil, fmt.Errorf("json: root must be an object or array")
}
