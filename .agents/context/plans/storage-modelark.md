# imagine-cli — Refactor & Modernization Plan

_Living document. Reference this throughout the migration; update as phases land._

---

## 0. Context

This repo is a fresh fork of `banana-cli`, renamed to `imagine-cli`. The original app is a Gemini-only image generator with both a Bubble Tea TUI and a CLI entrypoint. In daily use the TUI is overhead; the CLI is the product. We also want to add OpenAI's `gpt-image-2`, and the existing `UseVertex bool`-branch pattern will not scale to a third (or fourth) provider.

This plan takes the fork from "banana-cli source under a new name" to a lean, modular, multi-provider CLI named `imagine`. It lays the foundation (rename, TUI removal, modernization, consolidation) before any new provider arrives, so the OpenAI integration is a clean drop-in instead of surgery on a moving target.

### Guiding principles

- **The CLI is the product.** No TUI. Terminal interactivity stays minimal: CLI spinner, password-read on first-run key prompt.
- **Cobra + Fang for the command surface.** Subcommands declare their own flags; Fang styles the help output. No hand-rolled flag parsing or ownership tracker — cobra gives us that for free. (Reference: `/Users/ahmabora1/Dev/marina` uses the same stack.)
- **Providers are plug-ins.** Adding the Nth provider is one new directory + one `init()` that registers a factory + flag-binder with the provider registry. The generate command picks up its flags automatically.
- **Flags are declarative, not scattered.** Each provider owns the flags it claims. Capability gating (e.g. "grounding only on Gemini") lives in the provider's `PreRunE` hook, not in scattered `if model == "flash"` conditionals.
- **Config-first defaults.** Users set a default provider once. Per-command `--provider` flags are only needed when overriding.
- **No duplicated functions.** Shared code (file I/O, image utils, parallelism, filename resolution) lives exactly once in `internal/`. Provider code does provider-specific things only.
- **Describe is out of scope.** The `describe` subcommand keeps its current Gemini/Vertex behavior untouched. Making it provider-aware is a follow-up.

---

## 1. Phase roadmap

| # | Phase | Goal | Status |
|---|---|---|---|
| 1 | Rename & demolish TUI | Binary is `imagine`, Bubble Tea is gone, single CLI path | **✅ done** |
| 2 | Modernization — Go 1.26 + Cobra+Fang | Toolchain on 1.26, `flag` pkg replaced with Cobra, Fang styles help output, ctx cancellation | **✅ done** |
| 3 | Consolidation (DRY) | Shared utilities live once; orchestrator provider-agnostic | **✅ done** |
| 4 | Provider system + config default | Each provider self-registers; `--provider` flag; `default_provider` config; capability gating | **✅ done** |
| 4.1 | YAML config with provider namespacing | `config.yaml` with `providers:` map instead of flat top-level keys | **✅ done** |
| 4.2 | Fully provider-private flags | `-m`, `-s`, `-a`, `-g`, `-t`, `-I` move from common flags into each provider's `BindFlags`/`ReadFlags`. `cli.Options` shrinks; `Request` carries everything in `Options` map. | **✅ done** |
| 5 | OpenAI provider | `gpt-image-2` available end-to-end | **✅ done** |

Each phase is one PR, one atomic commit or small stack. Between phases, the app is always runnable.

---

## 2. Phase 1 — Rename to `imagine` + TUI removal **✅ DONE**

### What landed

- `cmd/banana/` → `cmd/imagine/` (via `git mv`).
- Module path: `github.com/AhmedAburady/banana-cli` → `github.com/AhmedAburady/imagine-cli` (bulk `sed` across `.go`/`.mod`/`.sum`/`.yml`).
- `ui/`, `views/`, `screenshots/`, empty `ghs/`, stale `banana` binary — deleted.
- `cmd/imagine/main.go` — trimmed to 44 lines (subcommand dispatch → flag parse → `cli.Run`). No TUI fallback; bare `imagine` prints help.
- `go.mod`: dropped `charmbracelet/{bubbletea,bubbles,lipgloss}`. Kept `briandowns/spinner`.
- `config/config.go`: default dir → `~/.config/imagine`. **No migration** (per user decision — treat as new app).
- Text rename (sed `banana → imagine`, `BANANA → IMAGINE`) applied to: `cli/cli.go`, `describe/*.go`, `api/vertex.go`, `.github/workflows/release.yml`, `README.md`, `release-notes.md`.
- `.gitignore`: added `/imagine` (kept `/banana` for stale builds).
- Help text still says "Gemini AI Image Generator" — **intentionally unpolished**; Phase 2 replaces the whole help subsystem with Fang.

### What Phase 1 did NOT do (intentional, handled elsewhere)

- `-vertex` flag still exists — removed in Phase 4 alongside `--provider` arrival.
- Go version in `go.mod` untouched — Phase 2 bumps.
- Help text not provider-aware — Phase 4 via cobra.

### Verify

```bash
go build -o imagine ./cmd/imagine
./imagine -p "a sunset" -n 1 -o /tmp/smoke
./imagine --help               # no TUI references; imagine everywhere
./imagine config show
ls ui/ views/                  # should not exist
grep -r bubbletea .            # should be empty
```

On a machine with `~/.config/banana/config.json`, first run should produce `~/.config/imagine/config.json` with the same key and log the migration.

---

## 3. Phase 2 — Modernization: Go 1.26 + Cobra+Fang **✅ DONE**

Two changes that reinforce each other: bump to Go 1.26 (toolchain + modernizers), and replace the stdlib `flag` package with **Cobra + Fang** for the CLI surface. Gave us the Fang-styled help output, free signal-based context cancellation, and a declarative command tree that Phase 4's provider system slotted into naturally.

### Reference implementation

`/Users/ahmabora1/Dev/marina` uses the same stack. Its entrypoint (`cmd/marina/main.go`) is 32 lines; subcommand files under `commands/` each build a `*cobra.Command`. That's the shape we're adopting.

