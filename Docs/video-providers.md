# Video Provider Support — Design Doc

> Status: **proposed** · Scope: generalize the provider spine from "image" to "media", then add fal.ai **Seedance 2.0** (text/image/reference → video) as the first video provider.
>
> Guiding principle (unchanged from [adding-a-provider.md](adding-a-provider.md)): adding a video provider must touch **only** its own `providers/<name>/` package plus one line in `providers/all/all.go`. No edits to `commands/`, `cli/`, `api/`, `config/`, `providers/gate.go`, or `cmd/imagine/main.go` — *after* the one-time spine generalization in §4.

---

## Table of contents

- [1. Goal](#1-goal)
- [2. What is already media-neutral](#2-what-is-already-media-neutral)
- [3. The fal / Seedance 2.0 API (researched)](#3-the-fal--seedance-20-api-researched)
- [4. Core generalization — the media spine](#4-core-generalization--the-media-spine)
- [5. References — multi-modal & provider-aware](#5-references--multi-modal--provider-aware)
- [6. The `fal` provider package](#6-the-fal-provider-package)
- [7. Cost guardrail](#7-cost-guardrail)
- [8. Locked decisions](#8-locked-decisions)
- [9. Open items to verify](#9-open-items-to-verify)
- [10. Build order](#10-build-order)
- [11. Why the next video provider is free](#11-why-the-next-video-provider-is-free)

---

## 1. Goal

Generate video from the CLI through the **same** provider abstraction, resolution precedence, batch runner, validation gate, config/secrets, and contract test that image providers already use. A video provider is a normal `providers.Provider`; "video-ness" is reduced to two declarative `Capabilities` fields plus the provider's own `Generate`.

First provider: fal.ai **Seedance 2.0**, all three modalities (text-to-video, image-to-video, reference-to-video) in both `normal` and `fast` tiers.

---

## 2. What is already media-neutral

The abstraction is ~90% media-agnostic today. These need **zero** changes:

| Subsystem | File | Why it already works |
|---|---|---|
| Registry / `Bundle` | `providers/registry.go` | Pure wiring; knows nothing about pixels. |
| Validation gate | `providers/gate.go` | Operates on flag names + `Info.Models`; media-blind. |
| Flag DSL | `providers/flagspec/` | Reflection over a tagged struct; works for any flags. |
| Resolution precedence | `commands/resolve.go` | `--provider` → `default_provider` → alphabetical. |
| Batch runner | `internal/batch/` | Fans entries to `api.RunGeneration`; counts results. |
| Contract test | `providers/providertest/contract.go` | Asserts Info/alias/flag invariants only — **a video provider passes as-is**. |
| Config + secret refs | `config/` | `api_key` / `provider_options`, `${ENV}`, `op://`. |
| Transport core | `internal/transport/transport.go` | `PostJSON`, error parsing, pooling. |

Even `providers.GeneratedImage{Data []byte, MimeType string}` is *structurally* already a generic byte+MIME asset — only its **name** says "image."

---

## 3. The fal / Seedance 2.0 API (researched)

Sources: `context/fal-seedance2/` (per-endpoint docs + `schema.json`), `docs.fal.ai` (queue + CDN), and the `fal-ai/fal-js` client source (`libs/client/src/storage.ts`, `config.ts`).

### 3.1 Endpoints & modalities

One model, three modalities × two tiers = six endpoints, **all sharing one input/output schema**:

```
bytedance/seedance-2.0/text-to-video            bytedance/seedance-2.0/fast/text-to-video
bytedance/seedance-2.0/image-to-video           bytedance/seedance-2.0/fast/image-to-video
bytedance/seedance-2.0/reference-to-video       bytedance/seedance-2.0/fast/reference-to-video
```

We model the **tier as the provider "model"** and **derive the modality at call time** from references — mirroring imagine's existing "`-i` flips to edit mode" philosophy (§6.3).

### 3.2 Input / output schema

Shared input (modality adds the image/ref fields):

| Field | Type | Notes |
|---|---|---|
| `prompt` | string, **required** | motion / action description |
| `resolution` | enum | `480p` / `720p` / `1080p` (default `720p`) |
| `duration` | enum | `auto` / `4`..`15` seconds (default `auto`) |
| `aspect_ratio` | enum | `auto`/`21:9`/`16:9`/`4:3`/`1:1`/`3:4`/`9:16` |
| `generate_audio` | bool | default `true` |
| `bitrate_mode` | enum | `standard` / `high` |
| `seed` | int? | optional |
| **i2v** `image_url` | string, **required** | start frame URL |
| **i2v** `end_image_url` | string? | optional end frame |
| **r2v** `image_urls[]` | string[] | ≤9, referenced as `@Image1`… |
| **r2v** `video_urls[]` | string[] | ≤3 |
| **r2v** `audio_urls[]` | string[] | ≤3 |

Output (all modalities): `{ "video": { "url": "https://…​.mp4" }, "seed": 42 }` — **a remote URL, not bytes**.

### 3.3 Authentication

```
Authorization: Key <FAL_KEY>
```

Confirmed from `schema.json` (`securitySchemes.apiKeyAuth` → header `Authorization`, "Fal Key") and every cURL example. Stored as `providers.fal.api_key` in config; supports `${ENV}` / `op://` refs for free.

### 3.4 Queue API (recommended transport)

Video gen runs for minutes, so we use the **queue API** (not the synchronous `https://fal.run/…` endpoint). `Generate` blocks internally doing submit → poll → fetch; the orchestrator never learns it's async.

```
# 1. Submit
POST https://queue.fal.run/bytedance/seedance-2.0/text-to-video
     Authorization: Key <FAL_KEY>
     Content-Type: application/json
     { "prompt": "...", "resolution": "720p", ... }
→ 200 {
    "request_id":  "764cabcf-…",
    "status_url":  "https://queue.fal.run/.../requests/764c…/status",
    "response_url":"https://queue.fal.run/.../requests/764c…/response",
    "cancel_url":  "https://queue.fal.run/.../requests/764c…/cancel",
    "queue_position": 0
  }

# 2. Poll (use the returned status_url verbatim; ?logs=1 for runner logs)
GET <status_url>?logs=1
→ { "status": "IN_QUEUE" | "IN_PROGRESS" | "COMPLETED",
    "queue_position": 2,                 # only IN_QUEUE
    "logs": [ {message, timestamp}, … ], # when logs enabled
    "metrics": { "inference_time": 3.4 },# only COMPLETED
    "error": "…", "error_type": "…" }    # only on failure

# 3. Fetch result once COMPLETED
GET <response_url>
→ { "video": { "url": "https://v3.fal.media/files/…​.mp4" }, "seed": 42 }

# 4. Cancel (best-effort, on ctx cancellation)
PUT <cancel_url>
→ 202 {"status":"CANCELLATION_REQUESTED"} | 400 ALREADY_COMPLETED | 404 NOT_FOUND
```

Poll loop: ~2–3 s interval with light backoff, `ctx`-aware (stop + best-effort `PUT cancel_url` on Ctrl-C). An SSE `text/event-stream` status endpoint also exists (optional, deferred).

### 3.5 CDN file upload (local ref → URL)

Seedance i2v/r2v want **URLs**, but `-i` gives local files. We upload each ref to fal's CDN. Exact protocol from `fal-js/storage.ts` (single-file path; multipart only triggers >90 MB, and the largest seedance ref is a 50 MB video, so **single-file covers everything**):

```
# 1. Initiate (auth header applied)
POST https://rest.fal.ai/storage/upload/initiate?storage_type=fal-cdn-v3
     Authorization: Key <FAL_KEY>
     Content-Type: application/json
     { "content_type": "image/png", "file_name": "ref0.png" }
→ { "file_url": "https://v3.fal.media/files/…​.png",
    "upload_url": "<signed PUT URL>" }

# 2. Upload bytes to the signed URL (NO auth header — credentials are in the URL)
PUT <upload_url>
    Content-Type: image/png
    <raw bytes>

# 3. Pass file_url as image_url / image_urls[i] / video_urls[i] / audio_urls[i]
```

REST base is `https://rest.fal.ai` (current; the older `rest.alpha.fal.ai` is superseded). Data URIs are accepted by the API but **explicitly not used** here (decision §8).

### 3.6 Output download

Result URLs (`https://v3.fal.media/...`) are public, need no auth, and **expire** per the account's media-retention setting. The provider downloads the mp4 bytes inside `Generate` and returns them — so the orchestrator's save path stays byte-based (§4).

### 3.7 Reference-to-video constraints

Enforced client-side where cheap (we hold the bytes); the rest fall through to API errors:

- ≤ **9** images, ≤ **3** videos, ≤ **3** audio; **≤ 12 files total** across modalities.
- Per file: image ≤ 30 MB; audio ≤ 15 MB each; videos ≤ 50 MB combined.
- Videos: combined duration 2–15 s, each ~480p–720p (defer to API).
- **If any audio ref is present, at least one image or video ref is required.**

---

## 4. Core generalization — the media spine

The only one-time core work. Each item is small and gated so existing image providers are byte-for-byte unaffected (zero-value defaults = today's behaviour).

| # | Location | Change |
|---|---|---|
| 1 | `providers/provider.go:84-92` | Rename `GeneratedImage` → **`GeneratedAsset`**, `Response.Images` → **`Response.Assets`**. Add `type GeneratedImage = GeneratedAsset` alias so element-type references keep compiling; only the `Response{Assets: …}` construction sites (gemini, vertex, openai ×2) change. |
| 2 | `providers/provider.go:59-67` | Add to `Capabilities`: `MediaKind` (`KindImage` default / `KindVideo`) and `RefKinds []MediaKind` (accepted reference classes; default = image-only). These two fields are the entire declarative surface a provider uses to announce "I make video / I ingest video+audio refs." |
| 3 | `internal/images/naming.go:29-54` | `ResolveFilename` takes a kind/default-ext parameter instead of hardcoding `.png`. Video → default `.mp4`, honors `-f out.mp4`; image keeps current `.png`/`.jpg` rules. |
| 4 | `api/orchestrator.go:196-227` (`saveOne`) | Derive the output extension from the asset's `MimeType`; run `ConvertToJPEG` / `EmbedPNGText` **only for `image/*`** assets. Video bytes are written verbatim. (`EmbedPNGText` already no-ops on non-PNG.) |
| 5 | `internal/transport/transport.go` | Add three reusable primitives: a `Key`-scheme auth injector (mirrors `Bearer`, ~8 lines), `GetJSON[Resp]` (status/result polling), `GetBytes` (download mp4 + upload helpers). Principled shared additions, not fal-specific. |
| 6 | cosmetics | Spinner reads `MediaKind` ("Generating video"); batch summary column `IMAGES` → `OUTPUT`. Nothing breaks without these. |

What is explicitly **not** here: no `VideoProvider` interface, no second orchestrator/registry, no `commands/`·`gate.go`·`main.go` edits. `Generate(ctx, req) (*Response, error)` is unchanged — async-ness lives inside the fal provider.

### Illustrative shapes

```go
// providers/provider.go
type MediaKind int
const ( KindImage MediaKind = iota; KindVideo /* KindAudio later */ )

type GeneratedAsset struct { Data []byte; MimeType string }
type GeneratedImage = GeneratedAsset      // back-compat alias

type Response struct { Assets []GeneratedAsset }

type Capabilities struct {
    Edit        bool
    Grounding   bool
    Thinking    bool
    ImageSearch bool
    MaxBatchN   int
    Sizes       []string
    MediaKind   MediaKind   // NEW — zero value = KindImage
    RefKinds    []MediaKind // NEW — nil = {KindImage}
    MaxN        int         // NEW (§7) — 0 = use global 1..20
}
```

---

## 5. References — multi-modal & provider-aware

All-three scope means `-i` must accept `.mp4/.mov/.mp3/.wav`, **and** acceptance must stay provider-aware (an image provider must still reject `-i clip.mp4`). We preserve the existing common-vs-provider split:

1. **Broaden the loader** (`internal/images`): extend the ext→MIME table with `video/*` and `audio/*` so `images.Load` can read those bytes. `Reference.MimeType` already classifies them by prefix.
2. **`cli.Validate`** keeps only the generic check: the ref exists and is a *known media file* (now including video/audio).
3. **`commands/validate.go`** (next to the existing gate adapters) adds the provider-aware check: each ref's MIME class must be in the **active provider's `Capabilities.RefKinds`**. Image providers default to `{KindImage}` and reject video/audio with a clear message.
4. The fal provider classifies each `req.References[i]` by MIME prefix and routes to `image_url`/`image_urls`/`video_urls`/`audio_urls` (§6.3).

---

## 6. The `fal` provider package

Same recipe as `providers/openai/`, plus one helper file for CDN uploads.

```
providers/fal/
  fal.go            ← Provider: New, Info, Generate + queue/route logic
  storage.go        ← uploadToFalStorage(ctx, bytes, mime, name) (url, error)
  options.go        ← tagged Options (flagspec)
  register.go       ← one providers.Register("fal", …) with flagspec closures
  help.go           ← Examples()
  contract_test.go  ← providertest.Contract(t, "fal")
```

Plus **one line** in `providers/all/all.go`.

### 6.1 Info / models / capabilities

```go
func (p *Provider) Info() providers.Info {
    return providers.Info{
        Name: "fal", DisplayName: "fal.ai", Summary: "fal.ai video models (Seedance 2.0)",
        DefaultModel: "bytedance/seedance-2.0",
        Models: []providers.ModelInfo{
            {ID: "bytedance/seedance-2.0",      Aliases: []string{"seedance", "seedance-2"}, Description: "Seedance 2.0 (normal tier)."},
            {ID: "bytedance/seedance-2.0/fast", Aliases: []string{"seedance-fast", "fast"},  Description: "Seedance 2.0 (fast tier — lower latency/cost)."},
        },
        Capabilities: providers.Capabilities{
            Edit:      true,                 // accepts reference inputs
            MaxBatchN: 1,                    // one video per call; -n fans out N calls (Gemini path)
            MediaKind: providers.KindVideo,
            RefKinds:  []providers.MediaKind{providers.KindImage, providers.KindVideo, providers.KindAudio},
            MaxN:      4,                    // cost guard (§7)
        },
    }
}
```

`MaxBatchN: 1` means `-n 3` issues three parallel queue submissions — exactly the existing Gemini/Vertex fan-out, no new orchestration.

### 6.2 Options & flags (reuse `-m`/`-s`/`-a`)

```go
type Options struct {
    Model       string `flag:"model,m"        desc:"Tier: seedance (default) or seedance-fast" enum:"@models"`
    Resolution  string `flag:"size,s"         desc:"Resolution: 480p, 720p, 1080p (default: 720p)" enum:"480p,720p,1080p" default:"720p"`
    AspectRatio string `flag:"aspect-ratio,a" desc:"Aspect ratio: auto,21:9,16:9,4:3,1:1,3:4,9:16" enum:"auto,21:9,16:9,4:3,1:1,3:4,9:16" default:"auto"`
    Duration    string `flag:"duration"       desc:"Seconds: auto or 4-15 (default: auto)" default:"auto"`
    Audio       bool   `flag:"audio"          desc:"Generate synchronized audio" default:"true"`
    Bitrate     string `flag:"bitrate"        desc:"Bitrate mode: standard, high" enum:"standard,high" default:"standard"`
    Seed        int    `flag:"seed"           desc:"Random seed (omit for random)" default:"-1"`
    EndImage    string `flag:"end-image"      desc:"i2v only: end-frame image path"`
}
func (o *Options) RequestLabel() string  { return o.Model }
func (o *Options) ResolvedModel() string { return o.Model }
```

`-s` carries resolution and `-a` aspect-ratio — idiomatic shared flag names (gemini/vertex/openai already share `-m`/`-s`); the gate permits them because `fal` lists them in `SupportedFlags`. New private flags: `--duration`, `--audio`, `--bitrate`, `--seed`, `--end-image`. (`--seed -1` sentinel = unset.)

### 6.3 Generate flow

```
Generate(ctx, req):
  1. Resolve endpoint:
       tier   = "" | "fast/"            (from opts.Model)
       refs   = classify(req.References) by MIME prefix → images[], videos[], audios[]
       modality:
         0 refs                              → text-to-video
         1 image, no video/audio (+EndImage) → image-to-video
         else (≥2 images / any video|audio)  → reference-to-video
       url = "https://queue.fal.run/bytedance/seedance-2.0/" + tier + modality
  2. Validate r2v constraints (§3.7) for the reference-to-video path.
  3. Upload every local ref via storage.go → fal CDN URLs (incl. EndImage).
  4. Build JSON body (prompt + resolution/duration/aspect_ratio/generate_audio/
     bitrate_mode/seed + image_url|image_urls|video_urls|audio_urls|end_image_url).
  5. Submit → poll status_url until COMPLETED (ctx-aware) → GET response_url.
       on status.error → return APIError(error_type, error).
  6. GetBytes(video.url) → Response{ Assets: [{Data: mp4, MimeType: "video/mp4"}] }.
```

i2v vs r2v disambiguation: an i2v end-frame is the explicit `--end-image` flag, **not** a second `-i`. So `-i a.png --end-image b.png` → i2v; `-i a.png -i b.png` → r2v.

### 6.4 register.go (unchanged recipe)

`providers.Register("fal", Bundle{ Factory: New, BindFlags/ReadFlags/ParseOptions: flagspec closures, SupportedFlags: flagspec.FieldNames(Options{}), Examples, Info, ConfigSchema: {api_key → FAL_KEY, secret, required} })`. No `Vision` (fal isn't a describer here).

---

## 7. Cost guardrail

Video is expensive: a 10 s **1080p** clip ≈ **$6.82**; `-n` multiplies it. Two cheap guards:

- `Capabilities.MaxN` (added in §4) lets `fal` cap `-n` low (e.g. 4). `cli.Validate` clamps against it (0 = keep the global 1..20).
- Optional: a confirmation prompt above a $-threshold (P3 polish). Pricing for estimate: 720p $0.3034/s · 1080p $0.682/s · fast tiers ~$0.2419/s · r2v with video refs ×0.6.

---

## 8. Locked decisions

1. **Scope v1 = all three modalities** (t2v + i2v + r2v) → multi-modal references in scope.
2. **Reuse `-m`/`-s`/`-a`** for tier/resolution/aspect-ratio; add `--duration`/`--audio`/`--bitrate`/`--seed`/`--end-image`.
3. **fal CDN upload for every reference** — no base64 data URIs.
4. **Queue API** transport (submit → poll → fetch), behind the synchronous `Generate`.
5. **Tier-as-model, modality-from-refs** (no separate flag for t2v/i2v/r2v).

---

## 9. Open items to verify

- **`storage/upload/initiate` response field names** — implemented to `{file_url, upload_url}` per `fal-js` source; confirm against a live 200 before shipping (single integration smoke test).
- **r2v exact size/duration limits** — encode the documented caps (§3.7); leave resolution/duration-bound rejections to the API message.
- **Media expiration window** — we always download immediately, so this only matters if a run is paused; no code impact expected.

---

## 10. Build order

1. **Spine** (§4): `Assets`/`GeneratedAsset` rename + alias; `MediaKind`/`RefKinds`/`MaxN` on `Capabilities`; `ResolveFilename` ext param; `saveOne` image-only gating; transport `Key` auth + `GetJSON` + `GetBytes`. Run the existing provider suites green (proves image providers unaffected).
2. **Multi-modal refs** (§5): broaden `images` loader; provider-aware ref-kind check in `commands/validate.go`; clamp `-n` to `MaxN`.
3. **`fal` package** (§6): `storage.go` upload → t2v → i2v (`--end-image`) → r2v (buckets + §3.7 constraints) → queue submit/poll/fetch/cancel → download. `contract_test.go`. One line in `all.go`.
4. **Polish** (P3): kind-aware spinner + batch column; cost confirmation; `--embed-metadata` cleanly skips video.

---

## 11. Why the next video provider is free

After step 1, "video-ness" is two declarative `Capabilities` fields + the provider's own `Generate`. Adding Kling, Veo, or Runway is the **same four-step recipe** as any image provider in [adding-a-provider.md](adding-a-provider.md): create the package, set `MediaKind: KindVideo` (+ `RefKinds`), implement `Generate`, add one line to `all.go`, write the one-line contract test. No core edits. That is the "seamless as adding an image provider" bar, met.
