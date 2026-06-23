# Video generation — fal & modelark (Seedance 2.0)

Both providers generate video from `imagine -p "..."`. Output is `.mp4`. The **tier is the model**; the **modality is derived from references** (no `-i` = text-to-video, one `-i` = image-to-video, multiple = reference-to-video).

| | `fal` | `modelark` |
|---|---|---|
| Backend | fal.ai queue API | BytePlus ModelArk direct API |
| Key | `fal-…` (https://fal.ai/dashboard/keys) | `ark-…` (ModelArk console) |
| Reference upload | fal's own CDN — **no setup** | **your S3 bucket** ([storage.md](storage.md)) |
| Tiers (`-m`) | `seedance` (default), `fast` | `seedance` (full, default), `fast`, `mini` |
| Resolutions (`-s`) | `480p`/`720p`/`1080p` | `480p`/`720p`; `1080p`/`4k` **full only** |
| `-n` cap | 4 | 4 |

## Setup

```bash
imagine providers add fal --api-key fal-XXX
imagine providers add modelark --api-key ark-XXX
# modelark only: configure a public-read S3 bucket once (see storage.md)
imagine storage set --endpoint https://… --bucket … --access-key … --secret-key …
imagine storage test
```

## Flags

| Flag | Long | fal | modelark | Notes |
|---|---|---|---|---|
| `-m` | `--model` | `seedance`/`fast` | `seedance`/`fast`/`mini` | tier; default `seedance` |
| `-s` | `--size` | `480p`/`720p`/`1080p` | `480p`/`720p`/`1080p`/`4k` | default `720p`; modelark `1080p`/`4k` full-only |
| `-a` | `--aspect-ratio` | `auto`,21:9,16:9,4:3,1:1,3:4,9:16 | `adaptive`,21:9,16:9,4:3,1:1,3:4,9:16 | default auto/adaptive |
|  | `--duration` | `auto` or `4`–`15` | `-1` (auto) or `4`–`15` | seconds |
|  | `--audio` | bool | bool | default `true` |
|  | `--end-image` | path | path (**not on mini**) | i2v end-frame; single `-i` only |
|  | `--bitrate` | `standard`/`high` | — | fal only |
|  | `--seed` | int | — | fal only |

## Modalities

| References | Modality | Notes |
|---|---|---|
| none | text → video | `--end-image` without `-i` is an error |
| one image | image → video | first frame; add `--end-image` for first+last frame |
| ≥2 refs, or any video/audio ref | reference → video | `--end-image` invalid here |

`-i a.png --end-image b.png` = first+last frame. `-i a.png -i b.png` = reference-to-video. Reference inputs accept `.png/.jpg/.jpeg/.gif/.webp` (image) and `.mp4/.mov/.mp3/.wav` (video/audio). modelark caps: ≤9 images (≤30 MB each), ≤3 videos (≤50 MB each), ≤3 audio (≤15 MB each); audio needs at least one image/video.

## Examples

```bash
imagine -p "a fox leaping through tall grass" --provider fal
imagine -p "slow zoom on the skyline" --provider modelark -i frame.png -s 1080p
imagine -p "pan to the right" --provider fal -i start.png --end-image end.png
imagine -p "match this look" --provider modelark -i ref1.png -i clip.mp4
imagine -p "logo intro" --provider fal -n 2 -f intro.mp4     # 2 parallel clips
```

## Batch

```yaml
intro:
  prompt: "neon city flythrough"
  provider: fal
  size: 1080p
  filename: intro.mp4
product:
  prompt: "rotating product shot"
  provider: modelark        # entry with input: needs storage configured
  input: ref.png
  filename: product.mp4
```

## Common pitfalls

- **`provider "modelark" needs S3-compatible storage …`** — modelark got `-i` with no `storage:` configured. Run `imagine storage set` (see [storage.md](storage.md)). Text-to-video isn't gated.
- **`tls: handshake failure`** uploading to a MinIO/RustFS bucket — set `path_style: true` in the `storage:` section.
- **`resolution 1080p exceeds the maximum 720p for model …`** — `1080p`/`4k` are modelark full-model only; use `-m seedance` or drop to `720p`.
- **`--end-image … not supported on model …mini…`** — mini doesn't do first+last frame; use `seedance`/`fast`.
- **`--duration must be -1 (auto) or between 4 and 15`** (modelark) — duration is an integer; `-1` means auto.
- **face rejection** — Seedance rejects references containing real human faces (server-side, surfaced in the error message).
- **`4k` won't play** — it's H.265/10-bit; some players/browsers can't decode it directly.
