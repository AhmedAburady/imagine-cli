# Storage — S3-compatible bucket for modelark references

Only **modelark** needs this. It fetches references **server-side from a URL** (rejects inline data), so every local `-i` reference — image, video, or audio — is uploaded to an S3-compatible bucket you control and the public URLs are passed to the API. ("Video provider" refers to the output; the bucket holds whatever reference kinds you pass.) `fal` does **not** need storage (it uses its own CDN).

## Commands

```bash
imagine storage set [flags]   # write/update the storage: section (merge — unset fields kept)
imagine storage              # show current config (secrets masked)
imagine storage show         # same
imagine storage test         # signed write → anonymous read → cleanup (verifies public-read)
imagine storage clear        # remove the storage: section
```

`storage set` is non-interactive with flags, or an interactive wizard in a TTY. Headless, always pass flags:

```bash
imagine storage set \
  --endpoint https://tos-ap-southeast-1.bytepluses.com \
  --bucket my-imagine-bucket \
  --access-key AK... \
  --secret-key "${S3_SECRET}"
imagine storage test
```

## Config fields

```yaml
storage:
  endpoint: "https://tos-ap-southeast-1.bytepluses.com"  # required (scheme included)
  region: "ap-southeast-1"                                # optional (signing; default us-east-1)
  bucket: "my-imagine-bucket"                             # required — dedicated, public-read
  access_key: "AK..."                                     # required — supports ${ENV} / op://
  secret_key: "op://Personal/imagine/secret"             # required — supports ${ENV} / op://
  path_prefix: "imagine-refs/"                            # optional (default imagine-refs/)
  public_url_base: ""                                     # optional — CDN/custom-domain read base
  path_style: false                                       # optional — true for MinIO/RustFS
```

| Field | Flag | Required | Notes |
|---|---|---|---|
| `endpoint` | `--endpoint` | yes | S3-compatible endpoint URL |
| `region` | `--region` | no | default `us-east-1` for signing |
| `bucket` | `--bucket` | yes | dedicated, **public-read** bucket |
| `access_key` | `--access-key` | yes | `${ENV}` / `op://` ok |
| `secret_key` | `--secret-key` | yes | `${ENV}` / `op://` ok |
| `path_prefix` | `--path-prefix` | no | default `imagine-refs/` |
| `public_url_base` | `--public-url-base` | no | overrides the read URL |
| `path_style` | — (config only) | no | `true` = path-style; default `false` = virtual-host |

## Key rules

- **Dedicated, public-read bucket.** Reads are anonymous (the provider has no credentials); imagine doesn't tag objects — readability is the bucket's job. Everything uploaded is world-readable by design; don't reuse a private bucket. `imagine storage test` verifies write + anonymous read.
- **Any S3-compatible backend**: BytePlus TOS, MinIO, RustFS, Cloudflare R2, Wasabi, AWS S3. No AWS SDK; requests are stdlib SigV4.
- **Addressing / `path_style`**: default virtual-host (`https://{bucket}.{host}/{key}`), **required by BytePlus TOS** (TOS rejects path-style). Set `path_style: true` for **MinIO / RustFS**, whose TLS cert covers the bare host but not a `{bucket}.` subdomain — otherwise uploads fail with a TLS handshake error. There is **no `--path-style` flag**; edit the YAML.
- **TOS host**: use the extranet host `tos-<region>.bytepluses.com` (not the `i…ibytepluses.com` intranet host) — the provider fetches over the public internet.
- **secret quoting**: an `op://` value must resolve to the raw reference. `secret_key: "op://…"` is correct; a double-wrapped `'"op://…"'` is treated as a literal string and the reference won't resolve.

## Common pitfalls

| Error | Fix |
|---|---|
| `provider "modelark" needs S3-compatible storage to upload references` | Run `imagine storage set`. Only fires when modelark has `-i` references. |
| `tls: handshake failure` on a PUT to `{bucket}.{host}` | MinIO/RustFS: set `path_style: true`. |
| `anonymous read of … failed — the bucket must be public-read` | Make the bucket public-read (dedicated bucket for imagine). |
| `… PUT … failed (status 403)` | Bad access/secret key, wrong region, or `op://` not resolving (check quoting). |
| `InvalidPathAccess` (BytePlus TOS) | You set `path_style: true` on TOS — remove it (TOS is virtual-host only). |
| sporadic `EOF` on a PUT | Transient (stale keep-alive); re-run. Not a config error. |

Full dev design: `Docs/storage.md` in the imagine-cli repo.
