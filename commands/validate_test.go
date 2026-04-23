package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// --- Fixtures ---------------------------------------------------------------

// Synthetic Info mirroring Gemini's shape: pro has no SupportedFlags, flash
// declares {thinking, image-search}.
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

// typedOpts implements ResolvedModeler.
type typedOpts struct{ Model string }

func (t *typedOpts) ResolvedModel() string { return t.Model }

// --- resolvedModelID --------------------------------------------------------

func TestResolvedModelID_FromResolvedModeler(t *testing.T) {
	got := resolvedModelID(&typedOpts{Model: "fake-flash"})
	if got != "fake-flash" {
		t.Errorf("got %q, want fake-flash", got)
	}
}

func TestResolvedModelID_FromMap(t *testing.T) {
	got := resolvedModelID(map[string]any{"model": "fake-pro"})
	if got != "fake-pro" {
		t.Errorf("got %q, want fake-pro", got)
	}
}

func TestResolvedModelID_Empty(t *testing.T) {
	if got := resolvedModelID(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
	if got := resolvedModelID(map[string]any{}); got != "" {
		t.Errorf("empty map: got %q, want empty", got)
	}
}

// --- advancedFlagSet --------------------------------------------------------

func TestAdvancedFlagSet_UnionAcrossModels(t *testing.T) {
	got := advancedFlagSet(fxInfo)
	want := map[string]bool{"thinking": true, "image-search": true}
	if len(got) != len(want) {
		t.Fatalf("size: got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing %q in advanced set", k)
		}
	}
}

func TestAdvancedFlagSet_Empty(t *testing.T) {
	info := providers.Info{Models: []providers.ModelInfo{
		{ID: "a"}, {ID: "b"}, // no SupportedFlags on either
	}}
	got := advancedFlagSet(info)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// --- enforceModelSupport ----------------------------------------------------

// bindFlagsOnly builds a minimal cobra command that declares fxBundle's
// flags and parses the given args. Used to drive cmd.Flags().Visit inside
// enforceModelSupport exactly as the real CLI would.
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
	// --size is not in advanced set (no model declares it), so it passes
	// regardless of resolved model.
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
	// nil Options → resolvedModelID returns "" → function short-circuits.
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

// --- modelsSupportingFlag ---------------------------------------------------

func TestModelsSupportingFlag_PrefersAlias(t *testing.T) {
	got := modelsSupportingFlag(fxInfo, "thinking")
	if len(got) != 1 || got[0] != "flash" {
		t.Errorf("got %v, want [flash]", got)
	}
}

func TestModelsSupportingFlag_FallsBackToID(t *testing.T) {
	info := providers.Info{Models: []providers.ModelInfo{
		{ID: "no-alias-model", SupportedFlags: []string{"extra"}},
	}}
	got := modelsSupportingFlag(info, "extra")
	if len(got) != 1 || got[0] != "no-alias-model" {
		t.Errorf("got %v, want [no-alias-model]", got)
	}
}
