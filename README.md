<div align="center">

# IMAGINE CLI

### Gemini AI Image Generator

A powerful command-line tool for generating and editing images using Google's Gemini AI.
Features both an interactive terminal UI and a scriptable CLI interface.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/AhmedAburady/imagine-cli?include_prereleases&v=104)](https://github.com/AhmedAburady/imagine-cli/releases)

[Features](#features) • [Installation](#installation) • [Quick Start](#quick-start) • [CLI Reference](#cli-reference) • [TUI Guide](#tui-guide)

![IMAGINE CLI TUI](screenshots/tui1.png)
![IMAGINE CLI TUI](screenshots/tui2.png?v=1)



</div>

---

## Features

| Feature | Description |
|---------|-------------|
| **Dual Interface** | Interactive TUI for exploration, CLI for scripting and automation |
| **Image Generation** | Create images from text prompts with Gemini AI |
| **Image Editing** | Transform existing images using reference-based editing |
| **Style Analysis** | Extract style descriptions from images with `describe` command |
| **Parallel Processing** | Generate up to 20 images simultaneously |
| **Flexible Output** | Control aspect ratio (1:1, 16:9, 9:16, 4:3, 3:4) and size (1K, 2K, 4K) |
| **Google Search Grounding** | Enhance prompts with real-time web search context |
| **Path Autocomplete** | Tab completion for file paths in TUI (supports `~` expansion) |

---

## Installation

### Pre-built Binaries (Recommended)

Download the latest release for your platform:

| Platform | Architecture | Download |
|----------|--------------|----------|
| **macOS** | Apple Silicon (M1/M2/M3) | [imagine-darwin-arm64](https://github.com/AhmedAburady/imagine-cli/releases/latest) |
| **macOS** | Intel | [imagine-darwin-amd64](https://github.com/AhmedAburady/imagine-cli/releases/latest) |
| **Linux** | x64 | [imagine-linux-amd64](https://github.com/AhmedAburady/imagine-cli/releases/latest) |
| **Linux** | ARM64 | [imagine-linux-arm64](https://github.com/AhmedAburady/imagine-cli/releases/latest) |
| **Windows** | x64 | [imagine-windows-amd64.exe](https://github.com/AhmedAburady/imagine-cli/releases/latest) |
| **Windows** | ARM64 | [imagine-windows-arm64.exe](https://github.com/AhmedAburady/imagine-cli/releases/latest) |

After downloading, make it executable (macOS/Linux):
```bash
chmod +x imagine-darwin-arm64
mv imagine-darwin-arm64 /usr/local/bin/imagine
```

### Using Go

```bash
go install github.com/AhmedAburady/imagine-cli/cmd/imagine@latest
```

### From Source

```bash
git clone https://github.com/AhmedAburady/imagine-cli.git
cd imagine-cli
go build -o imagine ./cmd/imagine
```

---

## Quick Start

### 1. Get credentials

- **Gemini**: free API key from [Google AI Studio](https://aistudio.google.com/app/apikey).
- **Vertex AI**:
  1. A GCP project with the Vertex AI API enabled.
  2. Run `gcloud auth application-default login` once on the machine — imagine uses Application Default Credentials, so there's no key to paste into the config.
  3. Put only the project id (and optional location) in `config.yaml`.
- **OpenAI** (Phase 5, not yet shipped): API key from [platform.openai.com](https://platform.openai.com).

### 2. Create the config file

imagine reads `~/.config/imagine/config.yaml` (or `config.yml`). Create it yourself — there's no `config set-*` command; just write the YAML:

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
      location: us-central1   # optional; defaults to "global"
```

Only include the providers you actually use. `default_provider` is optional — if omitted, imagine picks the first provider under `providers:` (alphabetical order).

#### Schema reference

| Field | Required | Notes |
|---|---|---|
| `default_provider` | No | Which provider to use when `--provider` is not passed. If empty, first `providers:` entry wins. |
| `providers.<name>.api_key` | For Gemini/OpenAI | Required by providers that authenticate with an API key. |
| `providers.<name>.provider_options` | Provider-specific | Free-form string map for extras. Vertex uses `gcp_project` (required) and `location` (optional, default `global`). |

#### Provider resolution precedence

```
--provider <name>      (CLI flag — highest priority)
  ↓
default_provider       (config)
  ↓
first under providers: (alphabetical)
  ↓
error
```

### 3. Generate your first image

```bash
imagine -p "a cyberpunk city at night with neon lights"
```

---

## CLI Reference

The CLI mode allows you to generate or edit images directly from the command line, perfect for scripting and automation.

### Basic Syntax

```
imagine [flags]              # generate (and edit, if -i is passed)
imagine describe [flags]     # analyze image style
imagine version              # print version
```

Run `imagine --help` for the full fang-styled help.

### Flags

| Flag | Long Form | Type | Description | Default |
|---|---|---|---|---|
| `-p` | `--prompt` | string | Prompt text or path to prompt file | *required* |
| `-o` | `--output` | string | Output directory | `.` |
| `-f` | `--filename` | string | Output filename (suffixed `_N` for n>1) | *none* |
| `-n` | `--count` | int | Number of images (1-20) | `1` |
| | `--aspect-ratio` | string | Aspect ratio | `Auto` |
| `-s` | `--size` | string | Image size (provider-specific: `1K`/`2K`/`4K` for Gemini/Vertex) | `1K` |
| `-i` | `--input` | string | Reference image/folder, repeatable (enables edit mode) | *none* |
| `-r` | `--replace` | bool | Use the input filename for output (single file only) | `false` |
| `-m` | `--model` | string | Model (provider-specific; aliases: `pro`, `flash` for Gemini/Vertex) | provider default |
| | `--provider` | string | Override active provider | config |
| `-g` | `--grounding` | bool | Google Search grounding (Gemini/Vertex) | `false` |
| `-t` | `--thinking` | string | Thinking level: `minimal` or `high` (Gemini/Vertex flash only) | `minimal` |
| | `--image-search` | bool | Image-search grounding (Gemini flash only) | `false` |
| `-v` | `--version` | | Show version | |
| `-h` | `--help` | | Show help | |

The config file is stored at `~/.config/imagine/config.json`.

### Describe Command

Analyze images and extract style descriptions:

```bash
imagine describe -i <image-or-folder> [flags]
```

| Flag | Description | Default |
|------|-------------|---------|
| `-i` | Input image or folder (required) | - |
| `-o` | Output file path | stdout |
| `-p` | Custom prompt (overrides default) | - |
| `-a` | Additional context (prepended to default) | - |
| `-json` | Output as structured JSON | `false` |

### Aspect Ratios

| Value | Use Case |
|-------|----------|
| `Auto` | **Default** - Let Gemini decide the best ratio |
| `1:1` | Square - Social media posts, profile pictures |
| `16:9` | Landscape - Desktop wallpapers, YouTube thumbnails |
| `9:16` | Portrait - Phone wallpapers, Instagram stories |
| `4:3` | Classic - Presentations, traditional photos |
| `3:4` | Portrait Classic - Portraits, posters |
| `2:3` | Portrait - Standard photo print ratio |
| `3:2` | Landscape - DSLR camera ratio |
| `5:4` | Near-square - Medium format photos |
| `4:5` | Portrait - Instagram portrait posts |
| `21:9` | Ultra-wide - Cinematic, ultrawide monitors |

### Image Sizes

| Value | Resolution | Best For |
|-------|------------|----------|
| `1K` | ~1024px | Quick previews, web use |
| `2K` | ~2048px | High-quality prints, detailed work |
| `4K` | ~4096px | Maximum quality, large prints |

---

## CLI Examples

### Generate Mode

Generate images from text prompts:

```bash
# Simple generation - creates 1 image in current directory
imagine -p "a mountain landscape at sunset"

# Multiple images with custom output folder
imagine -p "abstract geometric patterns" -n 5 -o ./my-patterns

# Widescreen wallpaper in 4K
imagine -p "northern lights over a snowy forest" -ar 16:9 -s 4K -o ~/Wallpapers

# Phone wallpaper
imagine -p "minimalist gradient with soft colors" -ar 9:16 -o ./phone-wallpapers

# With Google Search grounding for current/real-world topics
imagine -p "the latest Tesla Cybertruck design" -g
```

### Edit Mode

Transform existing images using the `-i` flag:

```bash
# Edit a single image
imagine -i ./photo.jpg -p "convert to watercolor painting style"

# Use multiple reference images from a folder
imagine -i ./reference-images/ -p "create a pattern inspired by these designs" -n 3

# Style transfer
imagine -i ./portrait.png -p "transform into anime art style" -o ./anime-versions

# Add effects
imagine -i ./landscape.jpg -p "add dramatic storm clouds and lightning"
```

### Describe Mode

Extract style descriptions from images:

```bash
# Plain text style description
imagine describe -i photo.jpg

# Analyze folder of style references (unified description)
imagine describe -i ./reference_images/

# Add style context to guide analysis
imagine describe -i image.png -a "2D flat vector art"

# Structured JSON output
imagine describe -i photo.jpg -json -o style.json
```

### Output Example

```
⠋ Generating 3 image(s)...

✓ generated_1_20260123_143052.png
✓ generated_2_20260123_143053.png
✓ generated_3_20260123_143054.png

Done: 3 success, 0 failed (12.4s)
Output: /Users/ahmed/my-images
```

---

## TUI Guide

The Terminal User Interface provides an interactive experience for image generation.

### Launching the TUI

```bash
imagine
```

### Main Menu

From the main menu, choose between:

- **Generate Image** - Create new images from text prompts
- **Edit Image** - Transform existing images with AI

### Form Fields

#### Generate Image Form

| Field | Description |
|-------|-------------|
| **Output Folder** | Where to save images (supports `~` and tab completion) |
| **Number of Images** | 1-20 images generated in parallel |
| **Prompt** | Your image description |
| **Aspect Ratio** | Select from 1:1, 16:9, 9:16, 4:3, 3:4 |
| **Image Size** | Select from 1K, 2K, 4K |
| **Grounding** | ON/OFF - Enable Google Search grounding |

#### Edit Image Form

Same as Generate, plus:

| Field | Description |
|-------|-------------|
| **Reference Path** | Image file or folder containing reference images |

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate between form fields |
| `←` `→` | Cycle through options (aspect ratio, size, grounding) |
| `Tab` | Accept path autocomplete suggestion |
| `Ctrl+N` | Insert newline in prompt field |
| `Ctrl+S` | Submit form and start generation |
| `Esc` | Go back to previous screen |
| `Enter` | Select menu item |
| `q` | Quit from main menu |
| `Ctrl+C` | Force quit |

### Path Autocomplete

The TUI supports intelligent path autocomplete:

- Start typing a path and press `Tab` to see suggestions
- Use `~` for home directory (e.g., `~/Pictures`)
- Works for both output folders and reference images

---

## Supported Formats

### Input (Reference Images)

| Format | Extensions |
|--------|------------|
| JPEG | `.jpg`, `.jpeg` |
| PNG | `.png` |
| GIF | `.gif` |
| WebP | `.webp` |

### Output

All generated images are saved as **PNG** format.

---

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                        IMAGINE CLI                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────┐         ┌─────────────┐        ┌─────────┐   │
│   │   TUI   │────────▶│             │───────▶│  Save   │   │
│   └─────────┘         │   Gemini    │        │  PNG    │   │
│                       │   API       │        │  Files  │   │
│   ┌─────────┐         │             │        └─────────┘   │
│   │   CLI   │────────▶│  (Parallel) │                      │
│   └─────────┘         └─────────────┘                      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

1. **Input**: Prompt text + optional reference images
2. **Processing**: Parallel API calls to Gemini (up to 20 concurrent)
3. **Output**: PNG images saved to specified folder

---

## API Key Configuration

IMAGINE CLI looks for your API key in the following order (first found wins):

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | `GEMINI_API_KEY` | Environment variable (highest priority) |
| 2 | `GOOGLE_API_KEY` | Alternative environment variable |
| 3 | Config file | `~/.config/imagine/config.json` |

This allows you to:
- Use environment variables to temporarily override the saved key
- Keep a default key in the config file for convenience
- Use different keys for different projects via env vars

---

## Vertex AI Configuration (Alternative)

Instead of using a Gemini API key, you can use **Vertex AI** with Google Cloud authentication. This is useful if you have a GCP project with better quotas or enterprise features.

### Prerequisites

1. A Google Cloud project with the Vertex AI API enabled
2. A service account with the **Vertex AI User** role (or equivalent)
3. Google Cloud CLI (`gcloud`) installed

### Setup

**Step 1: Authenticate with Google Cloud**
```bash
# Login with your Google account
gcloud auth application-default login

# Or use a service account
gcloud auth activate-service-account --key-file=your-service-account.json
export GOOGLE_APPLICATION_CREDENTIALS="path/to/your-service-account.json"
```

**Step 2: Configure your GCP project**

Option A: Save to config file (recommended)
```bash
imagine config set-project your-project-id
imagine config set-location global  # optional, defaults to global
```

Option B: Use environment variables
```bash
export GOOGLE_CLOUD_PROJECT="your-project-id"
export GOOGLE_CLOUD_LOCATION="global"  # optional
```

**Step 3: Use the `-vertex` flag**
```bash
imagine -p "a beautiful sunset" -vertex
imagine -p "cyberpunk city" -n 5 -vertex -ar 16:9
```

### Configuration Priority

Settings are loaded in this order (first found wins):

| Setting | Env Variable | Config Command | Default |
|---------|--------------|----------------|---------|
| GCP Project | `GOOGLE_CLOUD_PROJECT` | `imagine config set-project` | - |
| GCP Location | `GOOGLE_CLOUD_LOCATION` | `imagine config set-location` | `global` |

### Benefits of Vertex AI

- **No API key exposure** - Uses Google Cloud IAM authentication
- **Enterprise quotas** - Higher rate limits based on your GCP project
- **VPC Service Controls** - Network security features
- **Audit logging** - Cloud Audit Logs integration

---

## Troubleshooting

### API Key Issues

**No API key configured:**
```bash
# Easiest: save to config
imagine config set-key YOUR_API_KEY

# Or use environment variable
export GEMINI_API_KEY="your_key_here"
```

**Check current configuration:**
```bash
imagine config show    # Shows masked key
imagine config path    # Shows config file location
```

### "No images found in directory"

When using `-i` with a folder, ensure it contains supported image formats (.jpg, .jpeg, .png, .gif, .webp).

### API Rate Limits

If generating many images, you may hit rate limits. The tool handles this gracefully - failed images will show error messages while successful ones are saved.

---

## License

MIT License - see [LICENSE](LICENSE) for details.

---

## Contributing

Contributions are welcome! Feel free to:

- Report bugs
- Suggest features
- Submit pull requests

---

<div align="center">

Made with Go and Gemini AI

</div>
