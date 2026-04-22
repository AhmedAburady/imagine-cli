---
name: imagine-cli
description: imagine is a multi-provider command-line tool for generating and editing images via Google Gemini, Google Vertex AI, and OpenAI (gpt-image-2).
---

# imagine CLI

`imagine` is a multi-provider image-generation CLI. One binary, one YAML config file, three providers (gemini, vertex, openai). `-p "..."` generates; add `-i reference.png` and it edits — no separate subcommand.

## When to use

Use this skill whenever the user:

- Mentions the `imagine` command, any of its flags, providers (gemini, vertex, openai), or model aliases (`gpt-image-2`, `pro`, `flash`, `1.5`, etc.)
- Wants to generate or edit images from the command line
- Is setting up the tool for the first time
- Hits an error they don't understand (most imagine errors are self-explanatory but the full list + fixes live in [references/troubleshooting.md](references/troubleshooting.md))
- Asks which provider to pick for a task
- References sizes (`1K`, `2K`, `4K`, `1024x1024`, `3840x2160`, etc.)

## Workflow

Always walk these three pre-flight steps **before** running an imagine command.

### Step 1 — Is imagine installed?

```bash
command -v imagine || echo NOT_INSTALLED
```

If `NOT_INSTALLED`, decide the install method automatically — don't prompt the user to pick.

**Decision:** check for Go first. If the user has Go, install from source (faster, keeps the binary up-to-date with `go install …@latest` on re-runs). Otherwise fall back to the pre-built binary.

```bash
if command -v go >/dev/null 2>&1; then
  go install github.com/AhmedAburady/imagine-cli/cmd/imagine@latest
else
  # Detect platform, pick the matching release asset
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
```

After install, verify:
```bash
imagine --version
```

Windows users: download `imagine-windows-amd64.exe` (or `-arm64.exe`) from the releases page, rename to `imagine.exe`, and place on `%PATH%`.

### Step 2 — Does the config file exist with at least one provider?

```bash
cat ~/.config/imagine/config.yaml 2>/dev/null \
  || cat ~/.config/imagine/config.yml 2>/dev/null \
  || echo NO_CONFIG
```

Windows path: `%AppData%\imagine\config.yaml`.

If missing or the `providers:` block is empty, walk the user through creating one. Full schema and per-provider credential setup in [references/config.md](references/config.md). A ready-to-copy template sits at [assets/config.example.yaml](assets/config.example.yaml).

Minimal Gemini-only example:
```yaml
default_provider: gemini
providers:
  gemini:
    api_key: AIza-paste-key-here
```

### Step 3 — Resolve the active provider

Every `imagine` invocation runs under one active provider. Precedence:

```
--provider <name>          (CLI flag — highest)
  ↓
default_provider           (config.yaml)
  ↓
first under providers:     (alphabetical)
  ↓
error
```

`imagine providers show` prints the current state with `[active]` and `[default]` markers — use it when there's ambiguity.

## Common flags (every provider)

| Flag | Long | Purpose |
|---|---|---|
| `-p` | `--prompt` | Prompt (required). Also accepts a file path. |
| `-o` | `--output` | Output folder (default `.`) |
| `-f` | `--filename` | Output filename. Extension (`.png`/`.jpg`/`.webp`) drives format. With `-n >1`, `_1`, `_2`, … suffixes. |
| `-n` | `--count` | 1-20 images |
| `-i` | `--input` | Reference image/folder, repeatable. Flips to **edit mode**. |
| `-r` | `--replace` | Use input filename for output (single `-i` only; mutually exclusive with `-f`) |
|   | `--provider` | Per-invocation override |

## Provider-specific flags

Flags that don't belong to the active provider are rejected with a clear error. Deep detail per provider:

- **Gemini / Vertex** → [references/gemini.md](references/gemini.md). Flags: `-m pro/flash`, `-s 1K/2K/4K`, `-a aspect-ratio`, `-g grounding`, `-t thinking` (flash only), `-I image-search` (gemini flash only).
- **OpenAI** → [references/openai.md](references/openai.md). Flags: `-m gpt-image-2 family`, `-s shorthand/raw WxH`, `-q quality`, `--compression`, `--moderation`, `--background`. Includes the full size matrix and edit-mode size restriction.

When the user is unsure which provider to pick:

- Photorealism, text rendering, intricate prompts → **OpenAI `gpt-image-2`**
- Fast iteration, Google ecosystem, live-search grounding → **Gemini** or **Vertex**
- GCP-native auth / enterprise quotas → **Vertex**

## Describe subcommand

```bash
imagine describe -i photo.jpg                   # plain text
imagine describe -i ./styles/ -json -o style.json
imagine describe -i photo.jpg -vertex           # Vertex backend
```

**Gemini/Vertex only** — describe doesn't support OpenAI. Needs either `providers.gemini.api_key` or `providers.vertex.provider_options.gcp_project` configured. If the user has only OpenAI configured, describe prints friendly setup instructions (with `gcloud auth application-default login` for Vertex) and exits.

## Examples

```bash
# Generate (active provider from config)
imagine -p "a sunset over mountains"

# Batch with size + aspect (Gemini/Vertex)
imagine -p "cityscape" -n 3 -s 2K -a 16:9 -o ./city

# OpenAI — fast draft
imagine -p "logo idea" --provider openai -q low

# OpenAI — 4K hero banner as JPEG
imagine -p "hero banner" --provider openai -s 3840x2160 -q high -f hero.jpg

# Edit, keep input filename
imagine -p "add rain" -i photo.png -r

# Multi-reference edit (OpenAI supports up to 16 refs/call)
imagine -p "gift basket of these" --provider openai \
  -i lotion.png -i candle.png -i soap.png

# Vertex — same flags as Gemini, different auth
imagine -p "a cat" --provider vertex -n 3
```

For more, run `imagine --help` — provider-aware help surfaces the active provider's flags and a tailored EXAMPLES block.

## Anti-patterns — do NOT do these

- **Never** suggest `GEMINI_API_KEY`, `OPENAI_API_KEY`, or any env var for credentials. imagine does **not** read env vars. Config file only.
- **Never** suggest `imagine config set-*` — those commands don't exist. Users edit `config.yaml` directly.
- **Never** use `-ar`, `-is`, `-vertex` as single-dash flags. They're `--aspect-ratio`, `--image-search`, `--provider vertex`. (Describe keeps its own legacy `-vertex` flag — the exception.)
- **Never** combine `--background transparent` with `gpt-image-2`. Unsupported — use `-m 1.5`.
- **Never** combine `-f` with `-r` — mutually exclusive.
- **Never** suggest `--provider openai` for the describe subcommand. Describe is Gemini/Vertex only.

## Troubleshooting

When imagine errors, read [references/troubleshooting.md](references/troubleshooting.md) before guessing. Covers every error message the CLI can produce with its specific fix.
