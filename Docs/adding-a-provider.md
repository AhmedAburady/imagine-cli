# Developers Docs

## Adding a provider

This guide walks you through adding a new image-generation provider to imagine. The framework is designed so you edit **only** your provider's package plus a one-line entry in `providers/all/all.go` — never `commands/`, `cli/`, `api/`, `config/`, or `cmd/imagine/main.go`.

---

## Table of contents

- [What you'll write](#what-youll-write)
- [Step 1 — Create the package](#step-1--create-the-package)
- [Step 2 — Declare Options with flagspec](#step-2--declare-options-with-flagspec)
- [Step 3 — Implement Provider](#step-3--implement-provider)
- [Step 4 — Declare ConfigSchema (onboarding)](#step-4--declare-configschema-onboarding)
- [Step 5 — Register the Bundle](#step-5--register-the-bundle)
- [Step 6 — Add the contract test](#step-6--add-the-contract-test)
- [Step 7 — Wire into providers/all](#step-7--wire-into-providersall)
- [Worked example](#worked-example)
- [Non-HTTP providers](#non-http-providers)
- [Opting out of flagspec](#opting-out-of-flagspec)
- [Optional capability interfaces](#optional-capability-interfaces)
- [Testing checklist](#testing-checklist)
- [Reference — framework APIs](#reference--framework-apis)

---

## What you'll write

A typical HTTP-based provider ships as three `.go` files plus a one-line test:

```
providers/myprovider/
  myprovider.go        ← Provider impl: New, Info, Generate + wire types
  options.go           ← tagged Options struct
  register.go          ← Bundle registration with flagspec closures
  contract_test.go     ← one line: providertest.Contract(t, "myprovider")
```

Plus one line in `providers/all/all.go` to blank-import the package. That's it.

Expect ~120–150 lines total. The boilerplate that used to dominate provider code — HTTP plumbing, flag registration, enum validation — is handled by the framework.

---

## Step 1 — Create the package

```bash
mkdir providers/myprovider
```

Pick a short, lower-case name. It'll be the user-facing identifier (`--provider myprovider`, `providers.myprovider.api_key` in config).

---

## Step 2 — Declare Options with flagspec

Your provider's private flags live as struct tags on an `Options` struct. The framework reads the tags via reflection to:

- Register Cobra flags
- Validate enum/range values
- Apply defaults
- Resolve model aliases
- Populate the struct at parse time

### `options.go`

```go
package myprovider

import "strings"

// Options is the provider's private parameter struct. The framework binds
// Cobra flags from the tags and populates this struct per invocation.
// Generate type-asserts Request.Options to *Options.
type Options struct {
    Model   string `flag:"model,m"    desc:"Model version"                default:"v2" enum:"@models"`
    Size    string `flag:"size,s"     desc:"Image size: 1K, 2K, 4K"       default:"1K" enum:"1K,2K,4K"`
    Quality string `flag:"quality,q"  desc:"Rendering quality: low, high" enum:"low,high"`
    Steps   int    `flag:"steps"      desc:"Diffusion steps (10-100)"     default:"50" range:"10:100"`
    Fast    bool   `flag:"fast,F"     desc:"Skip safety checks"`
}

// RequestLabel implements providers.RequestLabeler — drives the spinner's
// per-invocation label. Optional.
func (o *Options) RequestLabel() string { return o.Model }

// ResolvedModel implements providers.ResolvedModeler — enables the
// model-level flag-support gate. Optional but strongly recommended.
func (o *Options) ResolvedModel() string { return o.Model }

// Normalize runs after reflection-based population. Use for trimming,
// case normalisation, or any cleanup the tags can't express. Optional.
func (o *Options) Normalize() {
    o.Quality = strings.ToLower(strings.TrimSpace(o.Quality))
}

// Validate runs after Normalize. Use for cross-field rules. Optional.
func (o *Options) Validate(info providers.Info) error {
    if o.Fast && o.Steps > 50 {
        return fmt.Errorf("--fast requires --steps <= 50")
    }
    return nil
}
```

### Struct tag grammar

| Tag | Meaning | Example |
|---|---|---|
| `flag:"name[,shorthand]"` | Register as a CLI flag | `flag:"model,m"` |
| `flag:"-"` | Skip (useful for fields you populate via hook) | — |
| `desc:"..."` | `--help` description | `desc:"Image size"` |
| `default:"..."` | Default value (string form, parsed per field type) | `default:"1K"` |
| `enum:"A,B,C"` | Allowed values; case-insensitive match, canonicalised to the listed form | `enum:"MINIMAL,HIGH"` |
| `enum:"@models"` | Special — allowed values are `Info.Models[*].ID` + aliases; resolves to canonical ID | — |
| `range:"min:max"` | Numeric range (int fields only) | `range:"0:100"` |

### Supported field types

`string`, `bool`, `int`. Covers every parameter the shipped providers use.

### Optional hooks

- `Normalize()` — runs after population, before validation
- `Validate(providers.Info) error` — cross-field checks the DSL can't express

Both are opt-in. The framework detects them via reflection — no interface declaration needed.

### Optional capability interfaces

| Interface | Method | When to implement |
|---|---|---|
| `providers.RequestLabeler` | `RequestLabel() string` | Spinner shows a per-invocation label (typically the resolved model) |
| `providers.ResolvedModeler` | `ResolvedModel() string` | Enables the model-level flag-support gate so `Info.Models[*].SupportedFlags` becomes enforced |

Both take one-line implementations. Skip only if your provider has no models or no model-specific flag rules.

---

## Step 3 — Implement Provider

Your provider struct implements `providers.Provider`:

```go
type Provider interface {
    Info() Info
    Generate(ctx context.Context, req Request) (*Response, error)
}
```

### `myprovider.go`

```go
// Package myprovider implements the Provider interface for Example AI.
package myprovider

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/AhmedAburady/imagine-cli/internal/transport"
    "github.com/AhmedAburady/imagine-cli/providers"
)

const (
    baseURL = "https://api.example.ai/v1"

    ModelV2     = "example-v2"
    ModelV2Pro  = "example-v2-pro"
)

// httpClient is shared — transport.NewClient provides pooling defaults.
var httpClient = transport.NewClient(120 * time.Second)

type Provider struct {
    apiKey string
}

// New validates auth and returns the provider. Called once per invocation
// from the Bundle factory in register.go. Auth is a flat map[string]string
// exposing every key under providers.<name> in config.yaml.
func New(auth providers.Auth) (providers.Provider, error) {
    key := auth.Get("api_key")
    if key == "" {
        return nil, errors.New("myprovider requires providers.myprovider.api_key in config")
    }
    return &Provider{apiKey: key}, nil
}

// Info returns static metadata. Read once at registration and cached.
func (p *Provider) Info() providers.Info {
    return providers.Info{
        Name:         "myprovider",
        DisplayName:  "Example AI",
        Summary:      "Example AI image models via api.example.ai",
        DefaultModel: ModelV2,
        Models: []providers.ModelInfo{
            {ID: ModelV2, Aliases: []string{"v2"}, Description: "Standard quality."},
            {ID: ModelV2Pro, Aliases: []string{"pro"}, Description: "Higher quality; supports fast mode.",
                SupportedFlags: []string{"fast"}},
        },
        Capabilities: providers.Capabilities{
            Edit:      true,
            MaxBatchN: 4, // API returns up to 4 images per call
            Sizes:     []string{"1K", "2K", "4K"},
        },
    }
}

// Generate issues one API call. The orchestrator has already split the
// user's -n into batches of <= Capabilities.MaxBatchN.
func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
    opts, ok := req.Options.(*Options)
    if !ok {
        return nil, fmt.Errorf("myprovider: internal: expected *Options, got %T", req.Options)
    }

    body := apiRequest{
        Model:  opts.Model,
        Prompt: req.Prompt,
        Size:   opts.Size,
        N:      req.N,
        Steps:  opts.Steps,
        Fast:   opts.Fast,
    }

    resp, err := transport.PostJSON[apiResponse](
        ctx, httpClient,
        baseURL+"/images/generate",
        transport.Bearer(p.apiKey),
        body,
    )
    if err != nil {
        return nil, err // already a *transport.APIError on non-2xx
    }

    imgs := make([]providers.GeneratedImage, 0, len(resp.Images))
    for _, img := range resp.Images {
        data, err := transport.DecodeB64(img.B64)
        if err != nil {
            return nil, err
        }
        imgs = append(imgs, providers.GeneratedImage{Data: data, MimeType: "image/png"})
    }
    if len(imgs) == 0 {
        return nil, errors.New("myprovider returned no images")
    }
    return &providers.Response{Images: imgs}, nil
}

// --- Wire types (private) --------------------------------------------------

type apiRequest struct {
    Model  string `json:"model"`
    Prompt string `json:"prompt"`
    Size   string `json:"size,omitempty"`
    N      int    `json:"n,omitempty"`
    Steps  int    `json:"steps,omitempty"`
    Fast   bool   `json:"fast,omitempty"`
}

type apiResponse struct {
    Images []struct {
        B64 string `json:"b64"`
    } `json:"images"`
}
```

### Key points

**`MaxBatchN`** — how many images per single API call. The orchestrator divides the user's `-n` into batches automatically. Set to `1` if the API does one image per call; set to the max the API accepts otherwise. Must be `>= 1`.

**Auth injectors** — pick the one matching your API:

| API pattern | Injector |
|---|---|
| `Authorization: Bearer <token>` | `transport.Bearer(token)` |
| `?api_key=<value>` in URL | `transport.QueryKey("api_key", value)` |
| Application Default Credentials, OAuth flows, custom headers | Implement `transport.Auth` yourself |
| No auth | `transport.NoAuth()` |

**Error handling** — `transport.PostJSON` and `PostMultipart` return `*transport.APIError` on non-2xx responses, with `StatusCode` and (when parseable) `Message`. Callers can `errors.As` to inspect the status for retry logic.

**Context cancellation** — transport already threads `ctx` through `http.NewRequestWithContext`. Ctrl+C propagates cleanly.

---

## Step 4 — Declare ConfigSchema (onboarding)

`imagine providers add <name>` is the entry point for users configuring your provider — interactive form in a TTY, flags in a script. Both modes read from the same per-provider **ConfigSchema**.

Write a `ConfigSchema()` method on your `*Provider` type that returns the fields to collect. Each field becomes a form input AND a `--<key-with-dashes>` flag on the sub-command.

```go
// ConfigSchema declares the fields `imagine providers add myprovider`
// collects. The framework uses this for both the interactive huh form
// and the non-interactive flag set — one declaration, both UIs.
func (p *Provider) ConfigSchema() []providers.ConfigField {
    return []providers.ConfigField{
        {
            Key:         "api_key",
            Title:       "API Key",
            Description: "API key from example.ai dashboard",
            Secret:      true,   // masked in the form (EchoModePassword)
            Required:    true,
        },
    }
}
```

### `ConfigField` reference

| Field | Purpose |
|---|---|
| `Key` | Storage key under `providers.<name>.<Key>` in config.yaml; `--<Key-with-dashes>` on the CLI (e.g. `api_key` → `--api-key`) |
| `Title` | Form label and human-readable name |
| `Description` | One-line help shown in the form and in `--help` |
| `Secret` | `true` masks input (password field) |
| `Required` | `true` means missing flag + non-TTY → error; TTY → form asks for it |
| `Default` | Default used when optional field is unset; pre-fills form input |

### Multi-field example (Vertex-style)

```go
func (p *Provider) ConfigSchema() []providers.ConfigField {
    return []providers.ConfigField{
        {Key: "gcp_project", Title: "GCP Project", Required: true,
         Description: "GCP project ID with the Vertex AI API enabled"},
        {Key: "location",    Title: "Location",    Default: "global",
         Description: "Vertex AI region (default: global)"},
    }
}
```

`providers add vertex --gcp-project X` writes both fields flat under `providers.vertex` in config.yaml. The interactive form asks for `gcp_project` only (location has a default).

### Reading the values back in `New()`

The factory reads the same keys via `auth.Get(key)`:

```go
func New(auth providers.Auth) (providers.Provider, error) {
    project := auth.Get("gcp_project")
    if project == "" { return nil, errors.New("vertex requires gcp_project") }
    location := auth.Get("location")
    if location == "" { location = "global" }
    return &Provider{project: project, location: location}, nil
}
```

One schema drives four things: form, flags, storage keys, and the factory's read pattern. If you add a field, the onboarding form and flags update automatically — no edits to `commands/`.

---

## Step 5 — Register the Bundle

### `register.go`

```go
package myprovider

import (
    "github.com/spf13/cobra"

    "github.com/AhmedAburady/imagine-cli/providers"
    "github.com/AhmedAburady/imagine-cli/providers/flagspec"
)

// init self-registers the provider. Triggered by the blank import in
// providers/all — never called explicitly.
func init() {
    p := &Provider{}
    info := p.Info()
    providers.Register("myprovider", providers.Bundle{
        Factory: New,
        BindFlags: func(cmd *cobra.Command) {
            // flagspec.Bind panics on malformed tags — programmer errors
            // discoverable at init time. No error handling needed.
            flagspec.Bind(cmd, Options{})
        },
        ReadFlags: func(cmd *cobra.Command) (any, error) {
            return flagspec.Read(cmd, Options{}, info)
        },
        SupportedFlags: flagspec.FieldNames(Options{}),
        Info:           info,
        ConfigSchema:   p.ConfigSchema(), // drives `providers add`
    })
}
```

That's the whole file. `flagspec.Bind`, `Read`, and `FieldNames` all derive from your `Options` struct via reflection. `ConfigSchema` is cached on the Bundle (not called via interface) so onboarding works before any auth exists — instantiating the provider would fail with empty credentials.

---

## Step 6 — Add the contract test

### `contract_test.go`

```go
package myprovider_test

import (
    "testing"

    "github.com/AhmedAburady/imagine-cli/providers/providertest"
)

func TestContract(t *testing.T) {
    providertest.Contract(t, "myprovider")
}
```

One line of code. Runs 12 invariants without any network:

| Invariant | What it checks |
|---|---|
| `InfoWellFormed` | `Name`, `DisplayName`, `DefaultModel` non-empty; `Models` non-empty; every model has an ID |
| `InfoNameMatchesRegistration` | `Info.Name` equals the string passed to `providers.Register` |
| `DefaultModelResolvable` | `ResolveModel(DefaultModel)` round-trips |
| `EmptyInputResolvesToDefault` | `ResolveModel("")` returns `DefaultModel` |
| `AliasesRoundTrip` | Every alias resolves to its canonical ID |
| `CanonicalIDsRoundTrip` | Every canonical ID resolves to itself |
| `NoDuplicateModelIDs` | No two `ModelInfo`s share an ID |
| `ModelSupportedFlagsSubsetOfBundle` | `ModelInfo.SupportedFlags` ⊆ `Bundle.SupportedFlags` |
| `MaxBatchNValid` | `Capabilities.MaxBatchN >= 1` |
| `BindFlagsIdempotent` | Calling `BindFlags` twice doesn't duplicate or panic |
| `ReadFlagsDefaultsSucceed` | Parsing no flags returns no error and non-nil options |
| `SupportedFlagsRegisteredByBindFlags` | Every name in `Bundle.SupportedFlags` is actually registered |

When the framework adds a new invariant, the harness updates it — every provider's suite enforces it automatically.

---

## Step 7 — Wire into providers/all

Add one blank import to `providers/all/all.go`:

```go
package all

import (
    _ "github.com/AhmedAburady/imagine-cli/providers/gemini"
    _ "github.com/AhmedAburady/imagine-cli/providers/openai"
    _ "github.com/AhmedAburady/imagine-cli/providers/vertex"
    _ "github.com/AhmedAburady/imagine-cli/providers/myprovider" // ← new
)
```

Done. `cmd/imagine/main.go` is never touched.

---

## Worked example

Put it all together and a user's full flow looks like this:

```bash
# One-time onboarding — interactive form in a terminal, deterministic flags in a script.
imagine providers add myprovider                      # opens huh form asking for api_key
imagine providers add myprovider --api-key sk-xxx     # non-interactive / CI-friendly

# Make it the default (or pass --provider per invocation)
imagine providers use myprovider

# Generate
imagine -p "sunset over kyoto" -m pro --fast --steps 40
```

What happens under the hood:

1. `cmd/imagine/main.go` imports `providers/all` → triggers your `init()` → registers the Bundle
2. `providers add myprovider` reads `Bundle.ConfigSchema`, prompts for missing required fields (or errors listing them under non-TTY), writes flat to `providers.myprovider.*` in `config.yaml`
3. Fang renders provider-aware `--help` showing your `Options` tags
4. `--provider myprovider` resolves the active provider
5. `enforceFlagSupport` confirms `--fast` and `--steps` are in your `SupportedFlags`
6. `flagspec.Read` populates `*Options`; `Normalize()` runs; `Validate(Info)` runs
7. `enforceModelSupport` confirms the resolved model (`example-v2-pro`) supports `--fast`
8. `New(auth)` reads credentials via `auth.Get("api_key")`
9. Orchestrator calls `Generate(ctx, req)` with `req.N` split into batches
10. Each image is written to disk by the shared orchestrator
11. Ctrl+C during the call propagates via ctx and aborts in-flight requests

---

## Non-HTTP providers

Not every provider is HTTP. **Vertex** uses Google's `genai` SDK with Application Default Credentials. It never touches the transport package.

A non-HTTP provider implements `Generate` directly against its SDK, and uses `ConfigSchema` to declare whatever credentials the SDK needs:

```go
func (p *Provider) ConfigSchema() []providers.ConfigField {
    return []providers.ConfigField{
        {Key: "gcp_project", Title: "GCP Project", Required: true},
        {Key: "location",    Title: "Location",    Default: "global"},
    }
}

func New(auth providers.Auth) (providers.Provider, error) {
    project := auth.Get("gcp_project")
    if project == "" { return nil, errors.New("vertex requires gcp_project") }
    location := auth.Get("location")
    if location == "" { location = "global" }
    return &Provider{project: project, location: location}, nil
}

func (p *Provider) Generate(ctx context.Context, req providers.Request) (*providers.Response, error) {
    opts, _ := req.Options.(*Options)
    client, err := genai.NewClient(ctx, &genai.ClientConfig{
        Project:  p.project,
        Location: p.location,
    })
    // ... SDK calls ...
}
```

Everything else — `Options`, flagspec, register, contract test — works identically. The transport package is opt-in; use it only when it fits.

---

## Opting out of flagspec

Flagspec covers the 80% case. For the 20% that doesn't fit, you can write `BindFlags` and `ReadFlags` by hand:

```go
providers.Register("myprovider", providers.Bundle{
    Factory: New,
    BindFlags: func(cmd *cobra.Command) {
        f := cmd.Flags()
        f.StringP("size", "s", "auto", "Image size: auto, or raw WxH")
        // ...
    },
    ReadFlags: func(cmd *cobra.Command) (any, error) {
        size, _ := cmd.Flags().GetString("size")
        // custom parsing: accept "auto", "1024x1024", or regexed WxH
        if err := validateSize(size); err != nil { return nil, err }
        // return any shape you like — could be map[string]any or a typed struct
        return &Options{Size: size}, nil
    },
    SupportedFlags: []string{"size", /* ... */},
    Info:           info,
})
```

**When to opt out:**

- Your flag's valid values can't be expressed as a closed enum (OpenAI's `-s` accepts raw `WxH` alongside shorthand)
- You need cross-field validation that's hard to express in `Validate(Info) error`
- You need to read a common flag (like `-f` / `--filename`) to infer a private flag's value (OpenAI reads the filename extension to pick `output_format`)

The framework doesn't care which approach you take. `Request.Options any` means your `Generate` type-asserts to whatever shape you returned.

OpenAI is the canonical example of an opt-out provider in this codebase — look at `providers/openai/flags.go` for the pattern.

---

## Optional capability interfaces

The framework uses type assertions to discover provider capabilities — nothing is required, but each enables a feature:

### `providers.RequestLabeler`

```go
type RequestLabeler interface {
    RequestLabel() string
}
```

Controls the per-invocation label in the spinner:

```
Generating (myprovider, example-v2-pro) 3 image(s)...
                         ^^^^^^^^^^^^^^ from RequestLabel()
```

Without this, users see only `(myprovider)`.

### `providers.ResolvedModeler`

```go
type ResolvedModeler interface {
    ResolvedModel() string
}
```

Enables the **model-level flag-support gate**. Once implemented, `Info.Models[*].SupportedFlags` becomes enforced — users attempting to use flags the resolved model doesn't accept get a helpful error:

```
--fast is not supported by model "example-v2" (supported by: [pro])
```

Strongly recommended if your provider has multiple models with different capability profiles.

### Future interfaces

The framework pattern is extensible: new capabilities are added as optional interfaces. Implementers opt in; non-implementers degrade gracefully. Check `providers/provider.go` for the current list.

---

## Testing checklist

- [ ] `go build ./...` clean
- [ ] `go test ./providers/myprovider/...` passes (runs the contract)
- [ ] `imagine providers add myprovider --help` shows your `ConfigSchema` fields as flags
- [ ] `imagine providers add myprovider --<key> <value>` writes flat to `config.yaml`
- [ ] `imagine providers add myprovider` with no flags (in a terminal) opens the interactive form
- [ ] `imagine providers add myprovider < /dev/null` (non-TTY, missing required) errors with the exact missing flags
- [ ] `imagine --provider myprovider --help` shows your `Options` flags, hides others
- [ ] `imagine --provider myprovider -p "test"` succeeds against the real API (or errors with a useful message)
- [ ] Setting a flag from another provider errors with `--<flag> is not supported by provider "myprovider"`
- [ ] If you implemented `ResolvedModeler`: setting a model-restricted flag on a non-supporting model errors
- [ ] `imagine providers` lists your provider (and marks it `[active, default]` after `providers use`)

---

## Reference — framework APIs

### Required to implement

| Package | API | Purpose |
|---|---|---|
| `providers` | `providers.Provider` interface (`Info`, `Generate`) | Every provider must implement |
| `providers` | `providers.Register(name, Bundle)` | Called from `init()` |

### Opt-in

| Package | API | Purpose |
|---|---|---|
| `providers/flagspec` | `Bind`, `Read`, `FieldNames` | Reflection-based flag DSL (panics on malformed tags) |
| `internal/transport` | `Client`, `PostJSON[R]`, `PostMultipart[R]`, `Bearer`, `QueryKey`, `NoAuth`, `APIError`, `DecodeB64` | Shared HTTP primitives |
| `providers/providertest` | `Contract(t, name)` | Standard contract test battery |
| `providers` | `RequestLabeler`, `ResolvedModeler` | Optional capability interfaces on `*Options` |
| `providers` | `Bundle.ConfigSchema []ConfigField` | Drives `providers add` form + flags |
| `providers` | `Auth.Get(key) string` | Flat credential bag read inside `New(auth)` |

### Never touched

`commands/`, `cli/`, `api/`, `config/`, `cmd/imagine/main.go` — provider authors never edit these. If a change there seems necessary, it's a framework gap worth discussing in an issue.

---

## Getting help

Existing providers are the best reference:

- **`providers/gemini/`** — HTTP + flagspec + transport (the canonical pattern)
- **`providers/vertex/`** — non-HTTP (SDK) + flagspec
- **`providers/openai/`** — opt-out of flagspec, keeps map-based options for flexibility

If something feels awkward, read all three before deciding which pattern fits your target API.
