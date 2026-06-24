# AGENTS.md
<context>
This is the **imagine** repository: a Go CLI for generating and editing images and videos through multiple AI providers.

Key facts about the codebase:
- One binary, one YAML config file (`~/.config/imagine/config.yaml`), no TUI beyond onboarding wizards.
- Uses Cobra for CLI, `charm.land/fang/v2` for execution context/styled help, and a custom provider abstraction.
- Every provider lives under `providers/<name>/`, registers via `init()`, and is blank-imported by `providers/all/all.go`.
- Most provider work touches *only* files under `providers/<name>/` plus one import line; framework packages are off-limits for provider work (see requirement 1 for the list and the rule).
- Secret resolution is lazy (`${ENV}` and `op://`) and lives in `config/secrets.go`. Provider selection is config-driven, not env-driven.
- A reusable S3-compatible storage brick lives in `internal/storage/`. Currently only `modelark` declares `Bundle.RequireStorage: true` (it needs references reachable by public URL); `fal` uploads references to its own CDN and needs no storage.
</context>

<task>
Prefer the provider-only change pattern, reuse the storage and transport bricks, and keep the framework generic.
</task>

<requirements>
1. **Providers are the primary extension point.** Most work happens under `providers/<name>/`. Treat `commands/`, `cli/`, `api/`, `config/`, `providers/gate.go`, and `cmd/imagine/main.go` as off-limits: do not edit them unless the user explicitly asks for a framework-level change, or you surface the need and get agreement first.
2. **Use the provider Bundle contract.** A new provider needs:
   - `providers/<name>/<name>.go` — `New`, `Info`, `Generate`, wire types.
   - `providers/<name>/options.go` — tagged `Options` struct for `providers/flagspec`.
   - `providers/<name>/register.go` — `providers.Register(...)` with `BindFlags`, `ReadFlags`, `ParseOptions`, `SupportedFlags`, `Info`, `Examples`, `ConfigSchema`.
   - `providers/<name>/help.go` — optional `Examples() string`.
   - `providers/<name>/contract_test.go` — one line: `providertest.Contract(t, "<name>")`.
   - One blank-import line in `providers/all/all.go`.
3. **Reuse framework primitives.** Use `internal/transport/` for HTTP, `internal/storage/` for S3-compatible uploads, and `providers/flagspec/` for flags. Do not hand-roll flag parsing or SigV4 signing.
4. **Lazy secret resolution.** Never resolve secrets in help or `providers show` paths. Resolve only inside the active provider's `Generate` path or when the user explicitly runs `storage test` / `providers add`.
5. **Reference-aware storage gating.** For providers with `Bundle.RequireStorage: true`, only enforce the storage config check when the invocation actually supplies references (`-i`, audio, end-image, etc.). Text-only generation must not require storage.
6. **Video providers.** Set `Capabilities.MediaKind = providers.KindVideo` and return `video/mp4` assets. If the API is asynchronous, poll for completion rather than blocking the request. See `providers/modelark/modelark.go` and `providers/fal/fal.go` as reference implementations.
7. **Preserve YAML comments and key order.** Use `config/save.go` helpers (`SaveProviderFields`, `SaveStorage`) instead of raw file writes.
8. **Run tests and vet before finishing.** After any Go code change, run:
   ```bash
   go test ./...
   go vet ./...
   ```
9. use `firecrawl` as your primary web search and research tool.
</requirements>

<constraints>
- Do not introduce environment-variable based provider selection or credential loading. The config file is the source of truth; values may use `${VAR}` or `op://` references.
- Do not add `imagine config set-*` subcommands. Users edit YAML directly.
- Do not turn `describe` into a provider-aware command unless explicitly asked; it keeps its own legacy flag parsing intentionally.
- Do not hand-edit `cmd/imagine/main.go` to register providers. Use `providers/all/all.go`.
- Do not duplicate the storage or transport logic in a provider. Route through `internal/storage` and `internal/transport`.
</constraints>

