package flagspec_test

import (
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// Reuses synthOptions and syntheticInfo from flagspec_test.go.

// --- Parse ------------------------------------------------------------------

func TestParse_AppliesDefaultsForMissingKeys(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	o := got.(*synthOptions)
	if o.Model != "canonical-pro" {
		t.Errorf("Model default: got %q, want canonical-pro", o.Model)
	}
	if o.Size != "1K" {
		t.Errorf("Size default: got %q, want 1K", o.Size)
	}
	if o.Score != 50 {
		t.Errorf("Score default: got %d, want 50", o.Score)
	}
}

func TestParse_OverridesValuesViaMap(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{
		"model":  "flash",
		"size":   "2K",
		"tag":    "16:9",
		"enable": true,
		"score":  75,
	}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	o := got.(*synthOptions)
	if o.Model != "canonical-flash" {
		t.Errorf("Model: got %q, want canonical-flash (alias resolved)", o.Model)
	}
	if o.Size != "2K" {
		t.Errorf("Size: got %q, want 2K", o.Size)
	}
	if o.Tag != "16:9" {
		t.Errorf("Tag: got %q, want 16:9", o.Tag)
	}
	if !o.Enable {
		t.Errorf("Enable: got false, want true")
	}
	if o.Score != 75 {
		t.Errorf("Score: got %d, want 75", o.Score)
	}
}

func TestParse_RejectsUnknownKeys(t *testing.T) {
	_, err := flagspec.Parse(synthOptions{}, map[string]any{
		"size":     "1K",
		"bogus":    "value",
		"unknown2": 42,
	}, syntheticInfo)
	if err == nil {
		t.Fatal("expected unknown-key error")
	}
	if !strings.Contains(err.Error(), "unknown key") {
		t.Errorf("error should mention unknown key, got %v", err)
	}
	if !strings.Contains(err.Error(), "bogus") || !strings.Contains(err.Error(), "unknown2") {
		t.Errorf("error should list both unknown keys, got %v", err)
	}
}

func TestParse_EnumRejectsInvalidValue(t *testing.T) {
	_, err := flagspec.Parse(synthOptions{}, map[string]any{"size": "8K"}, syntheticInfo)
	if err == nil || !strings.Contains(err.Error(), `invalid --size "8K"`) {
		t.Errorf("expected enum rejection, got %v", err)
	}
}

func TestParse_RangeEnforced(t *testing.T) {
	_, err := flagspec.Parse(synthOptions{}, map[string]any{"score": 200}, syntheticInfo)
	if err == nil || !strings.Contains(err.Error(), "must be 0-100") {
		t.Errorf("expected range error, got %v", err)
	}
}

// JSON decode produces float64 for all numbers; Parse must accept whole
// floats for int fields.
func TestParse_AcceptsFloat64ForIntFields(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{"score": float64(75)}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.(*synthOptions).Score != 75 {
		t.Errorf("Score: got %d, want 75", got.(*synthOptions).Score)
	}
}

func TestParse_RejectsFractionalFloatForIntField(t *testing.T) {
	_, err := flagspec.Parse(synthOptions{}, map[string]any{"score": 75.5}, syntheticInfo)
	if err == nil || !strings.Contains(err.Error(), "must be an integer") {
		t.Errorf("expected integer-coercion error, got %v", err)
	}
}

// Bool fields require an actual bool — quoted strings shouldn't silently
// coerce.
func TestParse_BoolRejectsString(t *testing.T) {
	_, err := flagspec.Parse(synthOptions{}, map[string]any{"enable": "true"}, syntheticInfo)
	if err == nil || !strings.Contains(err.Error(), "true or false") {
		t.Errorf("expected bool-coercion error, got %v", err)
	}
}

// Numeric YAML values for string fields (e.g. user wrote `size: 1024`
// without quotes) should coerce to text rather than erroring.
func TestParse_StringFieldAcceptsNumeric(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{"tag": 1024}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.(*synthOptions).Tag != "1024" {
		t.Errorf("Tag: got %q, want 1024 (rendered)", got.(*synthOptions).Tag)
	}
}

func TestParse_NormalizeHookCalled(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{"tag": "  16:9  "}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.(*synthOptions).Tag != "16:9" {
		t.Errorf("Normalize should have trimmed Tag: got %q", got.(*synthOptions).Tag)
	}
}

func TestParse_ValidateHookCalled(t *testing.T) {
	_, err := flagspec.Parse(validatedOptions{}, map[string]any{"a": "x"}, providers.Info{})
	if err == nil || err.Error() != "a requires b" {
		t.Errorf("expected cross-field error, got %v", err)
	}
}

func TestParse_ModelsEnumResolvesAlias(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{"model": "f"}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.(*synthOptions).Model != "canonical-flash" {
		t.Errorf("Model: got %q, want canonical-flash", got.(*synthOptions).Model)
	}
}

func TestParse_StrictEnumCanonicalised(t *testing.T) {
	got, err := flagspec.Parse(synthOptions{}, map[string]any{"strict": "minimal"}, syntheticInfo)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.(*synthOptions).Strict != "MINIMAL" {
		t.Errorf("Strict: got %q, want MINIMAL", got.(*synthOptions).Strict)
	}
}

// --- Parity ----------------------------------------------------------------
//
// Read and Parse must produce equivalent results for equivalent inputs;
// they're the CLI-side and YAML-side faces of the same provider schema.
// Without this guard they can drift silently as flagspec evolves.

// TestParse_ParityWithRead drives both code paths against the same
// inputs (a CLI flag set vs an equivalent map) and asserts the
// resulting Options structs match field-for-field.
func TestParse_ParityWithRead(t *testing.T) {
	cases := []struct {
		name string
		args []string         // for Read (cobra parsing)
		vals map[string]any   // for Parse (yaml/json values)
	}{
		{
			name: "all defaults",
			args: nil,
			vals: map[string]any{},
		},
		{
			name: "model alias",
			args: []string{"--model", "flash"},
			vals: map[string]any{"model": "flash"},
		},
		{
			name: "size enum + tag passthrough",
			args: []string{"--size", "2K", "--tag", "16:9"},
			vals: map[string]any{"size": "2K", "tag": "16:9"},
		},
		{
			name: "bool + int",
			args: []string{"--enable", "--score", "75"},
			vals: map[string]any{"enable": true, "score": 75},
		},
		{
			name: "case-insensitive enum canonicalised",
			args: []string{"--strict", "minimal"},
			vals: map[string]any{"strict": "minimal"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := bindAndSet(t, c.args...)
			read, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			parse, err := flagspec.Parse(synthOptions{}, c.vals, syntheticInfo)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			r, p := read.(*synthOptions), parse.(*synthOptions)
			if *r != *p {
				t.Errorf("Read/Parse mismatch:\n  Read  = %+v\n  Parse = %+v", *r, *p)
			}
		})
	}
}
