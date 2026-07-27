package providers_test

import (
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
)

var sizeInfo = providers.Info{
	Name:         "fake",
	DefaultModel: "fake-pro",
	Models: []providers.ModelInfo{
		{ID: "fake-pro", Aliases: []string{"pro"}},
		{ID: "fake-lite", Aliases: []string{"lite"}, Sizes: []string{"1K"}},
	},
	Capabilities: providers.Capabilities{Sizes: []string{"1K", "2K", "4K"}},
}

// --- SizesForModel ----------------------------------------------------------

func TestSizesForModel_FallsBackToCapabilities(t *testing.T) {
	got := sizeInfo.SizesForModel("fake-pro")
	if len(got) != 3 {
		t.Errorf("model without own Sizes should inherit the provider set, got %v", got)
	}
}

func TestSizesForModel_PrefersModelNarrowing(t *testing.T) {
	got := sizeInfo.SizesForModel("fake-lite")
	if len(got) != 1 || got[0] != "1K" {
		t.Errorf("got %v, want [1K]", got)
	}
}

func TestSizesForModel_UnknownIDFallsBack(t *testing.T) {
	got := sizeInfo.SizesForModel("not-a-model")
	if len(got) != 3 {
		t.Errorf("unknown ID should fall back to the provider set, got %v", got)
	}
}

// --- CheckSize --------------------------------------------------------------

func TestCheckSize_RejectsSizeAboveModelCeiling(t *testing.T) {
	err := sizeInfo.CheckSize("fake-lite", "4K")
	if err == nil {
		t.Fatal("expected 4K on a 1K-only model to be rejected")
	}
	msg := err.Error()
	if !strings.Contains(msg, "fake-lite") || !strings.Contains(msg, "1K") {
		t.Errorf("error should name the model and its accepted sizes: %q", msg)
	}
}

func TestCheckSize_AcceptsWithinModelCeiling(t *testing.T) {
	if err := sizeInfo.CheckSize("fake-lite", "1K"); err != nil {
		t.Errorf("expected 1K to be accepted: %v", err)
	}
}

func TestCheckSize_AcceptsProviderWideForUnrestrictedModel(t *testing.T) {
	if err := sizeInfo.CheckSize("fake-pro", "4K"); err != nil {
		t.Errorf("expected 4K on an unrestricted model to be accepted: %v", err)
	}
}

func TestCheckSize_CaseInsensitive(t *testing.T) {
	if err := sizeInfo.CheckSize("fake-lite", "1k"); err != nil {
		t.Errorf("comparison should be case-insensitive: %v", err)
	}
}

func TestCheckSize_EmptySizePasses(t *testing.T) {
	if err := sizeInfo.CheckSize("fake-lite", ""); err != nil {
		t.Errorf("empty size means provider default, should pass: %v", err)
	}
}

func TestCheckSize_NoDeclaredSizesPasses(t *testing.T) {
	info := providers.Info{Models: []providers.ModelInfo{{ID: "a"}}}
	if err := info.CheckSize("a", "4K"); err != nil {
		t.Errorf("provider declaring no sizes should not gate: %v", err)
	}
}

// --- CheckAspectRatio -------------------------------------------------------

var arInfo = providers.Info{
	Name:         "fake",
	Capabilities: providers.Capabilities{AspectRatios: []string{"1:1", "16:9", "8:1"}},
}

func TestCheckAspectRatio_AcceptsDeclaredRatio(t *testing.T) {
	for _, ar := range []string{"1:1", "16:9", "8:1"} {
		if err := arInfo.CheckAspectRatio(ar); err != nil {
			t.Errorf("%s should be accepted: %v", ar, err)
		}
	}
}

func TestCheckAspectRatio_RejectsUndeclaredRatio(t *testing.T) {
	err := arInfo.CheckAspectRatio("9:21")
	if err == nil {
		t.Fatal("expected an undeclared ratio to be rejected")
	}
	// The message carries the full list — it's where a user learns what's
	// accepted without re-running --help.
	for _, want := range []string{"9:21", "1:1", "16:9", "8:1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got %q", want, err)
		}
	}
}

func TestCheckAspectRatio_EmptyMeansAuto(t *testing.T) {
	if err := arInfo.CheckAspectRatio(""); err != nil {
		t.Errorf("empty ratio means the model chooses, should pass: %v", err)
	}
}

func TestCheckAspectRatio_NoDeclaredRatiosPasses(t *testing.T) {
	info := providers.Info{Name: "openai-like"}
	if err := info.CheckAspectRatio("anything"); err != nil {
		t.Errorf("provider declaring no ratios should not gate: %v", err)
	}
}
