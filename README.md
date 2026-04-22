<div align="center">

```
              ██╗███╗   ███╗ █████╗  ██████╗ ██╗███╗   ██╗███████╗
              ██║████╗ ████║██╔══██╗██╔════╝ ██║████╗  ██║██╔════╝
              ██║██╔████╔██║███████║██║  ███╗██║██╔██╗ ██║█████╗  
              ██║██║╚██╔╝██║██╔══██║██║   ██║██║██║╚██╗██║██╔══╝  
              ██║██║ ╚═╝ ██║██║  ██║╚██████╔╝██║██║ ╚████║███████╗
              ╚═╝╚═╝     ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝╚═╝  ╚═══╝╚══════╝
                                                                  
```

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
- [Usage](#usage)
  - [Common flags](#common-flags)
  - [Gemini and Vertex](#gemini-and-vertex)
  - [OpenAI](#openai)
  - [Describe](#describe)
  - [Providers show](#providers-show)
- [Output formats](#output-formats)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)
- [License](#license)

---

## Why imagine

One CLI. Three providers. No environment variables, no ceremony — a YAML config file and you're running.

- **Multi-provider** — Google Gemini (direct REST), Google Vertex AI (GCP), OpenAI (gpt-image-2).
- **Unified generate + edit** — `-p "..."` generates; add `-i reference.png` and the same command edits.
- **Parallelism built in** — up to 20 images per invocation. OpenAI batches up to 10 per API call automatically.
- **Fang-styled help** — `imagine --help` renders provider-aware help with examples, model lists, and size options for whichever provider is active.
- **Cobra-powered** — flags, subcommands, shell completion, man pages.

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

imagine reads one file — `~/.config/imagine/config.yaml` (or `config.yml`). Write it yourself with an editor; there are no `config set-*` commands. Only include the providers you actually use.

### Schema

```yaml
default_provider: gemini

providers:
  gemini:
    api_key: AIza-your-key-here

  openai:
    api_key: sk-your-openai-key-here

  vertex:
    provider_options:
      gcp_project: your-gcp-project-id
      location: us-central1       # optional, defaults to "global"
```

| Field | Required | Notes |
|---|---|---|
| `default_provider` | No | Which provider to use when `--provider` is not passed. Defaults to the first provider under `providers:` (alphabetical). |
| `providers.<name>.api_key` | For Gemini/OpenAI | Required by providers that authenticate with an API key. |
| `providers.<name>.provider_options` | Provider-specific | Free-form string map. Vertex uses `gcp_project` (required) and `location` (optional). |

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

- **Gemini** — get a free API key from [Google AI Studio](https://aistudio.google.com/app/apikey) and paste into `providers.gemini.api_key`.
- **OpenAI** — get an API key from [platform.openai.com](https://platform.openai.com) and paste into `providers.openai.api_key`.
- **Vertex AI** — no key in the config. Two steps on the machine:
  1. A GCP project with the Vertex AI API enabled.
  2. `gcloud auth application-default login` — imagine uses Application Default Credentials.

  Then put the project id (and optional location) in `providers.vertex.provider_options`.

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

## Usage

### Common flags

These flags work with any provider:

| Flag | Long | Description | Default |
|---|---|---|---|
| `-p` | `--prompt` | Prompt text or path to a prompt file | *required* |
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

Analyze an image and produce a style description usable as a generation prompt.

```bash
imagine describe -i <image-or-folder> [flags]
```

| Flag | Description | Default |
|---|---|---|
| `-i` | Input image or folder (required) | — |
| `-o` | Output file path | stdout |
| `-p` | Custom prompt (overrides default instruction) | — |
| `-a` | Additional instructions prepended to the default | — |
| `-json` | Output structured JSON | `false` |
| `-vertex` | Use Vertex AI instead of Gemini direct | `false` |

Describe uses Gemini or Vertex — whichever you have configured. It's functionally unchanged from earlier versions.

```bash
# Plain text description
imagine describe -i photo.jpg

# JSON style guide from a folder of references
imagine describe -i ./styles/ -json -o style.json

# Vertex backend
imagine describe -i photo.jpg -vertex
```

### Providers show

List the providers declared in your config, with which is active and which is the default:

```bash
imagine providers show
```

Output:

```
default_provider: gemini

providers:
  gemini  [active, default]
    api_key: AIzaSyAU...44b0
  openai
    api_key: sk-proj-...v4IA
  vertex
    provider_options:
      gcp_project: my-project
      location: global
```

Markers:
- `active` — what this binary would use right now (after `--provider`/default/first resolution)
- `default` — whatever's in `default_provider:`
- `unknown: not built into this binary` — a provider your config mentions but this binary wasn't compiled with

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

## Troubleshooting

**`no provider configured`** — create `~/.config/imagine/config.yaml` with at least one provider under `providers:`. See [Configuration](#configuration).

**`unknown model "xyz" for provider "..."`** — the active provider doesn't know that model. Run `imagine --help` to see the accepted models for the active provider.

**`--X is not supported by provider "Y"`** — you used a flag that belongs to a different provider. The error tells you which providers *do* support it. Example: `--grounding` is Gemini/Vertex-only; swap providers or drop the flag.

**`--background transparent is not supported by gpt-image-2`** — known OpenAI limitation; use `-m 1.5` for transparency.

**Ctrl+C hangs** — it shouldn't. imagine uses context cancellation; in-flight HTTP requests are aborted when you press Ctrl+C.

**Vertex "failed to create Vertex AI client"** — you haven't run `gcloud auth application-default login` yet, or the project id in your config is wrong / doesn't have the Vertex AI API enabled.

[↑ Back to top](#table-of-contents)

---

## Contributing

Bugs, features, and PRs welcome. The codebase is documented for provider authors — adding a new backend is one new directory under `providers/` + one blank-import line in `cmd/imagine/main.go`. See [plan.md](plan.md) for architecture notes.

---

## License

MIT — see [LICENSE](LICENSE).

---

<div align="center">

Built in Go. No TUI, no env vars, no ceremony.

</div>
