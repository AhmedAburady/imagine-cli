package batch

import (
	"strings"
	"testing"

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
