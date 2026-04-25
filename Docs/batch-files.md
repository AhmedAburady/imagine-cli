# Batch files

Run multiple `imagine` jobs from a single file. One `-p` invocation; many images, many providers, all in parallel.

---

## Table of contents

- [When to use](#when-to-use)
- [Quick start](#quick-start)
- [How `-p` decides](#how--p-decides)
- [The schema rule](#the-schema-rule)
- [Common keys (every entry)](#common-keys-every-entry)
- [Provider-private keys](#provider-private-keys)
  - [Gemini](#gemini)
  - [Vertex](#vertex)
  - [OpenAI](#openai)
- [Map form vs list form](#map-form-vs-list-form)
- [Filename behavior](#filename-behavior)
- [How CLI flags interact with batch files](#how-cli-flags-interact-with-batch-files)
- [Validation](#validation)
- [Parallelism](#parallelism)
- [Output](#output)
- [Complete examples](#complete-examples)
- [Errors and fixes](#errors-and-fixes)

---

## When to use

Reach for a batch file when:

- You want to generate many images with **different prompts** in one run.
- You want to mix **different providers** (`openai` for one, `gemini` for another) in one run.
- You want to script reproducible image sets — under version control, replayable.
- A single CLI invocation would need 10 different flag combinations.

Single-shot `imagine -p "a sunset"` is still the right tool for one-off generation. Batch is the tool for everything else.

---

## Quick start

```yaml
# batch.yaml
hero_shot:
  prompt: "A samurai at dusk, cinematic"
  provider: openai
  size: 1024x1024

product_photo:
  prompt: "Studio product photo, soft rim lighting"
  provider: gemini
  size: 2K
  count: 3
```

```bash
imagine -p batch.yaml -o ./out
```

Output:

```
╭───────────────┬──────────┬────────────────────────────────┬────────┬───────┬────────╮
│ ENTRY         │ PROVIDER │ MODEL                          │ IMAGES │ TIME  │ STATUS │
├───────────────┼──────────┼────────────────────────────────┼────────┼───────┼────────┤
│ hero_shot     │ openai   │ gpt-image-2                    │ 1/1    │ 14.2s │ ok     │
│ product_photo │ gemini   │ gemini-3-pro-image-preview     │ 3/3    │ 18.7s │ ok     │
╰───────────────┴──────────┴────────────────────────────────┴────────┴───────┴────────╯

Done: 4 success, 0 failed across 2 entries (18.7s)

Output: /abs/path/out
```

Files written: `out/hero_shot.png`, `out/product_photo_1.png`, `out/product_photo_2.png`, `out/product_photo_3.png`.

---

## How `-p` decides

`imagine` looks at the file extension after expanding `~`:

| Extension | Treated as |
|---|---|
| `.yaml` / `.yml` | YAML batch file |
| `.json` | JSON batch file |
| anything else (`.txt`, no extension, etc.) | plain prompt file — contents become the prompt text |
| value isn't a path | literal prompt text |

So `-p ./prompts/hero.yaml` runs a batch; `-p ./prompts/hero.txt` reads the file as one prompt; `-p "a sunset"` is literal.

---

## The schema rule

**Every key inside an entry is the long name of a CLI flag.** That's the whole rule. If `--quality` works on the command line, `quality:` works in YAML. If `--aspect-ratio` works on the command line, `aspect-ratio:` works in YAML.

This means you only have to learn the schema once: it's the same as `imagine --help`.

```yaml
hero:
  prompt: "..."          # = -p / --prompt
  provider: openai       # = --provider
  size: 1024x1024        # = -s / --size
  quality: high          # = -q / --quality
  count: 3               # = -n / --count
  output: ./out          # = -o / --output
  filename: hero.jpg     # = -f / --filename
  input: photo.png       # = -i / --input
```

Unknown keys cause a validation error listing the valid keys for that entry's provider.

---

## Common keys (every entry)

These work for any provider.

| Key | Type | Required | Notes |
|---|---|---|---|
| `prompt` | string | yes | The text prompt. Multi-line OK with YAML's `\|` block scalar. |
| `provider` | string | no | `openai`, `gemini`, or `vertex`. Falls back to `--provider` then config default. |
| `output` | string | no | Output folder. `~` expanded. Defaults to CLI `-o` (or `.`). |
| `filename` | string | no | Output filename. Extension picks format (`.png`, `.jpg`, `.webp`). Mutually exclusive with `replace`. |
| `count` | int | no | 1–20. Defaults to CLI `-n` (or 1). With `count > 1`, filenames get `_1`, `_2`, … suffixes. |
| `input` | string OR list of strings | no | Reference image file or folder, or list of them. Flips the entry into edit mode. `~` expanded. |
| `replace` | bool | no | Use input filename as output filename. Requires exactly one `input:` pointing at a single file. **Mutually exclusive with `filename`**. |

`input` accepts a single string for one file/folder, or a YAML/JSON list for multiple:

```yaml
single_ref:
  prompt: "..."
  input: photo.png

multi_ref:
  prompt: "..."
  input:
    - lotion.png
    - candle.png
    - soap.png

folder_ref:
  prompt: "..."
  input: ~/photos/        # all supported images recursively
```

---

## Provider-private keys

Each provider has its own set. Setting a key for a provider that doesn't claim it produces a clear error.

### Gemini

| Key | Type | Default | Notes |
|---|---|---|---|
| `model` | string | `pro` | `pro`, `flash`, or full canonical ID |
| `size` | string | `1K` | `1K` / `2K` / `4K` only |
| `aspect-ratio` | string | (model picks) | e.g. `16:9`, `4:3`, `1:1` |
| `grounding` | bool | `false` | Enable Google Search grounding |
| `thinking` | string | (off) | `minimal` or `high`. **Flash only** — pro rejects |
| `image-search` | bool | `false` | **Flash only** — pro rejects. Vertex doesn't expose this flag |

### Vertex

| Key | Type | Default | Notes |
|---|---|---|---|
| `model` | string | `pro` | `pro`, `flash`, or full canonical ID |
| `size` | string | `1K` | `1K` / `2K` / `4K` only |
| `aspect-ratio` | string | (model picks) | e.g. `16:9`, `4:3`, `1:1` |
| `grounding` | bool | `false` | Enable Google Search grounding |
| `thinking` | string | (off) | `minimal` or `high`. **Flash only** — pro rejects |

(No `image-search` — that's a Gemini direct-REST capability not exposed via Vertex AI.)

### OpenAI

| Key | Type | Default | Notes |
|---|---|---|---|
| `model` | string | `gpt-image-2` | `gpt-image-2` (alias `2`), `1.5`, `1`, `mini` (alias `1-mini`), `latest` |
| `size` | string | `auto` | `1K` / `2K` / `4K` shorthand, `auto`, or raw `WxH` (e.g. `1024x1024`, `1536x1024`) |
| `quality` | string | `auto` | `auto`, `low`, `medium`, `high` |
| `compression` | int | `100` | 0–100. Applies only when output format is JPEG or WebP (decided by `filename` extension) |
| `moderation` | string | `auto` | `auto` or `low` |
| `background` | string | `auto` | `auto`, `opaque`, `transparent`. **`transparent` requires PNG/WebP output AND a non-`gpt-image-2` model** |

OpenAI edit mode (when `input:` is set) restricts `size:` to `1024x1024`, `1536x1024`, `1024x1536`, or `auto`.

---

## Map form vs list form

YAML and JSON batch files both support two top-level shapes.

### Map form — entries keyed by name (recommended)

```yaml
hero_shot:
  prompt: "..."
castle:
  prompt: "..."
```

**Map keys must be bare stems** — no dots, no extension. `image1:` is fine; `image1.png:` is rejected. The extension is inferred from the `filename:` value (or defaults to `.png`).

Order is preserved in YAML; JSON map-form sorts keys alphabetically because Go's JSON decoder doesn't preserve insertion order.

### List form — anonymous entries, ordered

```yaml
- prompt: "first"
- prompt: "second"
  filename: explicit.png
```

List entries are anonymous. The summary table renders them by 1-based index. Without an explicit `filename:` they fall through to the same `generated_{n}_{timestamp}.png` default that single-shot mode uses — exactly what you'd get from `imagine -p "..."` with no `-f`.

### Pick map form when

- You want named outputs (`hero.png`, `castle.png`) without specifying `filename:` for each.
- The names are part of your script (you reference them later by filename).

### Pick list form when

- The order is meaningful and the names don't matter.
- You're generating from a programmatic source and don't want to invent keys.

---

## Filename behavior

Filenames go through the same `ResolveFilename` helper that single-shot mode uses. Per-entry, the inputs to that helper resolve in this order:

1. Entry's `filename:` value if set.
2. Else CLI `-f` value if set.
3. Else (map form only) the entry's key, passed through filename sanitization, used as the filename stem.
4. Else (list form, no key) the same timestamp default single-shot mode produces when `-f` is absent: `generated_{n}_{YYYYMMDD_HHMMSS}.png`.

### Sanitization

When the entry name becomes the filename stem, it's sanitized:

- Path separators (`/`, `\`) → underscore
- Reserved characters (`: * ? " < > |`) → underscore
- Whitespace runs → single underscore
- Leading/trailing dots and underscores trimmed

So `hero shot` becomes `hero_shot.png`; `dir/file` becomes `dir_file.png`. An entry named entirely of forbidden characters falls back to `entry`.

### Extension rules

When `filename:` includes an extension, it picks the format:

| Extension | Format |
|---|---|
| `.png` | PNG |
| `.jpg` / `.jpeg` | JPEG (converted from PNG if the API returns PNG) |
| `.webp` | WebP (OpenAI native; converted otherwise) |
| anything else | falls back to `.png` |

When the entry-name stem is used (no explicit extension), it defaults to `.png`. If you need a different format with the entry-name stem, set `filename: hero.jpg` and you get `hero.jpg`.

### Collision detection

Two entries that would write to the same path produce a validation error before any HTTP call:

```
filename collision: entry a and entry b both produce /abs/out/cover.png
```

This includes the `count > 1` numbered cases (e.g. one entry with stem `hero` count=3 and another with explicit `filename: hero_1.png` would collide on `hero_1.png`).

---

## How CLI flags interact with batch files

CLI flags act as **defaults**; entry values **override**.

```bash
imagine -p batch.yaml -n 3 -s 1024x1024 -o ./out
```

Every entry inherits `count: 3`, `size: 1024x1024`, `output: ./out` unless its own keys override. So you can put shared settings on the command line and per-entry differences in the file.

### Per-entry filtering for provider-private flags

If you set a CLI flag that's specific to one provider (`--thinking high`), and your batch mixes providers:

- Entries whose provider claims that flag inherit it.
- Entries whose provider doesn't claim it silently skip it.

So `imagine -p mixed.yaml --thinking high` applies `thinking: high` to gemini/vertex entries and ignores it for openai entries.

If **no entry's provider claims the flag**, that's an error before any HTTP call:

```
--thinking is not supported by any provider used in this batch (supported by: [gemini vertex])
```

### Top-level `--replace` is rejected in batch mode

```bash
imagine -p batch.yaml -r
# ↓
# ERROR: --replace is not allowed in batch mode (set replace: true per entry instead)
```

Per-entry `replace: true` is allowed. The single-input-file rule still applies per-entry.

### CLI overrides that work fine

| Common flag | Effect in batch mode |
|---|---|
| `-p file.yaml` | Triggers batch |
| `-o ./out` | Default output folder for entries that don't set `output:` |
| `-n 3` | Default count |
| `-f hero.png` | Default filename — but rare to use this in batch (every entry gets the same name; collisions guaranteed) |
| `-i ref.png` | Default `input:` for entries that don't set it |
| `--provider gemini` | Default provider |
| `-r` | **Rejected** — per-entry only |

---

## Validation

`imagine` validates **all entries** before making any HTTP request. If any entry has a problem, you get every error in one report:

```
batch validation:
  - entry hero: prompt is required
  - entry castle: invalid --size "8K" (valid: 1K, 2K, 4K)
  - entry villa: --thinking is not supported by model "gemini-3-pro-image-preview" (supported by: [flash])
  - filename collision: entry a and entry b both produce /tmp/out/cover.png
```

Then nothing runs. Fix all issues, re-run.

### What gets validated

- **Schema:** unknown keys, type mismatches (e.g. `count: "three"`), enum values, numeric ranges.
- **Required fields:** `prompt:` on every entry.
- **Provider:** must be configured (or set `provider:` per entry), and the resolved provider must support batch invocation.
- **Model-level rules:** flags must be honoured by the resolved model (`thinking` against `pro` errors out before HTTP).
- **Cross-field rules:** `filename` + `replace` mutual exclusion; OpenAI's `transparent` + `gpt-image-2` conflict; OpenAI edit-mode size restrictions.
- **References:** `input:` paths must exist, must be supported image formats, folders must contain images.
- **Filename collisions:** across all entries.
- **Cross-entry CLI flag claim:** every CLI-set provider-private flag must be claimed by at least one entry's provider.

---

## Parallelism

Two layers of parallelism, nestable.

### Inner: per-entry batches

Each entry's `count:` is split by the provider's `MaxBatchN`:

| Provider | `MaxBatchN` | `count: 5` becomes |
|---|---|---|
| Gemini, Vertex | 1 | 5 parallel HTTP calls |
| OpenAI | 10 | 1 HTTP call returning 5 images |

### Outer: per entry

Every entry runs in its own goroutine, all started together.

### Combined

A batch with two entries on Gemini, each `count: 5`, fires **10 parallel HTTP calls** (2 entries × 5 batches each). A batch with two entries on OpenAI, each `count: 5`, fires **2 parallel calls** (each returning 5 images).

There's **no global concurrency cap**. A 50-entry Gemini batch with `count: 1` each fires 50 HTTP calls simultaneously. Watch for provider rate limits.

---

## Output

A summary table at the end:

```
╭───────────────┬──────────┬────────────────────────────────┬────────┬───────┬────────╮
│ ENTRY         │ PROVIDER │ MODEL                          │ IMAGES │ TIME  │ STATUS │
├───────────────┼──────────┼────────────────────────────────┼────────┼───────┼────────┤
│ hero_shot     │ openai   │ gpt-image-2                    │ 1/1    │ 14.2s │ ok     │
│ product_photo │ gemini   │ gemini-3-pro-image-preview     │ 2/3    │ 18.7s │ partial│
│ failed_one    │ openai   │ gpt-image-2                    │ 0/1    │  3.1s │ failed │
╰───────────────┴──────────┴────────────────────────────────┴────────┴───────┴────────╯

product_photo — 1 failure(s):
  Image 3: 503 service unavailable

failed_one — 1 failure(s):
  Image 1: invalid prompt: content policy violation

Done: 3 success, 2 failed across 3 entries (18.7s)

Output: /abs/path/out
```

| Status | Meaning | Row color |
|---|---|---|
| `ok` | Every requested image succeeded | default |
| `partial` | Some succeeded, some failed | amber |
| `failed` | None succeeded | muted red |

Per-entry failure details print below the table — exact error per failed image — so you can see which prompts to fix.

Exit code is non-zero if any image failed.

---

## Complete examples

### Homogeneous batch — same provider, different prompts

```yaml
city:
  prompt: "Cyberpunk city at night, neon reflections"
  size: 2K
  aspect-ratio: 16:9

forest:
  prompt: "Misty forest at dawn, volumetric god rays"
  size: 2K
  aspect-ratio: 16:9

interior:
  prompt: "Cozy library, warm lighting, leather chairs"
  size: 2K
  aspect-ratio: 4:3
```

```bash
imagine -p scenes.yaml --provider gemini -o ./scenes
```

Three named files: `scenes/city.png`, `scenes/forest.png`, `scenes/interior.png`.

### Mixed providers — pick the right tool per job

```yaml
photoreal_product:
  prompt: "Product shot of a mechanical watch on dark marble"
  provider: openai
  size: 1024x1024
  quality: high
  filename: watch.jpg

logo_iterations:
  prompt: "Minimalist logo for a coffee shop, 3 variants"
  provider: openai
  count: 3
  size: 1024x1024

hero_banner:
  prompt: "Mountain panorama at sunset, ultra-wide"
  provider: gemini
  model: pro
  size: 4K
  aspect-ratio: 32:9

with_search:
  prompt: "Latest model of the Tesla Cybertruck, factory white"
  provider: gemini
  model: flash
  grounding: true
  image-search: true
```

```bash
imagine -p mixed.yaml -o ./out
```

### Edit mode — references per entry

```yaml
seasons_winter:
  prompt: "Make it deep winter, heavy snow"
  input: ./source/town.png
  filename: town_winter.png

seasons_spring:
  prompt: "Make it spring, cherry blossoms"
  input: ./source/town.png
  filename: town_spring.png

product_set:
  prompt: "Combine these into a single gift basket"
  provider: openai
  input:
    - ./refs/lotion.png
    - ./refs/candle.png
    - ./refs/soap.png
  filename: basket.png
```

### JSON form

```json
{
  "hero": {
    "prompt": "Cinematic landscape",
    "provider": "openai",
    "size": "1024x1024"
  },
  "castle": {
    "prompt": "Stone castle on a cliff",
    "provider": "gemini",
    "size": "2K",
    "aspect-ratio": "16:9"
  }
}
```

```bash
imagine -p scene.json -o ./out
```

(JSON list form works too, same shape — array of objects.)

### List form — programmatic generation

```yaml
- prompt: "frame 1 of a sequence"
  provider: gemini
- prompt: "frame 2 of a sequence"
  provider: gemini
- prompt: "frame 3 of a sequence"
  provider: gemini
```

Files: `generated_1_20260425_HHMMSS.png`, `generated_2_…`, `generated_3_…` — the same pattern single-shot mode uses when no `-f` is given. Set `filename:` per-entry if you need stable names.

### Multi-line prompts

```yaml
detailed:
  prompt: |
    A studio product photo of a vintage typewriter on weathered oak.
    Soft window light from the upper left, warm tone.
    Shallow depth of field, focus on the keys.
    Dust motes visible in the light beam.
  provider: openai
  size: 1024x1024
  quality: high
```

YAML `|` preserves newlines; `>` folds them into spaces. Both work; `|` is usually what you want for image prompts.

---

## Errors and fixes

| Error | Cause | Fix |
|---|---|---|
| `entry "foo.png": key must be a bare stem` | Map key has a dot | Rename to `foo:` and add `filename: foo.png` if you want the extension |
| `entry hero: prompt is required` | Missing `prompt:` field | Add it |
| `entry hero: unknown key(s) [bogus]` | Typo in a key name | Check the table for that entry's provider |
| `entry hero: invalid --size "8K"` | Value not in the enum | Use `1K`/`2K`/`4K` for Gemini/Vertex, or accepted values for OpenAI |
| `entry hero: --thinking is not supported by model "pro"` | Model-level rule | Set `model: flash` on the entry, or remove `thinking:` |
| `--thinking is not supported by any provider used in this batch` | CLI flag flowed in but no entry's provider claims it | Drop the CLI flag, or add an entry whose provider claims it |
| `filename collision: entry a and entry b both produce ...` | Two entries write to the same file | Set distinct `filename:` values, or pick distinct entry keys |
| `--replace is not allowed in batch mode` | Top-level `-r` with a batch file | Set `replace: true` on individual entries |
| `provider "X" does not support batch invocation` | Custom/legacy provider missing `Bundle.ParseOptions` | Use one of the shipped providers (gemini, vertex, openai) |
| `reference path does not exist: ...` | `input:` path doesn't resolve | Check the path; `~` is expanded, relative paths are from the cwd |
| `count must be between 1 and 20 (got 25)` | `count:` out of range | Use 1–20; for more, split across entries |

For provider-API-side errors (rate limits, server errors, content policy), the entry's status row shows `partial` or `failed` and the per-image error prints below the table.
