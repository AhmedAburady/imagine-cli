package flagspec_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/providers"
	"github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// Synthetic Options used across these tests. Covers every supported tag.
type synthOptions struct {
	Model  string `flag:"model,m"  desc:"Model"                default:"pro" enum:"@models"`
	Size   string `flag:"size,s"   desc:"Size: 1K, 2K, 4K"     default:"1K"  enum:"1K,2K,4K"`
	Tag    string `flag:"tag,t"    desc:"Free-form"`
	Strict string `flag:"strict"   desc:"Upper-only"           enum:"MINIMAL,HIGH"`
	Enable bool   `flag:"enable,e" desc:"Toggle"`
	Score  int    `flag:"score"    desc:"0-100"                default:"50" range:"0:100"`
	Hidden string `flag:"-"`
	Plain  string // no flag tag → ignored
}

func (o *synthOptions) Normalize() {
	o.Tag = strings.TrimSpace(o.Tag)
}

var syntheticInfo = providers.Info{
	Name:         "synth",
	DefaultModel: "canonical-pro",
	Models: []providers.ModelInfo{
		{ID: "canonical-pro", Aliases: []string{"pro"}},
		{ID: "canonical-flash", Aliases: []string{"flash", "f"}},
	},
}

func newCmd() *cobra.Command {
	return &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
}

// --- Bind -------------------------------------------------------------------

func TestBind_RegistersAllFlagsWithShorthandsAndDefaults(t *testing.T) {
	cmd := newCmd()
	if err := flagspec.Bind(cmd, synthOptions{}); err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	cases := []struct {
		name  string
		short string
		def   string
	}{
		{"model", "m", "pro"},
		{"size", "s", "1K"},
		{"tag", "t", ""},
		{"strict", "", ""},
		{"enable", "e", "false"},
		{"score", "", "50"},
	}
	for _, c := range cases {
		fl := cmd.Flags().Lookup(c.name)
		if fl == nil {
			t.Errorf("flag %q not registered", c.name)
			continue
		}
		if fl.Shorthand != c.short {
			t.Errorf("flag %q shorthand: got %q, want %q", c.name, fl.Shorthand, c.short)
		}
		if fl.DefValue != c.def {
			t.Errorf("flag %q default: got %q, want %q", c.name, fl.DefValue, c.def)
		}
	}
}

func TestBind_IgnoresDashAndUntaggedFields(t *testing.T) {
	cmd := newCmd()
	if err := flagspec.Bind(cmd, synthOptions{}); err != nil {
		t.Fatalf("Bind error: %v", err)
	}
	if cmd.Flags().Lookup("hidden") != nil {
		t.Error("flag:\"-\" field should not register")
	}
	if cmd.Flags().Lookup("plain") != nil {
		t.Error("untagged field should not register")
	}
}

func TestBind_Idempotent(t *testing.T) {
	cmd := newCmd()
	// First bind — canonical source of desc.
	if err := flagspec.Bind(cmd, synthOptions{}); err != nil {
		t.Fatalf("first Bind: %v", err)
	}
	origDesc := cmd.Flags().Lookup("model").Usage

	// Second bind against a struct with a different desc — must no-op on name collision.
	type second struct {
		Model string `flag:"model,m" desc:"Overridden desc" default:"flash" enum:"@models"`
	}
	if err := flagspec.Bind(cmd, second{}); err != nil {
		t.Fatalf("second Bind: %v", err)
	}
	if cmd.Flags().Lookup("model").Usage != origDesc {
		t.Errorf("second Bind overwrote desc: got %q, want %q", cmd.Flags().Lookup("model").Usage, origDesc)
	}
}

func TestBind_RejectsUnsupportedKind(t *testing.T) {
	type bad struct {
		Ratios []string `flag:"ratios"`
	}
	cmd := newCmd()
	err := flagspec.Bind(cmd, bad{})
	if err == nil || !strings.Contains(err.Error(), "unsupported kind") {
		t.Errorf("expected unsupported-kind error, got %v", err)
	}
}

func TestBind_RejectsBadDefaults(t *testing.T) {
	type bad struct {
		On bool `flag:"on" default:"maybe"`
	}
	cmd := newCmd()
	err := flagspec.Bind(cmd, bad{})
	if err == nil || !strings.Contains(err.Error(), "invalid bool default") {
		t.Errorf("expected bool default error, got %v", err)
	}
}

func TestBind_RejectsRangeOnNonInt(t *testing.T) {
	type bad struct {
		S string `flag:"s" range:"0:10"`
	}
	cmd := newCmd()
	err := flagspec.Bind(cmd, bad{})
	if err == nil || !strings.Contains(err.Error(), "range tag only valid on int") {
		t.Errorf("expected range-on-non-int error, got %v", err)
	}
}

