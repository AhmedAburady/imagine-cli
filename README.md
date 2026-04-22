<div align="center">

# BANANA CLI

### Gemini AI Image Generator

A powerful command-line tool for generating and editing images using Google's Gemini AI.
Features both an interactive terminal UI and a scriptable CLI interface.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/AhmedAburady/banana-cli?include_prereleases&v=104)](https://github.com/AhmedAburady/banana-cli/releases)

[Features](#features) • [Installation](#installation) • [Quick Start](#quick-start) • [CLI Reference](#cli-reference) • [TUI Guide](#tui-guide)

![BANANA CLI TUI](screenshots/tui1.png)
![BANANA CLI TUI](screenshots/tui2.png?v=1)



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
| **macOS** | Apple Silicon (M1/M2/M3) | [banana-darwin-arm64](https://github.com/AhmedAburady/banana-cli/releases/latest) |
| **macOS** | Intel | [banana-darwin-amd64](https://github.com/AhmedAburady/banana-cli/releases/latest) |
| **Linux** | x64 | [banana-linux-amd64](https://github.com/AhmedAburady/banana-cli/releases/latest) |
| **Linux** | ARM64 | [banana-linux-arm64](https://github.com/AhmedAburady/banana-cli/releases/latest) |
| **Windows** | x64 | [banana-windows-amd64.exe](https://github.com/AhmedAburady/banana-cli/releases/latest) |
| **Windows** | ARM64 | [banana-windows-arm64.exe](https://github.com/AhmedAburady/banana-cli/releases/latest) |

After downloading, make it executable (macOS/Linux):
```bash
chmod +x banana-darwin-arm64
mv banana-darwin-arm64 /usr/local/bin/banana
```

### Using Go

```bash
go install github.com/AhmedAburady/banana-cli/cmd/banana@latest
```

### From Source

```bash
git clone https://github.com/AhmedAburady/banana-cli.git
cd banana-cli
go build -o banana ./cmd/banana
```

---

## Quick Start

### 1. Get your API Key

Get a free Gemini API key from [Google AI Studio](https://aistudio.google.com/app/apikey).

### 2. Configure your API Key

**Option A: Save to config file (Recommended)**
```bash
banana config set-key YOUR_API_KEY
```

**Option B: Environment variable**
```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export GEMINI_API_KEY="your_api_key_here"
```

**Option C: Just run it**
```bash
# CLI will prompt you to enter and save your API key
banana -p "a sunset"

# TUI will show an API key input screen
banana
```

### 3. Generate Your First Image

**Using TUI (Interactive):**
```bash
banana
```

**Using CLI (One-liner):**
```bash
banana -p "a cyberpunk city at night with neon lights"
```

---

## CLI Reference

The CLI mode allows you to generate or edit images directly from the command line, perfect for scripting and automation.

### Basic Syntax

```
banana [flags]
banana describe [flags]
banana config <command>
```

Running `banana` without flags opens the interactive TUI.

### Flags

| Flag | Long Form | Type | Description | Default |
|------|-----------|------|-------------|---------|
| `-p` | | string | **Prompt** - The text description for image generation | *required* |
| `-o` | | string | **Output** - Directory to save generated images | `.` (current) |
| `-n` | | int | **Number** - How many images to generate (1-20) | `1` |
| `-ar` | | string | **Aspect Ratio** - Image dimensions ratio | `Auto` |
| `-s` | | string | **Size** - Output resolution | `1K` |
| `-g` | | bool | **Grounding** - Enable Google Search grounding | `false` |
| `-i` | | string | **Input** - Reference image/folder for edit mode | *none* |
| `-v` | `--version` | | Show version | |
| | `-vertex` | bool | Use Vertex AI instead of Gemini API | `false` |
| | `--help` | | Show help message | |

### Config Commands

Manage your API key configuration:

```bash
banana config set-key <KEY>   # Save your Gemini API key
banana config show            # Show current configuration (key is masked)
banana config path            # Show config file location
```

The config file is stored at `~/.config/banana/config.json`.

### Describe Command

Analyze images and extract style descriptions:

```bash
banana describe -i <image-or-folder> [flags]
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
banana -p "a mountain landscape at sunset"

# Multiple images with custom output folder
banana -p "abstract geometric patterns" -n 5 -o ./my-patterns

# Widescreen wallpaper in 4K
banana -p "northern lights over a snowy forest" -ar 16:9 -s 4K -o ~/Wallpapers

# Phone wallpaper
banana -p "minimalist gradient with soft colors" -ar 9:16 -o ./phone-wallpapers

# With Google Search grounding for current/real-world topics
banana -p "the latest Tesla Cybertruck design" -g
```

### Edit Mode

Transform existing images using the `-i` flag:

```bash
# Edit a single image
banana -i ./photo.jpg -p "convert to watercolor painting style"

# Use multiple reference images from a folder
banana -i ./reference-images/ -p "create a pattern inspired by these designs" -n 3

# Style transfer
banana -i ./portrait.png -p "transform into anime art style" -o ./anime-versions

# Add effects
banana -i ./landscape.jpg -p "add dramatic storm clouds and lightning"
```

### Describe Mode

Extract style descriptions from images:

```bash
# Plain text style description
banana describe -i photo.jpg

# Analyze folder of style references (unified description)
banana describe -i ./reference_images/

# Add style context to guide analysis
banana describe -i image.png -a "2D flat vector art"

# Structured JSON output
banana describe -i photo.jpg -json -o style.json
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
banana
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
│                        BANANA CLI                           │
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

BANANA CLI looks for your API key in the following order (first found wins):

| Priority | Source | Description |
|----------|--------|-------------|
| 1 | `GEMINI_API_KEY` | Environment variable (highest priority) |
| 2 | `GOOGLE_API_KEY` | Alternative environment variable |
| 3 | Config file | `~/.config/banana/config.json` |

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
banana config set-project your-project-id
banana config set-location global  # optional, defaults to global
```

Option B: Use environment variables
```bash
export GOOGLE_CLOUD_PROJECT="your-project-id"
export GOOGLE_CLOUD_LOCATION="global"  # optional
```

**Step 3: Use the `-vertex` flag**
```bash
banana -p "a beautiful sunset" -vertex
banana -p "cyberpunk city" -n 5 -vertex -ar 16:9
```

### Configuration Priority

Settings are loaded in this order (first found wins):

| Setting | Env Variable | Config Command | Default |
|---------|--------------|----------------|---------|
| GCP Project | `GOOGLE_CLOUD_PROJECT` | `banana config set-project` | - |
| GCP Location | `GOOGLE_CLOUD_LOCATION` | `banana config set-location` | `global` |

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
banana config set-key YOUR_API_KEY

# Or use environment variable
export GEMINI_API_KEY="your_key_here"
```

**Check current configuration:**
```bash
banana config show    # Shows masked key
banana config path    # Shows config file location
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
