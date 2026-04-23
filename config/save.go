package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ErrNoConfig is returned by Save when no config file exists yet. imagine
// does not auto-create configs via Save — `providers add` has its own
// auto-create path for the onboarding flow.
var ErrNoConfig = errors.New("no config file found")

// Save writes the top-level default_provider and vision_default_provider
// fields back to config.yaml. Preserves comments and unrelated keys.
// Returns ErrNoConfig when no config file exists.
func Save(cfg *Config) error {
	path, existing, err := readExistingConfig()
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(existing, &root); err != nil {
		return fmt.Errorf("parse existing config: %w", err)
	}
	top, err := documentMapping(&root)
	if err != nil {
		return err
	}

	changed := upsertScalarPrepend(top, "default_provider", cfg.DefaultProvider)
	if cfg.VisionDefaultProvider != "" {
		if upsertScalarPrepend(top, "vision_default_provider", cfg.VisionDefaultProvider) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeNodeFile(&root, path)
}

// SaveProviderFields writes flat key/value entries under
// providers.<name> in config.yaml, preserving every surrounding comment
// and unrelated key. Used by `imagine providers add`.
//
// When the config file doesn't exist yet this function creates it — this
// is the onboarding flow, and blocking on "no config" would be the wrong
// UX. When an older-shape `provider_options:` sub-mapping exists for the
// named provider, it's removed on write so the file migrates to the flat
// shape naturally.
func SaveProviderFields(name string, fields map[string]string) error {
	path, existing, err := readExistingConfig()
	if errors.Is(err, ErrNoConfig) {
		// First-time config — build minimal initial YAML.
		return writeInitialConfig(name, fields)
	}
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(existing, &root); err != nil {
		return fmt.Errorf("parse existing config: %w", err)
	}

	top, err := documentMapping(&root)
	if err != nil {
		return err
	}

	providersNode := findOrCreateMapping(top, "providers")
	providerNode := findOrCreateMapping(providersNode, name)

	// Migrate legacy provider_options sub-map if present. Treat the
	// removal itself as a change — otherwise, a config that has BOTH the
	// legacy shape AND the equivalent flat keys would never get cleaned
	// up, because setMappingScalar below would no-op for every key.
	changed := removeMappingKey(providerNode, "provider_options")

	// Deterministic write order so new keys appear alphabetically.
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if setMappingScalar(providerNode, k, fields[k]) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeNodeFile(&root, path)
}

// readExistingConfig returns the path and bytes of the current config
// file. Returns ErrNoConfig when neither config.yaml nor config.yml
// exists in the config directory.
func readExistingConfig() (string, []byte, error) {
	for _, path := range candidatePaths() {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, data, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
	}
	return "", nil, ErrNoConfig
}

// writeInitialConfig writes a minimal config.yaml with one provider entry.
// Called by SaveProviderFields when no existing config is present.
//
// Routes through the same node-tree + yaml.Encoder path SaveProviderFields
// uses on updates so the encoder handles quoting for YAML-special values.
// A naive printf-to-bytes approach would silently produce malformed YAML
// when a field value contains characters like `:`, `#`, or leading `[`.
func writeInitialConfig(name string, fields map[string]string) error {
	dir := DefaultConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var root yaml.Node
	top, err := documentMapping(&root)
	if err != nil {
		return err
	}

	upsertScalarPrepend(top, "default_provider", name)
	providerNode := findOrCreateMapping(findOrCreateMapping(top, "providers"), name)

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		setMappingScalar(providerNode, k, fields[k])
	}

	return writeNodeFile(&root, DefaultConfigPath())
}

// documentMapping returns the top-level mapping node of a parsed config.
func documentMapping(root *yaml.Node) (*yaml.Node, error) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		// Empty document — initialise with an empty mapping so the
		// caller can freely insert keys.
		root.Kind = yaml.DocumentNode
		root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		return root.Content[0], nil
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, errors.New("config: top-level YAML is not a mapping")
	}
	return top, nil
}

// upsertScalarPrepend updates key=value if it exists (preserving its
// position), or inserts at the top of the mapping if it's new. Used for
// top-level fields that read best at the head of the file.
func upsertScalarPrepend(mapping *yaml.Node, key, value string) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind != yaml.ScalarNode || k.Value != key {
			continue
		}
		v := mapping.Content[i+1]
		if v.Kind == yaml.ScalarNode && v.Value == value {
			return false
		}
		v.Kind = yaml.ScalarNode
		v.Value = value
		v.Tag = ""
		v.Style = 0
		return true
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	mapping.Content = append([]*yaml.Node{keyNode, valNode}, mapping.Content...)
	return true
}

// setMappingScalar upserts key=value in a YAML mapping node, appending
// when new. Returns true when the value actually changed.
func setMappingScalar(mapping *yaml.Node, key, value string) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind != yaml.ScalarNode || k.Value != key {
			continue
		}
		v := mapping.Content[i+1]
		if v.Kind == yaml.ScalarNode && v.Value == value {
			return false
		}
		v.Kind = yaml.ScalarNode
		v.Value = value
		v.Tag = ""
		v.Style = 0
		return true
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
	return true
}

// findOrCreateMapping returns the mapping node stored at key inside parent,
// creating an empty mapping if key is absent (or the existing value node
// isn't a mapping).
func findOrCreateMapping(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		k := parent.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			v := parent.Content[i+1]
			if v.Kind != yaml.MappingNode {
				v.Kind = yaml.MappingNode
				v.Tag = "!!map"
				v.Value = ""
				v.Content = nil
				v.Style = 0
			}
			return v
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valNode)
	return valNode
}

// removeMappingKey deletes key (and its value) from a mapping node, if
// present. Returns true when the mapping was actually modified so the
// caller can treat the removal as a change worth writing (needed for
// legacy-shape migrations where the flat values may already match).
func removeMappingKey(mapping *yaml.Node, key string) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		k := mapping.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return true
		}
	}
	return false
}

// writeNodeFile encodes root with 2-space indent and atomically replaces
// the file at path. Shared by Save and SaveProviderFields so the output
// style stays uniform.
func writeNodeFile(root *yaml.Node, path string) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		_ = enc.Close()
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return atomicWrite(path, buf.Bytes(), 0o600)
}

// atomicWrite writes data to path via a temp file in the same directory,
// then renames into place. Rename is atomic on POSIX and best-effort on
// Windows; a crash mid-write leaves the original file untouched rather
// than partially overwritten.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".imagine-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	success = true
	return nil
}
