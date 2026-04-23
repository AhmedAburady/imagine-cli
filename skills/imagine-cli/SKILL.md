---
name: imagine-cli
description: imagine is a multi-provider command-line tool for generating and editing images via Google Gemini, Google Vertex AI, and OpenAI (gpt-image-2). Use this skill whenever the user mentions imagine, wants to generate or edit images from the terminal, needs to set up an API key for Gemini / OpenAI / Vertex, switches default providers, or runs any `imagine providers` / `imagine describe` subcommand — even if they don't say the word "imagine" explicitly.
---

# imagine CLI

`imagine` is a multi-provider image-generation CLI. One binary, one YAML config file, three providers (gemini, vertex, openai). `imagine -p "..."` generates; add `-i reference.png` and the same command edits.

## When to use

Use this skill whenever the user:

- Mentions `imagine`, any of its flags, providers (gemini, vertex, openai), or model aliases (`gpt-image-2`, `pro`, `flash`, `1.5`, etc.)
- Wants to generate or edit images from the command line
- Is setting up the tool, adding an API key, or changing the default provider
- Runs any `imagine providers …` or `imagine describe` subcommand
- Hits an error — fixes live in [references/troubleshooting.md](references/troubleshooting.md)
- Asks which provider to pick for a task
- References sizes (`1K`, `2K`, `4K`, `1024x1024`, `3840x2160`, etc.)

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

# OpenAI (requires org verification for GPT Image at platform.openai.com)
imagine providers add openai --api-key sk-XXX

# Vertex AI — needs `gcloud auth application-default login` run on the machine first
imagine providers add vertex --gcp-project <gcp-project-id> --location us-central1
```

Each provider also accepts an optional `--vision-model` flag to override the default model used by `imagine describe`:

```bash
imagine providers add openai --api-key sk-XXX --vision-model gpt-5.4
imagine providers add gemini --api-key AIza-XXX --vision-model gemini-3.1-pro-preview
```

Defaults (used when `vision_model` is unset): `gemini-pro-latest` (gemini), `gemini-3-flash-preview` (vertex), `gpt-5.4-mini` (openai).

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
| `-p` | `--prompt` | Prompt (required). Also accepts a file path. |
| `-o` | `--output` | Output folder (default `.`) |
| `-f` | `--filename` | Output filename. Extension (`.png`/`.jpg`/`.webp`) drives format. With `-n >1`, `_1`, `_2`, … suffixes. |
| `-n` | `--count` | 1–20 images |
| `-i` | `--input` | Reference image/folder, repeatable. Flips to **edit mode**. |
| `-r` | `--replace` | Use input filename for output (single `-i` only; mutually exclusive with `-f`) |
|   | `--provider` | Per-invocation override |

`-f` and `-r` are mutually exclusive. `-r` requires exactly one `-i` pointing at a single file.

## Provider-specific flags

Setting a flag that doesn't belong to the active provider returns `--X is not supported by provider "Y" (supported by: [Z])`. Either drop the flag or switch providers with `--provider Z`.

- **Gemini / Vertex** → [references/gemini.md](references/gemini.md). Flags: `-m pro/flash`, `-s 1K/2K/4K`, `-a <aspect-ratio>`, `-g` (grounding), `-t minimal|high` (flash only), `-I` (image-search, Gemini flash only — Vertex does not support).
- **OpenAI** → [references/openai.md](references/openai.md). Flags: `-m gpt-image-2 family`, `-s shorthand or raw WxH`, `-q quality`, `--compression`, `--moderation`, `--background`. Edit-mode size is restricted to `1024x1024`, `1536x1024`, `1024x1536`, `auto`.

Provider pick heuristic:

- Photorealism, text rendering, intricate prompts → **OpenAI `gpt-image-2`**
- Fast iteration, Google ecosystem, live-search grounding → **Gemini**
- GCP-native auth / enterprise quotas → **Vertex**

## Describe subcommand

Analyse an image and produce a style description. Works with **all three providers** — each picks its own vision model.

```bash
imagine describe -i photo.jpg                                  # plain text, active describer
imagine describe -i ./styles/ --json -o style.json             # structured JSON from a folder
imagine describe -i photo.jpg --provider openai                # per-invocation override
imagine describe -i photo.jpg --provider vertex -m gemini-3.1-pro-preview   # model override
imagine describe --show-instructions                            # print built-in prompts, exit
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
- **openai**: `gpt-5.4-mini`

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
    api_key: sk-...
    vision_model: gpt-5.4-mini
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
