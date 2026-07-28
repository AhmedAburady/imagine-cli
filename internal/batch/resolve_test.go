package batch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/AhmedAburady/imagine-cli/cli"
	"github.com/AhmedAburady/imagine-cli/config"
	"github.com/AhmedAburady/imagine-cli/internal/batch"
	"github.com/AhmedAburady/imagine-cli/internal/images"
	_ "github.com/AhmedAburady/imagine-cli/providers/all"
)

// stubCmd builds a cobra command with the provider-private flags
// declared by gemini, vertex, and openai (the providers blank-imported
// above). Args are parsed so cmd.Flags().Visit reflects "what the user
// set explicitly" the same way the real CLI does.
func stubCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	f := cmd.Flags()
	// Provider-private flags (union across gemini/openai/vertex).
	f.StringP("model", "m", "", "")
	f.StringP("size", "s", "", "")
	f.StringP("aspect-ratio", "a", "", "")
	f.BoolP("grounding", "g", false, "")
	f.StringP("thinking", "t", "", "")
	f.BoolP("image-search", "I", false, "")
	f.StringP("quality", "q", "", "")
	f.Int("compression", 100, "")
	f.String("moderation", "", "")
	f.String("background", "", "")
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return cmd
}

// stubConfig returns a config with stubbed credentials so every
// provider's Factory succeeds without real keys. No HTTP fires inside
// Resolve — Generate is what would call the API, and we never run it.
func stubConfig() *config.Config {
	return &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {"api_key": "test-key"},
			"gemini": {"api_key": "test-key"},
			"vertex": {
				"gcp_project": "test-proj",
				"location":    "us-central1",
			},
		},
	}
}

func defaultCLI() *cli.Options {
	return &cli.Options{
		Output:    ".",
		NumImages: 1,
	}
}

// --- Single-entry happy path ------------------------------------------------

func TestResolve_SingleEntryUsesCLIDefaults(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": "x", "provider": "openai"}},
	}}
	cli := defaultCLI()
	cli.NumImages = 3
	cli.Output = "/tmp/out"

	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      cli,
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("got %d resolved, want 1", len(resolved))
	}
	r := resolved[0]
	if r.Params.NumImages != 3 {
		t.Errorf("NumImages: got %d, want 3 (CLI default)", r.Params.NumImages)
	}
	if r.Params.OutputFolder != "/tmp/out" {
		t.Errorf("OutputFolder: got %q, want /tmp/out", r.Params.OutputFolder)
	}
	if r.Params.OutputFilename != "hero" {
		t.Errorf("OutputFilename: got %q, want hero (entry key fallback)", r.Params.OutputFilename)
	}
}

// --- Entry overrides win over CLI -------------------------------------------

func TestResolve_EntryOverrideWinsOverCLI(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{
			"prompt":   "x",
			"provider": "openai",
			"count":    5,
			"output":   "/tmp/specific",
		}},
	}}
	cli := defaultCLI()
	cli.NumImages = 1
	cli.Output = "/tmp/global"

	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      cli,
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	r := resolved[0]
	if r.Params.NumImages != 5 {
		t.Errorf("NumImages: got %d, want 5 (entry override)", r.Params.NumImages)
	}
	if r.Params.OutputFolder != "/tmp/specific" {
		t.Errorf("OutputFolder: got %q, want /tmp/specific (entry override)", r.Params.OutputFolder)
	}
}

// --- Embed metadata ---------------------------------------------------------

func metaValue(tags []images.TextTag, key string) string {
	for _, t := range tags {
		if t.Key == key {
			return t.Value
		}
	}
	return ""
}

func TestResolve_EmbedMetadataFromCLI(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": "a cat", "provider": "openai"}},
	}}
	cli := defaultCLI()
	cli.EmbedMetadata = true

	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec: spec, CLIOptions: cli, Cmd: stubCmd(t), Config: stubConfig(), DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	m := resolved[0].Params.Metadata
	if metaValue(m, "prompt") != "a cat" || metaValue(m, "provider") != "openai" || metaValue(m, "model") == "" {
		t.Errorf("metadata not threaded from CLI flag: %+v", m)
	}
}

func TestResolve_EmbedMetadataEntryOverrides(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "on", Index: 0, Raw: map[string]any{"prompt": "x", "provider": "openai", "embed-metadata": true}},
		{Key: "off", Index: 1, Raw: map[string]any{"prompt": "y", "provider": "openai", "embed-metadata": false}},
	}}
	cli := defaultCLI() // CLI flag off; entries decide

	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec: spec, CLIOptions: cli, Cmd: stubCmd(t), Config: stubConfig(), DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve (entry-level embed-metadata must not be an unknown key): %v", err)
	}
	if len(resolved[0].Params.Metadata) == 0 {
		t.Error("entry embed-metadata:true should set Metadata")
	}
	if len(resolved[1].Params.Metadata) != 0 {
		t.Error("entry embed-metadata:false should leave Metadata empty")
	}
}

