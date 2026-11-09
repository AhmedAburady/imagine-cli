# Code Review — stacked `feat/video-seedance` + `feat/video-jobs` against `main`

**Date:** 2026-08-13
**Scope:** local-only. Topology: `feat/video-jobs` is stacked on `feat/video-seedance` (verified: `git merge-base --is-ancestor feat/video-seedance feat/video-jobs` → yes; both share merge-base `8f56db0` with `main`).
**Sizes:**
- `main..feat/video-seedance`: 55 files, +3,982/−168 (12 commits)
- `feat/video-seedance..feat/video-jobs`: 21 files, +1,684/−10 (4 commits)
- `main..feat/video-jobs` (cumulative): 68 files, +5,666/−178 (16 commits)

**Focus:** integration of the two layers, cross-branch consistency, duplication between the branches and against pre-existing code, and the cumulative efficiency picture.

**Verdict:** The stack is layered correctly and the boundary is clean: the seedance branch is the *provider/storage foundation*, the jobs branch is the *orchestration layer*, and the coupling between them is one function call and ~40 lines of shared plumbing — exactly what a good stack looks like. One confirmed bug (jobs layer), a handful of medium/low items. Mergeable after the High/Medium fixes; the rest is polish.

---

## 1. Stack architecture assessment

```
main
 └─ feat/video-seedance   provider + storage foundation (modelark, storage brick,
 │                        media-neutral framework, status reporting, gates)
 └─ feat/video-jobs       orchestration (registry, detached workers, ps monitor,
                          task-ID/phase/output persistence hooks)
```

**Layering is correct:**
- Jobs consumes seedance's extension points rather than duplicating them: the status reporter introduced in seedance (`providers.WithStatus`/`ReportStatus`) is reused by the worker for phase persistence; the new `ReportTaskID` follows the identical context-reporter pattern. The only change jobs makes inside modelark is **one line** (`providers.ReportTaskID(ctx, created.ID)`).
- No functionality is implemented twice across the two layers: dispatch does not re-implement validation or generation (argv re-execution), and `ps` does not re-implement provider logic (it renders registry state).
- Renames done in seedance (`Images`→`Assets`, `NumImages`→`NumAssets`) are applied consistently in jobs (no stale `Image`/`NumImages` references in the jobs-only files — verified by grep).

**Cost of the boundary:** jobs cannot introspect a *resolved* request (it stores argv + resolved prompt only), so `ps` has no per-entry detail for batch jobs, and `Model` is `""` for batch rows. Acceptable trade-off, worth a sentence in `Docs/jobs.md`.

## 2. Cross-branch consistency findings

### 2.1 [Low] Phase strings are a bare-string contract between the layers
`modelark` reports `"queued"`/`"running"`/`"downloading"`; `ps`'s `statusSummary`/`statusCell` and the orchestrator UI special-case the same literals. Any future provider using a different word (e.g. `"rendering"`) silently degrades the UI (falls into the "anything else" bucket — which does render, to its credit). Consider a shared `providers.Phase` type/const set (e.g. `PhaseQueued`) so drift is a compile error.

### 2.2 [Low] "recorded as they save" overclaim (see jobs review 3.2)
The docs describe behaviour the code doesn't have; this is a *stack* issue because the per-save hook point would live in seedance's orchestrator (`api.RunGeneration`), consumed by the jobs layer. Fix in whichever branch lands first.

### 2.3 [Info] Status channel vs reporter — two mechanisms, one purpose
Seedance ships `params.Status chan StatusUpdate` (UI, non-blocking, droppable) and jobs ships a context reporter (persistent, blocking DB write). They coexist cleanly and serve different consumers — the design comment in `orchestrator.go` explains the split. No action needed; flagged only because the dual mechanism is easy to confuse for future contributors.

## 3. Cumulative findings (whole stack, `main..feat/video-jobs`)

