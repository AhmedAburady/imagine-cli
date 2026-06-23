# modelark — BytePlus ModelArk video provider

> Scope: `providers/modelark/`, a video provider wrapping BytePlus ModelArk's **Dreamina Seedance 2.0** family via the direct ModelArk REST API. First consumer of the shared [storage brick](storage.md). Coexists with the [`fal`](video-providers.md) provider (also Seedance 2.0, via fal.ai's queue + CDN).

---

## Table of contents

- [1. Overview](#1-overview)
- [2. Models & tiers](#2-models--tiers)
- [3. Flags](#3-flags)
- [4. The ModelArk API](#4-the-modelark-api)
- [5. Generate flow](#5-generate-flow)
- [6. Per-model validation](#6-per-model-validation)
- [7. Coexistence with fal](#7-coexistence-with-fal)
- [8. Verified facts](#8-verified-facts)

---

## 1. Overview

A normal `providers.Provider`: tier-as-model, modality-derived-from-references (mirroring imagine's "`-i` flips to edit mode" philosophy). Output is `video/mp4` bytes. Local references are published as public URLs through the [storage brick](storage.md), so the provider declares `RequireStorage: true`.

```
providers/modelark/
  modelark.go       Provider: New, Info, Generate (classify → upload → create → poll → download), poll, cancel, validateReferenceConstraints
  options.go        tagged Options + Validate(info) + RequestLabel/ResolvedModel
  register.go       providers.Register("modelark", …) with RequireStorage: true
  help.go           Examples()
  contract_test.go  providertest.Contract(t, "modelark")
```

Plus one blank-import line in `providers/all/all.go`. No edits to `commands/`, `cli/`, `api/`, `config/`, `providers/gate.go`, or `cmd/imagine/main.go` — the storage brick + `RequireStorage` capability were the only framework additions, and they're generic.

Auth: `Authorization: Bearer <ARK_API_KEY>`, stored as `providers.modelark.api_key` (supports `${ENV}` / `op://`).

---

## 2. Models & tiers

`DefaultModel: dreamina-seedance-2-0-260128`. Tier is the "model"; `-m` accepts an alias or the full ID.

| Alias | Canonical ID | Max resolution | First+last frame |
|---|---|---|---|
| `seedance`, `seedance-2` | `dreamina-seedance-2-0-260128` | 480p / 720p / 1080p / **4k** | yes |
| `seedance-fast`, `fast` | `dreamina-seedance-2-0-fast-260128` | 480p / 720p | yes |
| `seedance-mini`, `mini` | `dreamina-seedance-2-0-mini-260615` | 480p / 720p | **no** |

`Capabilities`: `MediaKind: KindVideo`, `RefKinds: {image, video, audio}`, `Edit: true`, `MaxBatchN: 1` (so `-n>1` fans out N parallel tasks), `MaxN: 4` (cost guard). Endpoint IDs (`ep-…`) are not accepted via `-m` — only the three model IDs/aliases (matches fal; `enum:"@models"`).

---

## 3. Flags

| Flag | Long | Values | Default |
|---|---|---|---|
| `-m` | `--model` | `seedance` / `fast` / `mini` (or full ID) | `seedance` |
| `-s` | `--size` | `480p` / `720p` / `1080p` / `4k` | `720p` |
| `-a` | `--aspect-ratio` | `adaptive` / `21:9` / `16:9` / `4:3` / `1:1` / `3:4` / `9:16` | `adaptive` |
|  | `--duration` | `-1` (auto) or `4`–`15` seconds | `-1` |
|  | `--audio` | generate synchronized audio (bool) | `true` |
|  | `--end-image` | i2v end-frame image path (first+last frame; **not on mini**) | — |

`-m`/`-s`/`-a` are shared flag names (idempotent bind alongside fal/gemini/openai); the gate permits them because modelark lists them in `SupportedFlags`. `1080p`/`4k` are **full-model only** (fast/mini cap at 720p); `4k` is H.265/10-bit and may not play in every player/browser.

---

## 4. The ModelArk API

Base URL: `https://ark.ap-southeast.bytepluses.com/api/v3`.

```
POST   /contents/generations/tasks        create a task   → {"id":"cgt-…"}
GET    /contents/generations/tasks/{id}    poll status
DELETE /contents/generations/tasks/{id}    cancel (queued tasks only)
```

The request uses a `content[]` array of typed items (the wire types in `modelark.go`):

```jsonc
{
  "model": "dreamina-seedance-2-0-260128",
  "content": [
    { "type": "text", "text": "a fox leaping through tall grass" },
    { "type": "image_url", "image_url": { "url": "https://…/first.png" }, "role": "first_frame" }
  ],
  "generate_audio": true,     // sent explicitly — the API default is true
  "resolution": "720p",
  "ratio": "adaptive",
  "duration": -1,             // -1 = auto; NOT omitempty (omitempty would drop -1)
  "watermark": false
}
```

Content item types: `text`, `image_url`, `video_url`, `audio_url`. Roles: `first_frame`, `last_frame`, `reference_image`, `reference_video`, `reference_audio`.

Poll status enum: `queued` | `running` | `succeeded` | `failed` | `cancelled` | `expired`. On success, `content.video_url` is the output (URLs expire in ~24h — download immediately). On failure, `error{code,message}` is surfaced verbatim (the transport layer already extracts `error.message`).

`video_url` accepts **public URLs / asset IDs only — no Base64**. That is *why* the storage brick is required for references.

---

## 5. Generate flow

```
Generate(ctx, req):
  1. Classify req.References by images.KindOf → image / video / audio buckets.
  2. If any references: storage.Get(ctx) ONCE (resolve credentials once per generation).
  3. buildContent — determine the mutually-exclusive modality:
       0 refs                         → text. (--end-image without -i errors.)
       1 image only                   → first_frame (+ last_frame if --end-image).
       else                           → multimodal reference_image/video/audio. (--end-image invalid.)
     validateReferenceConstraints runs BEFORE any upload (cheap fail on local bytes).
     Each local ref → storage.UploadWith(ctx, sc, data, mime) → public URL → content item.
  4. POST …/tasks with Bearer auth → task id.
  5. Poll …/tasks/{id} every 5s:
       succeeded → content.video_url
       failed/expired/cancelled → surface error.code + error.message
       queued/running → keep polling
       any other/empty status → terminal error (no infinite hang)
       ctx.Done() → best-effort DELETE (queued tasks only), return ctx.Err()
  6. Download video_url (anonymous GET) → Response{Assets: [{Data, MimeType: "video/mp4"}]}.
```

Reference caps (enforced client-side before upload; count + size only — the API additionally enforces total-duration/pixel bounds server-side): images ≤9 (≤30 MB each), videos ≤3 (≤50 MB each), audio ≤3 (≤15 MB each); audio cannot be sent without at least one image or video.

i2v vs reference disambiguation: a single `-i` is first-frame; an end-frame is the explicit `--end-image` flag, not a second `-i`. `-i a.png --end-image b.png` → first+last frame; `-i a.png -i b.png` → multimodal reference.

---

## 6. Per-model validation

`Options.Validate` (called by flagspec after population, when `Model` is already the canonical ID) enforces what the flat enum/range tags can't:

- `duration` must be `-1` or `4..15` (the `range:"-1:15"` tag also admits 0..3, so `Validate` tightens it).
- resolution must not exceed the model's max (`1080p`/`4k` are full-only).
- `--end-image` (first+last frame) is unsupported on mini.

Tier limits come from an explicit `modelTiers` table keyed by **canonical ID** (max-resolution rank + first/last-frame bool), not from substring-matching the ID — a renamed ID can't silently misclassify a tier. An unrecognised ID (e.g. a newly shipped tier not yet in the table) skips per-model checks and defers to the server.

---

## 7. Coexistence with fal

Both providers wrap Seedance 2.0 with no conflict:

| | `fal` | `modelark` |
|---|---|---|
| Backend | fal.ai queue API | BytePlus ModelArk direct API |
| Reference upload | fal's own CDN (`RequireStorage: false`) | shared storage brick (`RequireStorage: true`) |
| Model IDs | `bytedance/seedance-2.0[/fast]` | `dreamina-seedance-2-0[-fast/-mini]-…` |
| Tiers | normal, fast | full, fast, mini |
| Extra flags | `--bitrate`, `--seed` | (none beyond shared + `--duration`/`--audio`/`--end-image`) |

Shared flags (`-m`/`-s`/`-a`) register idempotently; the active provider's help text wins. Batch files freely mix `provider: fal` and `provider: modelark`. `--end-image` exists on both.

---

## 8. Verified facts

Confirmed against official BytePlus docs (scrapes under `.firecrawl/`):

- Base URL, endpoints, `content[]` type+role model, `Authorization: Bearer`, status enum, `content.video_url`, `error{code,message}` — [ModelArk API reference](https://docs.byteplus.com/en/docs/ModelArk).
- Model IDs `dreamina-seedance-2-0-260128` / `-fast-260128` / `-mini-260615`; Mini API access from 2026-06-22.
- Resolution caps: full → 480p/720p/1080p/4k (4k = 10-bit H.265, lower rate limits); Fast/Mini → 480p/720p.
- `generate_audio` defaults to `true`; `duration` int `[4,15]` or `-1`; `frames`/`camera_fixed` unsupported on 2.0.
- Reference URLs are fetched server-side and accept URLs/asset-IDs only (no Base64); an unreadable bucket yields `OperationDenied.TosAccessDenied` — exactly what `imagine storage test`'s public-read check prevents.
- Seedance rejects references containing real human faces (server-side, HTTP 400) — surfaced via `error.message`.
