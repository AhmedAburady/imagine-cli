package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrNoConfig is returned by Save when no config file exists yet. imagine
// does not auto-create configs — the user must create the file with at
// least one provider entry before the CLI can mutate default_provider.
var ErrNoConfig = errors.New("no config file found")

// Save writes cfg back to the active config path, mutating only the
// default_provider field. It preserves comments, key ordering, and
// quoting in the source YAML.
//
// Everything else on *Config is ignored — Save is deliberately narrow.
// imagine's policy is that credentials and provider_options remain
// user-edited; default_provider is the one field the CLI owns.
//
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

	changed, err := setDefaultProvider(&root, cfg.DefaultProvider)
	if err != nil {
		return err
	}
	if !changed {
		return nil // idempotent: avoid touching mtime when value already matches
	}

	// Use an Encoder with SetIndent(2) so re-emitted YAML matches the
	// 2-space convention the project's own config examples use. Default
	// Marshal emits 4-space indent which would normalise every user's
	// file on the first write.
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		_ = enc.Close()
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return atomicWrite(path, buf.Bytes(), 0o600)
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

// setDefaultProvider updates or inserts the default_provider key at the
// top level of root's YAML mapping. Operating on the node tree preserves
// comments and key order for every other key in the file.
//
// Returns changed=true only if the value was actually modified, so the
// caller can skip writing when the file is already in the desired state.
func setDefaultProvider(root *yaml.Node, value string) (bool, error) {
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return false, errors.New("config: unexpected YAML structure (not a document)")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return false, errors.New("config: top-level YAML is not a mapping")
	}

	// Content alternates key, value, key, value, ...
	for i := 0; i < len(top.Content)-1; i += 2 {
		k := top.Content[i]
		if k.Kind != yaml.ScalarNode || k.Value != "default_provider" {
			continue
		}
		v := top.Content[i+1]
		if v.Kind == yaml.ScalarNode && v.Value == value {
			return false, nil
		}
		v.Kind = yaml.ScalarNode
		v.Value = value
		v.Tag = "" // let yaml pick the best scalar tag (plain string)
		v.Style = 0
		return true, nil
	}

	// Not present — prepend so the user sees it at the top of the file.
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "default_provider"}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value}
	top.Content = append([]*yaml.Node{keyNode, valNode}, top.Content...)
	return true, nil
}

// atomicWrite writes data to path via a temp file in the same directory,
// then renames into place. Rename is atomic on POSIX and best-effort on
// Windows; in both cases a crash mid-write leaves the original file
// untouched rather than partially overwritten.
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