### Confirmed bug
| # | Severity | Where | Issue |
|---|---|---|---|
| 1 | **High** | `internal/jobs/jobs.go` `AppendTaskID` | Read-modify-write lost update under concurrency. Reproduced: 20 concurrent appends → 1–4 survive. Triggered in practice by `-n 4` ModelArk runs (4 parallel task creations). See jobs review §3.1 for fix options. |

### Medium
| # | Where | Issue |
|---|---|---|
| 2 | `commands/run.go` + `Docs/jobs.md` | Outputs recorded only at run end; docs claim per-save recording (jobs review §3.2). |
| 3 | `internal/storage/storage.go` `UploadWith` | Waiting goroutine ignores its own ctx cancellation (seedance review §3.1). |
| 4 | `internal/storage/storage.go` `Delete` | Ignores HTTP status; returns nil on 4xx/5xx (seedance review §3.2). |
| 5 | `internal/transport/transport.go` `GetBytes` | Full in-memory buffering of up-to-200 MB downloads, no cap, error bodies fully read (seedance review §3.3). |
| 6 | `internal/jobs` liveness | PID-reuse can pin a dead job "running"; 30s grace can transiently mislabel slow-starting workers (jobs review §3.3, §3.7). |
| 7 | Storage schema | 4 manual mirrors of the storage config schema (`config.StorageConfig`, `ResolveStorage`, `commands.storageFields`, `storageConfigToMap`) — drift risk (seedance review §4.1). |
| 8 | `providers/flagspec` | `Read` vs `Parse` duplication + inconsistent malformed-default handling (panic vs silent ignore) (seedance review §4.2). |

### Low (grouped)
- **Efficiency:** double SHA-256 per upload; unbounded dedup map; `Configured()` config re-read per batch entry; `ps` re-opens DB + schema every tick; `readLogTail` reads whole log (seedance §5.1–5.3, jobs §5.1–5.2).
- **Edge cases:** download stall has no timeout; `--task-type` blocked on single-image 2.5 requests; UTF-8 truncation in error messages; explicit-empty flags bypass enums; collision detection assumes `.png`; pending jobs unstoppable; stop-vs-success status race (seedance §3.4–3.9, jobs §3.4–3.9).
- **Maintainability:** dual UI stacks (v1 + v2) extended by this stack; `ps.go` borrows helpers from `genprogress.go` (jobs §4.1–4.2).

## 4. Verification performed (all local, no remote)

| Check | Result |
|---|---|
| Stack topology (`merge-base`, `--is-ancestor`) | ✅ jobs is a strict superset; seedance is the parent |
| `go build ./...` on `feat/video-jobs` and on a `feat/video-seedance` worktree | ✅ both pass |
| `go vet ./...` on both branches | ✅ clean |
| `go test ./...` on both branches | ✅ all packages pass |
| `go test -race` on `internal/jobs` | ✅ passes — the §3.1 bug is logical, not a data race, so race detection does **not** catch it (repro done via a throwaway test) |
| `git grep` for stale renames / duplicated implementations across layers | ✅ none found |

## 5. Merge recommendations (priority order)

1. **Fix `AppendTaskID`** (High) — one-liner with a mutex or `json_insert`; add a concurrency test (the repro above belongs in `jobs_test.go`).
2. **Align outputs recording with docs** (Medium) — either call `RecordOutput` from the save path in `api.RunGeneration` via a hook, or correct `Docs/jobs.md`.
3. Storage robustness trio (Medium) — ctx-aware dedup wait, `Delete` status check, bounded download memory.
4. Storage schema single-source (Medium) — pick one declaration, derive the rest.
5. Schedule a follow-up: converge on one UI stack, shared phase constants, and the remaining Low items.

Both branches individually are in good shape — the seedance layer is the stronger of the two on raw code quality; the jobs layer is architecturally impressive but needs the concurrency fix before it can be trusted with parallel video jobs.
