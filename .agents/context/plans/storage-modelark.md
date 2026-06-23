## Final Plan: Reusable S3 Storage Brick + BytePlus ModelArk Provider

> **Design principle — LEGO, not Jenga.** Every piece below either *is* a reusable
> brick or *snaps into* one that already exists. No glue code, no provider-specific
> hacks in the shared layers. The litmus test for each part: *"when the next
> provider needs this, does it get it for free?"* Where the earlier draft
> re-implemented machinery the codebase already ships, this version plugs into the
> existing brick instead — those changes are called out as **[was glue → now brick]**.

---

### Validation summary (what the investigation + API research changed)

**API research (official BytePlus docs, scraped to `.firecrawl/byteplus-*.md`) — confirmed:**
- Base URL `https://ark.ap-southeast.bytepluses.com/api/v3`; endpoints `POST /contents/generations/tasks`, `GET .../{id}`, `DELETE .../{id}` — all correct.
- `content[]` type+role model, `Authorization: Bearer`, status enum (`queued|running|succeeded|failed|cancelled|expired`), `content.video_url`, `error{code,message}` — all correct.
- Model IDs `dreamina-seedance-2-0-260128`, `-fast-260128`, `-mini-260615` — all real. Endpoint IDs (`ep-…`) are **optional**; bare model IDs work.

