# imagine config file reference

imagine reads one YAML file. No environment variables, no `config set-*` commands. Users edit this file with any editor.

## Location

| OS | Path |
|---|---|
| Linux, macOS, *BSD | `~/.config/imagine/config.yaml` |
| Windows | `%AppData%\imagine\config.yaml` (typically `C:\Users\<name>\AppData\Roaming\imagine\config.yaml`) |

Both `config.yaml` and `config.yml` are accepted. imagine checks `.yaml` first, then `.yml`.

### macOS rationale

imagine intentionally uses `~/.config/imagine/` rather than `~/Library/Application Support/imagine/` even on macOS. Reasons: the XDG-style path has no spaces, it's easy to browse, and it plays nicely with dotfiles repos. Users who symlink their dotfiles directory get imagine's config along for free.

## Full schema

```yaml
default_provider: gemini         # optional — see precedence below

providers:
  gemini:
    api_key: AIza-your-key-here

  openai:
    api_key: sk-your-openai-key-here

  vertex:
    provider_options:
      gcp_project: your-gcp-project-id
      location: us-central1       # optional, defaults to "global"
```

### Fields

| Field | Required | Notes |
|---|---|---|
| `default_provider` | No | Which provider to use when `--provider` is omitted. If empty, imagine picks the first provider under `providers:` alphabetically. |
| `providers.<name>` | Yes (at least one) | Per-provider block. The `<name>` must be one of the providers compiled into this binary (currently `gemini`, `vertex`, `openai`). |
| `providers.<name>.api_key` | For Gemini/OpenAI | Required by providers that authenticate with an API key. Vertex does not use this field. |
| `providers.<name>.provider_options` | Provider-specific | Free-form string map for extras. Vertex uses `gcp_project` (required) and `location` (optional, default `global`). |

## Provider resolution precedence

imagine resolves the active provider per invocation:

```
--provider <name>                    # CLI flag — wins if set
  ↓
default_provider                     # from config.yaml
  ↓
first under providers:               # alphabetical order
  ↓
error: no provider configured
```

## Credential setup per provider

### Gemini (direct REST, API key)

1. Get a free API key at https://aistudio.google.com/app/apikey.
2. Paste into the config:

```yaml
providers:
  gemini:
    api_key: AIza-your-key-here
```

That's it.

### OpenAI (API key)

1. Get an API key at https://platform.openai.com.
2. Paste into the config:

```yaml
providers:
  openai:
    api_key: sk-your-openai-key-here
```

Needs organization verification enabled on the account for GPT Image models — see OpenAI's docs.

### Vertex AI (GCP project + ADC, no key in config)

Two setup steps on the machine:

1. A GCP project with the Vertex AI API enabled.
2. `gcloud auth application-default login` — imagine uses Application Default Credentials. No key to paste in the config.

Then add the project id (and optional location) to the config:

```yaml
providers:
  vertex:
    provider_options:
      gcp_project: my-project-id
      location: us-central1     # optional — "global" when omitted
```

If running in a CI environment or on a server, use a service account:
```bash
gcloud auth activate-service-account --key-file=service-account.json
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account.json"
```

imagine doesn't read `GOOGLE_APPLICATION_CREDENTIALS` itself — it's the standard env var that the Google SDK respects when building the ADC.

## Minimal configs by use case

**Just Gemini:**
```yaml
providers:
  gemini:
    api_key: AIza-your-key-here
```

**Just OpenAI:**
```yaml
providers:
  openai:
    api_key: sk-your-key-here
```

**Both, default to OpenAI:**
```yaml
default_provider: openai
providers:
  gemini:
    api_key: AIza-your-key-here
  openai:
    api_key: sk-your-key-here
```

**All three:**
```yaml
default_provider: gemini
providers:
  gemini:
    api_key: AIza-your-key-here
  openai:
    api_key: sk-your-key-here
  vertex:
    provider_options:
      gcp_project: my-project
      location: global
```

## Anti-patterns

- **Don't** include providers you haven't configured — they take up space and confuse `imagine providers show`.
- **Don't** add unknown top-level fields. imagine's YAML parser accepts them but doesn't do anything with them, so unused fields are dead weight.
- **Don't** quote string values unless they contain special YAML characters. `api_key: sk-...` is fine; `api_key: "sk-..."` is allowed but unnecessary.
- **Don't** check this file into a public git repo. API keys in the config are not protected by anything beyond `chmod 0600`.
- **Don't** mix YAML and JSON syntax. The old `config.json` format from banana-cli is not accepted.
