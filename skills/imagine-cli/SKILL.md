---
name: imagine-cli
description: imagine is a multi-provider command-line tool for generating and editing images via Google Gemini, Google Vertex AI, and OpenAI (gpt-image-2). Use this skill whenever the user mentions imagine, wants to generate or edit images from the terminal, needs to set up an API key — or sign in with a ChatGPT Plus/Pro subscription — for Gemini / OpenAI / Vertex, switches default providers, runs any `imagine providers` / `imagine describe` subcommand, or wants to run multiple image-generation jobs from a YAML/JSON batch file (single command, many prompts, parallel) — even if they don't say the word "imagine" explicitly.
---

# imagine CLI

`imagine` is a multi-provider image-generation CLI. One binary, one YAML config file, three providers (gemini, vertex, openai). `imagine -p "..."` generates; add `-i reference.png` and the same command edits.

## When to use

Use this skill whenever the user:

- Mentions `imagine`, any of its flags, providers (gemini, vertex, openai), or model aliases (`gpt-image-2`, `pro`, `flash`, `flash-lite`, etc.)
- Wants to generate or edit images from the command line
- Is setting up the tool, adding an API key, or changing the default provider
- Runs any `imagine providers …`, `imagine describe`, or `imagine metadata` subcommand
- Hits an error — fixes live in [references/troubleshooting.md](references/troubleshooting.md)
- Asks which provider to pick for a task
- References sizes (`1K`, `2K`, `4K`, `1024x1024`, `3840x2160`, etc.)
- Wants to run multiple jobs in one invocation, mix providers in a single run, or hands you a `.yaml` / `.yml` / `.json` file describing image-generation jobs — that's batch mode (`imagine -p batch.yaml`)
- Wants one prompt assembled from several files or snippets (style + subject + instruction) — repeat `-p`, see [Composing prompts from parts](#composing-prompts-from-parts)

## Workflow

Three pre-flight steps before running any generation command.

### Step 1 — Is imagine installed?

```bash
command -v imagine || echo NOT_INSTALLED
```

If `NOT_INSTALLED`, pick the install method automatically (don't prompt). Go available → install from source. Otherwise → pre-built binary.

```bash
if command -v go >/dev/null 2>&1; then
  go install github.com/AhmedAburady/imagine-cli/cmd/imagine@latest
else
  case "$(uname -s)-$(uname -m)" in
    Darwin-arm64)  ASSET=imagine-darwin-arm64 ;;
    Darwin-x86_64) ASSET=imagine-darwin-amd64 ;;
    Linux-x86_64)  ASSET=imagine-linux-amd64 ;;
    Linux-aarch64|Linux-arm64) ASSET=imagine-linux-arm64 ;;
    *) echo "Unsupported platform — download manually from https://github.com/AhmedAburady/imagine-cli/releases/latest"; exit 1 ;;
  esac
  curl -L -o imagine "https://github.com/AhmedAburady/imagine-cli/releases/latest/download/$ASSET"
  chmod +x imagine
  sudo mv imagine /usr/local/bin/imagine
fi
imagine --version
```

Windows: download `imagine-windows-amd64.exe` (or `-arm64.exe`) from the releases page, rename to `imagine.exe`, place on `%PATH%`.

### Step 2 — Is a provider configured?

```bash
imagine providers
```

If output is "No providers configured" or the command errors with "no provider configured", register one before running any generation command.

### Step 3 — Register a provider (non-interactive)

Always pass the credentials as flags. Don't run `imagine providers add <name>` without flags — that opens an interactive form intended for humans and hangs in a non-terminal environment.

```bash
# Gemini (free tier at https://aistudio.google.com/app/apikey)
imagine providers add gemini --api-key AIza-XXX

# OpenAI — API key (org verification required for GPT Image at platform.openai.com/settings)
imagine providers add openai --api-key sk-XXX

# Vertex AI — needs `gcloud auth application-default login` run on the machine first
imagine providers add vertex --gcp-project <gcp-project-id> --location us-central1
```

**OpenAI has two auth methods.** The flag form above is the API-key method — safe to run headless. The alternative is a ChatGPT **subscription** (gpt-image-2 billed to the user's Plus/Pro plan, no API key):

```bash
imagine providers add openai login    # opens the user's BROWSER for OAuth sign-in
```

Do **not** run `imagine providers add openai login` (or the bare `imagine providers add openai` picker) yourself in a non-interactive context — both block waiting on a human (a browser callback / a TTY prompt). If the user wants subscription auth, tell them to run `imagine providers add openai login` in their own terminal; once signed in, you generate exactly as with an API key. Config-wise this writes `auth_method: subscription` (tokens live in a separate `~/.config/imagine/openai-subscription-auth.json`, not in `config.yaml`).

Each provider also accepts an optional `--vision-model` flag to override the default model used by `imagine describe`:

```bash
imagine providers add openai --api-key sk-XXX --vision-model gpt-5.4
imagine providers add gemini --api-key AIza-XXX --vision-model gemini-pro-latest
```

Defaults (used when `vision_model` is unset): `gemini-pro-latest` (gemini), `gemini-3-flash-preview` (vertex), `gpt-5.5` (openai).

`imagine providers add <name>` writes to `~/.config/imagine/config.yaml` (creates the file on first run), preserves existing comments and unrelated keys, and writes atomically.

To see the exact required/optional flags for a provider:
```bash
imagine providers add <name> --help
```

### Step 4 — Set the default provider (optional)

```bash
imagine providers use <name>            # sets default_provider (image generation)
imagine providers use <name> --vision   # sets vision_default_provider (describe)
```

If `<name>` isn't configured or isn't built-in, imagine errors with the list of valid options. `--vision` additionally rejects providers that don't implement describe.

When `default_provider` is unset, imagine picks the alphabetically-first configured provider. `vision_default_provider` falls back to `default_provider` when unset.

## Provider resolution

Every invocation resolves an active provider in this order:

```
--provider <name>          (CLI flag — wins)
  ↓
default_provider           (config.yaml)
  ↓
first under providers:     (alphabetical)
  ↓
error: no provider configured
```

`imagine providers` shows which is currently active.

## Common flags (every provider)

| Flag | Long | Purpose |
|---|---|---|
| `-p` | `--prompt` | Prompt (required). Also accepts a file path. **Repeatable** - parts concatenate in order. |
|   | `--separator` | Text joining repeated `-p` parts (default `\n\n`; `\n`, `\t`, `\r` escapes interpreted) |
| `-o` | `--output` | Output folder (default `.`) |
| `-f` | `--filename` | Output filename. Extension (`.png`/`.jpg`/`.webp`) drives format. With `-n >1`, `_1`, `_2`, … suffixes. |
| `-n` | `--count` | 1–20 images |
| `-i` | `--input` | Reference image/folder, repeatable. Flips to **edit mode**. |
| `-r` | `--replace` | Use input filename for output (single `-i` only; mutually exclusive with `-f`) |
|   | `--embed-metadata` | Embed generation details (prompt, model, provider, references) into PNG output |
|   | `--provider` | Per-invocation override |

`-f` and `-r` are mutually exclusive. `-r` requires exactly one `-i` pointing at a single file.

### Composing prompts from parts

Repeat `-p` to assemble one prompt from reusable files plus one-off text. Each part resolves independently (existing file path → trimmed contents; anything else → literal text), then parts join in order.

```bash
imagine -p style.md -p "Make it night time." -p subject.md      # joined by a blank line
imagine -p style.md -p subject.md --separator "---"             # style.md \n---\n subject.md
imagine -p "a cat" -p "in space" --separator " | "              # a cat | in space
```

Rules that matter when scripting this:

- Empty parts are dropped, so `-p "$MAYBE_EMPTY"` is safe; all-empty is `prompt is required`.
- A separator with no newline and no surrounding whitespace goes on its own line (`---` → `\n---\n`). Pad it or write `\n` yourself for exact placement.
- `--separator ""` errors; a single space is the minimum.
- A batch file must be the only `-p`; combining it with other parts errors.
- A `-p` value that looks like a path but doesn't exist is used as literal text (no error) - check paths before building the command.

## Provider-specific flags

Setting a flag that doesn't belong to the active provider returns `--X is not supported by provider "Y" (supported by: [Z])`. Either drop the flag or switch providers with `--provider Z`.

- **Gemini / Vertex** → [references/gemini.md](references/gemini.md). Flags: `-m pro/flash/flash-lite`, `-s 1K/2K/4K` (flash-lite: 1K only), `-a <aspect-ratio>` (14 values: `1:1` `1:4` `1:8` `2:3` `3:2` `3:4` `4:1` `4:3` `4:5` `5:4` `8:1` `9:16` `16:9` `21:9`), `-g` (grounding, not on flash-lite), `-t minimal|high` (flash and flash-lite), `-I` (image-search, Gemini flash only — Vertex does not support).
- **OpenAI** → [references/openai.md](references/openai.md). Flags: `-m gpt-image-2 family`, `-s shorthand or raw WxH`, `-q quality`, `--compression`, `--moderation`, `--background`. Same flags for both auth methods (API key or ChatGPT subscription). Edit-mode size is restricted to `1024x1024`, `1536x1024`, `1024x1536`, `auto` on the **API-key route only** — the subscription route accepts any size in edit mode.

Provider pick heuristic:

- Photorealism, text rendering, intricate prompts → **OpenAI `gpt-image-2`**
- Fast iteration, Google ecosystem, live-search grounding → **Gemini**
- GCP-native auth / enterprise quotas → **Vertex**

## Batch mode

`-p` accepts a YAML / `.yml` / JSON file describing multiple jobs. File-vs-text decided by extension. Other extensions (`.txt`, none) read as plain prompt-file text.

### Schema rule

Every key inside an entry is the long name of a CLI flag — same set as `imagine --help`. Unknown keys error before any HTTP call.

### Top-level shape

Map form (recommended) — entries keyed by name; YAML preserves order, JSON sorts alphabetically; **map keys must be bare stems** (no `.png` / no dots):

```yaml
hero_shot:
  prompt: "..."
  provider: openai
castle:
  prompt: "..."
  provider: gemini
```

List form — entries are anonymous; the summary table identifies them by 1-based index. Without an explicit `filename:`, list-form entries fall back to the same `generated_{n}_{timestamp}.png` default that single-shot mode uses.

```yaml
- prompt: "first"
- prompt: "second"
```

### Common keys (every entry)

| Key | Type | Notes |
|---|---|---|
| `prompt` | string OR list | **Required**. Inline text (use YAML `\|` for multi-line), or a path to a file whose contents become the prompt (`~` expanded; relative paths resolve against the batch file's directory; trimmed; must be non-empty). A value that doesn't name an existing file is used literally. A list (`[style.md, "at night"]`) concatenates its parts like repeated `-p`. |
| `separator` | string | Joins a list `prompt:`. Default = CLI `--separator` (`\n\n`). Same escape and own-line rules. |
| `provider` | string | `gemini` / `vertex` / `openai`. Falls back to `--provider` then config default. |
| `output` | string | Output folder. `~` expanded. Default = CLI `-o` (or `.`). |
| `filename` | string | Full filename with extension. Mutually exclusive with `replace`. |
| `count` | int | 1–20. With `count > 1`, names get `_1`, `_2`, … suffix. |
| `input` | string OR list | Reference file/folder, or list of them. Flips entry into edit mode. `~` expanded. |
| `replace` | bool | Use input filename as output. Requires exactly one input pointing at a file. Mutually exclusive with `filename`. |
| `embed-metadata` | bool | Embed generation details into PNG output. |

### Provider-private keys

Setting a key for the wrong provider errors. Defaults are applied per provider; omit to use them.

**Gemini:** `model` (`pro`/`flash`/`flash-lite`/full ID, default `pro`), `size` (`1K`/`2K`/`4K`, default `1K`; **`flash-lite` accepts `1K` only**), `aspect-ratio` (one of `1:1` `1:4` `1:8` `2:3` `3:2` `3:4` `4:1` `4:3` `4:5` `5:4` `8:1` `9:16` `16:9` `21:9`), `grounding` (bool, **not on `flash-lite`**), `thinking` (`minimal`/`high`, **`flash` and `flash-lite` only**), `image-search` (bool, **`flash` only**).

**Vertex:** same as Gemini but **no `image-search`** (not exposed via Vertex AI).

**OpenAI:** `model` (`gpt-image-2`, alias `2`), `size` (`1K`/`2K`/`4K` shorthand, `auto`, or raw `WxH` like `1024x1024`, default `auto`), `quality` (`auto`/`low`/`medium`/`high`, default `auto`), `compression` (0–100 int, default `100`, jpeg/webp only), `moderation` (`auto`/`low`), `background` (`auto`/`opaque`).

OpenAI edit mode (entry has `input:`) restricts `size:` to `1024x1024` / `1536x1024` / `1024x1536` / `auto`.

### CLI flag interaction

CLI flags = defaults; entry values override.

```bash
imagine -p batch.yaml -n 3 -s 1024x1024 -o ./out
```

Per-entry filtering: a CLI flag flows to an entry only if the entry's provider claims it. So `--thinking high` against a mixed batch applies to gemini/vertex entries and silently skips openai entries. If **no** entry's provider claims the flag, validation errors before any HTTP call.

Top-level `--replace` is rejected in batch mode — set `replace: true` per-entry instead.

### Filename behavior

Resolution order: entry's `filename:` → CLI `-f` → entry name as stem (sanitized) → fallback. Sanitization replaces `/ \ : * ? " < > |` and whitespace with `_`. Entry-name stem with no extension defaults to `.png`. Cross-entry filename collisions error before HTTP.

### Validation

Up-front and exhaustive. Schema errors, missing prompts, bad enum values, model-level rule violations, missing reference paths, and filename collisions all surface together in one report. No HTTP fires until validation passes.

### Parallelism

Outer: one goroutine per entry. Inner: orchestrator splits each entry's `count:` by `MaxBatchN` (Gemini/Vertex = 1, OpenAI = 10). Two Gemini entries × `count: 5` → 10 parallel HTTP calls. **No global cap** — watch rate limits on large batches.

### Output

Summary table at the end with columns `ENTRY` / `PROVIDER` / `MODEL` / `IMAGES` (succeeded/total) / `TIME` / `STATUS` (`ok` / `partial` / `failed`). Per-entry failure detail prints below the table. Exit code non-zero if any image failed.

### Common errors and fixes

| Error | Fix |
|---|---|
| `entry "foo.png": key must be a bare stem` | Drop the extension from the key; use `filename: foo.png` for the file extension. |
| `entry hero: prompt is required` | Add `prompt:`. |
| `entry hero: unknown key(s) [...]` | Key not in that provider's schema; cross-check the tables above. |
| `entry hero: --thinking is not supported by model "pro"` | Set `model: flash` on the entry, or drop `thinking:`. |
| `entry hero: --size 4K is not supported by model "gemini-3.1-flash-lite-image"` | `flash-lite` renders 1K only. Drop `size:` or switch to `model: flash`. |
| `entry hero: --grounding is not supported by model "gemini-3.1-flash-lite-image"` | `flash-lite` has no Google Search grounding. Drop `grounding:` or switch to `model: flash`. |
| `--X is not supported by any provider used in this batch` | Drop the CLI flag, or add an entry whose provider claims it. |
| `filename collision: entry a and entry b both produce ...` | Set distinct `filename:` per entry. |
| `--replace is not allowed in batch mode` | Use per-entry `replace: true`. |
| `provider "X" does not support batch invocation` | Use one of the shipped providers (gemini, vertex, openai). |

### Example — mixed providers

```yaml
hero_shot:
  prompt: "A samurai at dusk, cinematic"
  provider: openai
  size: 1024x1024
  quality: high
  filename: hero.jpg

logo_iterations:
  prompt: "Minimalist coffee shop logo"
  provider: openai
  count: 3

panorama:
  prompt: "Mountain panorama at sunset"
  provider: gemini
  model: pro
  size: 4K
  aspect-ratio: 21:9
```

```bash
imagine -p mixed.yaml -o ./out
```

Full schema, more examples (edit mode, JSON, multi-line prompts), and an extended error/fix table: [`Docs/batch-files.md`](../../Docs/batch-files.md) in the imagine-cli repo.

## Metadata

Pass `--embed-metadata` during generation to write the prompt, model, provider, and reference paths into the output `.png` as standard `iTXt` text chunks. (Silently ignored for `.jpg` / `.webp`.)

```bash
imagine -p "cyberpunk city" --embed-metadata -f city.png
```

Read those values back with the `metadata` subcommand. Useful for agents inspecting prior outputs.

```bash
imagine metadata city.png                 # prints all fields in a readable table
imagine metadata city.png --prompt        # prints only the raw prompt (pipeable)
imagine metadata city.png --model --provider
imagine metadata city.png --reference-image
```

In [batch mode](#batch-mode), use the boolean key `embed-metadata: true` per entry.

## Describe subcommand

Analyse an image and produce a style description. Works with **all three providers** — each picks its own vision model.

```bash
imagine describe -i photo.jpg                                  # plain text, active describer
imagine describe -i ./styles/ --json -o style.json             # structured JSON from a folder
imagine describe -i photo.jpg --provider openai                # per-invocation override
imagine describe -i photo.jpg --provider vertex -m gemini-pro-latest   # model override
imagine describe --show-instructions                            # print built-in prompts, exit
imagine describe -i photo.jpg -p "Rate composition 1-10"       # custom instruction (replaces default)
imagine describe -i photo.jpg -a "Focus on lighting"           # extra context prepended to default
```

| Flag | Purpose |
|---|---|
| `-i` | Input image or folder (required) |
| `-o` | Output file path (default stdout) |
| `-p` | Custom instruction (replaces default) |
| `-a` | Additional context prepended to default |
| `-m` | Override the vision model for this invocation |
| `--provider` | Override the describer provider |
| `--json` | Emit structured JSON (StyleAnalysis schema) |
| `--show-instructions` | Print the built-in prompts for the active describer and exit |

Resolution order when `--provider` is omitted:
1. `vision_default_provider` in config (set via `imagine providers use <name> --vision`)
2. `default_provider` in config
3. First configured describer-capable provider

Default vision models per provider (overridable in config as `vision_model`):
- **gemini**: `gemini-pro-latest`
- **vertex**: `gemini-3-flash-preview`
- **openai**: `gpt-5.5` (both auth methods)

Describe works on both OpenAI auth methods.

Bare `imagine describe` (no flags) prints help and exits 0.

## Config file schema

Flat per-provider fields. Full schema, defaults, and legacy `provider_options:` migration notes in [references/config.md](references/config.md).

```yaml
default_provider: gemini               # image-gen default
vision_default_provider: openai        # describe default (optional; falls back to default_provider)

providers:
  gemini:
    api_key: AIza-...
    vision_model: gemini-pro-latest    # optional per-provider describe model
  openai:
    auth_method: api_key             # "api_key" or "subscription" (ChatGPT sign-in via `add openai login`)
    api_key: sk-...                  # only for auth_method: api_key
    vision_model: gpt-5.5
  vertex:
    gcp_project: my-project-id
    location: global                   # optional — "global" when omitted
    vision_model: gemini-3-flash-preview
```

Older configs with `providers.vertex.provider_options.gcp_project` still load; the next `imagine providers add` / `use` rewrites them flat.

## Examples

```bash
# Generate with active provider
imagine -p "a sunset over mountains"

# Batch, size + aspect (Gemini/Vertex)
imagine -p "cityscape" -n 3 -s 2K -a 16:9 -o ./city

# OpenAI, fast draft
imagine -p "logo idea" --provider openai -q low

# OpenAI, 4K hero banner as JPEG
imagine -p "hero banner" --provider openai -s 3840x2160 -q high -f hero.jpg

# Compose a prompt from reusable files plus a one-off instruction
imagine -p prompts/style.md -p "Make it night time." -p prompts/subject.md

# Edit, keep input filename
imagine -p "add rain" -i photo.png -r

# Multi-reference edit (OpenAI accepts up to 16 refs per call)
imagine -p "gift basket of these" --provider openai \
  -i lotion.png -i candle.png -i soap.png

# Vertex, same flags as Gemini
imagine -p "a cat" --provider vertex -n 3
```

`imagine --help` is provider-aware — hides flags from providers other than the active one, renders a tailored EXAMPLES block.

## Troubleshooting

[references/troubleshooting.md](references/troubleshooting.md) — every error message the CLI produces with its fix.
