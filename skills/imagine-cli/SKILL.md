---
name: imagine-cli
description: imagine is a command-line tool that generates and edits images with Google Gemini, Vertex AI and OpenAI (gpt-image-2.5), describes images, and runs many jobs from one YAML/JSON batch file. Use it whenever the user wants images generated or edited from the terminal, mentions imagine or its providers, needs to set up a Gemini / OpenAI / Vertex key or a ChatGPT sign-in, or hands over a batch file of image jobs, even if they never say "imagine".
---

# imagine

`imagine -p "prompt"` generates an image with the configured default provider. Add `-i photo.png` and the same command edits.

## Rules

1. **Use the default provider and model.** Don't add `--provider` or `-m` unless the user names a provider or model. `imagine providers` shows which one is active.
2. **Ask before generating** when the request leaves out something that changes the result: size or aspect ratio, how many images, where to save, file name or format. Ask once, offer the valid values for the active provider (below), then run. Skip asking when the user already said, or told you to use defaults.
3. **Never run interactive commands.** `imagine providers add <name>` without credential flags and `imagine providers add openai login` wait for a human. Ask the user to run them in their own terminal.
4. **Batch files:** the entry key is the file name, there is no `name:` key, and settings shared by every entry go on the command, not in each entry. See [Batch files](#batch-files).

## Setup

```bash
command -v imagine || go install github.com/AhmedAburady/imagine-cli/cmd/imagine@latest
imagine providers                      # what is configured, and which is active
```

Without Go, download the binary for the platform from https://github.com/AhmedAburady/imagine-cli/releases/latest and put it on `PATH`.

Register a provider with flags (never the bare form):

```bash
imagine providers add gemini --api-key AIza-XXX
imagine providers add openai --api-key sk-XXX
imagine providers add vertex --gcp-project <id> --location global   # needs `gcloud auth application-default login` first
imagine providers use <name>                                         # set the default; add --vision for describe
```

OpenAI can also bill a ChatGPT plan instead of an API key: the user runs `imagine providers add openai login` (browser sign-in) themselves.

## Generate and edit

| Flag | Purpose |
|---|---|
| `-p` | Prompt text or a file path. Repeat to join parts in order (`-p style.md -p "at night"`). |
| `-o` | Output folder (default `.`) |
| `-f` | File name. `.jpg` gives JPEG, `.webp` WebP (OpenAI with an API key only), anything else PNG. With `-n` > 1: `name_1.png`, `name_2.png`, ... |
| `-n` | Number of images, 1-20 |
| `-i` | Reference image or folder, repeatable. Switches to edit mode. |
| `-r` | Name the output after the single `-i` file (not with `-f`) |
| `--embed-metadata` | Store prompt, model and provider in the PNG (`imagine metadata file.png` reads them back) |

### Provider options

Values to offer when asking (rule 2). A flag the active provider doesn't have is an error.

**Gemini and Vertex** (details: [references/gemini.md](references/gemini.md))
- `-m pro` (default) / `flash` (Nano Banana 2.1) / `flash-lite`
- `-s 1K` (default) / `2K` / `4K`; `flash-lite` is 1K only
- `-a` aspect ratio: `1:1 1:4 1:8 2:3 3:2 3:4 4:1 4:3 4:5 5:4 8:1 9:16 16:9 21:9`; omit for auto
- `-g` web grounding (not `flash-lite`), `-I` image search (`flash` only), `-t minimal|medium|high` thinking (`flash`, `flash-lite`; `medium` is `flash` only)

**OpenAI** (details: [references/openai.md](references/openai.md))
- `-m flare` (default) / `sunburst` (best for precise edits) / `image2`
- `-s auto` (default), `1K`, `2K`, `4K`, or `WxH` (e.g. `1536x1024`; edges multiple of 16, max edge 3840, ratio up to 3:1)
- `-q auto|low|medium|high`, plus `xhigh|max` on flare and sunburst
- `--background auto|opaque|transparent` (transparent: PNG or WebP output), `--moderation auto|low`, `--compression 0-100` (JPEG/WebP)
- On a ChatGPT sign-in the backend picks its own image model and output size (the prompt steers the aspect), and `-q xhigh|max` is refused; use an API key when the exact model, size or quality matters.

## Batch files

`-p jobs.yaml` (or `.yml`, `.json`) runs every entry in parallel after validating them all.

```yaml
hero:                         # writes hero.png; the key is the file name
  prompt: "A samurai at dusk, cinematic"
banner:                       # writes banner.png
  prompt: prompts/banner.md   # a file path works as a prompt
  aspect-ratio: "21:9"        # set only what differs from the command
icons:                        # count 3 writes icons_1.png ... icons_3.png
  prompt: "Minimal app icon, flat"
  count: 3
```

```bash
imagine -p jobs.yaml -o renders/ -s 2K    # -o, -s and other flags apply to every entry
```

- **The key is the name.** `hero:` writes `hero.png`. Keys are bare stems (no `.png`, no dots). There is no `name:` key; use `filename:` only for a different name or a `.jpg`.
- **Shared settings come from the command.** `-o`, `-n`, `-s`, `-m` and the rest are defaults for every entry. Don't repeat `output:` or any other shared value per entry; set a key in an entry only when that entry is different.
- **Entry keys are long flag names.** Common: `prompt` (required), `separator`, `provider`, `output`, `filename`, `count`, `input`, `replace`, `embed-metadata`. Plus the active provider's own (`model`, `size`, `aspect-ratio`, `quality`, ...).
- `-r` is not allowed on a batch command; use `replace: true` in the entry.
- A relative prompt file path resolves from the batch file's folder; `input:` and `output:` paths resolve from the current directory.

Full schema: https://github.com/AhmedAburady/imagine-cli/blob/main/Docs/batch-files.md

## Describe

Turns images into a style description you can reuse as a prompt.

```bash
imagine describe -i photo.jpg                     # prose
imagine describe -i ./refs/ --json -o style.json  # structured
imagine describe -i photo.jpg -a "focus on lighting"
```

`-m` overrides the vision model (defaults: `gemini-3.8-flash` on Gemini and Vertex, `gpt-6.1-sol` on OpenAI, `gpt-6-luna` on a Free or Go ChatGPT sign-in); `-e` sets the reasoning effort.

## When something fails

Error messages name the fix. The full list is in [references/troubleshooting.md](references/troubleshooting.md); config details in [references/config.md](references/config.md).
