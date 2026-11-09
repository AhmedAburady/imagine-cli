# Code Review — `feat/video-seedance` (individual, vs `main`)

**Date:** 2026-08-13
**Scope:** local-only, `git diff main..feat/video-seedance`
**Commits:** 12 (eef3d16 → 2346cf9) · **Size:** 55 files, +3,982/−168
**Focus:** static analysis of code quality, maintainability, edge cases, duplication, efficiency/performance.

**Verdict:** Strong branch. Well-architected, heavily commented, well-tested. No high-severity issues found. The findings below are mostly medium/low — robustness gaps around the storage brick and download path, plus some maintainability duplication.

---

## 1. What the branch does

- **BytePlus ModelArk video provider** (`providers/modelark/`) — Seedance 2.0/2.5/fast/mini via the direct REST API, with a per-model spec table as single source of truth for sizes, durations, reference caps, and 2.5-only flags.
- **S3-compatible storage brick** (`internal/storage/`) — pure-stdlib SigV4 signer, content-addressed public uploads for references (ModelArk fetches references server-side, rejects Base64), `imagine storage set/show/test/clear`.
- **Framework extensions** — media-neutral renames (`Images` → `Assets`, `NumImages` → `NumAssets`), image/video/audio reference loading, provider status reporting + token surfacing, `RequireStorage` gate, `flagspec` `path:"file"` tag support, batch-relative path resolution, per-model flag gating (`gate.go`), per-provider `-n` caps.

## 2. What's good (keep doing this)

- **Single source of truth:** `modelSpecs` in `options.go` drives `Info.Models`, capability lists, flag validation, and reference caps. Adding a model touches one table.
- **Deliberate, documented trade-offs:** create-retry only on 429 (POST not idempotent), poll GET retried on 5xx/429/network (paid tasks must not be orphaned), unknown model → permissive spec → server validates. Each constant explains *why*.
- **Error messages carry recovery info:** poll errors embed the task ID (find it in the ModelArk console); storage errors tell the user to run `imagine storage set`.
- **Framework design:** `gate.go` centralises flag/model gating so CLI and batch share one rule set; `Bundle.RequireStorage` makes the storage gate generic for future providers; `RequestLabeler`/`ResolvedModeler` split is clean.
- **Testing:** 12 behavioural tests on the modelark lifecycle (happy path, wire bodies, retries, cancel, modality matrix), AWS SigV4 test vectors in `storage_test.go`, flagspec round-trips. Full suite passes locally: `go build ./...`, `go vet ./...`, `go test ./...` all green on this branch.
- **Docs:** `Docs/modelark.md`, `Docs/storage.md`, `Docs/framework-internals.md` are accurate against the code (verified claims like `-1` duration wire handling, 2.5-only knob omission, path-style rationale).

## 3. Findings — code quality & edge cases

### 3.1 [Medium] `storage.UploadWith` waiter ignores `ctx`
`internal/storage/storage.go` — a goroutine waiting on an in-flight upload (`<-r.done`) blocks even if **its own** context is cancelled. The wait lasts until the first uploader finishes or the 180s client timeout fires. With `-n 4` concurrent generations and one slow 200 MB video upload, cancel (Ctrl+C / `ps stop`) feels ignored.
*Fix:* `select { case <-r.done: …; case <-ctx.Done(): … }` and/or evict on cancellation.

### 3.2 [Medium] `storage.Delete` ignores the response status
`Delete` returns `nil` for 403/404/500 as long as the HTTP round-trip succeeded. `Test`'s cleanup silently "succeeds", and no caller can distinguish real deletion from failure. The doc says "Errors are returned but callers typically ignore them" — but they *can't* see the errors that matter.
*Fix:* check `resp.StatusCode < 200 || >= 300` like `putObject` does.

### 3.3 [Medium] `transport.GetBytes` buffers the whole download in memory
`io.ReadAll` on up-to-200 MB video responses, **before** the status check. Consequences: (a) peak memory is 2× file size (the raw buffer plus `GeneratedAsset.Data`); (b) an error response body (e.g. a large HTML page from a misconfigured URL) is fully downloaded before being discarded; (c) no size cap — a misbehaving server can stream gigabytes into RAM.
*Fix:* stream to a temp file, or `io.LimitReader` error bodies and cap the success path (or stream through `GeneratedAsset` with a reader interface later).

### 3.4 [Low] Downloads have no stall detection
`downloadClient` (`NewStreamingClient`) bounds dial/TLS/headers but imposes **no deadline on the body read**. A server that trickles bytes forever hangs `Generate` indefinitely — no poll deadline applies to the download phase.
*Fix:* progress-based idle timeout (last-byte-at > N minutes → abort), or a generous overall body deadline.

### 3.5 [Low] `--task-type` rejected for single-image input on 2.5
`buildContent` errors when `--task-type` is set with exactly one `-i` image, even though 2.5's omni task types can plausibly apply to single-image requests. Deliberate and documented in the error text, but it's restrictive; worth a doc line in `Docs/modelark.md` (currently not mentioned).