// --- Per-entry CLI flag filtering -------------------------------------------

// Mixed-provider batch with --thinking high on the CLI: gemini entries
// should pick it up (gemini claims --thinking); openai entries should
// silently ignore it (openai doesn't claim --thinking).
func TestResolve_CLIFlagFlowsOnlyToClaimingProvider(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{
			"prompt":   "x",
			"provider": "gemini",
			"model":    "flash", // flash supports thinking
		}},
		{Key: "castle", Index: 1, Raw: map[string]any{
			"prompt":   "y",
			"provider": "openai",
		}},
	}}

	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t, "--thinking", "high"),
		Config:          stubConfig(),
		DefaultProvider: "gemini",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(resolved) != 2 {
		t.Fatalf("got %d resolved, want 2", len(resolved))
	}
	// Both entries succeeded; openai didn't choke on --thinking.
}

// --- Cross-entry unclaimed flag --------------------------------------------

func TestResolve_RejectsCLIFlagNoEntryProviderClaims(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": "x", "provider": "openai"}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t, "--thinking", "high"),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil {
		t.Fatal("expected rejection: --thinking unclaimed by openai")
	}
	if !strings.Contains(err.Error(), "thinking") {
		t.Errorf("error should mention --thinking, got %v", err)
	}
}

// --- Per-entry prompt file resolution --------------------------------------

func TestResolve_EntryPromptReadsFileContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hero.md")
	if err := os.WriteFile(path, []byte("  a samurai at dusk, cinematic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": path, "provider": "openai"}},
	}}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := resolved[0].Request.Prompt; got != "a samurai at dusk, cinematic" {
		t.Errorf("Prompt: got %q, want trimmed file contents", got)
	}
}

func TestResolve_EntryPromptLiteralWhenNotAFile(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": "a sunset over mountains", "provider": "openai"}},
	}}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := resolved[0].Request.Prompt; got != "a sunset over mountains" {
		t.Errorf("Prompt: got %q, want literal text unchanged", got)
	}
}

func TestResolve_EntryPromptRejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(path, []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": path, "provider": "openai"}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty-file error, got %v", err)
	}
}

func TestResolve_EntryPromptRelativeToBatchDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hero.md"), []byte("a knight in the rain\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Spec.Path lives in dir, so the bare relative prompt path resolves
	// next to the batch file — not against the process working directory.
	spec := &batch.Spec{Path: filepath.Join(dir, "batch.yaml"), Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": "hero.md", "provider": "openai"}},
	}}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := resolved[0].Request.Prompt; got != "a knight in the rain" {
		t.Errorf("Prompt: got %q, want file contents resolved against batch dir", got)
	}
}

// --- Per-entry prompt concatenation ----------------------------------------

func TestResolve_EntryPromptListConcatenates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "base.md"), []byte("# Base\nwatercolour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Mixed parts: a file next to the batch file, then literal text.
	spec := &batch.Spec{Path: filepath.Join(dir, "batch.yaml"), Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{
			"prompt":   []any{"base.md", "at night"},
			"provider": "openai",
		}},
	}}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "# Base\nwatercolour\n\nat night"
	if got := resolved[0].Request.Prompt; got != want {
		t.Errorf("Prompt:\ngot  %q\nwant %q", got, want)
	}
}

func TestResolve_EntrySeparatorOverridesCLI(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "cli_default", Index: 0, Raw: map[string]any{
			"prompt": []any{"a", "b"}, "provider": "openai",
		}},
		{Key: "entry_override", Index: 1, Raw: map[string]any{
			"prompt": []any{"a", "b"}, "separator": "---", "provider": "openai",
		}},
	}}
	opts := defaultCLI()
	opts.Separator = " | " // padded → inline, verbatim
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      opts,
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := resolved[0].Request.Prompt; got != "a | b" {
		t.Errorf("CLI separator: got %q, want %q", got, "a | b")
	}
	if got := resolved[1].Request.Prompt; got != "a\n---\nb" {
		t.Errorf("entry separator: got %q, want %q", got, "a\n---\nb")
	}
}

func TestResolve_EntryPromptEmptyListRejected(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": []any{}, "provider": "openai"}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "prompt is required") {
		t.Errorf("expected prompt-is-required error, got %v", err)
	}
}

