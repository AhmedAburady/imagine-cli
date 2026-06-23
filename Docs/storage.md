# Storage — the S3-compatible upload brick

> Scope: a generic, dependency-free storage brick (`internal/storage/`) that publishes local references as public URLs, plus the `imagine storage` command and the `RequireStorage` provider gate. First consumer: the [`modelark`](modelark.md) video provider.

---

## Table of contents

- [1. Why a storage brick exists](#1-why-a-storage-brick-exists)
- [2. Config schema](#2-config-schema)
- [3. Addressing — virtual-host vs path-style](#3-addressing--virtual-host-vs-path-style)
- [4. The `imagine storage` command](#4-the-imagine-storage-command)
- [5. The `RequireStorage` gate](#5-the-requirestorage-gate)
- [6. Internals](#6-internals)
- [7. Security model](#7-security-model)
- [8. Troubleshooting](#8-troubleshooting)

---

## 1. Why a storage brick exists

Some provider APIs fetch reference media **server-side** from a URL and reject inline Base64 — BytePlus ModelArk's `video_url` accepts public URLs / asset IDs only. The CLI, however, takes local files (`-i ref.png`). The gap is bridged by uploading each local reference to an S3-compatible bucket the user controls and passing the resulting **public URL** to the provider.

The brick is **backend-agnostic** — it speaks raw [AWS Signature V4](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html) against any S3-compatible endpoint (BytePlus TOS, MinIO, RustFS, Cloudflare R2, Wasabi, AWS S3) with **zero AWS SDK dependencies** (a SigV4 PUT is ~150 lines of stdlib `crypto/hmac`+`crypto/sha256`; the SDK pulls dozens of transitive packages for one call).

It is **not** a provider — no model, no `Generate`. It lives in its own top-level `storage:` config section, keeping provider resolution, `providers show`, and model lookup clean.

| File | Purpose |
|---|---|
| `internal/storage/storage.go` | `Configured`, `Get`, `Upload`/`UploadWith`, `Test`, `Delete`, content-addressed keys + singleflight dedup |
| `internal/storage/sigv4.go` | Minimal AWS SigV4 signer (`signRequest` / `signRequestService`) |
| `internal/storage/storage_test.go` | SigV4 vectors (S3 GET-Object + AWS test-suite get-vanilla), URL/key construction |
| `commands/storage.go` | `imagine storage` command tree |
| `config/{config,secrets,save}.go` | `StorageConfig`, `ResolveStorage`, `SaveStorage`/`ClearStorage` |

---

## 2. Config schema

Top-level `storage:` section (sibling of `providers:`):

```yaml
storage:
  endpoint: "https://tos-ap-southeast-1.bytepluses.com"  # required
  region: "ap-southeast-1"                                # optional (default us-east-1 for signing)
  bucket: "my-imagine-bucket"                             # required
  access_key: "AK..."                                     # required — supports ${ENV} / op://
  secret_key: "op://Personal/imagine/secret"             # required — supports ${ENV} / op://
  path_prefix: "imagine-refs/"                            # optional (default imagine-refs/)
  public_url_base: ""                                     # optional — CDN/custom-domain read base
  path_style: false                                       # optional — see §3 (default false = virtual-host)
```

| Field | Required | Notes |
|---|---|---|
| `endpoint` | Yes | S3-compatible endpoint URL, scheme included. |
| `region` | No | SigV4 region; empty defaults to `us-east-1` (most S3-compatible servers ignore it). |
| `bucket` | Yes | A **dedicated, public-read** bucket for imagine (see §7). |
| `access_key` | Yes | Access key ID. `${ENV}` / `op://` references resolved like provider secrets. |
| `secret_key` | Yes | Secret access key. Same reference support. |
| `path_prefix` | No | Key prefix for uploaded objects. Default `imagine-refs/`. |
| `public_url_base` | No | Overrides the read URL (CDN / custom domain). Write path always targets the bucket directly. |
| `path_style` | No | `false` (default) = virtual-host; `true` = path-style. See §3. **Config-file only** — no flag/wizard field. |

Secret resolution reuses the exact `${ENV}` → `op://` engine used for provider credentials (`config.ResolveStorage`, mirroring `ResolveProvider`). See the [README Credentials section](../README.md#secret-references--keep-plaintext-out-of-configyaml).

---

## 3. Addressing — virtual-host vs path-style

S3 has two URL conventions for reaching `{bucket}/{key}`:

| Style | URL | Set with |
|---|---|---|
| **Virtual-host** (default) | `https://{bucket}.{host}/{key}` | `path_style: false` (or omit) |
| **Path-style** | `https://{host}/{bucket}/{key}` | `path_style: true` |

The default is **virtual-host** because **BytePlus TOS supports virtual-host only, not path-style** ([TOS S3 compatibility](https://docs.byteplus.com/en/docs/tos/docs-compatibility-with-amazon-s3)) — a path-style request to TOS returns `InvalidPathAccess`. Use the **extranet** endpoint host `tos-<region>.bytepluses.com` (not the `i…ibytepluses.com` intranet host) because the provider fetches the URL over the public internet ([TOS region & endpoint](https://docs.byteplus.com/en/docs/tos/docs-region-and-endpoint)).

Set `path_style: true` for backends that require path-style — notably **MinIO** and **RustFS**, where the TLS certificate covers the bare host (`s3.example.com`) but **not** the `{bucket}.s3.example.com` subdomain. The virtual-host default against such a server fails with a TLS handshake error (the wildcard cert doesn't exist); flipping `path_style` makes the request target the cert's host. See [§8](#8-troubleshooting).

The signer is addressing-agnostic: it signs whatever `Host` and path the URL carries, so the canonical host/URI come out correct for both styles automatically — no bucket-awareness in `sigv4.go`.

---

## 4. The `imagine storage` command

Built entirely from the existing onboarding engine (`collectFields`/`wizardFill`/`registerFieldFlags`) — the same dual-mode (flags / TTY wizard / non-TTY error) behaviour every provider's `providers add` has.

```bash
imagine storage              # show current config (secrets masked) or "not configured"
imagine storage show         # explicit alias
imagine storage set [flags]  # write/update the storage: section (merge semantics)
imagine storage test         # round-trip a marker object: signed write → anonymous read → cleanup
imagine storage clear        # remove the storage: section
```

`storage set` flags mirror the schema (`--endpoint`, `--region`, `--bucket`, `--access-key`, `--secret-key`, `--path-prefix`, `--public-url-base`). It is a **merge**: unset fields keep their stored value (and pre-fill the wizard), so `imagine storage set --bucket newbucket` changes only the bucket. `path_style` has no flag — edit the YAML by hand.

`storage test` is the contract check: it does a **signed PUT** of a random marker, an **anonymous GET** of the resulting public URL, compares bytes, then best-effort deletes the marker. A failed anonymous read means the bucket isn't public-read — the exact failure ModelArk hits when it can't fetch a reference.

---

## 5. The `RequireStorage` gate

`providers.Bundle` carries one bool:

```go
// RequireStorage marks a provider that needs the shared S3-compatible
// storage brick to publish local references as public URLs.
RequireStorage bool
```

A provider sets `RequireStorage: true` and inherits a configured-storage gate with no other framework changes. The gate is **reference-aware**: it fires only when the provider needs the brick **and local references are actually present** — text-to-video (no `-i`) uploads nothing and is never blocked.

The gate fires at both dispatch sites (a provider's `Generate` only runs after one of them):

- **Single-shot** — `commands/root.go` PreRunE: `if bundle.RequireStorage && len(opts.RefInputs) > 0 && !storage.Configured()`.
- **Batch** — `internal/batch/resolve.go` per entry (PreRunE early-returns on `IsBatch`, so this is required for parity), keyed on the entry's resolved inputs.

`storage.Configured()` is cheap: it loads the config file but does **not** resolve secrets (no `op://` round-trip), so the gate never triggers a 1Password prompt.

The next storage-backed provider is a one-line `RequireStorage: true` — no edits to `commands/`, `cli/`, `api/`, or `gate.go`.

---

## 6. Internals

**Content-addressed keys + dedup.** The object key is `{path_prefix}{sha256(data)}{ext}`, so an identical re-upload is an idempotent re-PUT. A package-level singleflight (keyed by endpoint+bucket+key) means identical reference bytes upload **once** across concurrent `Generate` calls (`-n>1`) and reused provider instances (batch mode); failures are evicted so a later call retries.

**Resolve once.** `Get(ctx)` loads + resolves the config (one `op://` round-trip). Providers uploading several references call `Get` once and then `UploadWith(ctx, sc, data, mime)` per reference, avoiding N config reads / 2N `op read` subprocess spawns. `Upload(ctx, data, mime)` is the single-object convenience wrapper (resolve + upload).

**SigV4.** `signRequest` is the `s3`-service wrapper; `signRequestService(…, service)` is the generic signer (used by tests against the AWS get-vanilla `service` fixture). Empty region defaults to `us-east-1`; empty credentials return an error rather than signing garbage. Writes are signed; the canonical path is built from the **decoded** `URL.Path` (single-encoded, matching S3's rule and what Go puts on the wire — no double-encoding).

**No object ACLs.** Uploads are plain (only the request is signed). Read access is the **bucket's** responsibility, not per-object — see §7.

---

## 7. Security model

imagine's storage backend is a **dedicated, public-read S3-compatible bucket** used only for imagine reference uploads. The model is deliberately simple:

- **Writes** are authenticated (SigV4 with your access/secret key).
- **Reads** are anonymous — the provider fetches reference URLs server-side with no credentials, so the bucket must allow public reads. imagine does **not** tag individual objects with an ACL; readability is the bucket's job.
- Use a **dedicated** bucket, not a shared/private one — everything uploaded is world-readable by design. Objects are content-addressed reference media (the images/videos/audio you pass with `-i`), not secrets.
- `imagine storage test` verifies exactly this (signed write succeeds **and** anonymous read returns the bytes).

Credentials never land in plaintext if you use `${ENV}` / `op://` references (resolved lazily, only when an upload is about to happen).

---

## 8. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `provider "X" needs S3-compatible storage to upload references — run \`imagine storage set\` first` | A `RequireStorage` provider was given `-i` references with no storage configured. Run `imagine storage set`. (Text-to-video without `-i` is *not* gated.) |
| `tls: handshake failure` on PUT to `{bucket}.{host}/...` | Virtual-host addressing hit a server whose cert doesn't cover the bucket subdomain (MinIO/RustFS). Set `path_style: true` in the `storage:` section. |
| `… failed (status 403)` on `storage test` write | Bad access/secret key, wrong region, or the key lacks PutObject. Check credentials; remember `op://` must resolve to the raw secret (a double-quoted `'"op://…"'` value is treated as a literal, not a reference). |
| `anonymous read of … failed — the bucket must be public-read` | The signed write succeeded but the public GET didn't. Make the bucket public-read (a dedicated bucket for imagine). |
| Sporadic `EOF` / `connection reset` on a PUT | Transient transport blip (often a stale keep-alive connection the server already closed; PUT isn't auto-retried). Re-run — it's not a config or signing error. |
| `InvalidPathAccess` from BytePlus TOS | You set `path_style: true` against TOS, which is virtual-host-only. Remove `path_style` (or set it `false`). |