<examples>

### Example 1: Adding a new provider

Given: "Add a provider `foo` that POSTs to a single image generation endpoint."

Expected approach:
1. Read `Docs/adding-a-provider.md` and one existing provider (e.g., `providers/gemini/`).
2. Create `providers/foo/foo.go` with `New`, `Info`, and `Generate`.
3. Create `providers/foo/options.go` with a tagged `Options` struct.
4. Create `providers/foo/register.go` that calls `providers.Register("foo", ...)`.
5. Add `providers/foo/contract_test.go`.
6. Add `_ "github.com/AhmedAburady/imagine-cli/providers/foo"` to `providers/all/all.go`.
7. Run `go test ./...` and `go vet ./...`.

No edits to `commands/root.go` or `cmd/imagine/main.go`.

### Example 2: Fixing a storage bug

Given: "Uploads fail when the bucket public URL base is set."

Expected approach:
1. Inspect `internal/storage/storage.go` (`objectURL`, `virtualHostBase`, `putObject`).
2. Add or update unit tests in `internal/storage/storage_test.go`.
3. Fix the URL construction bug, keeping `PublicURLBase` behavior consistent for reads only.
4. Run `go test ./internal/storage/...`.

### Example 3: Extending a provider's flag set

Given: "Add a `--quality` flag to the OpenAI provider."

Expected approach:
1. Add a field to `providers/openai/options.go` with `flag:"quality,q" desc:"..." enum:"low,medium,high,auto"`.
2. Consume the new field in `providers/openai/openai.go`.
3. `providers/openai/register.go` already derives `SupportedFlags` from `flagspec.FieldNames(Options{})`, so no manual list update is needed.
4. Run the contract test: `go test ./providers/openai/...`.

</examples>

## Reference Details

### Reference Docs (read on demand)

- `Docs/adding-a-provider.md` — full provider Bundle contract, worked example, validation gate.
- `Docs/framework-internals.md` — flag-ownership model, `providerOrder`, help rendering / provider hint, Cobra+Fang, context cancellation.
- `Docs/batch-files.md` — batch-file modes, per-entry resolution, two-layer parallelism.
- `Docs/storage.md` — S3-compatible storage brick, SigV4, public-read testing.
- `Docs/video-providers.md` / `Docs/modelark.md` — video provider patterns and the ModelArk integration.

### Provider Resolution Precedence

```
--provider <name>                    # CLI flag
  ↓
default_provider                     # config.yaml
  ↓
first provider under providers:      # alphabetical
  ↓
error
```

### Architecture Map

```
cmd/imagine/main.go         Entry point — fang.Execute wrapper; imports providers/all.
commands/                   Cobra command tree:
  root.go                     NewRootCmd, flag binding, PreRunE branches on opts.IsBatch
  resolve.go                  Provider/default resolution, providerOrder
  validate.go                 Thin adapters over providers/gate.go
  help.go                     Provider-aware flag visibility + examples
  run.go                      runGeneration + runBatch dispatcher + reference loading
  providers.go                `imagine providers show`
  storage.go                  `imagine storage set/show/test/clear`
  describe.go                 describe subcommand (legacy, untouched)
  version.go                  version subcommand
cli/cli.go                  Common flags, IsBatch sentinel, CommonFlagNames.
api/orchestrator.go         RunGeneration — goroutine fan-out, MaxBatchN batching.
config/config.go            YAML loader + StorageConfig.
config/secrets.go           ${ENV} / op:// resolution (lazy).
config/save.go              Atomic, comment-preserving YAML writes.
internal/
  batch/                      Batch-file load, resolve, run
  images/                     MIME detection, Reference loader, ResolveFilename
  paths/                      ExpandTilde
  storage/                    S3-compatible upload brick (SigV4, dedup, public-read test)
  transport/                  HTTP client, JSON helpers, Bearer/NoAuth
providers/                  The extension point:
  provider.go                 Provider interface, Info, Request, Response, Capabilities
  registry.go                 Bundle contract + Register/Get/List
  gate.go                     Flag/model validation rules
  flagspec/                   Reflection-based flag DSL
  all/                        Blank-import bundle of all shipped providers
  gemini/, vertex/, openai/, fal/, modelark/
describe/                   Legacy describe subcommand.
```

