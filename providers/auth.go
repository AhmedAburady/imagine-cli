package providers

// Auth is the per-provider credential envelope handed to a factory. APIKey
// covers the common case; Options is a free-form string map for extras like
// Vertex's gcp_project and location.
type Auth struct {
	APIKey  string
	Options map[string]string
}
