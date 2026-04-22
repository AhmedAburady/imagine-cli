# imagine troubleshooting

Most errors point at the exact fix. Here's what they mean and how to resolve.

## `no provider configured. Create <path> with a providers: entry (see README for schema)`

The config file is missing or has no providers configured. Walk the user through creating one — see `references/config.md`.

Quick check:
```bash
cat ~/.config/imagine/config.yaml 2>/dev/null || cat ~/.config/imagine/config.yml 2>/dev/null || echo missing
```

## `unknown provider "xyz" (available: [gemini openai vertex])`

User passed `--provider xyz` or set `default_provider: xyz` in config with a name that doesn't match any built-in provider. Fix:

- Check spelling — only `gemini`, `vertex`, `openai` are built in.
- If they meant one of those, correct the config.

If they named a provider that SHOULD exist (like a new one someone added), the binary was built without it — they need a rebuild with that provider's blank-import in `cmd/imagine/main.go`.

## `unknown model "xyz" for provider "gemini" (accepted: [...])`

The `-m` value isn't a valid alias or canonical ID for the active provider.

- For **Gemini/Vertex**: `pro`, `flash`, or full IDs `gemini-3-pro-image-preview` / `gemini-3.1-flash-image-preview`.
- For **OpenAI**: `2`, `1.5`, `1`, `mini`, `1-mini`, `latest`, or full canonical IDs.

Run `imagine --help` for the active provider's accepted list (shown under MODELS in the EXAMPLES section).

## `--X is not supported by provider "Y" (supported by: [Z])`

User set a flag that belongs to a different provider. Two fixes:

1. **Drop the flag** if they don't need it.
2. **Switch providers** by adding `--provider Z` or changing `default_provider` in config.

Common cases:
- `--grounding` / `-g` → Gemini / Vertex only.
- `--image-search` / `-I` → Gemini only (not Vertex).
- `--thinking` / `-t` → Gemini / Vertex only.
- `--quality` / `-q` → OpenAI only.
- `--compression` / `--moderation` / `--background` → OpenAI only.
- `--aspect-ratio` / `-a` → Gemini / Vertex only. OpenAI takes raw `-s WxH` instead.

## `Number of images must be between 1 and 20`

`-n` is out of range. Values 1-20 only. For more than 20, run the command twice with different output folders.

## `--background transparent requires PNG or WebP output`

Transparent backgrounds can't be expressed in JPEG. Change `-f` to `.png` or `.webp`, or drop `--background transparent`.

## `--background transparent is not supported by gpt-image-2`

Per the OpenAI docs, gpt-image-2 doesn't currently support transparent backgrounds. Two options:

- Use `-m 1.5` (or `gpt-image-1.5`) — older model that supports transparency.
- Drop `--background transparent` and accept an opaque background.

## `openai edit endpoint only accepts size 1024x1024, 1536x1024, 1024x1536, or auto`

Edit mode has tighter size constraints than generate. Either:

- Change `-s` to one of the four allowed values.
- Drop `-i` to switch back to generate mode (if edit wasn't needed).

## `-f and -r are mutually exclusive`

The flags fight each other:
- `-f` sets a custom output filename.
- `-r` uses the input filename for output.

Pick one. For `-r` to work, there must be exactly one `-i` pointing at a single file (not a folder).

## `-r flag requires -i with an input image file` / `only works with a single input file, not multiple` / `not a folder`

`-r` needs exactly one `-i` pointing at a single file. It doesn't work with:
- No `-i` (no input to preserve the name of)
- Multiple `-i` (which filename would be used?)
- `-i ./folder/` (same ambiguity)

## `failed to load references: reference path does not exist: <path>`

The `-i` path doesn't exist. Check the path. Tilde (`~`) expansion IS supported, so `~/Pictures/foo.png` works.

## `unsupported image format: .xyz`

Only `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp` are accepted as reference images. Convert first:

```bash
sips -s format jpeg input.heic --out input.jpg   # macOS
```

## `no images found in reference directory: <path>`

The `-i` folder exists but has no supported images in it (non-recursive — subfolders aren't scanned). Check that the images are in the top level of the folder.

## Vertex: `failed to create Vertex AI client`

- `gcloud auth application-default login` hasn't been run, OR
- the `gcp_project` in config is wrong / doesn't have the Vertex AI API enabled.

Fix:
```bash
gcloud auth application-default login
gcloud services enable aiplatform.googleapis.com --project=<project-id>
```

Then verify the project id in `~/.config/imagine/config.yaml`:
```yaml
providers:
  vertex:
    provider_options:
      gcp_project: your-actual-project-id
```

## OpenAI: 403 / "Organization verification required"

OpenAI requires organization verification for GPT Image models. Complete the verification at https://platform.openai.com/settings/organization/general.

## OpenAI: 429 rate limit

Happens at high `-n` values or short loops. imagine doesn't auto-retry. Options:

- Wait and re-run.
- Reduce `-n`.
- Upgrade the OpenAI tier.

## Ctrl+C hangs

It shouldn't. imagine uses context cancellation; in-flight HTTP requests abort when you press Ctrl+C. If a command is stuck after SIGINT, it's a bug — report it.

## `imagine --version` prints "dev" on a built release

The release-workflow ldflags injection was broken at some point — `main.version` must be the target. If the binary came from `go install` rather than a tagged release, it prints the commit info from `debug.ReadBuildInfo` or falls back to "dev".

## Help output looks wrong for a provider

`imagine --help` is provider-aware — it renders the active provider's flags and hides the other providers'. If the active provider isn't what you expected:

- Check `imagine providers show` to see what's configured and which is active.
- Override for this invocation: `imagine --provider openai --help`.
- Set an explicit default: add `default_provider: openai` to config.yaml.
