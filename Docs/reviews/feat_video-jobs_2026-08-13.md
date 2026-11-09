# Code Review — `feat/video-jobs` (individual, vs `main`)

**Date:** 2026-08-13
**Scope:** local-only, `git diff main..feat/video-jobs`
**Commits:** 16 (eef3d16 → 76748fc) · **Size:** 68 files, +5,666/−178
**Note:** `feat/video-jobs` is stacked on `feat/video-seedance` (12 shared commits). This review covers the full cumulative diff; the 4 branch-specific commits (34a8b14 → 76748fc, `internal/jobs/*`, `commands/{ps,worker,detach}.go`) are the primary focus, with the seedance layer treated as context (see the seedance review for its findings).
**Focus:** static analysis of code quality, maintainability, edge cases, duplication, efficiency/performance.

**Verdict:** Clever, well-documented architecture (detached worker re-executing the recorded argv; SQLite WAL registry; liveness repair). One **confirmed concurrency bug** (lost task-ID updates, reproduced below) and one doc/behaviour mismatch need fixing before merge. Everything else is medium/low polish.

---

## 1. What the branch adds (on top of seedance)

- **Persistent job registry** (`internal/jobs/`) — single SQLite DB (pure-Go `modernc.org/sqlite`, WAL + busy-timeout), shared by detached workers, the dispatching CLI, and `imagine ps`.
- **Detached background dispatch** — video runs detach by default; `--detach` backgrounds anything; `--wait` forces foreground. Worker is `imagine __worker --job-id <id>`, spawned in its own session (`setsid`), stdio wired to a per-job log.
- **`imagine ps`** — TUI monitor (Bubble Tea v2) with `--plain`/`--json`, stop / rerun / delete, liveness repair for crashed workers.
- **Worker bookkeeping hooks** — `providers.WithTaskReporter`/`ReportTaskID` (modelark persists each `cgt-…` at create time), phase persistence via the existing status reporter, output/token recording.

## 2. What's good (keep doing this)

- **One code path for every run:** the worker re-executes the recorded argv through the normal root command (detach flags stripped, `--wait` appended). Dispatch, rerun, and foreground runs share validation, batching, and saving — no second serialization format for "a generation".
- **Robust registry design:** WAL + `busy_timeout` + `SetMaxOpenConns(1)` makes multi-process overlap safe without a daemon; `RepairLiveness` self-heals crashed/SIGKILLed workers; pending rows get a 30s grace period.
- **Recoverability by design:** ModelArk task IDs and phases are persisted the moment they exist, so a dead worker doesn't orphan a paid server-side task.
- **Signal handling is correct end-to-end** (verified against `fang` v2 source): `fang.WithNotifySignal(Interrupt, SIGTERM)` cancels the ctx → provider poll sees `ctx.Done()` → best-effort task cancel → row marked `stopped` (abort path writes the row *before* `os.Exit(130)`).
- **Windows/unix process primitives split** with build tags; `Stop` degrades to hard-kill on Windows with repair afterwards.
- **Docs** (`Docs/jobs.md`, README, SKILL.md) are detailed and mostly accurate; policy matrix and layout tables match the code.
- **Verified locally:** `go build ./...`, `go vet ./...`, `go test ./...` (incl. `-race` on `internal/jobs`) all pass on this branch.

## 3. Findings — code quality & edge cases

### 3.1 [High] `AppendTaskID` loses updates under concurrency — **confirmed by repro**
`internal/jobs/jobs.go`: `AppendTaskID` = `Get` (read list) → `append` in memory → `UPDATE` (write list). With `SetMaxOpenConns(1)` this is serialized per statement but **not** atomic across the read-modify-write, so two goroutines interleave Get/Get/Exec/Exec and the last write wins.
This is not theoretical: `-n 4` on modelark (`MaxBatchN: 1`) runs 4 concurrent `Generate` calls → 4 concurrent `ReportTaskID` → `AppendTaskID`.
**Repro** (throwaway test, 20 goroutines × 1 append, `-race -count=3`): got **2 / 4 / 1** surviving task IDs instead of 20 — no race-detector warning because the DB serializes access; the loss is logical.
*Fix options:* a per-ID mutex in `Store`, or a single SQL statement (SQLite JSON1 `json_insert(task_ids, '$[#]', ?)` — modernc builds with JSON1), or read `task_ids` + write inside one transaction.

### 3.2 [Medium] Outputs are recorded only at run end, not "as they save"
`RecordOutput` is called **once** in `runGeneration` after the whole run finishes (and again on the abort path). Docs (`Docs/jobs.md` §4) claim "output file paths (recorded per asset as they save — partial results included)".
Consequences: (a) during a long `-n 4` video run, `ps` shows zero outputs until the last task completes; (b) if the worker is SIGKILLed mid-run, already-saved files exist on disk but the row records none — `RepairLiveness` then marks it failed with no artifact list. The `ActiveJob.RecordOutput` API was clearly designed for per-save calls (it persists running totals). Wire it at the save site (an orchestrator callback) or correct the docs.

### 3.3 [Medium] PID-reuse can pin a dead job as "running" forever
`processAlive` = `kill(pid, 0)` (unix). If the worker died and the PID was recycled by an unrelated process, `RepairLiveness` never repairs the row. The code comment acknowledges this ("acceptable for a per-user CLI registry") — reasonable for a local tool, but consider also recording the worker's process start time and comparing (Linux `/proc/<pid>/stat`) for a cheap fix, or an age-based fallback ("running > 48h with no phase change → failed").

