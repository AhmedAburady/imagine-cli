# API Updates Report - Gemini & OpenAI

Research date: 2026-08-06. Method: Firecrawl scrapes of official docs, API references, changelogs, and deprecation pages (raw captures in `.firecrawl/`), plus the OpenAI documented OpenAPI spec.

## 1. What the app uses today (baseline)

| Provider | Path | Models | Request shape |
|---|---|---|---|
| `gemini` | `POST generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | `gemini-3-pro-image` (default), `gemini-3.1-flash-image`, `gemini-3.1-flash-lite-image` | `generationConfig.imageConfig{aspectRatio,imageSize}` (1K/2K/4K, 14 ratios), `thinkingConfig.thinkingLevel` (MINIMAL/HIGH), tools `{googleSearch:{}}` and standalone `{imageSearch:{}}` |
| `vertex` | genai SDK `GenerateContent` | same three | SDK `ImageConfig{AspectRatio,ImageSize}`, `ThinkingConfig.ThinkingLevel`, `Tool{GoogleSearch}`; no image-search |
| `openai` (API key) | `/v1/images/generations` (JSON), `/v1/images/edits` (multipart `image[]`) | `gpt-image-2` (default), `gpt-image-1.5`, `gpt-image-1`, `gpt-image-1-mini`, `chatgpt-image-latest` | `model, prompt, n(<=10), size, quality, output_format, output_compression, moderation, background`; edit sizes restricted client-side to 1024x1024 / 1536x1024 / 1024x1536 / auto; `gpt-image-2` + transparent background blocked client-side |
| `openai` (subscription) | `chatgpt.com/backend-api/codex/responses` (SSE) | same, via `image_generation` tool, driver model `gpt-5.5` | unofficial/undocumented route |
| describe | Gemini: `gemini-pro-latest`; Vertex: `gemini-3-flash-preview`; OpenAI: `gpt-5.5` (`/chat/completions`, efforts none/low/medium/high/xhigh) | | |

## 2. Breaking / urgent

### 2.1 OpenAI deprecated 4 of the 5 image models the app lists

Announced 2026-06-02 ([deprecations](https://developers.openai.com/api/docs/deprecations)):

| Model in app | Shutdown | Replacement |
|---|---|---|
| `gpt-image-1` | **Oct 23, 2026** | `gpt-image-2` |
| `gpt-image-1.5` | **Dec 1, 2026** | `gpt-image-2` |
| `gpt-image-1-mini` | **Dec 1, 2026** | `gpt-image-2` |
| `chatgpt-image-latest` | **Dec 1, 2026** | `gpt-image-2` |

`dall-e-2`/`dall-e-3` were already shut down 2026-05-12 (not used by the app). `gpt-image-2` has a dated snapshot `gpt-image-2-2026-04-21` for pinning.

Action: mark the four models deprecated in `Info().Models` (mirror the Gemini `DeprecatedAliases` pattern), and surface a warning when they are selected. After the shutdown dates, drop them.

### 2.2 Gemini: image-search tool shape changed in the docs

Current generateContent docs show image search nested inside the google_search tool:

```json
"tools": [{"google_search": {"searchTypes": {"webSearch": {}, "imageSearch": {}}}}]
```

The app sends a standalone sibling tool `{"imageSearch": {}}` (providers/gemini/gemini.go). The standalone shape no longer appears in official docs. It may still be accepted server-side, but this needs an empirical check; if it 400s, switch to the nested `searchTypes` form. Interactions API equivalent: `tools=[{"type": "google_search", "search_types": ["web_search", "image_search"]}]`.

### 2.3 Vertex: grounding on `gemini-3.1-flash-image` listed as Not supported

The Vertex model card for `gemini-3.1-flash-image` (updated 2026-08-03) lists Grounding as **Not supported**, while the `gemini-3-pro-image` card lists Google Search grounding as **Supported**. The app's Vertex provider advertises `--grounding` for both pro and flash. Model cards have been inaccurate before, so verify with a real call; if accurate, move `grounding` out of flash's `SupportedFlags` on Vertex only.

## 3. New API shapes and parameters worth adopting

### 3.1 Gemini Interactions API (GA, recommended by Google)

`POST https://generativelanguage.googleapis.com/v1beta/interactions`. Google's image-generation docs now lead with it; `generateContent` "remains fully supported" but Interactions is recommended for all new development.

Shape: `{model, input: [{type:"text"|"image"|"video", ...}], response_format: {type:"image", aspect_ratio, image_size, mime_type}, tools: [{type:"google_search", search_types:[...]}], generation_config: {thinking_level}, previous_interaction_id}`.

