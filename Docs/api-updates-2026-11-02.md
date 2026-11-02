# API updates - OpenAI, Gemini API, Vertex AI (2026-11-02)

Research date: 2026-11-02. Method: Firecrawl scrapes of the OpenAI developer and Codex docs, the Gemini API docs (models, changelog, deprecations, pricing), the Vertex AI model cards, lifecycle page and release notes, and the go-genai and adk-go release pages. Builds on `Docs/api-updates-2026-09-09.md` (OpenAI) and `Docs/api-updates-report.md` (Google, 2026-08-07).

## What this branch changed

| Area | Change | Live-verified |
|---|---|---|
| SDKs | `google.golang.org/genai` v1.43.0 -> v1.77.0; adk-go v0.3.0 -> `google.golang.org/adk/v2` v2.6.0 | describe on both platforms |
| OpenAI text model | `gpt-6.1-sol` replaces the retired `gpt-5.5` as the describe default (both routes) and the ChatGPT sign-in driver model; Free and Go sign-ins get `gpt-6-luna`, read from the token's plan. Efforts `low, medium, high, xhigh` | describe on both routes; sign-in image path not yet |
| Gemini/Vertex describe | default `gemini-3.8-flash` on both; Vertex drops `minimal`; `gemini-4-argon` via `-m` | both platforms |
| Gemini/Vertex image | `flash` = `gemini-nano-banana-2.1`; 3.1 Flash Image IDs resolve to it; `512` gone; `--thinking medium` (flash only); Vertex 2.1 checked for `global`; `-I` on Vertex `flash` | both platforms (`-t medium`, `-g`, `-I -g`) |
| OpenAI image | `--background transparent` (png/webp only); dated snapshots accepted and sent as written | API-key route |
| Gemini API | optional `providers.gemini.store: false` | generate and describe |
| CLI | absolute `-f` saves where it points; error text keeps flag names as written | end to end |

## OpenAI (since 2026-09-09)

1. **`gpt-5.5` retired from Codex for ChatGPT sign-in on 2026-10-14, 19:00 UTC.** Replacement on Plus, Pro, Business, Enterprise and Edu: `gpt-6.1-sol`; Free and Go: `gpt-6-luna`. API-key use is not affected (https://learn.chatgpt.com/docs/models#gpt-55-retirement). This broke the sign-in route's describe and driver model in v1.3.8.
2. **`gpt-6.1-sol` reasoning effort.** The model page lists `low, medium (default), high, xhigh, max` (https://developers.openai.com/api/docs/models/gpt-6.1-sol), but Chat Completions rejects `max` for it ("Supported values are: 'low', 'medium', 'high', and 'xhigh'", live, 2026-11-02). It does not support `none`.
3. **No new image models.** `gpt-image-2.5-flare` and `gpt-image-2.5-sunburst` are still the newest; pricing is unchanged.
4. **Shutdowns.** `gpt-image-1` shut down 2026-10-23; `gpt-image-1-mini`, `gpt-image-1.5` and `chatgpt-image-latest` shut down 2026-12-01 (https://developers.openai.com/api/docs/deprecations). None are in the repo.
5. **Transparent backgrounds.** `background: transparent` is in the generate and edit references for the GPT image models (https://developers.openai.com/api/reference/resources/images/methods/generate).
6. **Dated snapshots.** The references list `gpt-image-2.5-flare-2026-09-08`, `gpt-image-2.5-sunburst-2026-09-08` and `gpt-image-2-2026-04-21`.
7. **New edit parameter `markup`.** A transparent PNG overlaid on the first image, GPT Image 2.5 only, not combinable with `mask` (https://developers.openai.com/api/reference/resources/images/methods/edit). Not implemented.
8. **`user` deprecated in favour of `safety_identifier`.** The repo sends neither.
9. **Rate-limit tiers renamed** Free / Build / Launch / Grow. The 2.5 models allow 20 / 150 / 250 images per minute on Build / Launch / Grow; Free has no access (https://developers.openai.com/api/docs/models/gpt-image-2.5-flare).
10. **Codex image model.** The Codex docs still say built-in image generation "uses `gpt-image-2`" (https://learn.chatgpt.com/docs/image-generation) and the Codex CLI still hardcodes `IMAGE_MODEL = "gpt-image-2"`. The sign-in route's 2.5 behaviour needs a new live probe.

## Gemini API and Vertex AI (since 2026-08-07)

1. **`gemini-nano-banana-2.1`** (GA 2026-10-06, both platforms). Google's named replacement for `gemini-3.1-flash-image`. Sizes `1K, 2K, 4K` (no `512`); thinking `minimal, medium (default), high`; web and image search grounding; Vertex: `global` only, and `seed`, `temperature`, `topP`, `topK` return errors. Image output $30/M tokens, about half the per-image price of 3.1 Flash Image (https://ai.google.dev/gemini-api/docs/models/gemini-nano-banana-2.1, https://ai.google.dev/gemini-api/docs/pricing).
2. **`gemini-3.1-flash-image`** has no shutdown date on the Gemini API; Vertex keeps it to "May 28, 2027 or later".
3. **Text models.** `gemini-3.8-flash` (GA 2026-09-02; Vertex to 2027-09 or later) and `gemini-4-argon` (GA 2026-10-13, flagship) both reject `minimal` thinking. `gemini-3.7-flash` and `gemini-3.5-flash` are deprecated; `gemini-3.6-flash` retires on Vertex on 2026-11-19.
4. **Request storage.** Since 2026-10-26 new paid Gemini API projects store generateContent requests by default; `store=false` opts a request out (https://ai.google.dev/gemini-api/docs/changelog). Vertex rejects the field.
5. **Vertex image search on 2.1 works** (verified live, 2026-11-02). The Vertex model card lists it; the Vertex image guide is silent. With `searchTypes{webSearch, imageSearch}` Vertex returned `imageSearchQueries` and 6 to 7 grounding chunks in two of two calls, matching the Gemini API. With `imageSearch` alone neither platform returned grounding metadata for the test prompt. A 429 `RESOURCE_EXHAUSTED` on 2.1 cleared on retry (shared quota).
6. **Imagen** is shut down on both platforms. Not used by the repo.
7. **SDKs.** go-genai v1.44 to v1.77 list no breaking changes; v1.76 added `store`. adk-go's current line is v2 (v2.6.0 adds resume of long Gemini 4 answers); v1.9.0 is a maintenance branch.

## Open items

1. **ChatGPT sign-in route** (probed live 2026-11-02 on a Business plan): generation and describe work with the `gpt-6.1-sol` driver. The backend still runs `gpt-image-2-codex` whatever `-m` says and resets `size` to `auto`; a size stated in the instructions is followed. Follow-ups: pass the size through the instructions, and warn when the model is swapped.
2. **`markup` edit parameter** (OpenAI) is not implemented.
