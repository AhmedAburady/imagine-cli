package providers

import "testing"

func TestVisionNormalizeEffort(t *testing.T) {
	v := &Vision{Efforts: []string{"low", "medium", "high"}, DefaultEffort: "medium"}

	// Empty → default.
	if got, err := v.NormalizeEffort(""); got != "medium" || err != nil {
		t.Errorf(`empty: got %q,%v; want "medium",nil`, got, err)
	}
	// Case-insensitive + trim.
	if got, err := v.NormalizeEffort("  HIGH "); got != "high" || err != nil {
		t.Errorf(`"  HIGH ": got %q,%v; want "high",nil`, got, err)
	}
	// Unsupported → error listing the valid set.
	if _, err := v.NormalizeEffort("xhigh"); err == nil {
		t.Error("xhigh should be rejected for low/medium/high provider")
	}

	// Provider with no effort control passes anything through.
	none := &Vision{}
	if got, err := none.NormalizeEffort("whatever"); got != "whatever" || err != nil {
		t.Errorf("no-efforts provider should pass through, got %q,%v", got, err)
	}
}

func TestVisionNormalizeEffortForModel(t *testing.T) {
	v := &Vision{DefaultModel: "def", Efforts: []string{"low", "medium", "high"}, DefaultEffort: "medium"}

	// Default model (empty or exact match): validate + apply default.
	if got, err := v.NormalizeEffortForModel("", ""); got != "medium" || err != nil {
		t.Errorf("default model empty → medium; got %q,%v", got, err)
	}
	if _, err := v.NormalizeEffortForModel("xhigh", "def"); err == nil {
		t.Error("default model should reject xhigh")
	}

	// Custom model: explicit effort passes through, empty stays empty (no forced default).
	if got, err := v.NormalizeEffortForModel("xhigh", "custom"); got != "xhigh" || err != nil {
		t.Errorf("custom model explicit → passthrough; got %q,%v", got, err)
	}
	if got, err := v.NormalizeEffortForModel("", "custom"); got != "" || err != nil {
		t.Errorf("custom model empty → no forced default; got %q,%v", got, err)
	}
}
