# Gemini and Vertex provider reference

Gemini (direct REST) and Vertex (Gemini via GCP) share the same model catalogue and flag set. The only difference is authentication. When a flag applies to both, the reference says "Gemini/Vertex".

## Models

Aliases resolve to canonical IDs. Omit `-m` to use the default.

| Alias | Canonical ID | Notes |
|---|---|---|
| `pro` | `gemini-3-pro-image` | **Default.** Highest quality. Does NOT support `--thinking` or `--image-search`. |
| `flash` | `gemini-3.1-flash-image` | Faster. Supports `--thinking` and `--image-search` (Gemini only). |
| `flash-lite`, `lite` | `gemini-3.1-flash-lite-image` | Fastest and cheapest (Nano Banana 2 Lite). **1K only** and no `--grounding` / `--image-search`. Supports `--thinking`. |

`-m` also accepts the full canonical ID directly.

Google renamed `pro` and `flash` at GA by dropping the `-preview` suffix. The retired spellings `gemini-3-pro-image-preview` and `gemini-3.1-flash-image-preview` still resolve to the GA IDs, so pinned scripts and batch files keep working — but they aren't advertised in `--help`, and the results table reports the GA ID that actually ran. Prefer the aliases (`pro`, `flash`) or the GA IDs in new work.

## Flags

| Flag | Long | Values | Notes |
|---|---|---|---|
| `-m` | `--model` | `pro` / `flash` / `flash-lite` / full ID | Default `pro` |
| `-s` | `--size` | `512`, `1K`, `2K`, `4K` | Default `1K`. Not pixels; Gemini picks resolution within each tier. **`512` is `flash` only; `flash-lite` accepts `1K` only.** |
| `-a` | `--aspect-ratio` | 14 values: `1:1`, `1:4`, `1:8`, `2:3`, `3:2`, `3:4`, `4:1`, `4:3`, `4:5`, `5:4`, `8:1`, `9:16`, `16:9`, `21:9` | Omit for auto. All three models accept the full set; anything else is rejected locally. |
| `-g` | `--grounding` | bool | Google Search grounding, pulling live web context into the prompt. **Not on `flash-lite`.** |
| `-t` | `--thinking` | `minimal` / `high` | **`flash` and `flash-lite` only** — pro rejects it. Higher thinking = better reasoning, more tokens |
| `-I` | `--image-search` | bool | **Gemini `flash` only** (Vertex does NOT support this). Image Search grounding. |

## Capability matrix

| Feature | Gemini | Vertex |
|---|---|---|
| Generate | ✅ | ✅ |
| Edit (single ref) | ✅ | ✅ |
| Edit (multiple refs) | ✅ | ✅ |
| Edit (folder of refs) | ✅ | ✅ |
| Grounding (`-g`, pro/flash) | ✅ | ✅ |
| Thinking (`-t`, flash/flash-lite) | ✅ | ✅ |
| Image Search (`-I`, flash) | ✅ | ❌ |
| `512` size (flash) | ✅ | ✅ |
| `flash-lite` model | ✅ | ✅ |
| MaxBatchN (images per API call) | 1 | 1 |

Because `MaxBatchN=1`, imagine's orchestrator issues `-n` parallel API calls — not one batched call. That's fine for small batches but adds latency for `-n 10+`.

## Examples

```bash
# Basic
imagine -p "a sunset over mountains"

# Multiple images, 2K size, widescreen
imagine -p "a cityscape" -n 3 -s 2K -a 16:9 -o ./city

# Flash with high thinking
imagine -p "intricate diagram of a watch mechanism" -m flash -t high

# Flash-lite: fastest and cheapest, 1K only
imagine -p "die-cut sticker of an avocado" -m flash-lite -a 1:1

# Grounding (adds live web context)
imagine -p "the latest design trends in 2026" -g

# Image search (Gemini flash only)
imagine -p "cat wearing a Supreme hoodie" -m flash -I

# Edit, keep the input filename
imagine -p "convert to watercolor" -i photo.png -r

# Edit with multiple references
imagine -p "blend these styles" -i refA.png -i refB.png -n 4

# Vertex (same flags, different auth)
imagine -p "a cat" --provider vertex -n 3
```

## Output

- **Format:** PNG (Gemini-native). If `-f` ends in `.jpg`/`.jpeg`, imagine converts locally at quality 95 (orchestrator-side, not API-side).
- **Resolution:** determined by `-s` tier and `-a`. Approximate:
  - `512` → ~512px on the long edge (`flash` only)
  - `1K` → ~1024px on the long edge
  - `2K` → ~2048px
  - `4K` → ~3840px

Exact dimensions are Gemini's choice — the API picks based on aspect ratio + size tier. Use OpenAI if you need deterministic pixel dimensions.

## Common pitfalls

- **`-t` on `-m pro` errors out.** Thinking is gated to `flash` and `flash-lite`; imagine rejects the flag at validation time rather than sending it.
- **Any `-s` but `1K` on `-m flash-lite` errors out.** That model renders 1K only. Drop `-s` (1K is the default) or switch to `flash`.
- **`-s 512` on `-m pro` errors out.** Only `flash` documents the 512 tier.
- **`-g` on `-m flash-lite` errors out** on both providers. Note the Vertex `flash` model card lists grounding as unsupported, but imagine still sends it: grounding leaves no visible trace in the output, so blocking it locally would remove a capability with no way to find out the card was wrong.
- **`-I` with Vertex errors out.** Vertex doesn't expose the image-search tool. imagine rejects the flag at validation time.
- **Grounding adds latency.** Expect 10–20% longer generation times with `-g`.
- **No streaming.** imagine always waits for the full image. Some Gemini tiers support streaming but imagine doesn't surface it.
- **`9:21` is not a valid ratio on either provider**, despite both Vertex model cards listing it - Vertex returns 400 INVALID_ARGUMENT (verified 2026-08-07), same as the direct Gemini API. Use `1:8` or `1:4` for tall formats.

## Aspect ratio reference

All 14 are accepted by `pro`, `flash`, and `flash-lite`. Dimensions shown for the `1K` tier:

| Ratio | 1K dimensions | Shape |
|---|---|---|
| `1:1` | 1024x1024 | square |
| `3:2` | 1264x848 | landscape |
| `4:3` | 1200x896 | landscape |
| `5:4` | 1152x928 | landscape |
| `16:9` | 1376x768 | widescreen |
| `21:9` | 1584x672 | ultrawide |
| `4:1` | 2048x512 | banner |
| `8:1` | 3072x384 | ultra-wide banner |
| `2:3` | 848x1264 | portrait |
| `3:4` | 896x1200 | portrait |
| `4:5` | 928x1152 | portrait |
| `9:16` | 768x1376 | vertical / story |
| `1:4` | 512x2048 | tall |
| `1:8` | 384x3072 | ultra-tall |

`2K` doubles each dimension and `4K` quadruples them, so `pro` and `flash` reach 12288x1536 at `8:1 4K`. `flash-lite` is 1K only — the middle column is its whole range, and it rounds the extreme ratios slightly differently (observed `8:1` → 2928x352, `4:1` → 2064x512). Treat the table as the target, not a guarantee: exact pixels are always the model's choice.