### Shipped Providers

- `gemini` — Google Gemini direct REST, image generation, `MaxBatchN=1`.
- `vertex` — Gemini via Vertex AI SDK, image generation, shares flags with Gemini.
- `openai` — OpenAI `/v1/images`, `MaxBatchN=10`, derives output format from `-f` extension.
- `fal` — fal.ai video/image models, async polling.
- `modelark` — BytePlus ModelArk / Dreamina Seedance 2.0 video, requires S3 storage for references.

### Build Commands

```bash
go build -o imagine ./cmd/imagine
go build -ldflags "-X main.version=v1.0.0" -o imagine ./cmd/imagine

# Cross-compile
GOOS=linux   GOARCH=amd64 go build -o imagine-linux-amd64   ./cmd/imagine
GOOS=darwin  GOARCH=arm64 go build -o imagine-darwin-arm64  ./cmd/imagine
GOOS=windows GOARCH=amd64 go build -o imagine-windows-amd64.exe ./cmd/imagine
```

### Common CLI Flags

| Short | Long | Purpose | Default |
|---|---|---|---|
| `-p` | `--prompt` | Prompt text, prompt file, or batch file | required |
| `-o` | `--output` | Output folder | `.` |
| `-f` | `--filename` | Output filename; extension drives format | auto |
| `-n` | `--count` | Number of assets (1-20, provider may cap lower) | `1` |
| `-i` | `--input` | Reference image/video/folder, repeatable | — |
| `-r` | `--replace` | Use input filename for output (single `-i`) | false |
| | `--provider` | Override active provider | config |
| | `--max-parallel` | Cap concurrent requests (0 = unlimited) | 0 |
| | `--embed-metadata` | Embed PNG text chunks | false |

### Config File Example

```yaml
default_provider: gemini

providers:
  gemini:
    api_key: "${GEMINI_API_KEY}"
  openai:
    api_key: "op://Personal/OpenAI/api_key"
  vertex:
    gcp_project: your-project-id
    location: us-central1
  modelark:
    api_key: your-ark-api-key

storage:
  endpoint: https://tos-ap-southeast-1.bytepluses.com
  region: ap-southeast-1
  bucket: imagine-refs
  access_key: "${STORAGE_ACCESS_KEY}"
  secret_key: "op://Personal/Storage/secret_key"
  path_prefix: imagine-refs/
```

### Storage Brick

Providers that fetch references server-side declare `Bundle.RequireStorage: true`. The framework gates on `storage.Configured()` only when references are present.

- `internal/storage/storage.go` — config, content-addressed upload dedup, public URL construction.
- `internal/storage/sigv4.go` — pure stdlib SigV4 signing.
- `commands/storage.go` — `imagine storage set/show/test/clear`.

Run `imagine storage test` to verify signed writes and anonymous reads.

### Release Process

Git tags `v*` trigger `.github/workflows/release.yml`:
1. `VERSION=${GITHUB_REF#refs/tags/}`
2. Build with `-ldflags "-X main.version=$VERSION"`.
3. Produce 6 cross-compiled binaries.
4. Attach to GitHub Release.

## Coding Standards

Follow the conventions documented in `.agents/rules/`:
- **Architecture Ethos**: `.agents/rules/architecture-ethos.md`

### Non-Goals

- No env-var based credential/provider selection (config file only).
- No `imagine config set-*` commands.
- No first-run interactive key prompt.
- No provider-aware refactor of `describe`.
- No streaming / `partial_images` for OpenAI (yet).
- No TUI beyond the `huh`-based onboarding wizards.