### 3.4 [Low] SQLite DSN is unescaped
`Open()` builds `"file:" + filepath.Join(dir, "jobs.db") + "?_pragma=…"`. A data dir containing `?` or `#` breaks URI parsing; Windows `file:C:\Users\…` backslash paths are untested against modernc's URI parser. Use `url.PathEscape` on the path component, and add a Windows CI run.

### 3.5 [Low] Pending jobs can't be stopped from `ps`
`stopJobAction` returns "no worker PID yet" for `pending` rows (PID set only at `MarkRunning`). A user who spots a stuck pending job has no action available. Consider deleting/aborting pending rows directly from `ps`.

### 3.6 [Low] Stop-vs-success race mislabels the outcome
`runWorker`: `case ctx.Err() != nil:` takes precedence over `err == nil`. If a stop signal lands in the same instant a run completes successfully, the job is recorded `stopped` with outputs present. Cosmetic, but the outputs-vs-status mismatch may confuse `ps` readers. Prefer checking terminal evidence (all results present) before `ctx.Err()`.

### 3.7 [Low] `RepairLiveness` 30s grace can mislabel a slow-starting worker
A heavily loaded machine where the worker's `Open()+Get()+MarkRunning()` takes >30s after `Create` gets the row marked `failed: worker process died` by a concurrent `ps` — the worker then overwrites it when it finishes. Self-heals, but `ps` displays a transient lie. Cheap fix: write the pid at `Create` time (spawn returns it; row is created *after* spawn in `Launch`… actually the row is created *before* spawn — reorder so `pid` is known when the row is inserted, and skip pending rows with a live pid).

### 3.8 [Low] Worker strips only literal `--detach` spellings
`workerArgs` removes `--detach`, `--detach=true`, `--detach=false` but not pflag-valid variants like `--detach=1`/`--detach=t`. Harmless today because the appended `--wait` always wins in `shouldDetach` — but it's a fragile invariant; a note in `workerArgs` or stripping by prefix would make it explicit.

### 3.9 [Low] TUI details pane shows stale content when the row disappears
`psModel.Update` refreshes the table each tick; if the selected job vanishes (deleted elsewhere), `m.selected()` returns nil and the details viewport keeps rendering the old job. Minor.

## 4. Findings — duplication & maintainability

### 4.1 [Medium] Two UI framework stacks in one binary
`ps.go`/`genprogress.go`/`ui.go`/`main.go` use `charm.land/*/v2`, while `providers.go`, `providers_add.go`, `describe.go`, and `internal/batch/run.go` still use `charmbracelet/*` v1. This predates the branch (main already shipped v2), but the jobs branch **extends** the split by adding more v2 consumers instead of converging. Result: two lipgloss, two bubbles, and a pseudo-versioned v1 bubbles module in `go.mod`; larger binary; two APIs to know. Recommend a follow-up migration of the remaining v1 consumers (they're mostly `huh` forms — check huh's v2 status).

### 4.2 [Low] `ps.go` depends on helpers living in `genprogress.go`
`tickMsg`/`tickCmd`/`fmtDuration` are defined in `genprogress.go` and consumed by `ps.go`. Same package, so it compiles — but the job monitor's liveness depends on the progress UI's file. Moving shared tick/format helpers to `ui.go` would express intent better.

### 4.3 [Low] `Store.List` filters in Go after loading the whole table
`SELECT *` + in-Go filtering of terminal jobs older than 24h. Fine at CLI scale (hundreds of rows), but the SQL `WHERE` version is trivial and scales.

## 5. Findings — efficiency & performance

### 5.1 [Low] `ps` re-opens SQLite (and re-runs schema) every second
The TUI tick reloads via `loadJobsList` → `jobs.Open()` → `MkdirAll` + `sql.Open` + `db.Exec(schema)` + `RepairLiveness` (a `kill(0)` per running job), every second, forever. Acceptable locally, but wasteful: keep one `Store` open per TUI session (WAL readers are cheap) and run `RepairLiveness` on a slower cadence or on entry only.

### 5.2 [Low] `readLogTail` reads the entire log file
`os.ReadFile` then takes the last 20 lines. A multi-hour worker log is MBs — fine, but a seek-based tail (or a cap) would be kinder to the details pane on every refresh.

### 5.3 [Info] `NewID` randomness is 3 bytes (16.7M/sec space)
Collision risk is negligible for a personal CLI, and the format is sortable — fine. Worth a comment for future readers who scale it up.

### 5.4 [Info] Dependency choice is sound
`modernc.org/sqlite` keeps the build CGO-free; the indirect adds (`libc`, `mathutil`, `memory`) are the standard cost of that choice. `go 1.26.0` is already the module's floor.

## 6. Checklist

| Category | Assessment |
|---|---|
| Build / vet / tests (local) | ✅ `go build`, `go vet`, `go test ./...` (+`-race` on `internal/jobs`) pass |
| Concurrency | ❌ 3.1 lost-update (confirmed by repro); rest is sound (mutexed `RecordOutput`, single-conn store) |
| Error handling / recovery | ✅ strong (liveness repair, task-ID persistence, signal path verified) |
| Edge cases | ⚠️ 3.3–3.9 |
| Duplication | ⚠️ 4.1–4.3 |
| Performance | ✅ adequate for a local tool; see 5.1–5.2 |
| Documentation | ✅ thorough; one overclaim (3.2) |
| Tests | ✅ policy matrix, lifecycle, repair, TUI basics; gaps: concurrent `AppendTaskID` (the bug!), `Launch`/`Spawn`, `--plain`/`--json` renderers |
