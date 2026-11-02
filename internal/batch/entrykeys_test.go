package batch

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/api"
	"github.com/AhmedAburady/imagine-cli/providers"
)

func TestCheckEntryKeys(t *testing.T) {
	b := providers.Bundle{SupportedFlags: []string{"model", "size"}, Info: providers.Info{Name: "fake"}}
	if err := checkEntryKeys(map[string]any{"model": "x"}, b); err != nil {
		t.Errorf("provider key rejected: %v", err)
	}
	err := checkEntryKeys(map[string]any{"name": "hero"}, b)
	if err == nil || !strings.Contains(err.Error(), "filename stem") {
		t.Errorf("name: want the entry-key hint, got %v", err)
	}
	err = checkEntryKeys(map[string]any{"colour": "red"}, b)
	if err == nil || !strings.Contains(err.Error(), "prompt") || !strings.Contains(err.Error(), "size") {
		t.Errorf("unknown key: want common and provider keys listed, got %v", err)
	}
}

// An absolute filename: and a relative one naming the same file collide.
func TestDetectCollisions_AbsoluteAndRelativeSameFile(t *testing.T) {
	dir := t.TempDir()
	mk := func(name, filename string) Resolved {
		return Resolved{DisplayName: name, Params: api.Params{OutputFolder: dir, OutputFilename: filename, NumImages: 1}}
	}
	got := detectCollisions([]Resolved{mk("a", "hero.png"), mk("b", filepath.Join(dir, "hero.png"))})
	if len(got) != 1 {
		t.Fatalf("want one collision, got %v", got)
	}
	if got := detectCollisions([]Resolved{mk("a", "hero.png"), mk("b", filepath.Join(dir, "sub", "hero.png"))}); len(got) != 0 {
		t.Errorf("distinct files reported as colliding: %v", got)
	}
}
