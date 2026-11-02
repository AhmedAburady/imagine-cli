# OpenAI provider reference

## Auth methods

The `openai` provider authenticates one of two mutually-exclusive ways, set by `auth_method` in config and chosen at `providers add` time. Models, flags, sizes, and output formats below are **identical** for both — only billing, the endpoint, and onboarding differ.

| | `api_key` (default) | `subscription` |
|---|---|---|
| Billed to | OpenAI Platform (pay-as-you-go) | the user's ChatGPT plan (Free, Go, Plus, Pro, Business, Enterprise, Edu; the language model is picked by plan) |
| Credential | `sk-…` in `config.yaml` (or an `op://` / `${VAR}` ref) | OAuth token cached in `~/.config/imagine/openai-subscription-auth.json` (`0600`), refreshed silently |
| Endpoint | `api.openai.com/v1/images` (generate = JSON, edit = multipart), `Authorization: Bearer <api_key>` | OpenAI's ChatGPT/Codex Responses backend (SSE) — an unpublished endpoint, treat as best-effort |
| Set up | `imagine providers add openai --api-key sk-…` (headless-OK) | `imagine providers add openai login` (**opens a browser** — needs a human; do NOT run headless) |

Config stanza for each:

```yaml
openai:
  auth_method: api_key       # api_key route
  api_key: sk-...
# --- or ---
openai:
  auth_method: subscription  # subscription route; tokens live in the separate 0600 file, not here
```

`auth_method` is inferred from the presence of `api_key` when omitted, so older API-key configs keep working unchanged. Switching methods = re-run `imagine providers add openai` (or edit `auth_method:`). Only the active method's credential is read, so an unused `op://` key in the stanza is never fetched.

**Agent note:** never run `imagine providers add openai login` or the bare `imagine providers add openai` picker in a non-interactive context — both block on a human (browser callback / TTY prompt). For subscription auth, instruct the user to run the `login` command themselves; then generate normally.

## Models

Aliases resolve to canonical IDs. Omit `-m` to use the default.

| Alias | Canonical ID | Notes |
|---|---|---|
| `flare`, `2.5` | `gpt-image-2.5-flare` | Default. Fast, high-quality everyday generation and editing. |
| `sunburst` | `gpt-image-2.5-sunburst` | Most capable; tighter editing precision, longer generation times. |
| `image2`, `2` | `gpt-image-2` | Previous flagship. |

`-m` also accepts the full canonical ID directly, and the dated snapshots `gpt-image-2.5-flare-2026-09-08`, `gpt-image-2.5-sunburst-2026-09-08` and `gpt-image-2-2026-04-21`. A snapshot is sent as written, so the run stays pinned to that version, and it follows the same rules as its base model.

## Flags (OpenAI-private — rejected by Gemini/Vertex)

| Flag | Long | Values | Default |
|---|---|---|---|
| `-m` | `--model` | See models above | `flare` (`gpt-image-2.5-flare`) |
| `-s` | `--size` | Shorthand / raw / auto — see size matrix below | `auto` |
| `-q` | `--quality` | `low`, `medium`, `high`, `auto`; `xhigh`, `max` on the 2.5 models only | `auto` |
| | `--compression` | 0-100 integer (jpeg/webp only) | `100` |
| | `--moderation` | `auto`, `low` | `auto` |
| | `--background` | `auto`, `opaque`, `transparent` | `auto`. `transparent` needs a png or webp output (`-f` not `.jpg`); preview on `image2` |

## Size matrix

### Shorthand

| `-s` value | Maps to |
|---|---|
| `1K` | `1024x1024` |
| `2K` | `2048x2048` |
| `4K` | `3840x2160` |
| `auto` | Model picks (default) |

### Popular raw dimensions

Pass any of these directly to `-s`:

| Dimensions | Shape |
|---|---|
| `1024x1024` | Square |
| `1536x1024` | Landscape |
| `1024x1536` | Portrait |
| `2048x2048` | 2K square |
| `2048x1152` | 2K landscape |
| `3840x2160` | 4K landscape |
| `2160x3840` | 4K portrait |

### Custom `WxH` constraints

ANY `WxH` is accepted if it satisfies all of these:

- Each edge ≤ 3840 pixels
- Both edges are multiples of 16
- Long-edge / short-edge ratio ≤ 3:1
- Total pixel count between 655,360 and 8,294,400

imagine enforces all four constraints client-side at flag-parse time, so a bad size errors before any API call, in edit mode (`-i`) too.

**Subscription route caveat** (probed 2026-11-02): the request goes to a language model (`gpt-6.1-sol`; `gpt-6-luna` on Free and Go plans), which calls the image tool. The backend resets the tool to its own model, `gpt-image-2-codex`, for any accepted `-m` model or snapshot, and resets `size` to `auto`, so the language model picks the size: usually 1254x1254. `-q` is passed through. Use the API-key route when the exact model or size matters.

## Output format

Driven by `-f`'s extension:

| `-f` ext | API returns | Local conversion |
|---|---|---|
| `.png` (default) | PNG | none |
| `.jpg` / `.jpeg` | JPEG | none (API encodes) |
| `.webp` | WebP | none |
| anything else | PNG | none |

This is a win over Gemini — for JPEG, OpenAI's API encodes server-side, avoiding a local re-encode round-trip.

## Quality

Per OpenAI's docs:
- `low` — fast drafts, thumbnails, iteration. Fewest tokens.
- `medium` — balanced.
- `high` — final assets. Most tokens.
- `auto` — model picks based on prompt (default).

Cost scales with quality + size. `low` quality on `1024x1024` is the cheapest point.

## Batching

**API-key route:** `MaxBatchN = 10`. imagine's orchestrator batches `-n` into single API calls up to 10 images each:

- `-n 3` → 1 API call returning 3 images
- `-n 15` → 2 API calls (10 + 5) in parallel

This is faster and cheaper than Gemini's 1-per-call pattern for multi-image runs.

**Subscription route:** `MaxBatchN = 1` (the Responses image tool yields one image per call), so `-n N` issues N parallel calls — same result, more requests. Generation takes longer per image than the API-key route.

## Examples

```bash
# Fast draft
imagine -p "a red apple" --provider openai -q low

# Batched — one API call returns 3 images
imagine -p "logo variants" --provider openai -n 3

# 4K landscape, high quality, JPEG
imagine -p "hero banner" --provider openai -s 3840x2160 -q high -f hero.jpg

# Custom aspect dimensions
imagine -p "movie poster" --provider openai -s 1024x1536

# Edit with a reference
imagine -p "make it winter" --provider openai -i photo.png

# Multi-reference edit (up to 16 refs per API call)
imagine -p "gift basket with these items" \
  --provider openai \
  -i lotion.png -i candle.png -i soap.png -i bath-bomb.png

# JPEG with compression
imagine -p "thumbnail" --provider openai -f thumb.jpg --compression 70

# Less restrictive moderation (if default rejects legitimate prompts)
imagine -p "medical illustration of a heart" --provider openai --moderation low
```

## Common pitfalls

- **`-g` / `-t` / `--image-search` are NOT valid for OpenAI.** They're Gemini/Vertex-only. imagine rejects them with a clear error.
- **`-a` (aspect-ratio) is NOT valid for OpenAI.** OpenAI uses explicit `-s WxH` dimensions. Use `-s 1536x1024` instead of `-a 16:9`.
- **Sizes outside the envelope are rejected before the call.** `-s 1000x1000` errors (not a multiple of 16), as does anything past the edge, ratio or pixel limits.
- **Org verification.** OpenAI requires organization verification for GPT Image models. API returns 403 if unverified.