### 3.6 [Low] `truncateMsg` can split a UTF-8 rune
`transport.go` — `msg[:errBodyMaxLen-3] + "..."` slices bytes. A multi-byte character at the boundary produces invalid UTF-8 in a user-visible error. Trivial to fix by rune-aware truncation.

### 3.7 [Low] Poll failure budget off-by-one vs comment
`pollMaxFailures = 6` with the check `failures > pollMaxFailures` means **7** consecutive failures are tolerated and the 8th errors out. Comment says "budget … 6". Cosmetic only — either adjust the constant or the comparison.

### 3.8 [Low] No retry on reference uploads
`putObject` does a single PUT. Create and poll both have retry budgets, but a transient network blip on a 200 MB upload (the longest, most failure-prone call) aborts the whole generation. A bounded retry on network errors is worth considering (PUT of the same content-addressed key is idempotent).

### 3.9 [Low] Explicit-empty flag values bypass enum validation
`flagspec.Read` skips the enum check when the value is `""`. `imagine --aspect-ratio ""` silently passes validation and the ratio is then omitted on the wire (API decides). Same for batch `aspect-ratio: ""`. Defensible, but surprising; consider treating explicit-empty as invalid for enum'd fields.

## 4. Findings — duplication & maintainability

### 4.1 [Medium] Storage schema mirrored in 4 places
`config.StorageConfig` (struct + yaml tags), `ResolveStorage`'s field list, `commands.storageFields` (`[]ConfigField`), and `storageConfigToMap` all enumerate the same 8 fields. Adding a field (e.g. a future `use_https`) requires 4 coordinated edits; forgetting one desyncs `storage show`/`set`/`test` silently. *Consider* deriving the field list and map from one declaration.

### 4.2 [Medium] `flagspec.Read` and `flagspec.Parse` are ~60% duplicated
Both implement the same string/bool/int population, enum/range/path handling, and the *identical* Normalize/Validate hook-invocation block (copy-pasted 4× total). Behavioural drift has already happened: `Bind`/`Read` **panic** on a malformed `default:` tag while `Parse` **silently ignores** it (`if err == nil { v = parsed }`). Extract shared helpers (`applyHooks`, `parseDefault`, tag validation) and make error handling consistent.

### 4.3 [Low] Shorthand length not validated
`parseFlagTag` doesn't check that the shorthand is a single char. Cobra will panic at `init()` anyway, which is acceptable "loud failure", but the error is cryptic — a one-line check in `Bind` would make the programmer error obvious.

## 5. Findings — efficiency & performance

### 5.1 [Medium] Double SHA-256 per upload
`objectKey` hashes the full payload for content addressing, then `putObject` calls `hashPayload(data)` — a **second** full pass over up-to-200 MB bytes — for the SigV4 `x-amz-content-sha256` header. Hash once, reuse for both.

### 5.2 [Low] Upload dedup map grows for the process lifetime
`uploads` entries are never evicted on success (only failures). Fine for a CLI process, but a long batch with thousands of distinct references retains every URL/byte-hash entry. Add a comment acknowledging the trade-off, or bound it.

### 5.3 [Low] `storage.Configured()` re-reads config per batch entry
`resolveOne` calls `storage.Configured()` (full config load + parse) for **every** entry with references. A 100-entry batch = 100 file reads. Cache the result per batch run (it can't change mid-run meaningfully).

### 5.4 [Low] Collision detection assumes `.png`
`detectCollisions` calls `ResolveFilename` with `AssetMime` empty → `.png` names. Mixing image and video entries with the same custom `-f` stem produces a **false-positive collision** (real files would be `foo.png` + `foo.mp4`). Since MIME isn't known at resolve time, at minimum note the approximation; a stem+count check would be as effective.

### 5.5 [Info] Directory loads are unbounded-parallel
`images.loadDir` reads all files concurrently with no concurrency cap (pre-existing pattern, now amplified by 200 MB videos: a 20-video directory = 4 GB resident). Size caps in `validateReferenceConstraints` run *after* the bytes are in memory. Worth a concurrency bound and/or an early size check at load time.

## 6. Checklist

| Category | Assessment |
|---|---|
| Build / vet / tests (local) | ✅ `go build`, `go vet`, `go test ./...` all pass |
| Error handling | ✅ strong; see 3.2, 3.4 |
| Concurrency | ✅ dedup singleflight solid; see 3.1 |
| Edge cases | ✅ good coverage of API contract changes (unknown status → terminal); see 3.5–3.9 |
| Duplication | ⚠️ 4.1, 4.2 |
| Performance | ⚠️ 5.1–5.4 (all modest at CLI scale) |
| Documentation | ✅ excellent, verified against code |
| Tests | ✅ good; gaps: upload dedup/eviction, `Delete` status, truncation, `storage set` merge |
