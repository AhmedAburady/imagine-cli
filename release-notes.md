# BANANA CLI v1.1.5

## What's New

### Custom Output Filename (`-f`)

New `-f` flag lets you name your output file instead of getting the default timestamped filename.

```bash
# Single image — saves as sloth.png
banana -p "cute sloth" -f sloth.png

# Multiple images — auto-suffixed
banana -p "cute sloth" -f sloth.png -n 5
# → sloth_1.png, sloth_2.png, ..., sloth_5.png

# Combine with output folder
banana -p "cute sloth" -o ~/Documents -f sloth.png
```

### JPEG Output Support

Specify `.jpg` (or `.jpeg`) in `-f` to get a JPEG file. The API response is automatically re-encoded at quality 95 — fully parallel, no sequential bottleneck.

```bash
banana -p "cute sloth" -f sloth.jpg
banana -p "cute sloth" -f sloth.jpg -n 5
# → sloth_1.jpg, sloth_2.jpg, ...
```

**Notes:**
- `-f` and `-r` are mutually exclusive (validation error if both are set)
- Any extension other than `.jpg`/`.jpeg` falls back to `.png`
- Works with both Gemini API and Vertex AI (`-vertex`)

---

# BANANA CLI v1.1.4

## What's New

### Flash Model & Thinking Config

Switch between **Pro** and **Flash** models. Flash supports configurable thinking levels for faster or deeper reasoning.

```bash
# Use Flash model
banana -p "a sunset" -m flash

# Flash with high thinking
banana -p "a complex scene" -m flash -t high
```

| Flag | Description | Default |
|------|-------------|---------|
| `-m` | Model: `pro`, `flash` | `pro` |
| `-t` | Thinking level: `minimal`, `high` (Flash only) | `minimal` |

### Image Search Grounding

New `-is` flag enables image search grounding — lets the model reference real images during generation. Flash model only.

```bash
banana -p "a cat wearing a supreme hoodie" -m flash -is
```

### Multiple Reference Images (`-i`)

The `-i` flag is now **repeatable** and supports **shell globs**. Pass multiple reference images without needing a folder.

```bash
# Multiple explicit references
banana -i a.png -i b.png -p "merge these styles"

# Shell glob expansion (put -i last)
banana -p "add rain" -i *.png
```

### TUI Parity

The interactive TUI now exposes all new CLI features: **Model**, **Thinking Level**, and **Image Search**. Flash-only options appear dynamically when you select the Flash model.

### Responsive 2-Column Layout

TUI forms now use a compact 2-column layout for settings and adapt to your terminal size — no more cropped content or wasted space.

---

# BANANA CLI v1.1.3

## What's New

### Persistent GCP Configuration

Save your Vertex AI settings to the config file instead of using environment variables every time.

```bash
# Save GCP project and location (one-time setup)
banana config set-project your-project-id
banana config set-location global

# View all settings
banana config show
```

**New Config Commands:**
| Command | Description |
|---------|-------------|
| `banana config set-project <ID>` | Save GCP project ID |
| `banana config set-location <LOC>` | Save GCP location (default: global) |
| `banana config show` | Display all configured settings |

**Configuration Priority:**
1. Environment variables (highest priority)
2. Config file (`~/.config/banana/config.json`)
3. Default values

### Vertex AI Image Options

The `-ar` (aspect ratio) and `-s` (image size) flags now work correctly with Vertex AI.

```bash
banana -p "a sunset" -vertex -ar 16:9 -s 2K
```

---

# BANANA CLI v1.1.2

## What's New

### Vertex AI Support (`-vertex`)

Use Google Cloud's Vertex AI instead of the direct Gemini API. Perfect for enterprise users with GCP projects who want better quotas, IAM-based authentication, and no API key exposure.

```bash
# Set up (one-time)
gcloud auth application-default login
banana config set-project your-project-id

# Generate with Vertex AI
banana -p "a sunset over mountains" -vertex

# Edit with Vertex AI
banana -i photo.png -p "make it watercolor" -vertex

# Describe with Vertex AI
banana describe -i photo.jpg -vertex
```

**New Config Commands:**
```bash
banana config set-project <PROJECT_ID>   # Save GCP project
banana config set-location <LOCATION>    # Save GCP location (default: global)
banana config show                       # View all settings
```

**Configuration Priority:** Environment variables > Config file

**Benefits:**
- No API key in URLs - uses Google Cloud IAM authentication
- Enterprise-tier quotas based on your GCP project
- Works with service accounts for automation
- Same models as direct API (`gemini-3-pro-image-preview`)

**Required IAM Role:** `roles/aiplatform.user` (Vertex AI User)

---

# BANANA CLI v1.1.1

## What's New

