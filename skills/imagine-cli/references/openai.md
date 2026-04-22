# OpenAI provider reference

Uses OpenAI's `/v1/images` endpoints. Generate is JSON-bodied; edit is multipart/form-data. Both authenticate with `Authorization: Bearer <api_key>`.

## Models

Aliases resolve to canonical IDs. Omit `-m` to use the default.

| Alias | Canonical ID | Notes |
|---|---|---|
| `2` | `gpt-image-2` | **Default.** Flagship — flexible sizes, high-fidelity inputs. |
| `1.5` | `gpt-image-1.5` | Previous flagship. Use when `gpt-image-2` rejects a feature (e.g. transparent backgrounds). |
| `1` | `gpt-image-1` | First generation. |
| `mini` / `1-mini` | `gpt-image-1-mini` | Fastest, cheapest. |
| `latest` | `chatgpt-image-latest` | ChatGPT-variant latest. |

`-m` also accepts the full canonical ID directly.

## Flags (OpenAI-private — rejected by Gemini/Vertex)

| Flag | Long | Values | Default |
|---|---|---|---|
| `-m` | `--model` | See models above | `gpt-image-2` |
| `-s` | `--size` | Shorthand / raw / auto — see size matrix below | `auto` |
| `-q` | `--quality` | `low`, `medium`, `high`, `auto` | `auto` |
| | `--compression` | 0-100 integer (jpeg/webp only) | `100` |
| | `--moderation` | `auto`, `low` | `auto` |
| | `--background` | `auto`, `opaque`, `transparent` | `auto` |

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

The API validates server-side; imagine pattern-checks the format client-side but lets the server enforce the constraints.

### Edit-mode size restriction

When `-i` is passed (edit mode), `/v1/images/edits` only accepts: `1024x1024`, `1536x1024`, `1024x1536`, `auto`. Using `-s 2K`/`4K`/larger `WxH` in edit mode errors **before** the API call — imagine checks this client-side.

## Output format

Driven by `-f`'s extension:

| `-f` ext | API returns | Local conversion |
|---|---|---|
| `.png` (default) | PNG | none |
| `.jpg` / `.jpeg` | JPEG | none (API encodes) |
| `.webp` | WebP | none |
| anything else | PNG | none |

This is a win over Gemini — for JPEG, OpenAI's API encodes server-side, avoiding a local re-encode round-trip.

## Transparent backgrounds

`--background transparent` requires:
- Output format is `png` or `webp` (NOT `jpeg`)
- Model is NOT `gpt-image-2` (docs say it doesn't support transparency yet — use `-m 1.5` instead)

imagine rejects both misconfigurations at validation time with a clear error.

## Quality

Per OpenAI's docs:
- `low` — fast drafts, thumbnails, iteration. Fewest tokens.
- `medium` — balanced.
- `high` — final assets. Most tokens.
- `auto` — model picks based on prompt (default).

Cost scales with quality + size. `low` quality on `1024x1024` is the cheapest point.

## Batching

`MaxBatchN = 10`. imagine's orchestrator batches `-n` into single API calls up to 10 images each:

- `-n 3` → 1 API call returning 3 images
- `-n 15` → 2 API calls (10 + 5) in parallel

This is faster and cheaper than Gemini's 1-per-call pattern for multi-image runs.

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

# Transparent sticker (1.5 only — gpt-image-2 doesn't support transparency)
imagine -p "sticker logo" --provider openai -m 1.5 --background transparent -f sticker.png

# JPEG with compression
imagine -p "thumbnail" --provider openai -f thumb.jpg --compression 70

# Less restrictive moderation (if default rejects legitimate prompts)
imagine -p "medical illustration of a heart" --provider openai --moderation low
```

## Common pitfalls

- **`-g` / `-t` / `--image-search` are NOT valid for OpenAI.** They're Gemini/Vertex-only. imagine rejects them with a clear error.
- **`-a` (aspect-ratio) is NOT valid for OpenAI.** OpenAI uses explicit `-s WxH` dimensions. Use `-s 1536x1024` instead of `-a 16:9`.
- **Edit mode rejects large sizes.** `-i photo.png -s 4K` errors out — use `-s 1024x1024` / `1536x1024` / `1024x1536` / `auto` for edits.
- **`--background transparent` + `gpt-image-2`** → errors. Use `-m 1.5`.
- **Org verification.** OpenAI requires organization verification for GPT Image models. API returns 403 if unverified.