func TestBind_RejectsBadRange(t *testing.T) {
	type bad struct {
		N int `flag:"n" range:"10:5"`
	}
	cmd := newCmd()
	err := flagspec.Bind(cmd, bad{})
	if err == nil || !strings.Contains(err.Error(), "min > max") {
		t.Errorf("expected range min>max error, got %v", err)
	}
}

// --- Read -------------------------------------------------------------------

// bindAndSet is a test helper: bind, set args, parse.
func bindAndSet(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := newCmd()
	if err := flagspec.Bind(cmd, synthOptions{}); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return cmd
}

func TestRead_UsesDefaultsWhenFlagsUnset(t *testing.T) {
	cmd := bindAndSet(t)
	got, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	o := got.(*synthOptions)
	if o.Model != "canonical-pro" {
		t.Errorf("Model default: got %q, want %q (alias 'pro' → canonical)", o.Model, "canonical-pro")
	}
	if o.Size != "1K" {
		t.Errorf("Size default: got %q, want %q", o.Size, "1K")
	}
	if o.Score != 50 {
		t.Errorf("Score default: got %d, want 50", o.Score)
	}
}

func TestRead_ModelsEnumResolvesAlias(t *testing.T) {
	cmd := bindAndSet(t, "--model", "flash")
	got, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.(*synthOptions).Model != "canonical-flash" {
		t.Errorf("Model: got %q, want canonical-flash", got.(*synthOptions).Model)
	}
}

func TestRead_ModelsEnumRejectsUnknown(t *testing.T) {
	cmd := bindAndSet(t, "--model", "bogus")
	_, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err == nil || !strings.Contains(err.Error(), "unknown model") {
		t.Errorf("expected unknown-model error, got %v", err)
	}
}

func TestRead_EnumCaseInsensitiveCanonicalised(t *testing.T) {
	cmd := bindAndSet(t, "--strict", "minimal")
	got, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.(*synthOptions).Strict != "MINIMAL" {
		t.Errorf("Strict: got %q, want MINIMAL (canonicalised)", got.(*synthOptions).Strict)
	}
}

func TestRead_EnumRejectsInvalid(t *testing.T) {
	cmd := bindAndSet(t, "--size", "8K")
	_, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err == nil || !strings.Contains(err.Error(), `invalid --size "8K"`) {
		t.Errorf("expected enum rejection, got %v", err)
	}
}

func TestRead_FreeFormStringAcceptsAnyValue(t *testing.T) {
	cmd := bindAndSet(t, "--tag", "16:9")
	got, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.(*synthOptions).Tag != "16:9" {
		t.Errorf("Tag: got %q, want 16:9", got.(*synthOptions).Tag)
	}
}

func TestRead_RangeEnforced(t *testing.T) {
	cases := []struct {
		score  string
		wantOK bool
	}{
		{"0", true},
		{"50", true},
		{"100", true},
		{"-1", false},
		{"101", false},
	}
	for _, c := range cases {
		cmd := bindAndSet(t, "--score", c.score)
		_, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
		if c.wantOK && err != nil {
			t.Errorf("score=%s: unexpected error %v", c.score, err)
		}
		if !c.wantOK && err == nil {
			t.Errorf("score=%s: expected error", c.score)
		}
	}
}

func TestRead_BoolFlag(t *testing.T) {
	cmd := bindAndSet(t, "--enable")
	got, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !got.(*synthOptions).Enable {
		t.Error("Enable: expected true")
	}
}

func TestRead_NormalizeHookCalled(t *testing.T) {
	cmd := bindAndSet(t, "--tag", "  16:9  ")
	got, err := flagspec.Read(cmd, synthOptions{}, syntheticInfo)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.(*synthOptions).Tag != "16:9" {
		t.Errorf("Normalize should have trimmed Tag: got %q", got.(*synthOptions).Tag)
	}
}

// Separate type to test Validate hook in isolation.
type validatedOptions struct {
	A string `flag:"a"`
	B string `flag:"b"`
}

func (o *validatedOptions) Validate(_ providers.Info) error {
	if o.A != "" && o.B == "" {
		return errors.New("a requires b")
	}
	return nil
}

func TestRead_ValidateHookCalled(t *testing.T) {
	cmd := newCmd()
	if err := flagspec.Bind(cmd, validatedOptions{}); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	cmd.SetArgs([]string{"--a", "x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	_, err := flagspec.Read(cmd, validatedOptions{}, providers.Info{})
	if err == nil || err.Error() != "a requires b" {
		t.Errorf("expected cross-field error, got %v", err)
	}
}

// --- FieldNames -------------------------------------------------------------

func TestFieldNames_ReturnsDeclaredOrder(t *testing.T) {
	got := flagspec.FieldNames(synthOptions{})
	want := []string{"model", "size", "tag", "strict", "enable", "score"}
	if len(got) != len(want) {
		t.Fatalf("FieldNames: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FieldNames[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