**API research — corrected the draft:**
- **`generate_audio` defaults to `true`** (not false) — send it explicitly.
- **`frames` and `camera_fixed` are not supported on Seedance 2.0** — the draft correctly omits them; keep omitting.
- **Resolution caps are per-model:** full model → 480p/720p/1080p/4k (4k is 10-bit, much lower rate limits); **Fast/Mini cap at 720p**. A flat enum can't express this → validate per-model.
- **Duration is an integer** 4–15 or `-1` (auto) — so `Duration int` is right, but `range:"-1:15"` leaks 0–3, and `json:",omitempty"` does **not** omit `-1`. Fix both.
- **`video_url` accepts public URLs / asset IDs only — no Base64.** (image/audio also accept Base64, but we deliberately don't use that — see *Why URL-based*.) This is *why* a storage brick is genuinely required for video references.
- **Mini API availability ~2026-06-22** — verify before shipping; keep listed.

**Codebase findings (pattern + DRY alignment):**
- `imagine storage set` must reuse the existing `ConfigField` → `collectFields`/`wizardFill` engine, not a bespoke `huh` form. **[was glue → now brick]**
- Secret resolution (`resolveValue`/`expandEnv`) is unexported and map-driven; a new `config.ResolveStorage(ctx)` (mirroring `ResolveProvider`) is required — don't duplicate env/`op://` logic. **[was glue → now brick]**
- `storage.Get` must take `ctx` (the whole secret path is ctx-cancellable for `op read`).
- `SaveStorage` reuses the `save.go` node-tree helpers; it's ~15 lines, not a re-mirror.
- The `RequireStorage` gate must fire in **both** single-shot (root PreRunE) and **batch** (`internal/batch/resolve.go`) — PreRunE returns early on `IsBatch`, so batch would otherwise skip it.
- `--end-image` (first+last-frame) is **kept** — ModelArk supports `first_frame`/`last_frame` roles.

**Governing full-schema sweep (every create/retrieve/error param read end-to-end):**
- ✅ Request contract verbatim: `POST …/api/v3/contents/generations/tasks`, headers `Content-Type: application/json` + `Authorization: Bearer $ARK_API_KEY`, `model` (accepts model ID *or* endpoint ID), `content[]`. The official curl example uses `dreamina-seedance-2-0-260128` with `reference_image`×2 + `reference_video` + `reference_audio` + `generate_audio:true` + `watermark:false` → `{"id":"cgt-…"}`. Our wire types match exactly.
- ✅ `resolution` 4k *is* valid (tutorial code samples send `"resolution":"4k"`), but **4k + 1080p are full-model only; Fast/Mini → 480p/720p**. Plan's enum + per-model `Validate` already correct.
- ✅ `duration` int `[4,15]` or `-1`; `frames`/`camera_fixed` unsupported on 2.0; `generate_audio` default **true**; `service_tier`/`priority`/`draft`/`safety_identifier`/`return_last_frame`/`callback_url`/`execution_expires_after` are real but intentionally unused. (API's own `duration` default is **5**; we default to `-1`/auto — note the billing caveat: auto picks 4–15s.)
- ✅ Status enum + "only `queued` is cancellable", `error{code,message}` (object/`null`), `content.video_url`/`last_frame_url` — all confirmed.
- ⚠️ **New:** first+last-frame (`--end-image`) is **full + Fast only — not Mini**. `Validate` must reject `--end-image` with `-m mini`.
- ⚠️ **New:** Seedance 2.0 **rejects reference images/videos containing real human faces** server-side (`Input{Image,Video}SensitiveContentDetected.PrivacyInformation`, HTTP 400). Surface the message (transport already extracts `error.message`); document it.
- ⚠️ **New:** ModelArk fetches reference URLs server-side → an unreadable bucket yields `OperationDenied.TosAccessDenied` (403). This is exactly what `imagine storage test`'s public-read check prevents — good alignment.
- ⚠️ **New:** 4k on the full model has much lower rate limits (15 RPM / 1 concurrent for individuals vs 180/3). With `MaxBatchN:1`, `-n>1` at 4k can 429 — document.
- ⚠️ **New:** `enum:"@models"` rejects endpoint IDs (`ep-…`), so only the three model IDs/aliases work via `-m`. Acceptable limitation (matches fal); note it.

---

### Architecture Overview

Three independently-shippable bricks that compose:

1. **`internal/storage/`** — generic, S3-compatible upload brick (any S3-compatible backend; minimal SigV4 in pure stdlib, no AWS SDK). Reusable by *any* provider.
2. **`imagine storage` command** — config + public-access test, built entirely from existing onboarding bricks.
3. **`providers/modelark/`** — first consumer of the storage brick; a BytePlus ModelArk video provider that plugs into the Provider contract exactly like fal.

The storage brick and its `RequireStorage` hook are the durable deliverable; modelark is the proof that a new provider snaps in with zero edits to shared layers.

---

### Part 1 — Config Layer (the storage brick's schema + secret resolution)

**`config/config.go`** — add a top-level `storage:` section. Storage is *not* a provider (no model, no Generate), so it gets its own section rather than being forced into the `providers:` map — keeping `providers show`/`use`/model-resolution clean.

```go
type Config struct {
    DefaultProvider       string                    `yaml:"default_provider,omitempty"`
    VisionDefaultProvider string                    `yaml:"vision_default_provider,omitempty"`
    Storage               *StorageConfig            `yaml:"storage,omitempty"`
    Providers             map[string]ProviderConfig `yaml:"providers,omitempty"`
}

type StorageConfig struct {
    Endpoint      string `yaml:"endpoint"`                  // e.g. "https://tos-ap-southeast-1.bytepluses.com"
    Region        string `yaml:"region,omitempty"`         // e.g. "ap-southeast-1"
    Bucket        string `yaml:"bucket"`
    AccessKey     string `yaml:"access_key"`               // supports ${ENV} / op://
    SecretKey     string `yaml:"secret_key"`               // supports ${ENV} / op://
    PathPrefix    string `yaml:"path_prefix,omitempty"`    // e.g. "imagine-refs/"
    PublicURLBase string `yaml:"public_url_base,omitempty"`// CDN/custom-domain override
}
```

**`config/secrets.go`** — add `ResolveStorage`, the exact twin of `ResolveProvider`, reusing the existing `resolveValue` (env expansion + 1Password, ctx-threaded). **[was glue → now brick]** This is the only correct way to honour `${ENV}`/`op://` on storage credentials without duplicating resolution logic:

```go
// ResolveStorage returns a copy of the storage config with every field resolved
// through expandEnv + 1Password, mirroring ResolveProvider. nil when unconfigured.
func (c *Config) ResolveStorage(ctx context.Context) (*StorageConfig, error) {
    if c == nil || c.Storage == nil {
        return nil, nil
    }
    out := *c.Storage
    for _, f := range []*string{&out.Endpoint, &out.Region, &out.Bucket,
        &out.AccessKey, &out.SecretKey, &out.PathPrefix, &out.PublicURLBase} {
        v, err := resolveValue(ctx, *f)
        if err != nil {
            return nil, fmt.Errorf("storage: %w", err)
        }
        *f = v
    }
    return &out, nil
}
```

**`config/save.go`** — add `SaveStorage(*StorageConfig)`, reusing the existing node-tree helpers (`findOrCreateMapping(top, "storage")`, `setMappingScalar`, `removeMappingKey`, `writeNodeFile`). It differs from `SaveProviderFields` only in target mapping (top-level `storage` vs `providers.<name>`), so it's a thin flatten-then-upsert, not a re-implementation:

```go
func SaveStorage(sc *StorageConfig) error {
    // read-or-create config (same path as SaveProviderFields) →
    // node := findOrCreateMapping(top, "storage") →
    // setMappingScalar for each non-empty field (deterministic order) →
    // writeNodeFile
}
```

Config YAML shape:
```yaml
storage:
  endpoint: "https://tos-ap-southeast-1.bytepluses.com"
  region: "ap-southeast-1"
  bucket: "my-bucket"
  access_key: "AK..."
  secret_key: "${S3_SECRET}"      # or op://vault/s3/secret, or a literal
  path_prefix: "imagine-refs/"    # optional
  public_url_base: ""             # optional: CDN/custom domain
providers:
  modelark: { api_key: ... }
```

---

### Part 2 — `internal/storage/` Brick

Reuses `config.StorageConfig` directly (no second struct), `images.ExtForMime` for key extensions, and `transport` for the public-read GET. SigV4 is the only net-new primitive.

| File | Purpose | ~Lines |
|---|---|---|
| `storage.go` | `Configured()`, `Get(ctx)`, `Upload(ctx, data, mime) (url, err)`, `Test(ctx, cfg)`, `Delete(ctx, key)` + sha256 singleflight | ~140 |
| `sigv4.go` | Minimal AWS SigV4 signing for S3-compatible PUT/DELETE (pure `crypto/hmac`+`crypto/sha256`) | ~90 |
| `storage_test.go` | URL construction, prefix logic, SigV4 against AWS published test vectors | ~70 |

**Key APIs:**

```go
package storage

// Configured reports whether a usable storage: section exists (cheap; no secret
// resolution, no op:// prompt). Used by the RequireStorage gate.
func Configured() bool

// Get loads + resolves storage config (${ENV}/op://). ctx cancels op reads.
func Get(ctx context.Context) (*config.StorageConfig, error)

// Upload PUTs data to the user's S3-compatible bucket and returns the public URL.
// Key = path_prefix + sha256(data) + ExtForMime(mime). Identical bytes dedup via
// a package-level singleflight, so concurrent/repeated refs upload once — the
// reusable home for the pattern fal hand-rolls per-provider.
func Upload(ctx context.Context, data []byte, contentType string) (publicURL string, err error)

// Test round-trips a marker object to prove public-read works, then cleans up.
func Test(ctx context.Context, cfg *config.StorageConfig) error

// Delete removes one object by key (best-effort cleanup).
func Delete(ctx context.Context, key string) error
```

**Upload flow:**
1. `Get(ctx)` → resolved config.
2. Key: `{path_prefix}{sha256_hex}{ExtForMime(mime)}` — content-addressed (idempotent re-PUT, free dedup).
3. SigV4-sign a PUT (service `s3`, region from config — the standard for S3-compatible backends incl. BytePlus TOS, MinIO, R2, Wasabi).
4. Execute via a `transport`-pooled client (raw `net/http` for the signed body, as `transport` explicitly permits — same precedent as fal's CDN PUT).
5. Public URL: `{public_url_base | endpoint}/{bucket}/{key}` (path-style).

**Test flow (`imagine storage test`):**
1. PUT `{prefix}imagine-test-{uuid}.txt` (random marker; `google/uuid` is already a dep).
2. `transport.GetBytes(ctx, client, publicURL, transport.NoAuth())` — unauthenticated read.
3. Verify body == marker.
4. `Delete` the marker (best-effort).
5. Report the verified URL, or fail with the HTTP status.

**Why no AWS SDK:** a SigV4 PUT is ~90 lines of stdlib; the SDK pulls dozens of transitive packages for one call. Zero AWS deps today; `transport` already drops to raw `net/http` for exactly this kind of signed PUT. Generic by construction — *S3-compatible*, not AWS-specific.

**Why URL-based (not Base64):** ModelArk's `video_url` accepts **URLs/asset-IDs only** — Base64 is impossible for the video-reference case the brick exists to serve. Keeping every ref on one URL-based path (upload → public URL) is uniform and mirrors fal exactly; Base64 would be a per-kind special case (a Jenga move) that still can't cover video. The brick stays general; providers stay simple.

---

### Part 3 — `imagine storage` Command (built from existing onboarding bricks)

**`commands/storage.go`** (package `commands`) reuses the onboarding engine already powering `providers add`. **[was glue → now brick]** The storage fields are modelled as `[]providers.ConfigField`, then handed to the existing `collectFields` / `wizardFill` / `registerFieldFlags` / `missingFlagsError` / `toFlag` — so the dual-mode behaviour (flags / TTY wizard / non-TTY error) and secret masking come for free, identical to every provider:

```
imagine storage           → show current config (non-secrets shown, secrets masked) or "not configured"
imagine storage set       → collectFields(cmd, storageFields) → wizardFill → config.SaveStorage(...)
imagine storage test      → storage.Test(ctx, cfg)
imagine storage clear     → remove the storage: section (removeMappingKey + writeNodeFile)
```

The field set (drives both `--flags` and the wizard):

| Key | Flag | Title | Secret | Required | Default |
|---|---|---|---|---|---|
| `endpoint` | `--endpoint` | Endpoint | no | yes | — |
| `region` | `--region` | Region | no | no | — |
| `bucket` | `--bucket` | Bucket | no | yes | — |
| `access_key` | `--access-key` | Access Key | no | yes | — |
| `secret_key` | `--secret-key` | Secret Key | **yes** | yes | — |
| `path_prefix` | `--path-prefix` | Path Prefix | no | no | `imagine-refs/` |
| `public_url_base` | `--public-url-base` | Public URL Base | no | no | — |

Net-new (small): `storage show` masks secret values when printing. `providers show` lists names only and `providers add` only collects, so there's no existing masked-display helper — this is the one genuinely new ~10-line piece, not a "mirror".

**Registration** — one line in `commands/root.go`:
```go
root.AddCommand(
    newDescribeCmd(describeHint),
    newVersionCmd(version),
    newProvidersCmd(),
    newStorageCmd(),   // ← new
    newMetadataCmd(),
)
```

Reduces Part 3 from the draft's ~200 lines (bespoke form) to ~90 (wiring + masked show).

---

### Part 4 — The `RequireStorage` Hook (generic framework brick)

**`providers/registry.go`** — one field on `Bundle`:

```go
// RequireStorage marks a provider that needs the shared S3 storage brick to
// publish local references as public URLs. Generic: any future provider sets
// this and inherits the configured-storage gate + helpful onboarding error,
// with no other framework changes.
RequireStorage bool
```

All existing providers keep the zero value (`false`) — no edits. modelark sets `true`.

**The gate fires everywhere generation is dispatched — uniformly, not per-provider:**

1. **Single-shot** — `commands/root.go` PreRunE, right after `bundle, _ := providers.Get(active)`:
   ```go
   if bundle.RequireStorage && !storage.Configured() {
       return fmt.Errorf("provider %q needs S3-compatible storage — run `imagine storage set` first", active)
   }
   ```
2. **Batch** — `internal/batch/resolve.go`, per resolved entry (PreRunE early-returns on `IsBatch`, so this is **required**, not optional, for parity). The per-entry bundle is already in scope there; same two-line check. No import cycle (`internal/batch → internal/storage → config`).

Because the gate keys off a generic capability bit, the next storage-backed provider is a one-line `RequireStorage: true` — the LEGO test passes.

---

### Part 5 — `providers/modelark/` (first consumer; mirrors fal)

| File | Purpose | ~Lines |
|---|---|---|
| `modelark.go` | Provider, `New`, `Info`, `Generate` (classify→upload→create→poll→download), `poll`, `cancel`, `validateReferenceConstraints` | ~220 |
| `options.go` | Tagged `Options` + `Normalize`/`Validate(info)` + `RequestLabel`/`ResolvedModel` | ~55 |
| `register.go` | `init()` flagspec closures + `RequireStorage: true` + `ConfigSchema` | ~40 |
| `help.go` | `Examples()` | ~30 |
| `contract_test.go` | `providertest.Contract(t, "modelark")` | ~10 |

**`options.go`** — `--end-image` restored; `Duration int` (matches API); per-model validation via `Validate`:
```go
type Options struct {
    Model       string `flag:"model,m"        desc:"Tier: seedance (default), fast, or mini" enum:"@models"`
    Resolution  string `flag:"size,s"         desc:"Resolution: 480p/720p (fast,mini); +1080p,4k (full)" enum:"480p,720p,1080p,4k" default:"720p"`
    AspectRatio string `flag:"aspect-ratio,a" desc:"adaptive,21:9,16:9,4:3,1:1,3:4,9:16" enum:"adaptive,21:9,16:9,4:3,1:1,3:4,9:16" default:"adaptive"`
    Duration    int    `flag:"duration"       desc:"Seconds: 4-15, or -1 for auto" default:"-1" range:"-1:15"`
    Audio       bool   `flag:"audio"          desc:"Generate synchronized audio" default:"true"`
    EndImage    string `flag:"end-image"      desc:"i2v only: end-frame image path (first+last frame)"`
}

// Validate is called by flagspec after population — the idiomatic home for the
// rules the flat enum/range can't express:
//   - resolution 1080p/4k → full model only (fast/mini cap at 720p)
//   - duration must be -1 or 4..15 (range tag allows 0..3; tighten here)
//   - --end-image (first+last frame) → full + fast only (NOT mini)
func (o *Options) Validate(info providers.Info) error { ... }

func (o *Options) RequestLabel() string  { return o.Model }
func (o *Options) ResolvedModel() string { return o.Model }
```
(Drops `--bitrate`, `--seed`, `--watermark` per prior decision. Audio default `true` matches the API default.)

**`modelark.go` wire types:**
```go
const baseURL = "https://ark.ap-southeast.bytepluses.com/api/v3"

type urlObject struct{ URL string `json:"url"` }

type contentItem struct {
    Type     string     `json:"type"`               // text | image_url | video_url | audio_url
    Text     string     `json:"text,omitempty"`
    ImageURL *urlObject `json:"image_url,omitempty"`
    VideoURL *urlObject `json:"video_url,omitempty"`
    AudioURL *urlObject `json:"audio_url,omitempty"`
    Role     string     `json:"role,omitempty"`     // first_frame|last_frame|reference_image|reference_video|reference_audio
}

type createReq struct {
    Model         string        `json:"model"`
    Content       []contentItem `json:"content"`
    GenerateAudio bool          `json:"generate_audio"`      // explicit (API default true)
    Resolution    string        `json:"resolution,omitempty"`
    Ratio         string        `json:"ratio,omitempty"`
    Duration      int           `json:"duration"`            // -1 = auto; NOT omitempty (would only drop 0, never -1)
    Watermark     bool          `json:"watermark"`           // always false
}

type createResp struct{ ID string `json:"id"` }              // "cgt-..."

type taskResp struct {
    Status  string       `json:"status"`                     // queued|running|succeeded|failed|cancelled|expired
    Content *taskContent `json:"content,omitempty"`
    Error   *taskError   `json:"error,omitempty"`
}
type taskContent struct{ VideoURL string `json:"video_url"` } // last_frame_url omitted (return_last_frame not requested)
type taskError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

**Generate flow (mirrors fal's modality switch; uploads through the storage brick instead of fal's CDN):**
1. Classify references by `images.KindOf` into image/video/audio buckets.
2. Determine the (mutually-exclusive, per API) mode:
   - **0 refs → text.** `--end-image` without `-i` → error (mirrors fal).
   - **1 image only → first-frame.** With `--end-image` → load it; **first+last** mode (`first_frame` + `last_frame` roles; full+fast only, rejected for mini in `Validate`).
   - **else → multimodal reference** (`reference_image`/`reference_video`/`reference_audio`). `--end-image` invalid here.
3. `validateReferenceConstraints` *before* any upload (cheap fail on local bytes), mirroring fal with ModelArk's limits: images ≤9 (≤30MB each); videos ≤3 (≤50MB each); audio ≤3 (≤15MB each), audio cannot be sent alone.
4. Upload every local ref via `storage.Upload(ctx, data, mime)` → public URL; build `content[]` with the right type+role. (`-i` is file-only today; URL refs are a future loader option, out of scope.)
5. Prepend the prompt as `{type:"text", text:...}`.
6. `POST {baseURL}/contents/generations/tasks` with `transport.Bearer(apiKey)` → `id`.
7. Poll `GET .../tasks/{id}` every 5s on the documented status enum:
   - `succeeded` → `content.video_url`
   - `failed`/`expired` → surface `error.code` + `error.message`
   - `cancelled` → surface
   - `ctx.Done()` → best-effort `DELETE .../tasks/{id}` (only `queued` tasks are cancellable; `running` can't be — same best-effort spirit as fal's cancel).
8. Download via `transport.GetBytes(ctx, downloadClient, videoURL, transport.NoAuth())` (output URLs expire in 24h — download immediately).
9. Return `Response{Assets: [{Data, MimeType: "video/mp4"}]}`.

**Info:**
```go
Info{
    Name:         "modelark",
    DisplayName:  "BytePlus ModelArk",
    Summary:      "BytePlus ModelArk video models (Dreamina Seedance 2.0 direct API)",
    DefaultModel: "dreamina-seedance-2-0-260128",
    Models: []ModelInfo{
        {ID: "dreamina-seedance-2-0-260128",      Aliases: []string{"seedance", "seedance-2"},      Description: "Seedance 2.0 (highest quality; 1080p/4k)."},
        {ID: "dreamina-seedance-2-0-fast-260128", Aliases: []string{"seedance-fast", "fast"},        Description: "Seedance 2.0 Fast (≤720p)."},
        {ID: "dreamina-seedance-2-0-mini-260615", Aliases: []string{"seedance-mini", "mini"},        Description: "Seedance 2.0 Mini (≤720p; no first+last frame; API ~2026-06-22 — verify)."},
    },
    Capabilities: Capabilities{
        Edit:      true,
        MaxBatchN: 1,
        MediaKind: KindVideo,
        RefKinds:  []MediaKind{KindImage, KindVideo, KindAudio},
        MaxN:      4,
    },
}
```

**register.go** — fal's pattern verbatim, plus the two extras:
```go
providers.Register("modelark", providers.Bundle{
    Factory:      New,
    BindFlags:    func(cmd *cobra.Command) { flagspec.Bind(cmd, Options{}) },
    ReadFlags:    func(cmd *cobra.Command) (any, error) { return flagspec.Read(cmd, Options{}, info) },
    ParseOptions: func(v map[string]any, _ providers.Common) (any, error) { return flagspec.Parse(Options{}, v, info) },
    SupportedFlags: flagspec.FieldNames(Options{}),
    Examples:       Examples,
    Info:           info,
    ConfigSchema:   (&Provider{}).ConfigSchema(),  // api_key (Bearer ARK_API_KEY)
    RequireStorage: true,                           // ← the only new field
})
```

---

### Part 6 — `providers/all/all.go`

One line — the entire registration cost of a new provider:
```go
_ "github.com/AhmedAburady/imagine-cli/providers/modelark" // ← new
```

---

### Complete File Manifest

**New files (9):**
| File | ~Lines |
|---|---|
| `internal/storage/storage.go` | ~140 |
| `internal/storage/sigv4.go` | ~90 |
| `internal/storage/storage_test.go` | ~70 |
| `commands/storage.go` | ~90 |
| `providers/modelark/modelark.go` | ~220 |
| `providers/modelark/options.go` | ~55 |
| `providers/modelark/register.go` | ~40 |
| `providers/modelark/help.go` | ~30 |
| `providers/modelark/contract_test.go` | ~10 |

**Modified files (6):**
| File | Change |
|---|---|
| `config/config.go` | `StorageConfig` struct + `Storage` field (~20 lines) |
| `config/secrets.go` | `ResolveStorage(ctx)` reusing `resolveValue` (~18 lines) |
| `config/save.go` | `SaveStorage` reusing node-tree helpers (~18 lines) |
| `providers/registry.go` | `RequireStorage bool` on `Bundle` (1 line + doc) |
| `commands/root.go` | register `newStorageCmd()` + single-shot gate (~4 lines) |
| `internal/batch/resolve.go` | per-entry `RequireStorage` gate (~3 lines) |
| `providers/all/all.go` | blank import for modelark (1 line) |

No edits to `cli/`, `api/`, `providers/gate.go`, `providers/flagspec/`, or `cmd/imagine/main.go` — the brick contract holds.

---

### Co-existence with fal

- Separate packages, separate registration, separate model IDs; both wrap Seedance 2.0 with no conflict.
- Shared flags (`-m`, `-s`, `-a`) register idempotently — active provider's help text wins.
- fal keeps its own CDN upload (`RequireStorage` stays `false`); modelark uses the shared brick.
- Batch files freely mix `provider: fal` and `provider: modelark`.
- `--end-image` exists on both (fal i2v end-frame ↔ modelark first+last-frame).

---

### What I'd verify during implementation

1. `go build ./...` clean; `go vet ./...`.
2. `go test ./internal/storage/...` — SigV4 against AWS published test vectors; key/URL construction.
3. `go test ./providers/modelark/...` — `providertest.Contract` passes (Info/aliases/flag consistency).
4. `imagine storage set --endpoint … --bucket … --access-key … --secret-key …` writes `storage:` (comments preserved).
5. `imagine storage test` proves public read against a real bucket; `imagine storage show` masks the secret; `imagine storage clear` removes the section.
6. `imagine --provider modelark -p "…"` (text→video), `-i start.png` (i2v), `-i start.png --end-image end.png` (first+last), and a multimodal reference set — each produces an mp4.
7. Gate: `modelark` with no `storage:` errors *before* any HTTP call, in both single-shot and a batch entry.
8. `imagine -p batch.yaml` mixing fal + modelark entries; `imagine providers show` lists modelark.
9. Per-model validation: `-m fast -s 1080p` and `-m fast -s 4k` rejected with a clear message; `--duration 2` rejected.

---

### Intentionally out of scope (for now)

- Base64/asset-ID reference modes (URL-based via the storage brick is the single uniform path).
- `service_tier`, `priority`, `callback_url`, `execution_expires_after`, `return_last_frame`, `draft` (1.5 Pro).
- URL references via `-i` (loader is file-only).
- A per-backend SigV4 `service` override (defaults to `s3`, correct for S3-compatible backends).