New capabilities it unlocks: multi-turn editing via `previous_interaction_id` (no need to resend prior images), video input, interleaved text+image output, server-side state. Note the [May 2026 breaking change](https://ai.google.dev/gemini-api/docs/interactions-breaking-changes-may-2026): responses moved from `outputs` to a `steps` schema. Migration guide: [migrate-to-interactions](https://ai.google.dev/gemini-api/docs/migrate-to-interactions).

No action required now (generateContent stays supported); track as the future direction.

### 3.2 Gemini: new `512` (0.5K) image size on flash

`ImageConfig.imageSize` documented values are now `512`, `1K`, `2K`, `4K` ([API reference](https://ai.google.dev/api/generate-content#ImageConfig)); the 3.1 Flash size table includes a 512px column (~0.25MP, 747 tokens). Vertex card: "512, 1K, 2K, 4K (Preview)". The app's `-s` enum is `1K,2K,4K` only.

Action: add `512` to Sizes for `gemini-3.1-flash-image` (and flash-lite? - flash-lite is documented as 1K-only, so no). Value is the literal string `"512"`, not `0.5K`.

### 3.3 Gemini: `generationConfig.responseFormat` (new, alongside imageConfig)

generateContent's `GenerationConfig` gained `responseFormat` (`ResponseFormatConfig`) with an `image` sub-object (`ImageResponseFormat`): `mimeType`, `delivery` (`INLINE` or `URI`), `aspectRatio`, `imageSize` (as enums). This allows requesting a specific output MIME type and URI delivery instead of inline base64. The legacy `imageConfig` is **not** deprecated in generateContent (it is only deprecated inside the Interactions API's own GenerationConfig), so the current app shape is still valid. Vertex docs still show `imageConfig` as well.

Optional adoption: `mimeType` could replace the "first image/* part" sniffing; `delivery: URI` avoids large base64 payloads.

### 3.4 Gemini: aspect-ratio set differences (9:21)

- Gemini API `ImageConfig` docs: 14 ratios (matches the app exactly).
- Vertex model cards (both pro and flash): 15 ratios, adding **`9:21`**.
- Per-model docs tables: pro table lists only 10 ratios; flash-lite "adds" 10 ratios.

The app already notes "Google's published per-model tables disagree". New data point: Vertex accepts `9:21`. Consider adding `9:21` for Vertex (verify empirically on the direct Gemini API before adding it there).

### 3.5 Gemini: video-to-image input (flash only)

Since 2026-05-28, `gemini-3.1-flash-image` accepts a video file (or one public YouTube URL) as input context. The app's reference-image plumbing (`-i`) is image-only; supporting video would be a new feature, gated to flash.

### 3.6 OpenAI: streaming image generation (`stream`, `partial_images`)

Both `/v1/images/generations` and `/v1/images/edits` now accept `stream: true` and `partial_images` (0-3), returning SSE events `image_generation.partial_image` and `image_generation.completed` (confirmed in the OpenAPI spec `CreateImageRequest`/`CreateImageEditRequest`). The app already has SSE machinery (`streamClient`, `scanSSE`) for the subscription route, so progress streaming on the API-key route is cheap to add. Each partial image costs +100 output tokens.

### 3.7 OpenAI: edit-size client restriction is outdated for gpt-image-2

The app hard-rejects edit sizes other than 1024x1024 / 1536x1024 / 1024x1536 / auto. Current spec text for `size` on the edits endpoint: for `gpt-image-2` (and `gpt-image-2-2026-04-21`), arbitrary `WIDTHxHEIGHT` is supported - both edges divisible by 16, aspect ratio between 1:3 and 3:1, max edge 3840px, total pixels 655,360-8,294,400 (resolutions above 2560x1440 experimental). The 3-size limit still applies to the 1.x models.

Action: scope the edit-size validation per model; allow arbitrary valid WxH when `model == gpt-image-2*`.

### 3.8 OpenAI: `input_fidelity` on edits

New `input_fidelity` parameter controls how strongly input details are preserved. For `gpt-image-2` the API does not allow changing it (always high fidelity) - the app correctly omits it. No change needed; do not start sending it for gpt-image-2.

### 3.9 OpenAI: `moderation_details` on blocked errors

`moderation_blocked` errors may now carry `error.moderation_details{moderation_stage: input|output|unknown, categories: [...]}`. The app's error path surfaces the message only; parsing this object would improve the CLI's moderation error output.

## 4. Confirmed still-correct behavior

- `gpt-image-2` does not support `background: transparent` - the app's client-side guard matches the docs ("Requests with background: transparent aren't supported for this model").
- Gemini `thinkingLevel` enum is `MINIMAL/LOW/MEDIUM/HIGH` (uppercase) - the app's canonicalization matches; per image docs, flash/lite support `minimal` (default) and `high`. Thinking cannot be disabled on Gemini 3 image models.
- `n` max 10 on `/v1/images` - matches `MaxBatchN: 10`.
- `output_format`/`output_compression`/`moderation` shapes unchanged; `response_format` remains DALL-E-only (app doesn't send it - correct).
- `gpt-5.5` (describe default + subscription driver) is current, not deprecated, vision-capable; efforts `none, low, medium (default), high, xhigh` match the app's `Vision.Efforts`.
- Vertex `gemini-3-flash-preview` (describe default) still available, no shutdown date; recommended successor is `gemini-3.6-flash` (GA 2026-07-21).
- Gemini `gemini-pro-latest` alias family still documented; current pro is `gemini-3.1-pro-preview` (GA successor pending).
- Preview image model IDs the app maps (`gemini-3-pro-image-preview`, `gemini-3.1-flash-image-preview`) shut down 2026-06-25 - already past; the alias mapping keeps old configs working.
- `gemini-2.5-flash-image` (not used by the app) shuts down 2026-10-02. Imagen 4 family shut down 2026-08-17. Not app-relevant.

## 5. Housekeeping notes

- Gemini REST examples moved from `v1beta` to `v1` (`/v1/models/{model}:generateContent`) - the models are GA. v1beta still works; consider switching the base URL to v1.
- Gemini sampling params `temperature`, `top_p`, `top_k` deprecated (2026-07-21) - the app never sends them; no action.
- OpenAI docs moved from platform.openai.com/docs to developers.openai.com/api/docs - update doc links in README/help text.
- Vertex docs moved under `docs.cloud.google.com/gemini-enterprise-agent-platform/` - same for doc links.
- On Vertex, 4K output is still marked Preview for both pro and flash.
- Newer OpenAI text flagship exists: GPT-5.6 family (`gpt-5.6` -> `gpt-5.6-sol`, plus `gpt-5.6-terra`, `gpt-5.6-luna`), vision-capable. Candidates to replace `gpt-5.5` as describe default/subscription driver later; also note the new `reasoning.mode: "pro"` parameter shape appearing in deprecation guidance.
- OpenAI Responses API now has an `input_image_mask` option on the `image_generation` tool (mask by file_id) - subscription-route only, undocumented for the Codex backend the app uses.

## 6. Suggested priority order

1. (2.1) Deprecate/warn on `gpt-image-1`, `gpt-image-1.5`, `gpt-image-1-mini`, `chatgpt-image-latest` - hard shutdowns Oct 23 / Dec 1, 2026.
2. (2.2) Test `--image-search` on Gemini flash; migrate to nested `searchTypes` if the standalone tool fails.
3. (2.3) Test `--grounding` on Vertex flash; adjust `SupportedFlags` if it errors.
4. (3.7) Relax edit-size validation for `gpt-image-2`.
5. (3.2) Add `512` size for Gemini flash.
6. (3.6) Optional: `stream`/`partial_images` progress on the OpenAI API-key route.
7. (3.1) Track Interactions API for a future provider revision.

## Sources

- [Gemini image generation (Interactions API)](https://ai.google.dev/gemini-api/docs/image-generation)
- [Gemini image generation (generateContent API)](https://ai.google.dev/gemini-api/docs/generate-content/image-generation)
- [Gemini API reference - generateContent](https://ai.google.dev/api/generate-content) (GenerationConfig, ImageConfig, ResponseFormatConfig, ThinkingLevel)
- [Gemini deprecations](https://ai.google.dev/gemini-api/docs/deprecations)
- [Gemini release notes](https://ai.google.dev/gemini-api/docs/changelog)
- [Migrate to Interactions API](https://ai.google.dev/gemini-api/docs/migrate-to-interactions) / [Interactions breaking changes May 2026](https://ai.google.dev/gemini-api/docs/interactions-breaking-changes-may-2026)
- [Gemini models](https://ai.google.dev/gemini-api/docs/models)
- [Vertex: Gemini 3.1 Flash Image card](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/gemini/3-1-flash-image) / [Gemini 3 Pro Image card](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/gemini/3-pro-image) / [Google models on Vertex](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/google-models) / [Vertex image generation guide](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/capabilities/image-generation)
- [OpenAI image generation guide](https://developers.openai.com/api/docs/guides/image-generation)
- [OpenAI deprecations](https://developers.openai.com/api/docs/deprecations)
- [OpenAI models](https://developers.openai.com/api/docs/models) / [gpt-image-2](https://developers.openai.com/api/docs/models/gpt-image-2) / [gpt-5.5](https://developers.openai.com/api/docs/models/gpt-5.5) / [gpt-5.6-sol](https://developers.openai.com/api/docs/models/gpt-5.6-sol)
- OpenAI documented OpenAPI spec (app.stainless.com/api/spec/documented/openai/openapi.documented.yml): `CreateImageRequest`, `CreateImageEditRequest`
