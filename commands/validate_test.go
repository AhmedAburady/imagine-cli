package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// --- Fixtures ---------------------------------------------------------------

// Synthetic Info mirroring Gemini's shape: pro has no SupportedFlags,
// flash declares {thinking, image-search}.
var fxInfo = providers.Info{
	Name:         "fake",
	DefaultModel: "fake-pro",
	Models: []providers.ModelInfo{
		{ID: "fake-pro", Aliases: []string{"pro"}},
		{ID: "fake-flash", Aliases: []string{"flash"}, SupportedFlags: []string{"thinking", "image-search"}},
	},
}

var fxBundle = providers.Bundle{
	Info:           fxInfo,
	SupportedFlags: []string{"model", "size", "thinking", "image-search"},
}

// typedOpts implements providers.ResolvedModeler.
type typedOpts struct{ Model string }

func (t *typedOpts) ResolvedModel() string { return t.Model }

// bindFlagsOnly builds a minimal cobra command that declares fxBundle's
// flags and parses the given args. Drives cmd.Flags().Visit inside the
// commands-package adapters exactly as the real CLI would.
func bindFlagsOnly(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	f := cmd.Flags()
	f.String("model", "", "")
	f.String("size", "", "")
	f.String("thinking", "", "")
	f.Bool("image-search", false, "")
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return cmd
}

// --- enforceModelSupport (single-shot adapter over providers.CheckModel) ----

func TestEnforceModelSupport_RejectsAdvancedFlagOnUnsupportingModel(t *testing.T) {
	cmd := bindFlagsOnly(t, "--thinking", "high")
	err := enforceModelSupport(cmd, fxBundle, &typedOpts{Model: "fake-pro"})
	if err == nil {
		t.Fatal("expected rejection, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "--thinking") {
		t.Errorf("error should mention the flag: %q", msg)
	}
	if !strings.Contains(msg, "fake-pro") {
		t.Errorf("error should mention the model: %q", msg)
	}
	if !strings.Contains(msg, "flash") {
		t.Errorf("error should hint the alias that supports it: %q", msg)
	}
}

func TestEnforceModelSupport_AcceptsAdvancedFlagOnSupportingModel(t *testing.T) {
	cmd := bindFlagsOnly(t, "--thinking", "high")
	err := enforceModelSupport(cmd, fxBundle, &typedOpts{Model: "fake-flash"})
	if err != nil {
		t.Errorf("expected accept, got %v", err)
	}
}

func TestEnforceModelSupport_IgnoresNonAdvancedFlags(t *testing.T) {
	cmd := bindFlagsOnly(t, "--size", "2K")
	err := enforceModelSupport(cmd, fxBundle, &typedOpts{Model: "fake-pro"})
	if err != nil {
		t.Errorf("non-advanced flag should be unaffected, got %v", err)
	}
}

func TestEnforceModelSupport_NoOpWhenNoAdvancedFlagsDeclared(t *testing.T) {
	plainBundle := providers.Bundle{
		Info: providers.Info{
			Name:   "plain",
			Models: []providers.ModelInfo{{ID: "only-model"}},
		},
	}
	cmd := bindFlagsOnly(t, "--thinking", "high")
	err := enforceModelSupport(cmd, plainBundle, &typedOpts{Model: "only-model"})
	if err != nil {
		t.Errorf("no-advanced-flags providers should no-op, got %v", err)
	}
}

func TestEnforceModelSupport_OptsOutWhenNoResolver(t *testing.T) {
	cmd := bindFlagsOnly(t, "--thinking", "high")
	err := enforceModelSupport(cmd, fxBundle, nil)
	if err != nil {
		t.Errorf("nil options should short-circuit, got %v", err)
	}
}

func TestEnforceModelSupport_WorksWithMapOptions(t *testing.T) {
	cmd := bindFlagsOnly(t, "--thinking", "high")
	err := enforceModelSupport(cmd, fxBundle, map[string]any{"model": "fake-pro"})
	if err == nil || !strings.Contains(err.Error(), "fake-pro") {
		t.Errorf("expected map-based fallback to reject: %v", err)
	}
}
