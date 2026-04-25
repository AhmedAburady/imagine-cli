package providers_test

import (
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
)

var gateInfo = providers.Info{
	Name:         "fake",
	DefaultModel: "fake-pro",
	Models: []providers.ModelInfo{
		{ID: "fake-pro", Aliases: []string{"pro"}},
		{ID: "fake-flash", Aliases: []string{"flash"}, SupportedFlags: []string{"thinking", "image-search"}},
	},
}

var gateBundle = providers.Bundle{
	Info:           gateInfo,
	SupportedFlags: []string{"model", "size", "thinking", "image-search"},
}

type fakeOpts struct{ Model string }

func (f *fakeOpts) ResolvedModel() string { return f.Model }

// --- ResolvedModelID --------------------------------------------------------

func TestResolvedModelID_FromResolvedModeler(t *testing.T) {
	if got := providers.ResolvedModelID(&fakeOpts{Model: "fake-flash"}); got != "fake-flash" {
		t.Errorf("got %q, want fake-flash", got)
	}
}

func TestResolvedModelID_FromMap(t *testing.T) {
	if got := providers.ResolvedModelID(map[string]any{"model": "fake-pro"}); got != "fake-pro" {
		t.Errorf("got %q, want fake-pro", got)
	}
}

func TestResolvedModelID_Empty(t *testing.T) {
	if got := providers.ResolvedModelID(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
	if got := providers.ResolvedModelID(map[string]any{}); got != "" {
		t.Errorf("empty map: got %q, want empty", got)
	}
}

// --- AdvancedFlagSet --------------------------------------------------------

func TestAdvancedFlagSet_UnionAcrossModels(t *testing.T) {
	got := providers.AdvancedFlagSet(gateInfo)
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
		{ID: "a"}, {ID: "b"},
	}}
	if got := providers.AdvancedFlagSet(info); len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// --- ModelsSupportingFlag ---------------------------------------------------

func TestModelsSupportingFlag_PrefersAlias(t *testing.T) {
	got := providers.ModelsSupportingFlag(gateInfo, "thinking")
	if len(got) != 1 || got[0] != "flash" {
		t.Errorf("got %v, want [flash]", got)
	}
}

func TestModelsSupportingFlag_FallsBackToID(t *testing.T) {
	info := providers.Info{Models: []providers.ModelInfo{
		{ID: "no-alias-model", SupportedFlags: []string{"extra"}},
	}}
	got := providers.ModelsSupportingFlag(info, "extra")
	if len(got) != 1 || got[0] != "no-alias-model" {
		t.Errorf("got %v, want [no-alias-model]", got)
	}
}

// --- CheckBundle ------------------------------------------------------------

func TestCheckBundle_AcceptsClaimedFlags(t *testing.T) {
	errs := providers.CheckBundle([]string{"thinking", "size"}, gateBundle)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

// --- CheckModel -------------------------------------------------------------

func TestCheckModel_RejectsAdvancedFlagOnUnsupportingModel(t *testing.T) {
	errs := providers.CheckModel([]string{"thinking"}, gateBundle, &fakeOpts{Model: "fake-pro"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	msg := errs[0].Error()
	if !strings.Contains(msg, "fake-pro") || !strings.Contains(msg, "flash") {
		t.Errorf("error should name model and supporting alias: %q", msg)
	}
}

func TestCheckModel_AcceptsOnSupportingModel(t *testing.T) {
	errs := providers.CheckModel([]string{"thinking"}, gateBundle, &fakeOpts{Model: "fake-flash"})
	if len(errs) != 0 {
		t.Errorf("expected accept, got %v", errs)
	}
}

func TestCheckModel_NoOpWithoutResolver(t *testing.T) {
	errs := providers.CheckModel([]string{"thinking"}, gateBundle, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

// --- CheckClaimedSomewhere --------------------------------------------------

func TestCheckClaimedSomewhere_AcceptsIfAnyBundleClaims(t *testing.T) {
	other := providers.Bundle{SupportedFlags: []string{"unrelated"}}
	errs := providers.CheckClaimedSomewhere([]string{"thinking"}, []providers.Bundle{gateBundle, other})
	if len(errs) != 0 {
		t.Errorf("expected accept (gate bundle claims thinking), got %v", errs)
	}
}

func TestCheckClaimedSomewhere_RejectsIfNoBundleClaims(t *testing.T) {
	other := providers.Bundle{SupportedFlags: []string{"unrelated"}}
	errs := providers.CheckClaimedSomewhere([]string{"thinking"}, []providers.Bundle{other})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}
