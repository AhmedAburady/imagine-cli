# Gemini and Vertex provider reference

Gemini (direct REST) and Vertex (Gemini via GCP) share the same model catalogue and flag set. The only difference is authentication. When a flag applies to both, the reference says "Gemini/Vertex".

## Models

Aliases resolve to canonical IDs. Omit `-m` to use the default.

| Alias | Canonical ID | Notes |
|---|---|---|
| `pro` | `gemini-3-pro-image-preview` | **Default.** Highest quality. Does NOT support `--thinking` or `--image-search`. |
| `flash` | `gemini-3.1-flash-image-preview` | Faster. Supports `--thinking` and `--image-search` (Gemini only). |

`-m` also accepts the full canonical ID directly.

## Flags

| Flag | Long | Values | Notes |
|---|---|---|---|
| `-m` | `--model` | `pro` / `flash` / full ID | Default `pro` |
| `-s` | `--size` | `1K`, `2K`, `4K` | Default `1K`. Not pixels — Gemini picks resolution within each tier. |
| `-a` | `--aspect-ratio` | e.g. `1:1`, `16:9`, `9:16`, `4:3`, `3:4`, `21:9` | Omit for auto |
| `-g` | `--grounding` | bool | Google Search grounding — pulls live web context into the prompt |
| `-t` | `--thinking` | `minimal` / `high` | **Flash only.** Higher thinking = better reasoning, more tokens |
| `-I` | `--image-search` | bool | **Gemini flash only** (Vertex does NOT support this). Image Search grounding. |

## Capability matrix

| Feature | Gemini | Vertex |
|---|---|---|
| Generate | ✅ | ✅ |
| Edit (single ref) | ✅ | ✅ |
| Edit (multiple refs) | ✅ | ✅ |
| Edit (folder of refs) | ✅ | ✅ |
| Grounding (`-g`) | ✅ | ✅ |
| Thinking (`-t`, flash) | ✅ | ✅ |
| Image Search (`-I`, flash) | ✅ | ❌ |
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
  - `1K` → ~1024px on the long edge
  - `2K` → ~2048px
  - `4K` → ~3840px

Exact dimensions are Gemini's choice — the API picks based on aspect ratio + size tier. Use OpenAI if you need deterministic pixel dimensions.

## Common pitfalls

- **`-t` on `-m pro` does nothing.** Thinking is flash-only. imagine accepts the flag but the pro model ignores it.
- **`-I` with Vertex errors out.** Vertex doesn't expose the image-search tool. imagine rejects the flag at validation time.
- **Grounding adds latency.** Expect 10–20% longer generation times with `-g`.
- **No streaming.** imagine always waits for the full image. Some Gemini tiers support streaming but imagine doesn't surface it.