### Deliverables — Go 1.26 half

- `go.mod`: `go 1.26.0` + `toolchain go1.26.0`. Release workflow `setup-go` bumps from `1.23` to `1.26`.
- `go fix ./...` applied (the revamped Go 1.26 `go fix` is now a modernizer that suggests idiomatic updates).
- Free wins absorbed (no code changes required):
  - **Faster `io.ReadAll`** — HTTP response bodies and image files read faster.
  - **`image/jpeg` reimplementation** — `convertToJPEG` becomes faster / more accurate.
  - **`fmt.Errorf("x")` lower alloc** — no change.
- Adopted explicitly:
  - **`errors.AsType[T]`** — replaces `errors.As` where type-safe and cleaner (e.g. Gemini `*GeminiError` unwrap paths).
  - **`new(expression)`** — cleaner optional-pointer field population (review spots like `*ThinkingConfig`).

### Deliverables — Cobra+Fang half

- New deps (exact versions pulled from marina's `go.mod`):
  - `github.com/spf13/cobra` v1.10.x
  - `charm.land/fang/v2` v2.0.x
- `cmd/imagine/main.go` becomes ~15 lines:
  ```go
  package main

  import (
      "context"
      "log/slog"
      "os"
      "syscall"

      "charm.land/fang/v2"
      "github.com/AhmedAburady/imagine-cli/commands"
  )

  var version = "dev"

  func main() {
      slog.SetDefault(slog.New(slog.DiscardHandler))
      root := commands.NewRootCmd(version)
      if err := fang.Execute(context.Background(), root,
          fang.WithVersion(version),
          fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
      ); err != nil {
          os.Exit(1)
      }
  }
  ```
- New package `commands/` (following marina's layout):
  - `commands/root.go` — `NewRootCmd(version) *cobra.Command`. Root has a `RunE` that runs generation — `imagine -p "..."` generates directly, no `generate` subcommand. All generate/edit flags live on the root command. In Phase 4 the root's `RunE` becomes the provider dispatcher.
  - `commands/config.go` — `imagine config {show,set-key,set-project,set-location,path}` subcommands. Replaces `cli.HandleConfigCommand`.
  - `commands/describe.go` — thin wrapper that keeps `describe.HandleDescribeCommand` behavior intact (describe stays out-of-scope; we just put it behind cobra so `imagine --help` shows it).
  - `commands/version.go` — explicit `imagine version` subcommand (fang already handles `-v`/`--version` on the root; this is a parity subcommand like marina has).

**Why no `generate` subcommand**: the overwhelmingly common invocation is `imagine -p "..."`. Forcing `imagine generate -p "..."` adds typing to every run for no gain. Config/describe/version stay as subcommands because they're the rare paths. This mirrors marina (`marina` bare → TUI; `marina config/hosts/etc.` → subcommands).

**Generate vs edit — no subcommand either.** The root command handles both: `-i <ref>` flips it to edit mode (same as today's `cli/cli.go:101` "enables edit mode" wording). No `imagine edit` subcommand — one path, one flag, intent inferred:

```
imagine -p "a cat"                          # generate
imagine -p "make it cartoon" -i photo.png   # edit (single ref)
imagine -p "merge these" -i a.png -i b.png  # edit (multi-ref, repeatable -i)
imagine -p "add rain" -i ./refs/            # edit (folder of refs)
```
- `cli/` package shrinks: `PrintHelp`, `PrintVersion`, `HandleConfigCommand` deleted (replaced by cobra/fang). `PromptForAPIKey`, `Run`, `Validate`, `Options` retained — `Run` now takes the cobra-parsed options struct.
- **Flag syntax changes** (one breaking change users will notice):
  - Old single-dash long flags → double-dash. `-vertex` → `--vertex`, `-help` → `--help`, `-version` → `--version`. (Short forms stay: `-p`, `-o`, `-n`, `-m`, `-t`, `-s`, `-f`, `-i`, `-r`, `-g`.)
  - `-is` (two-letter short) → `--image-search` long. Cobra doesn't do multi-letter short flags idiomatically.
  - `-ar` (two-letter short) → `--aspect-ratio` long with no short (or `-a` if free).

### Context propagation

- `fang.WithNotifySignal` gives us a context that cancels on SIGINT/SIGTERM. It threads through `cobra.Command.Context()`.
- `api.RunGeneration(ctx, cfg)` — add ctx parameter.
- `api.GenerateImage` / `GenerateImageVertex` — switch `http.Client.Do(req)` to `http.NewRequestWithContext(ctx, ...)` so Ctrl+C actually kills in-flight requests.

### Steps

1. **Toolchain bump**: edit `go.mod` (`go 1.26.0` + `toolchain go1.26.0`); edit `.github/workflows/release.yml` (`go-version: '1.26'`). `go mod tidy`.
2. **Add deps**: `go get github.com/spf13/cobra@latest charm.land/fang/v2@latest`.
3. **Create `commands/` package** with `root.go`, `generate.go`, `config.go`, `describe.go`, `version.go`. Generate.go wraps `cli.Run`; config.go reproduces current config subcommand behavior with cobra flags; describe.go shells out to `describe.HandleDescribeCommand`.
4. **Rewrite `cmd/imagine/main.go`** as the 15-line fang launcher above.
5. **Delete from `cli/cli.go`**: `PrintHelp`, `PrintVersion`, `HandleConfigCommand`, `ParseFlags`, the flag definitions. Keep `Options`, `Validate`, `Run`, `PromptForAPIKey`.
6. **Thread ctx**: `api.RunGeneration(ctx, ...)` → provider HTTP calls via `http.NewRequestWithContext`.
7. **Apply modernizers**: `go fix ./...`, review diff, commit the modernizations separately (so the Cobra/Fang swap and the stdlib cleanups are bisectable).
8. **Update help examples** in README + release-notes.md to reflect new flag spellings.

### Verify

```bash
go version                                    # go1.26.x
go fix -diff ./...                            # empty after step 7
go build -o imagine ./cmd/imagine
./imagine --help                              # Fang-styled USAGE/COMMANDS/FLAGS boxes
./imagine config --help                       # config subcommand help
./imagine -p "a sunset" -n 1 -o /tmp/smoke    # generation on root command
./imagine config show
./imagine version
./imagine -p "a long prompt" -n 5 & sleep 2 && kill -INT %1   # Ctrl+C cancels in-flight HTTP
```

### What actually landed vs. plan

- Toolchain: `go.mod` now says `go 1.26.0`; installed Go is 1.26.2; workflow uses `1.26`.
- `go fix ./...` returned no diffs. Free-wins from Go 1.26 stdlib improvements are in by virtue of the toolchain bump.
- Cobra v1.10.2 and `charm.land/fang/v2` v2.0.1 added.
- `cmd/imagine/main.go` is 29 lines (~15-line target + blank-imports for providers in Phase 4).
- `commands/` package built: `root.go` (generation on root RunE), `config.go`, `describe.go`, `version.go`.
- `cli/cli.go` shrank from ~520 → 274 lines; `ParseFlags`/`PrintHelp`/`PrintVersion`/`HandleConfigCommand`/`GetVersion`/`stringSlice` all deleted.
- `ctx context.Context` threaded through `api.RunGeneration` + `GenerateImage` + `GenerateImageVertex`; Gemini HTTP uses `http.NewRequestWithContext`.
- Flag renames shipped: `-vertex` → `--vertex`, `-is` → `--image-search`, `-ar` → `--aspect-ratio`, `-help` → `--help`, `-version` → `--version`. Short flags preserved: `-p -o -f -n -s -g -i -r -m -t`.
- Fang added `imagine completion` and hidden `imagine man` commands for free.

### Non-goals (kept as planned)

- Provider abstraction deferred to Phase 4.
- Huh (interactive forms) skipped — single `PromptForAPIKey` uses `term.ReadPassword`.
- `--vertex` kept through Phase 2 (removed in Phase 4).

---

## 4. Phase 3 — Consolidation (DRY pass) **✅ DONE**

Extracted shared utilities out of `api/` into `internal/` so every provider can reuse them. Introduced `images.Reference` as the provider-neutral handle for reference images — each provider encodes raw bytes as its API demands.

### What landed

- **`internal/paths/paths.go`** (25 lines) — `ExpandTilde`.
- **`internal/images/images.go`** (154 lines) — `Reference{MimeType, Data}`, `IsSupported`, `MimeType`, `Load`, `CountInDir` + parallel dir loader.
- **`internal/images/jpeg.go`** (23 lines) — `ConvertToJPEG` at quality 95.
- **`internal/images/naming.go`** (60 lines) — `ResolveFilename(FilenameParams) string` (single source of truth for `-f` / `-r` / default precedence).
- **`api/orchestrator.go`** (new) — `RunGeneration` is now provider-agnostic; Phase 4 swapped its bool-branch for a `Provider` interface.
- **`api/gemini.go`** shrank from 575 → 248 lines (eventually deleted in Phase 4).
- **`api/vertex.go`** now takes raw bytes from `Reference` directly — dropped the base64 encode/decode round-trip.
- **`cli/prompt.go`** (44 lines, new) — `PromptForAPIKey` split out.
- Call sites in `cli/cli.go` and `describe/describe.go` all swapped to use `paths.*` / `images.*`.

### Explicit scope cut

Describe keeps its own inline loaders (`loadImagesFromDir`, `loadSingleImage`) because it builds `*genai.Part` directly. Deduping those requires touching describe behaviour; deferred to a follow-up.

---

## 4.archive. Phase 3 — original consolidation inventory (reference)

_Kept for traceability. The moves below all landed._

### What currently sits in the wrong place

Inventory of today's structure (audited against `api/gemini.go`, `api/vertex.go`, `cli/cli.go`):

| Function/logic | Currently in | Should be in |
|---|---|---|
| `ExpandTilde` | `api/gemini.go:157` | `internal/paths/paths.go` |
| `IsSupportedImage`, `GetImageMimeType`, `supportedExts` | `api/gemini.go:149-184` | `internal/images/images.go` |
| `LoadReferences`, `loadImagesFromDir`, `loadSingleImage`, `FindImagesInDir` | `api/gemini.go:186-332` | `internal/images/images.go` |
| `convertToJPEG` | `api/gemini.go:342` | `internal/images/jpeg.go` |
| `RunGeneration` (goroutine fan-out, file save, filename resolution) | `api/gemini.go:354-456` | `api/orchestrator.go` (provider-agnostic) |
| `GenerateImage` (Gemini HTTP call) | `api/gemini.go:458-574` | `providers/gemini/gemini.go` (Phase 4) |
| `GenerateImageVertex` | `api/vertex.go` | `providers/vertex/vertex.go` (Phase 4) |
| `HandleConfigCommand` | `cli/cli.go:401-484` | `cli/config_cmd.go` |
| `PromptForAPIKey` | `cli/cli.go:487-518` | `cli/prompt.go` |
| Flag parsing | `cli/cli.go:91-123` | `cli/flags.go` (Phase 4 reshapes this heavily) |
| Validation | `cli/cli.go:131-221` | `cli/validate.go` |

### Target structure after Phase 3

```
cmd/imagine/main.go            # thin dispatcher (~40 lines)
cli/
  cli.go                       # Run() orchestration only
  flags.go                     # flag definitions (Phase 4 makes this declarative)
  validate.go                  # validation
  config_cmd.go                # `imagine config` subcommand
  prompt.go                    # first-run key prompt
internal/
  images/images.go             # MIME detection, reference loaders, glob expansion
  images/jpeg.go               # convertToJPEG
  paths/paths.go               # ExpandTilde and friends
api/
  orchestrator.go              # RunGeneration: parallelism, file save, filename resolution
  types.go                     # GenerationResult, GenerationOutput (common)
  # gemini.go stays as-is in Phase 3; Phase 4 moves it.
config/config.go               # unchanged structurally; just the migration from Phase 1
describe/                      # untouched (out of scope)
```

### Filename-resolution rule (extract from orchestrator)

The logic at `api/gemini.go:387-432` decides filenames via this precedence:
1. `-f` custom filename, with `_N` suffix when `n>1`, extension-aware
2. `-r` preserve input filename (single file only), with `_N` suffix when `n>1`
3. Default: `generated_{N}_{YYYYMMDD_HHMMSS}.png`

Plus: `.jpg`/`.jpeg` extension triggers `convertToJPEG` at quality 95.

Move this to `internal/images/naming.go` as `ResolveFilename(cfg, index int) string` and `MaybeConvert(data []byte, filename string) ([]byte, error)`. Keep the precedence and the `-f`/`-r` mutual-exclusion contract identical.

### Verify

- `go build ./...` green.
- All existing CLI commands (Gemini direct + Vertex) still produce identical outputs to a pre-Phase-3 baseline (manual smoke: same prompt, same seed-free comparison just checks files are produced in the right place with the right names).
- `grep -rn "func ExpandTilde\|func IsSupportedImage\|func LoadReferences"` shows exactly one definition per function.

---

## 5. Phase 4 — Provider system + config default **✅ DONE**

The core architectural phase. Replaced the `UseVertex` bool-branch with a `Provider` interface + registry. Adding a new provider is now one new directory under `providers/` plus one blank-import line in `cmd/imagine/main.go`.

### What landed

**New `providers/` package** — the extension point:
- `providers/provider.go` — `Info`, `ModelInfo`, `Capabilities`, `Request`, `Response`, `GeneratedImage`, `Provider` interface.
- `providers/auth.go` — `Auth{GeminiAPIKey, OpenAIAPIKey, GCPProject, GCPLocation}`.
- `providers/registry.go` — `Bundle{Factory, BindFlags, SupportedFlags, ReadFlags, Info}` + `Register`/`Get`/`List`/`ProvidersSupportingFlag`.

**Gemini moved into `providers/gemini/`** (fully self-contained):
- `gemini.go` — `Provider.Generate`, wire types, `httpClient`, `ModelPro`/`ModelFlash` constants.
- `flags.go` — idempotent `BindFlags` (shared with Vertex), `ReadFlags`, `OwnedFlags` for `--grounding`/`--thinking`/`--image-search`.
- `register.go` — `init()` self-registers with the registry.

**Vertex moved into `providers/vertex/`**:
- `vertex.go` — `Provider.Generate` using `google.golang.org/genai`; `Info` mirrors Gemini's models but `Capabilities.ImageSearch = false`.
- `register.go` — `init()` registers; shares Gemini's `BindFlags`/`ReadFlags`; `SupportedFlags = ["grounding", "thinking"]` (no image-search).

**Orchestrator rewritten**:
- `api/orchestrator.go` now takes `(ctx, Provider, Request, Params)`. Loops through batches respecting `Info().Capabilities.MaxBatchN` (Gemini/Vertex: 1 = one HTTP call per image; OpenAI in Phase 5: 10 = batched).
- `api/gemini.go` and `api/vertex.go` deleted — their code lives in `providers/`.
- `api.Config` dissolved into `providers.Request` + `api.Params`.

**Root command PreRunE enforces the contract**:
- Resolves active provider: `--provider` flag → `IMAGINE_PROVIDER` env → `config.default_provider` → `"gemini"`.
- Rejects provider-private flags the user set but the active provider doesn't `SupportedFlags`-include → `"--image-search is not supported by provider 'vertex' (supported by: [gemini])"`.
- Resolves `-m` aliases (`pro`/`flash`) to canonical model IDs via active provider's `Info.Models`.
- Falls back to `Info.DefaultModel` when `-m` is empty.

**Config schema** (breaking — no auto-migration, treated as fresh app):
- `api_key` → `gemini_api_key`
- `openai_api_key` added (empty default; Phase 5 populates).
- `default_provider` added (env `IMAGINE_PROVIDER` overrides).

**Config subcommands** (cobra tree):
- `set-key`, `set-openai-key`, `set-project`, `set-location`, `set-default-provider`, `show`, `path`.
- `set-default-provider` validates the name exists in the registry before saving.
- `show` prints all five fields, masking keys.

**`--vertex` flag removed**. Use `--provider vertex` instead.

### Smoke evidence

```
imagine --provider nope              → unknown provider error lists available
imagine -m bogus                     → unknown model error lists aliases + IDs
imagine --provider vertex --image-search → capability-gating error with providers-list
imagine config set-default-provider bogus → rejected before save
imagine config show                  → all five fields rendered
imagine                              → help (not an error)
```

---

## 5.archive. Phase 4 — original design notes (reference)

_Kept for traceability. All of the below landed as documented above, with one small deviation: `SupportedFlags` replaced the planned `OwnedFlags` because Gemini and Vertex share the same flags (grounding, thinking) so "exclusive owner" was the wrong abstraction — it's "does this provider support this flag" not "who owns it"._

### Command shape

Generation lives on the root command (not a `generate` subcommand — see Phase 2 rationale). The invocation is:

```
imagine -p "..."                          # uses default provider from config
imagine -p "..." --provider openai        # override via flag
imagine -p "..." --provider openai -q high
```

`--provider` is a persistent root-command flag. Provider-specific flags (like `-q`/`--quality` for OpenAI) also attach to the root command — ownership enforcement in `PreRunE` rejects flags that don't belong to the active provider.

### Provider package shape

```go
// providers/provider.go
type Provider interface {
    Info() Info
    Generate(ctx context.Context, req Request) (*Response, error)
}

type Info struct {
    Name         string               // "gemini", "vertex", "openai"
    DisplayName  string               // "Google Gemini"
    Summary      string               // one-line description for `--help`
    Models       []ModelInfo
    DefaultModel string
    Capabilities Capabilities
}

type ModelInfo struct {
    ID              string             // "gemini-3-pro-image-preview"
    Aliases         []string           // ["pro"]
    Description     string
    SupportedFlags  []string           // e.g. flash: ["thinking", "image-search"]
}

type Capabilities struct {
    Edit          bool
    Masking       bool
    Grounding     bool
    Thinking      bool
    MaxBatchN     int
    Sizes         []string
    MaxReferences int
}

type Request struct {
    Prompt      string
    N           int
    Model       string
    Size        string
    AspectRatio string
    References  []RefImage
    // Provider-specific parsed options (cobra already parsed them)
    Options     map[string]any
}
```

### How Cobra integrates

1. **Registry** in `providers/registry.go`:
   ```go
   type ProviderBundle struct {
       Provider   func(Auth) (Provider, error)  // factory
       BindFlags  func(cmd *cobra.Command)      // attach provider-specific flags
       ReadFlags  func(cmd *cobra.Command) map[string]any  // collect parsed values
   }

   var registry = map[string]ProviderBundle{}
   func Register(name string, b ProviderBundle)
   func Get(name string) (ProviderBundle, bool)
   func List() []string
   ```
2. **Self-registration** via `init()` in each provider package:
   ```go
   // providers/gemini/gemini.go
   func init() {
       providers.Register("gemini", providers.ProviderBundle{
           Provider:  New,
           BindFlags: bindFlags,
           ReadFlags: readFlags,
       })
   }
   ```
3. **Root command** (which Phase 2 already built as the generate entry point) gets extended:
   - Common flags (`-p`, `-o`, `-n`, `-i`, `-f`, `-r`, `-m`, `-s`, `--aspect-ratio`) stay on the root cmd.
   - `--provider` persistent flag resolves active provider (flag → `IMAGINE_PROVIDER` env → `config.DefaultProvider` → `"gemini"`).
   - `PreRunE`: calls `activeProvider.BindFlags(cmd)` dynamically? **No — cobra requires flags declared before `Execute`.** Instead: every registered provider's `BindFlags` runs at command construction time, attaching all provider-specific flags to the root cmd. Ownership enforcement happens in `PreRunE`: iterate visited flags (`cmd.Flags().Changed(name)`); for each, look up owner in a map built during `BindFlags`; if owner != active provider, error.
4. **Help output** — fang formats cobra's native help. Cobra doesn't natively say "these flags only apply when --provider=X", so we either (a) add a post-help printer hook that groups flags by owning provider in the `FLAGS` section, or (b) add an `imagine providers` subcommand that lists each provider with its flags. Defer the specific choice to implementation time — whichever is cleaner once we see fang's rendering in practice.

### Flow summary

```
Cobra parses common + all provider flags
  ↓
PreRunE resolves active provider
  ↓
PreRunE rejects any set flag whose owner != active provider
  ↓
PreRunE validates -m against active provider's Models
  ↓
PreRunE validates capability-gated flags (e.g. -g + !Grounding → error)
  ↓
RunE: build Request (Options = provider.ReadFlags(cmd))
  ↓
orchestrator.RunGeneration(ctx, req, provider)
```

### Default provider in config

Add to `config/config.go:Config`:

```go
type Config struct {
    GeminiAPIKey    string `json:"gemini_api_key"`
    OpenAIAPIKey    string `json:"openai_api_key"`
    GCPProject      string `json:"gcp_project,omitempty"`
    GCPLocation     string `json:"gcp_location,omitempty"`
    DefaultProvider string `json:"default_provider,omitempty"` // "gemini" | "vertex" | "openai"
}
```

New subcommand: `imagine config set-default-provider <name>`. `imagine config show` displays the default-provider line.

Precedence for provider selection per invocation: `--provider` flag → `IMAGINE_PROVIDER` env → `config.DefaultProvider` → built-in default `"gemini"`.

### Config schema change

Fresh app (no migration from banana-cli per the earlier decision). Phase 4 reshapes the config file:
- `api_key` → `gemini_api_key` (clean rename, no in-place migration).
- New field `openai_api_key` (empty default).
- New field `default_provider` (empty = built-in default).

Users with an existing `~/.config/imagine/config.json` from earlier phases will need to re-run `imagine config set-key <K>`. Since we explicitly treated this as a new app in Phase 1, this is consistent.

### Gemini/Vertex migration into the new system

- `api/gemini.go:GenerateImage` moves to `providers/gemini/gemini.go:(*Gemini).Generate`.
- `api/vertex.go:GenerateImageVertex` moves to `providers/vertex/vertex.go:(*Vertex).Generate`.
- Each provides its `Info()` with:
  - **Gemini**: Models `pro`/`flash` + full IDs; Capabilities `{Edit, Grounding, Thinking, MaxBatchN=1}`.
  - **Vertex**: same as Gemini but `Grounding: false` (Vertex path doesn't send the search tool today — confirm during migration).
- The "Flash-only" rule for thinking/image-search gets expressed via `ModelInfo.SupportedFlags []string` (per-model capability) rather than scattered if-statements.

### The `--vertex` flag

Removed outright. `--provider vertex` is the replacement. No back-compat alias — fork is new, no users depend on it.

### Verify

```bash
# provider ownership enforced
./imagine -p "x" --provider gemini --quality high
# Error: --quality is not valid for provider 'gemini' (used by: openai)

# default provider from config
./imagine config set-default-provider openai
./imagine -p "x"                              # no --provider flag; uses openai from config
IMAGINE_PROVIDER=gemini ./imagine -p "x"      # env overrides config
./imagine -p "x" --provider vertex            # flag overrides env

# model alias resolution
./imagine --provider gemini -m pro -p "x"     # resolves to gemini-3-pro-image-preview

# capability gating
./imagine --provider openai -g -p "x"
# Error: grounding (-g) is not supported by provider 'openai'

# help — Fang-styled, grouped by provider
./imagine --help                              # common flags + provider-scoped sections

# sanity: existing flows still work with new config schema
./imagine config set-key <GEMINI_KEY>         # writes gemini_api_key
./imagine config show                         # shows "Gemini API Key: xxx..."
```

---

## 6. Phase 5 — OpenAI provider **✅ DONE**

### What landed

- **`providers/openai/`** — full implementation, ~550 lines split across `openai.go`, `flags.go`, `register.go`.
- **Generate endpoint** (`POST /v1/images/generations`, JSON) with `model`, `prompt`, `n`, `size`, `quality`, `output_format`, `output_compression`, `moderation`, `background`.
- **Edit endpoint** (`POST /v1/images/edits`, multipart) with per-reference image parts. Edit-mode size gate rejects sizes outside `{1024x1024, 1536x1024, 1024x1536, auto}` before the API call.
- **Model default**: `gpt-image-2` (confirmed working live — flipped from the plan's `gpt-image-1.5` default once I confirmed the API accepts it). Aliases: `2`, `1.5`, `1`, `mini`, `1-mini`, `latest`.
- **Size handling**: `1K` → `1024x1024`, `2K` → `2048x2048`, `4K` → `3840x2160`. Raw `WxH` passes through. Empty → `auto`.
- **`output_format` auto-inferred from `-f` extension** — `-f cat.jpg` sends `output_format=jpeg` to the API, which returns JPEG bytes directly (no local re-encoding).
- **`--background transparent`** rejects: (a) when output format is JPEG (via `-f *.jpg`), (b) when active model is `gpt-image-2` (docs say unsupported).
- **180s HTTP client timeout** — longer than Gemini's 120s to accommodate OpenAI's "complex prompts may take up to 2 minutes" note.
- **Orchestrator batching** exercised for the first time — `MaxBatchN=10` means `-n 3` uses ONE API call returning 3 images (verified live: 3 images in 32.9s).

### Smoke evidence (real API calls)

```
imagine -p "a red apple" --provider openai -s 1024x1024 -q low
  → 1 image, 24s, valid PNG 1024x1024

imagine -p "a logo" --provider openai -n 3 -q low
  → 3 images in single API call, 33s

imagine -p "add a green leaf" --provider openai -i apple.png
  → edit endpoint via multipart, 62s

imagine -p "sketch" --provider openai -f test.jpg
  → -f's .jpg extension drove output_format=jpeg; API returned JPEG directly

imagine -p x --provider openai -m bogus
  → "unknown model 'bogus' ... (accepted: gpt-image-2 2 gpt-image-1.5 1.5 ...)"

imagine -p x --provider openai --background transparent -f x.jpg
  → "--background transparent requires PNG or WebP output"

imagine -p x --provider openai -g
  → "--grounding is not supported by provider 'openai' (supported by: [gemini vertex])"
```

### Design deviations from plan

- **Shared flags gone**: Phase 4.2 moved `-m`, `-s`, `-a` into each provider. OpenAI's `-m` (with its own alias list) and `-s` (with its own shorthand mapping) coexist with Gemini's via idempotent BindFlags.
- **No explicit `--output-format` flag**: inferred from `-f` extension instead (user preference). Keeps CLI surface lean.
- **Multi-turn / streaming / partial_images / `--input-fidelity`** — explicitly out of scope.
- **`dall-e-2`/`dall-e-3`** — not exposed. Could be added later if wanted.

Add `providers/openai/` implementing the Spec + Generate interface. This phase proves the Phase 4 abstraction: adding a provider should touch no code outside its own directory (except the one `_ "..."` import line).

### Source-of-truth docs

All in `context/gpt-image/`:
- `new-image-model.md` — announces `gpt-image-2` (2026-04-21).
- `image-generation.md` — overview, size rules, quality, streaming, multi-turn.
- `create-image.md` — `POST /v1/images/generations` full request/response.
- `edit-image.md` — `POST /v1/images/edits` (multipart), narrower size set.
- `image-variation.md` — variations endpoint (dall-e-2 only; not using).
- `gpt-image.md` — vision/analyze (not for generation path).

### Known ambiguity: `gpt-image-2` availability

The docs in `context/gpt-image/` disagree internally:
- `image-generation.md` uses `gpt-image-2` in curl examples for generate & edit.
- `create-image.md:35-45` typed `ImageModel` enum lists only `gpt-image-1.5/1/1-mini`.
- `edit-image.md:57-73` model param lists same reduced set.
- Model announcement is yesterday (2026-04-21); API reference likely lags the guide.

**Decision**: ship default model as `gpt-image-1.5` (known-good), accept `gpt-image-2` as an opt-in via `-m gpt-image-2`. First real API call during implementation tells us the truth; if `gpt-image-2` returns 200, flip the default in a follow-up commit.

### Provider spec

```go
Spec{
    Name:         "openai",
    DisplayName:  "OpenAI",
    DefaultModel: "gpt-image-1.5",  // flip to gpt-image-2 once verified
    Models: []ModelInfo{
        {ID: "gpt-image-2",     Description: "Flagship; verify availability"},
        {ID: "gpt-image-1.5",   Description: "Default; stable"},
        {ID: "gpt-image-1",     Description: "Previous gen"},
        {ID: "gpt-image-1-mini",Description: "Fastest; cheapest"},
    },
    Capabilities: Capabilities{
        Edit:        true,
        Masking:     true,
        Grounding:   false,
        ImageSearch: false,
        Thinking:    false,
        MaxBatchN:   10,        // per /images/generations docs, n up to 10
    },
    Flags: []FlagSpec{
        {Name: "quality", Short: "q", Default: "auto",
         AllowedValues: []string{"low", "medium", "high", "auto"}},
        {Name: "output-format", Default: "png",
         AllowedValues: []string{"png", "jpeg", "webp"}},
        {Name: "compression", Kind: Int, Default: "100",
         Help: "0-100, for jpeg/webp only"},
        {Name: "moderation", Default: "auto",
         AllowedValues: []string{"auto", "low"}},
        {Name: "background", Default: "auto",
         AllowedValues: []string{"auto", "opaque"}},  // transparent unsupported on gpt-image-2
    },
}
```

### HTTP details

- **Auth**: `Authorization: Bearer $OPENAI_API_KEY`. Load from `OPENAI_API_KEY` env → `config.OpenAIAPIKey`.
- **Generate** (`POST /v1/images/generations`): JSON body with `{model, prompt, n, size, quality, output_format, ...}`. Response `{data: [{b64_json}]}`.
- **Edit** (`POST /v1/images/edits`): **multipart/form-data**, `image[]=@file.png` repeated (up to 16), `mask=@mask.png` optional, rest of fields as form parts. This is a different wire format from generate — isolate in a separate function.
- **Timeout**: docs warn "complex prompts may take up to 2 minutes." Use a 180s client timeout for OpenAI specifically (Gemini's 120s stays as-is). The orchestrator passes a ctx with the longer deadline when active provider is OpenAI, or the provider's `Client()` uses its own `http.Client`.

### Size handling

- **Generate** accepts: `1024x1024`, `1536x1024`, `1024x1536`, `2048x2048`, `2048x1152`, `3840x2160`, `2160x3840`, `auto`. Other sizes allowed if edge≤3840, both multiples of 16, total pixels 655,360-8,294,400, aspect ratio ≤3:1.
- **Edit** accepts only: `1024x1024`, `1536x1024`, `1024x1536`, `auto`.
- **Shorthand** (`1K`, `2K`, `4K`) maps to `1024x1024` / `2048x2048` / `3840x2160` respectively. Inside the openai provider only — other providers define their own shorthand if they want.
- In edit mode, the provider validates against the narrow set; reject `2K`/`4K` with a clear error.

### Parallelism

OpenAI supports `n` natively, so `MaxBatchN=10`. The orchestrator (Phase 3) already respects this: for `-n 5`, it's one API call, not five goroutines. For `-n 25`, orchestrator splits into 3 batches (10+10+5) and parallelizes those three calls.

### Reference-image loading for edits

The orchestrator already loads references into `[]RefImage{MimeType, Data}` (raw bytes). OpenAI's provider base64-encodes on the generate-with-references path (adding `image_url: "data:image/png;base64,..."`) or streams them as multipart fields on the edit endpoint. Either way, the provider handles the encoding; the orchestrator hands over raw bytes.

### New config & CLI surface

- `imagine config set-openai-key <KEY>` subcommand.
- `imagine config show` adds OpenAI API key line (masked).
- `--provider openai` enables the provider. With `config set-default-provider openai`, no per-command flag needed.
- Gemini-exclusive flags (`-g`, `-is`, `-t`, `-ar`) are rejected with a clear message when provider is OpenAI.

### Verify

```bash
./imagine config set-openai-key sk-...
./imagine config set-default-provider openai

./imagine -p "a tabby cat hugging an otter" -n 2 -o /tmp/oai
# → /tmp/oai/generated_1_*.png, /tmp/oai/generated_2_*.png

./imagine -p "make it winter" -i /tmp/oai/generated_1_*.png -o /tmp/oai-edit
# → uses /v1/images/edits multipart

./imagine -p "a cat" -s 2048x2048 -q high -n 1 -o /tmp/oai-hd
./imagine -p "a cat" -s 4K -n 1 -o /tmp/oai-4k
./imagine -p "a cat" --background transparent -n 1 -o /tmp/fail
# → error: transparent not supported by gpt-image-2 (if that's the active model)

./imagine -g -p "a cat"
# → error: grounding not supported by provider 'openai'

./imagine --provider gemini -p "a cat" -o /tmp/gem    # Gemini still works unchanged
```

---

## 7. Appendix — per-file change inventory

### Files deleted (Phase 1)

- `ui/*.go` (entire dir, 7 files, ~1,485 lines)
- `views/*.go` (entire dir, 3 files, ~339 lines)
- `screenshots/` (directory)
- TUI chunks of `cmd/banana/main.go` (lines 21-457)

### Files moved (all ✅)

| From | To | Phase |
|---|---|---|
| `cmd/banana/main.go` | `cmd/imagine/main.go` | 1 |
| `cli/cli.go` (flag parsing, help, version, config subcommand) | `commands/root.go` (generate on root), `commands/config.go`, `commands/version.go` | 2 |
| `cli/cli.go` (describe subcommand entry) | `commands/describe.go` (thin wrapper around `describe.HandleDescribeCommand`) | 2 |
| `cli/cli.go` (prompt helper) | `cli/prompt.go` | 3 |
| `api/gemini.go` (image/path utils) | `internal/images/images.go`, `internal/paths/paths.go` | 3 |
| `api/gemini.go` (filename resolution) | `internal/images/naming.go` | 3 |
| `api/gemini.go` (JPEG converter) | `internal/images/jpeg.go` | 3 |
| `api/gemini.go` (orchestrator) | `api/orchestrator.go` | 3 |
| `api/gemini.go` (Gemini HTTP + wire types) | `providers/gemini/gemini.go` | 4 |
| `api/vertex.go` | `providers/vertex/vertex.go` | 4 |
| `api/Config` struct | dissolved into `providers.Request` + `api.Params` | 4 |

### Files added (all ✅ through Phase 4)

- `cmd/imagine/main.go` — trimmed (P1), rewritten as fang launcher with provider blank-imports (P2+P4).
- `commands/root.go`, `commands/config.go`, `commands/describe.go`, `commands/version.go` — P2.
- `internal/paths/paths.go` — P3.
- `internal/images/images.go`, `jpeg.go`, `naming.go` — P3.
- `api/orchestrator.go` — P3 (rewritten to use `Provider` in P4).
- `cli/prompt.go` — P3.
- `providers/provider.go`, `providers/auth.go`, `providers/registry.go` — P4.
- `providers/gemini/gemini.go`, `providers/gemini/flags.go`, `providers/gemini/register.go` — P4.
- `providers/vertex/vertex.go`, `providers/vertex/register.go` — P4.

**Phase 5 will add**: `providers/openai/openai.go`, `providers/openai/edit.go`, `providers/openai/flags.go`, `providers/openai/register.go`.

### Files deleted through phases

| File | When | Reason |
|---|---|---|
| `ui/` (7 files, 1,485 lines) | P1 | TUI removal |
| `views/` (3 files, 339 lines) | P1 | TUI removal |
| `screenshots/` | P1 | TUI-specific assets |
| `api/gemini.go` | P4 | Gemini code moved to `providers/gemini/` |
| `api/vertex.go` | P4 | Vertex code moved to `providers/vertex/` |

### Dependency evolution

| Phase | Added | Removed |
|---|---|---|
| 1 ✅ | — | `charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss` |
| 2 ✅ | `github.com/spf13/cobra` v1.10.2, `charm.land/fang/v2` v2.0.1 | — |
| 3 ✅ | — | — |
| 4 ✅ | — | — |

### Config file evolution

| Phase | Format | Schema |
|---|---|---|
| pre-P1 (banana) | JSON at `~/.config/banana/config.json` | flat: `api_key`, `gcp_project`, `gcp_location` |
| post-P1 ✅ | JSON at `~/.config/imagine/config.json` | same fields, new dir |
| post-P4 ✅ | JSON at `~/.config/imagine/config.json` | flat: `gemini_api_key`, `openai_api_key`, `gcp_project`, `gcp_location`, `default_provider` |
| post-P4.1 ✅ | **YAML** at `~/.config/imagine/config.yaml` | nested under `providers:` namespace |

Current schema (post-P4.1):

```yaml
default_provider: openai
providers:
  gemini:
    api_key: AIza...
  openai:
    api_key: sk-...
  vertex:
    project: my-project
    location: global
```

Go types:
```go
type Config struct {
    DefaultProvider string                    `yaml:"default_provider,omitempty"`
    Providers       map[string]ProviderConfig `yaml:"providers,omitempty"`
}
type ProviderConfig struct {
    APIKey   string `yaml:"api_key,omitempty"`
    Project  string `yaml:"project,omitempty"`  // vertex only
    Location string `yaml:"location,omitempty"` // vertex only
}
```

`map[string]ProviderConfig` (not a typed struct per provider) means adding a new provider requires zero changes to the config package — just a new key under `providers:`. No auto-migration from json; users re-save keys once.

### Flags evolution

| Flag | P1 ✅ | P2 ✅ | P4 ✅ | P5 (planned) |
|---|---|---|---|---|
| `-p`, `-o`, `-f`, `-n`, `-i`, `-r` | unchanged | unchanged | common | common |
| `-h`/`-help` | `-help` (single-dash) | `--help`/`-h` (cobra/fang) | — | — |
| `-v`/`-version` | `-version`/`-v` | `--version`/`-v` (fang) | — | — |
| `-vertex` | single-dash | `--vertex` (double-dash) | **removed**, use `--provider vertex` | — |
| `-is` | two-letter short | `--image-search` | gated: Gemini only | — |
| `-ar` | two-letter short | `--aspect-ratio` | common | common |
| `-g` | `--grounding` | unchanged | gated: Gemini/Vertex only | — |
| `-t` | `--thinking` | unchanged | gated: Gemini/Vertex + flash model | — |
| `-m`, `-s` | hardcoded validation | hardcoded validation | **provider-specific**: validated via active provider's `Info` | openai models + sizes added |
| `--provider` | — | — | **new** — flag/env/config/default precedence | same |
| `-q`/`--quality`, `--output-format`, `--compression`, `--moderation`, `--background` | — | — | — | **new**, openai-only |

### Must-preserve behaviours (all ✅ through Phase 4)

- Shell-glob residual handling for `-i *.png` → now in `commands/root.go:RunE` (positional `args` appended to `opts.RefInputs`).
- `-f` / `-r` mutual exclusion → `root.MarkFlagsMutuallyExclusive("filename", "replace")` in `commands/root.go`.
- `-r` requires exactly one `-i` pointing at a single file → `cli.Options.Validate()`.
- `.jpg` / `.jpeg` output triggers JPEG conversion at quality 95 → `internal/images/jpeg.go:ConvertToJPEG` invoked by `api/orchestrator.go:saveOne`.
- Filename resolution precedence (`-f` → `-r` → timestamped) → `internal/images/naming.go:ResolveFilename` (single source of truth).
- Parallel image loading preserves directory order → `internal/images/images.go:loadDir` (unchanged semantics).

### Explicit scope cuts

- **Describe subcommand is untouched** (functionally). Phase 2 wraps it in a thin cobra command so it appears in `imagine --help`, but its flag parsing stays in `describe.HandleDescribeCommand` for now. Its own `-vertex` branch (`describe/agent.go:111-126`) stays. Making describe provider-aware is a follow-up PR once generate/edit is proven.
- **Bubble Tea v2 is out of scope.** The TUI is removed, not upgraded. If a future `imagine preview` needs a TUI, it gets built on BubbleTea v2 from scratch.
- **Huh v2 is out of scope.** One interactive prompt (`PromptForAPIKey`) uses `term.ReadPassword` and is fine. Adopt huh only if first-run setup grows.
- **Multi-turn / Responses API for OpenAI is out of scope.** Phase 5 uses only `/v1/images/generations` and `/v1/images/edits`.
- **Streaming / partial images is out of scope.** Phase 5 always requests full images.