func TestResolve_EntryPromptRejectsNonStringPart(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"prompt": []any{"a", 7}, "provider": "openai"}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "prompt:") {
		t.Errorf("expected prompt type error, got %v", err)
	}
}

// --- Collision detection ---------------------------------------------------

func TestResolve_RejectsFilenameCollision(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "a", Index: 0, Raw: map[string]any{
			"prompt": "x", "provider": "openai", "filename": "cover.png",
		}},
		{Key: "b", Index: 1, Raw: map[string]any{
			"prompt": "y", "provider": "openai", "filename": "cover.png",
		}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Errorf("expected collision error, got %v", err)
	}
}

// --- Provider cache --------------------------------------------------------

func TestResolve_CachesProviderAcrossEntries(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "a", Index: 0, Raw: map[string]any{"prompt": "x", "provider": "openai"}},
		{Key: "b", Index: 1, Raw: map[string]any{"prompt": "y", "provider": "openai"}},
		{Key: "c", Index: 2, Raw: map[string]any{"prompt": "z", "provider": "openai"}},
	}}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(resolved) != 3 {
		t.Fatalf("got %d resolved, want 3", len(resolved))
	}
	// Same Provider instance should be reused across entries that
	// share a provider name. Comparing pointer identity.
	if resolved[0].Provider != resolved[1].Provider {
		t.Error("entries 0 and 1 should share the same Provider instance (cache hit)")
	}
	if resolved[1].Provider != resolved[2].Provider {
		t.Error("entries 1 and 2 should share the same Provider instance (cache hit)")
	}
}

// --- Error cases -----------------------------------------------------------

func TestResolve_RejectsMissingPrompt(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{"provider": "openai"}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Errorf("expected prompt-required error, got %v", err)
	}
}

func TestResolve_RejectsUnknownProvider(t *testing.T) {
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{
			"prompt": "x", "provider": "bogus",
		}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected unknown-provider error, got %v", err)
	}
}

// --- input: [] explicitly clears a global -i default ----------------------------

func TestResolve_InputEmptyListClearsCLIDefault(t *testing.T) {
	cli := defaultCLI()
	cli.RefInputs = []string{"/tmp/global_ref.png"}

	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "hero", Index: 0, Raw: map[string]any{
			"prompt": "x",
			"input":  []any{},
		}},
	}}
	resolved, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      cli,
		Cmd:             stubCmd(t),
		Config:          stubConfig(),
		DefaultProvider: "openai",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("got %d resolved, want 1", len(resolved))
	}
	r := resolved[0]
	if len(r.Request.References) != 0 {
		t.Errorf("References: got %d, want 0 (input: [] should override global -i)", len(r.Request.References))
	}
	if r.Params.RefInputPath != "" {
		t.Errorf("RefInputPath: got %q, want empty (input: [] should override global -i)", r.Params.RefInputPath)
	}
}

// TestResolve_ProviderResolutionFailureIsMemoised verifies that a broken
// secret reference (here, a missing env var) fails fast for every entry that
// targets the same provider. The fix this guards: without memoisation, a
// 100-entry batch with a shared op:// ref to a locked vault would spawn the
// `op` CLI 100 times, each waiting out the 5s timeout. With memoisation,
// the first failure is cached and every subsequent entry returns it
// immediately. Observable proxy: every entry contributes the same env-var
// error to the aggregate report, and the run completes near-instantly.
func TestResolve_ProviderResolutionFailureIsMemoised(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {"api_key": "${BATCH_TEST_DEFINITELY_NOT_SET_XYZ}"},
		},
	}
	spec := &batch.Spec{Entries: []batch.Entry{
		{Key: "a", Index: 0, Raw: map[string]any{"prompt": "x", "provider": "openai"}},
		{Key: "b", Index: 1, Raw: map[string]any{"prompt": "y", "provider": "openai"}},
		{Key: "c", Index: 2, Raw: map[string]any{"prompt": "z", "provider": "openai"}},
	}}
	_, err := batch.Resolve(batch.ResolveContext{
		Spec:            spec,
		CLIOptions:      defaultCLI(),
		Cmd:             stubCmd(t),
		Config:          cfg,
		DefaultProvider: "openai",
	})
	if err == nil {
		t.Fatal("expected aggregate error, got nil")
	}
	got := strings.Count(err.Error(), "BATCH_TEST_DEFINITELY_NOT_SET_XYZ")
	if got != 3 {
		t.Errorf("want 3 entries to surface the memoised error, got %d in:\n%s", got, err.Error())
	}
}
