package providers

// Auth is the flat credential bag handed to a provider factory. Keys are
// whatever the provider's ConfigSchema declares (e.g. "api_key",
// "gcp_project", "location"). Providers read fields via Get(key) — no
// special APIKey field to keep semantics uniform across providers.
type Auth map[string]string

// Get returns the value at key, or "" if unset. Equivalent to map indexing;
// exists purely for call-site readability at the Provider boundary.
func (a Auth) Get(key string) string { return a[key] }

// Has reports whether key is present AND non-empty.
func (a Auth) Has(key string) bool { return a[key] != "" }
