# Framework internals

Cross-cutting mechanics that span the whole CLI rather than any one provider. Read this when you need the *why* behind flag handling, help rendering, or request cancellation — adding a provider only requires `Docs/adding-a-provider.md`.

---

## Table of contents

- [Flag ownership model](#flag-ownership-model)
- [Provider resolution precedence](#provider-resolution-precedence)
- [Help rendering and the pre-parsed provider hint](#help-rendering-and-the-pre-parsed-provider-hint)
- [Cobra + Fang](#cobra--fang)
- [Context threading and cancellation](#context-threading-and-cancellation)

---

## Flag ownership model

Every flag is exactly one of two kinds:

- **Common** — provider-agnostic, bound on the root command and listed in `cli.CommonFlagNames` (`cli/cli.go`). Today: `prompt`, `output`, `filename`, `count`, `input`, `replace`, `provider`, `max-parallel`, `embed-metadata`, plus `help`/`version`.
- **Provider-private** — declared by a provider's tagged `Options` struct and surfaced through `Bundle.SupportedFlags`.

`cli.CommonFlagNames` is the single source of truth: both the single-shot validation gate (`commands/validate.go`) and the batch path (`internal/batch/resolve.go`) read from it. **Any flag not listed there must be claimed by at least one provider's `Bundle.SupportedFlags`**, or validation rejects it.

### Shared flag names are idempotent

Providers may share a flag name — Gemini and Vertex both expose `-m`, `-s`, `-a`, `-t`. `flagspec.Bind` is idempotent: before registering a flag it checks `flags.Lookup(name) != nil` and skips if the flag already exists (`providers/flagspec/flagspec.go`). Whichever provider binds first wins the flag's `--help` description; that ordering is controlled by `providerOrder`.

### `providerOrder`

`commands/resolve.go:providerOrder(first)` returns the active/hinted provider first, then every other registered provider in registry order. The active provider therefore binds its shared flags first and owns their help text. With no hint it returns providers in `providers.List()` order.

The validation rules themselves live in `providers/gate.go` (`CheckBundle`, `CheckModel`, `CheckClaimedSomewhere`) — see the [validation gate](adding-a-provider.md#validation-gate) section of the provider guide. Adding a flag or model makes it enforced everywhere automatically; you never edit `gate.go`.

---

## Provider resolution precedence

```
--provider <name>                    # CLI flag
  ↓
default_provider                     # config.yaml
  ↓
first provider under providers:      # alphabetical (sortedProviderKeys)
  ↓
error
```

No environment variables participate — the config file is the only source of truth for *which* provider runs. (`${VAR}` / `op://` references resolve credential *values*, not provider selection.)

---

## Help rendering and the pre-parsed provider hint

`imagine --help` is provider-aware: it shows the active provider's flags and examples and hides the rest. The tricky part is that Fang renders help before normal flag parsing, so the active provider must be known *before* `fang.Execute`.

`main()` peeks at `os.Args[1:]` via `commands.ProviderHintFromArgs` (and `DescriberHintFromArgs` for `describe`) to resolve a best-effort provider hint (`cmd/imagine/main.go`, `commands/resolve.go`). `NewRootCmd` uses that hint to mark other providers' flags `Hidden` on the `pflag.Flag` before Fang sees the command (`commands/help.go:applyProviderFlagVisibility`). Common flags and the active provider's flags stay visible; everything else is hidden.

This is why flag visibility is set at command-construction time via `Hidden`, not through a custom help function.

---

## Cobra + Fang

`charm.land/fang/v2` wraps the Cobra root command (`cmd/imagine/main.go`). Fang provides:

- **Styled help and version output.** Fang renders its own help, so `cmd.SetHelpFunc` is bypassed — flag visibility is controlled through `Hidden` (see above), not a help hook.
- **Signal-based context cancellation.** `fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM)` ties the root context to Ctrl+C / SIGTERM, which cancels in-flight HTTP requests (see below).
- **Version injection.** `fang.WithVersion(v)` surfaces the `main.version` value the release workflow injects via `-ldflags`.

---

## Context threading and cancellation

Every provider's `Generate(ctx, req)` receives a `context.Context`, and all HTTP calls use `http.NewRequestWithContext`. Because Fang binds that context to the interrupt signal, pressing Ctrl+C cancels inflight requests immediately rather than waiting for them to return. The orchestrator (`api/orchestrator.go`) and the batch runner (`internal/batch/run.go`) both propagate the same context to every goroutine they spawn, so a single interrupt aborts the entire fan-out.
