<div align="center">

<img src=".assets/cover.jpg" alt="imagine" />

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/AhmedAburady/imagine-cli?include_prereleases)](https://github.com/AhmedAburady/imagine-cli/releases)

</div>

---

## Table of contents

- [Why imagine](#why-imagine)
- [Installation](#installation)
  - [go install](#go-install)
  - [From source](#from-source)
  - [Pre-built binaries](#pre-built-binaries)
- [Configuration](#configuration)
  - [Schema](#schema)
  - [Provider resolution](#provider-resolution)
  - [Credentials](#credentials)
- [Quick start](#quick-start)
- [Batch runs and automation](#batch-runs-and-automation)
- [Usage](#usage)
  - [Common flags](#common-flags)
  - [Gemini and Vertex](#gemini-and-vertex)
  - [OpenAI](#openai)
  - [Describe](#describe)
  - [Provider management](#provider-management)
- [Output formats](#output-formats)
- [AI agent skill](#ai-agent-skill)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## Why imagine

The best image models out there — Nano Banana, Nano Banana 2, and gpt-image-2 — are stuck behind web UIs. There's no official way to reach them from a terminal.

I built [banana-cli](https://github.com/AhmedAburady/banana-cli) first — a focused CLI for Google's image models. imagine is the next step: same idea, built to be extensible. One tool that can grow to support whatever good image models come next, across any provider.

- **The models that matter** — Nano Banana (`gemini-3-pro-image-preview`), Nano Banana 2 (`gemini-3.1-flash-image-preview`), and gpt-image-2. Direct API access, no middlemen.
- **Built for workflows** — pipe into scripts, run inside loops, chain with other CLI tools. Anywhere a command runs, imagine runs.
- **Concurrent generation** — `-n 10` fires off 10 images in one invocation. No clicking, no waiting for one to finish before starting the next.
- **Batch runs from a file** — `imagine -p batch.yaml` describes many jobs in one file: different prompts, different providers, different sizes. Every entry runs in parallel; validation is exhaustive before any HTTP fires; results land in a styled summary table. Built for scripts and CI.
- **Iterate fast** — tweak the prompt, rerun, compare. Generate multiple variations in one shot with `-n` and keep what works. The terminal loop is the creative loop.
- **Generate and edit in one command** — `-p "..."` generates; add `-i reference.png` and the same command switches to edit mode.
- **One config file, no env vars** — set your keys once in `~/.config/imagine/config.yaml` and forget about it.
- **Extensible by design** — adding a new provider is one directory under `providers/` and one import line. As new models ship, imagine can keep up.

[↑ Back to top](#table-of-contents)

---

## Installation

### go install

Requires Go 1.26 or later.

```bash
go install github.com/AhmedAburady/imagine-cli/cmd/imagine@latest
```

This drops an `imagine` binary in `$GOBIN` (or `$GOPATH/bin`). Make sure that directory is on your `$PATH`.

### From source

```bash
git clone https://github.com/AhmedAburady/imagine-cli.git
cd imagine-cli
go build -o imagine ./cmd/imagine
./imagine --help
```

### Pre-built binaries

Download from [Releases](https://github.com/AhmedAburady/imagine-cli/releases):

| Platform | Architecture | Binary |
|---|---|---|
| macOS | Apple Silicon | `imagine-darwin-arm64` |
| macOS | Intel | `imagine-darwin-amd64` |
| Linux | x64 | `imagine-linux-amd64` |
| Linux | ARM64 | `imagine-linux-arm64` |
| Windows | x64 | `imagine-windows-amd64.exe` |
| Windows | ARM64 | `imagine-windows-arm64.exe` |

On macOS/Linux:
```bash
chmod +x imagine-darwin-arm64
mv imagine-darwin-arm64 /usr/local/bin/imagine
```

[↑ Back to top](#table-of-contents)

---

## Configuration

imagine reads one file. Location depends on your OS:

| OS | Path |
|---|---|
| Linux / macOS / *BSD | `~/.config/imagine/config.yaml` |
| Windows | `%AppData%\imagine\config.yaml` (typically `C:\Users\<you>\AppData\Roaming\imagine\config.yaml`) |

Both `config.yaml` and `config.yml` extensions are accepted. You can edit the file by hand OR use `imagine providers add <name>` / `providers use` / `providers select` — both paths preserve your comments and formatting.

> macOS note: imagine intentionally uses `~/.config/imagine/` rather than `~/Library/Application Support/imagine/`. The XDG-style path has no spaces, is easy to browse, and plays nicely with dotfiles repos.

### Schema

```yaml
default_provider: gemini              # image-generation default
vision_default_provider: openai       # optional — describe default, falls back to default_provider

providers:
  gemini:
    api_key: AIza-your-key-here
    vision_model: gemini-pro-latest   # optional — defaults to gemini-pro-latest

  openai:
    api_key: sk-your-openai-key-here
    vision_model: gpt-5.4-mini        # optional — defaults to gpt-5.4-mini

  vertex:
    gcp_project: your-gcp-project-id
    location: us-central1             # optional, defaults to "global"
    vision_model: gemini-3-flash-preview
```

| Field | Required | Notes |
|---|---|---|
| `default_provider` | No | Provider used for image generation when `--provider` is omitted. Defaults to the first provider under `providers:` (alphabetical). |
| `vision_default_provider` | No | Provider used for `imagine describe` when `--provider` is omitted. Falls back to `default_provider` when empty. |
| `providers.gemini.api_key` | Yes | Google AI Studio API key. |
| `providers.openai.api_key` | Yes | OpenAI platform API key. |
| `providers.vertex.gcp_project` | Yes | GCP project id with the Vertex AI API enabled. |
| `providers.vertex.location` | No | Vertex region. Defaults to `global`. |
| `providers.<name>.vision_model` | No | Model `imagine describe` uses for that provider. Defaults are `gemini-pro-latest` (gemini/vertex) and `gpt-5.4-mini` (openai). |

Older configs that nested Vertex credentials under `provider_options:` still load — they're auto-migrated to flat on the next `imagine providers` write.

### Provider resolution

The active provider is resolved per-invocation with this precedence:

```
--provider <name>          # CLI flag — highest priority
  ↓
default_provider           # config.yaml
  ↓
first under providers:     # alphabetical
  ↓
error (no provider configured)
```

### Credentials

Easiest path — use `providers add` (interactive form in a terminal, non-interactive via flags):

```bash
imagine providers add gemini --api-key AIza-your-key
imagine providers add openai --api-key sk-your-key
imagine providers add vertex --gcp-project your-gcp-project-id
```

Or edit `config.yaml` by hand (shape above). Either way:

- **Gemini** — get a free API key from [Google AI Studio](https://aistudio.google.com/app/apikey).
- **OpenAI** — get an API key from [platform.openai.com](https://platform.openai.com).
- **Vertex AI** — no key. Two steps on the machine first:
  1. A GCP project with the Vertex AI API enabled.
  2. `gcloud auth application-default login` — imagine uses Application Default Credentials.

[↑ Back to top](#table-of-contents)

---

## Quick start

```bash
imagine -p "a cyberpunk city at night with neon lights"
```

Uses `default_provider` from your config, writes a timestamped PNG to the current directory.

```bash
imagine -p "make it winter" -i city.png --provider openai
```

Switches to OpenAI for this invocation and uses `/v1/images/edits` because `-i` was passed.

[↑ Back to top](#table-of-contents)

---

## Batch runs and automation

Hand `-p` a YAML, YML, or JSON file and imagine runs every entry in parallel — different prompts, different providers, different sizes — in one command. Built for scripts, CI, and reproducible image sets.

```yaml
# scenes.yaml
hero:
  prompt: "A samurai at dusk, cinematic"
  provider: openai
  size: 1024x1024
  quality: high

panorama:
  prompt: "Mountain panorama at sunset"
  provider: gemini
  model: pro
  size: 4K
  aspect-ratio: 32:9

product_iterations:
  prompt: "Minimalist coffee shop logo"
  provider: openai
  count: 3
```

```bash
imagine -p scenes.yaml -o ./out
```

Output:

```
╭───────────────────┬──────────┬────────────────────────────┬────────┬───────┬────────╮
│ ENTRY             │ PROVIDER │ MODEL                      │ IMAGES │ TIME  │ STATUS │
├───────────────────┼──────────┼────────────────────────────┼────────┼───────┼────────┤
│ hero              │ openai   │ gpt-image-2                │ 1/1    │ 14.2s │ ok     │
│ panorama          │ gemini   │ gemini-3-pro-image-preview │ 1/1    │ 18.7s │ ok     │
│ product_iterations│ openai   │ gpt-image-2                │ 3/3    │ 12.1s │ ok     │
╰───────────────────┴──────────┴────────────────────────────┴────────┴───────┴────────╯

Done: 5 success, 0 failed across 3 entries (18.7s)
Output: /abs/path/out
```

- **One file, many jobs** — every entry runs in its own goroutine, in parallel; each has its own prompt, provider, model, count.
- **Mix providers in one run** — different entries can target different providers in the same file. CLI flags act as defaults; entry values override.
- **Schema is just CLI flag names** — every key inside an entry is the long name of an `imagine` flag (`prompt`, `provider`, `model`, `size`, `quality`, `count`, `filename`, `input`, `replace`, …). Nothing new to learn.
- **Up-front, exhaustive validation** — schema errors, model-level rule violations (`thinking` against gemini's `pro` model), missing references, and filename collisions all surface in one report before any HTTP call. No half-run batches.
- **JSON works too** — same shape, swap `.yaml` for `.json`. List form (`- prompt: "..."`) supported alongside map form.

Full schema, every parameter, error/fix table, and worked examples (mixed providers, edit mode, JSON form, multi-line prompts): **[Docs/batch-files.md](Docs/batch-files.md)**.

[↑ Back to top](#table-of-contents)

---

## Usage

### Common flags

These flags work with any provider:

| Flag | Long | Description | Default |
|---|---|---|---|
| `-p` | `--prompt` | Prompt text, plain prompt-file path, or YAML/JSON [batch-file](#batch-runs-and-automation) path | *required* |
| `-o` | `--output` | Output directory | `.` |
| `-f` | `--filename` | Output filename. Extension (`.png`/`.jpg`/`.webp`) drives the image format. With `-n >1`, filenames get `_N` suffixes. | auto |
| `-n` | `--count` | Number of images (1–20) | `1` |
| `-i` | `--input` | Reference image or folder, repeatable; presence flips the command into edit mode | — |
| `-r` | `--replace` | Use the input filename for output (single `-i` file only) | `false` |
|  | `--provider` | Override the active provider for this invocation | config |
| `-v` | `--version` | Print version | — |
| `-h` | `--help` | Show provider-aware help | — |

Provider-specific flags live with each provider below. When you set a flag that the active provider doesn't support, imagine errors out clearly and tells you which provider *does* support it.

### Gemini and Vertex

Models and flags are shared between Gemini (direct REST) and Vertex (Gemini via GCP).

| Flag | Long | Description | Default |
|---|---|---|---|
| `-m` | `--model` | `pro` or `flash` (or full ID) | `pro` |
| `-s` | `--size` | `1K`, `2K`, or `4K` | `1K` |
| `-a` | `--aspect-ratio` | e.g. `1:1`, `16:9`, `9:16`, `4:3`, `3:4`, `21:9` | Auto |
| `-g` | `--grounding` | Google Search grounding | `false` |
| `-t` | `--thinking` | `minimal` or `high` (flash only) | Auto |
| `-I` | `--image-search` | Image Search grounding (Gemini flash only) | `false` |

**Examples**

```bash
# Multi-image generation
imagine -p "a sunset" -n 3 -s 2K -a 16:9

# Flash model with high thinking
imagine -p "futuristic city" -m flash -t high

# Edit a photo, keep its filename
imagine -p "add rain" -i photo.png -r

# Image Search grounding (Gemini flash only)
imagine -p "cat wearing a hoodie" -m flash -I
```

**Vertex** — same flags, add `--provider vertex`:

```bash
imagine -p "a sunset" --provider vertex -n 3
```

Vertex does not support `--image-search`.

### OpenAI

Uses `gpt-image-2` by default.

| Flag | Long | Description | Default |
|---|---|---|---|
| `-m` | `--model` | `gpt-image-2`, `1.5`, `1`, `mini`, `1-mini`, `latest` (or full ID) | `gpt-image-2` |
| `-s` | `--size` | `1K` / `2K` / `4K` shorthand, `auto`, or raw `WxH` (e.g. `1536x1024`) | `auto` |
| `-q` | `--quality` | `low`, `medium`, `high`, `auto` | `auto` |
|  | `--compression` | 0–100 (jpeg/webp only) | `100` |
|  | `--moderation` | `auto`, `low` | `auto` |
|  | `--background` | `auto`, `opaque`, `transparent` | `auto` |

**Size shorthand**

| Short | Dimensions |
|---|---|
| `1K` | `1024x1024` |
| `2K` | `2048x2048` |
| `4K` | `3840x2160` |
| `auto` | model picks (default) |

**Popular raw dimensions**

| Dimensions | Shape |
|---|---|
| `1024x1024` | square |
| `1536x1024` | landscape |
| `1024x1536` | portrait |
| `2048x2048` | 2K square |
| `2048x1152` | 2K landscape |
| `3840x2160` | 4K landscape |
| `2160x3840` | 4K portrait |

Any `WxH` is accepted if: edge ≤ 3840px, both multiples of 16, long:short ≤ 3:1, total pixels 655,360–8,294,400.

**Edit-mode restriction** — OpenAI's `/v1/images/edits` only accepts `1024x1024`, `1536x1024`, `1024x1536`, `auto`. Using `-i` with `-s 2K` / `4K` / larger raw dimensions errors before the API call.

**Output format** — inferred from `-f` extension:
- `-f cat.png` → API returns PNG
- `-f cat.jpg` → API returns JPEG directly (no local re-encode)
- `-f cat.webp` → API returns WebP

**Transparent background** — requires PNG or WebP output (not JPEG). `gpt-image-2` does not currently support transparent backgrounds per the OpenAI docs; use `-m 1.5` for transparency.

**Examples**

```bash
# Fast draft
imagine -p "a red apple" --provider openai -q low

# Batched — one API call returns 3 images (MaxBatchN=10)
imagine -p "logo variants" --provider openai -n 3

# 4K landscape, high quality, JPEG output
imagine -p "hero banner" --provider openai -s 3840x2160 -q high -f hero.jpg

# Edit with a reference
imagine -p "make it winter" --provider openai -i photo.png

# Transparent sticker (1.5 only)
imagine -p "sticker" --provider openai -m 1.5 --background transparent -f sticker.png
```

### Describe

Analyze an image and produce a style description usable as a generation prompt. Works across all three providers — each picks its own vision model.

```bash
imagine describe -i <image-or-folder> [flags]
```

| Flag | Description | Default |
|---|---|---|
| `-i` | Input image or folder (required) | — |
| `-o` | Output file path | stdout |
| `-p` | Custom instruction (replaces default) | — |
| `-a` | Additional context prepended to the default instruction | — |
| `-m` | Override the provider's vision model for this invocation | config / provider default |
|   | `--provider` | Override the describer provider for this invocation | — |
|   | `--json` | Emit structured JSON (`StyleAnalysis` schema) | `false` |
|   | `--show-instructions` | Print the built-in prompts for the active describer and exit | `false` |

**Provider resolution** for describe:

```
--provider <name>          # CLI flag — wins
  ↓
vision_default_provider    # config.yaml
  ↓
default_provider           # config.yaml
  ↓
first describer-capable provider configured
  ↓
error
```

Default vision models per provider:

| Provider | Default | Override |
|---|---|---|
| gemini | `gemini-pro-latest` | `providers.gemini.vision_model` OR `-m <id>` |
| vertex | `gemini-3-flash-preview` | `providers.vertex.vision_model` OR `-m <id>` |
| openai | `gpt-5.4-mini` | `providers.openai.vision_model` OR `-m <id>` |

**Examples**

```bash
# Plain text, active describer (vision default → default)
imagine describe -i photo.jpg

# Structured JSON from a folder of style references
imagine describe -i ./styles/ --json -o style.json

# Per-invocation provider + model override
imagine describe -i photo.jpg --provider openai -m gpt-5.4

# See what instruction the active describer sends
imagine describe --show-instructions
```

**Set a persistent describe default** different from the image-gen default:

```bash
imagine providers use openai --vision      # sets vision_default_provider
imagine providers select --vision          # interactive picker
```

### Provider management

Four subcommands cover inspection and configuration. Every write is atomic and preserves your file's comments.

| Command | Purpose |
|---|---|
| `imagine providers` | List configured providers with status pills and capability badges |
| `imagine providers add <name>` | Register credentials (interactive form in a TTY, flags otherwise) |
| `imagine providers use <name>` | Set `default_provider` |
| `imagine providers use <name> --vision` | Set `vision_default_provider` |
| `imagine providers select` | Interactive picker for `default_provider` |
| `imagine providers select --vision` | Interactive picker for `vision_default_provider` (filtered to describers) |

Listing output:

```
  PROVIDERS

  ●  gemini   ACTIVE   DEFAULT    generate  describe
  ·  openai            VISION     generate  describe
  ·  vertex                       generate  describe

  3 configured  ·  /Users/you/.config/imagine/config.yaml
```

Pills + badges:
- `●` green bullet — the currently-active image-gen provider
- `ACTIVE` — same info, explicit
- `DEFAULT` — matches `default_provider:` in config
- `VISION` — matches `vision_default_provider:` (only shown when it differs from `DEFAULT`)
- `NOT BUILT-IN` — a provider your config lists that this binary wasn't compiled with
- `generate` / `describe` — the capabilities this provider implements

`providers add <name> --help` shows the exact fields for each provider (api_key, vision_model, gcp_project, location as applicable). Non-TTY invocation with missing required fields errors with the exact flag names — deterministic output for scripts and CI.

[↑ Back to top](#table-of-contents)

---

## Output formats

**Input** (reference images for edit mode): `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`

**Output** — driven by the `-f` filename extension:
- `.png` (default)
- `.jpg` / `.jpeg` — For Gemini/Vertex, imagine converts locally at quality 95. For OpenAI, the API returns JPEG directly.
- `.webp` — OpenAI only.

[↑ Back to top](#table-of-contents)

---

## AI agent skill

If you use an AI coding agent (Claude Code, Cursor, Cline, Codex, Amp, Gemini CLI, Copilot, and others), install the bundled imagine skill and your agent will know the whole tool — config file schema, provider resolution, flag ownership per provider, size matrix, error handling, the works. It'll even auto-install the CLI if needed.

Install via the [`skills`](https://skills.sh) CLI — pick whichever package manager you have:

```bash
npx skills add AhmedAburady/imagine-cli
# or
bunx skills add AhmedAburady/imagine-cli
# or
pnpm dlx skills add AhmedAburady/imagine-cli
```

The installer asks which agents to install for, then symlinks the skill into each agent's skills directory. After that, a prompt like "use imagine to generate a cyberpunk city banner" triggers the skill automatically.

The skill source lives at [`skills/imagine-cli/`](skills/imagine-cli/) in this repo.

[↑ Back to top](#table-of-contents)

---

## Development

imagine is built around a small provider framework so adding a new backend is almost entirely local to its own package. You write a tagged `Options` struct, implement `Generate`, and register a Bundle — the framework handles Cobra flag binding, validation, HTTP plumbing, model-level flag enforcement, and test coverage.

- **[Docs/adding-a-provider.md](Docs/adding-a-provider.md)** — step-by-step guide for adding a new provider (file layout, `flagspec` tags, `transport` helpers, `providertest` harness, worked example).

Key packages for provider authors:

| Package | Purpose |
|---|---|
| [`providers/flagspec`](providers/flagspec/) | Reflection-based flag DSL — declare flags as struct tags |
| [`internal/transport`](internal/transport/) | Shared HTTP primitives: `PostJSON[R]`, auth injectors, `APIError`, base64 decode |
| [`providers/providertest`](providers/providertest/) | Contract test harness — one-line `TestContract` runs 12 invariants |
| [`providers`](providers/) | Core interfaces: `Provider`, `Bundle`, `RequestLabeler`, `ResolvedModeler` |

Files you **don't** edit when adding a provider: `commands/`, `cli/`, `api/`, `config/`, `cmd/imagine/main.go`. If a change there seems necessary, that's a framework gap worth an issue.

[↑ Back to top](#table-of-contents)

---

## Troubleshooting

**`no provider configured`** — create the config file with at least one provider under `providers:`. The path is OS-specific; run `imagine -p test` with no config and the error tells you the exact path. See [Configuration](#configuration).

**`unknown model "xyz" for provider "..."`** — the active provider doesn't know that model. Run `imagine --help` to see the accepted models for the active provider.

**`--X is not supported by provider "Y"`** — you used a flag that belongs to a different provider. The error tells you which providers *do* support it. Example: `--grounding` is Gemini/Vertex-only; swap providers or drop the flag.

**`--background transparent is not supported by gpt-image-2`** — known OpenAI limitation; use `-m 1.5` for transparency.

**Ctrl+C hangs** — it shouldn't. imagine uses context cancellation; in-flight HTTP requests are aborted when you press Ctrl+C.

**Vertex "failed to create Vertex AI client"** — you haven't run `gcloud auth application-default login` yet, or the project id in your config is wrong / doesn't have the Vertex AI API enabled.

[↑ Back to top](#table-of-contents)

---

## Contributing

Bugs, features, and PRs welcome. Adding a new provider is one new directory under `providers/` plus one blank-import line in [`providers/all/all.go`](providers/all/all.go) — see [Development](#development) and the full [adding-a-provider guide](Docs/adding-a-provider.md).

---

## License

MIT — see [LICENSE](LICENSE).

---

<div align="center">

Built in Go. No TUI, no env vars, no ceremony.

</div>