### Replace Flag (`-r`)

New `-r` flag preserves the input filename for the output when editing images.

```bash
# Without -r: generates "generated_1_20260201_143052.png"
banana -i photo.png -p "make it cartoon"

# With -r: outputs "photo.png" (replaces original)
banana -i photo.png -p "make it cartoon" -r

# With -r and multiple images: outputs "photo_1.png", "photo_2.png", etc.
banana -i photo.png -p "make it cartoon" -r -n 3
```

**Notes:**
- Only works with single input files (not folders)
- When `-n > 1`, adds index suffix to preserve the original

---

# BANANA CLI v1.0.9

## What's New

### New `describe` Command

Analyze images and extract style descriptions using AI. Perfect for creating consistent style prompts.

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

**Features:**
- Single image or folder analysis
- Multiple images = unified style description
- `-p` flag for custom prompts (overrides default)
- `-a` flag for additional context (prepended to default)
- `-json` flag for comprehensive structured output
- Output to file (`-o`) or stdout

### Prompt File Support

- `-p` flag now accepts a file path in addition to text
- Useful for complex JSON prompts that are hard to escape in shell
- Supports any text file: `.json`, `.md`, `.txt`, etc.

```bash
# Text prompt (as before)
banana -p "a sunset over mountains"

# Load prompt from file
banana -p prompt.json -n 3
banana -p ~/prompts/calligraphy.txt -ar 1:1
```

### Auto Version Detection

- Version now auto-detected from Go build info when installed via `go install`
- No more "dev" version when installing from module

---

# BANANA CLI v1.0.8

(Broken release - use v1.0.9 instead)

---

# BANANA CLI v1.0.7

(Broken release - use v1.0.9 instead)

---

# BANANA CLI v1.0.6

## What's New

### Performance

- HTTP connection pooling for faster concurrent requests
- Parallel loading and base64 encoding of reference images
- Request timeout handling (120s)

---

# BANANA CLI v1.0.5

## What's New

### Security

- API key input is now hidden in both TUI and CLI
- TUI shows `•••••` as you type
- CLI uses standard hidden input (like sudo/ssh)

---

# BANANA CLI v1.0.4

## What's New

### Expanded Aspect Ratios

- Added "Auto" as the default aspect ratio - lets Gemini choose the best ratio for your prompt
- Now supports 11 aspect ratios: Auto, 1:1, 16:9, 9:16, 4:3, 3:4, 2:3, 3:2, 5:4, 4:5, 21:9
- Added ultra-wide 21:9 for cinematic shots

### Improved TUI

- Horizontal scrolling for aspect ratio selector with ◀ ▶ indicators

---

# BANANA CLI v1.0.3

## What's New

### API Key Configuration System

No more exporting environment variables! BANANA CLI now saves your API key securely in your `~/.config/banana` folder.

**New config commands:**
```bash
banana config set-key YOUR_API_KEY   # Save your key
banana config show                    # View config (key is masked)
banana config path                    # Show config file location
```

**Auto-prompt for API key:**
- **CLI**: If no API key is found, you'll be prompted to enter one
- **TUI**: Shows a dedicated API key input screen on first launch

**Priority-based lookup:**
1. `GEMINI_API_KEY` environment variable
2. `GOOGLE_API_KEY` environment variable
3. Config file (`~/.config/banana/config.json`)

This lets you override the saved key with env vars when needed.

### Version Flag

```bash
banana --version
banana -v
```

### Code Quality

- Refactored TUI architecture for better separation of concerns
- API key view extracted to dedicated module

## Upgrade

```bash
go install github.com/AhmedAburady/banana-cli/cmd/banana@latest
```

Or download the binary for your platform from the releases page.

## Quick Start

```bash
# Save your API key once
banana config set-key YOUR_GEMINI_API_KEY

# Generate images
banana -p "a cyberpunk city at night" -n 3

# Or use the interactive TUI
banana
```

---

# BANANA CLI v1.0.0 - v1.0.2

AI-powered image generation and editing using Google's Gemini API.

## Features

### Dual Interface
- **Interactive TUI** - Beautiful terminal UI with gradient banner and intuitive navigation
- **CLI Mode** - Scriptable command-line interface for automation

```bash
banana -p "A beautiful sunset"
```

### Image Generation
- Generate images from text prompts
- Edit existing images with AI
- Support for reference images (single file or folder)

### Performance
- **Parallel Processing** - Generate up to 20 images simultaneously
- Optimized API calls with minimal memory footprint

### Customization
- **Aspect Ratios**: 1:1, 16:9, 9:16, 4:3, 3:4
- **Image Sizes**: 1K, 2K, 4K
- **Google Search Grounding** - Enhance prompts with real-time web context
