package providers

// UnusedSecretKeys returns config keys that belong only to a non-active auth
// method, so the secret resolver can skip fetching their (possibly remote)
// references. A stanza carrying both an API key and auth_method: subscription,
// for instance, should not pay the 1Password round-trip for the unused key.
//
// Returns nil — meaning "resolve everything" — when the provider isn't
// multi-auth or the active method can't be determined cheaply from raw config.
// A field shared with the active method is never skipped, so no needed secret
// is ever withheld.
func UnusedSecretKeys(name string, raw map[string]string) []string {
	b, ok := Get(name)
	if !ok || len(b.AuthMethods) == 0 {
		return nil
	}
	active := raw["auth_method"]
	if active == "" {
		return nil
	}

	used, other := map[string]bool{}, map[string]bool{}
	matched := false
	for _, m := range b.AuthMethods {
		dst := other
		if m.Key == active {
			dst = used
			matched = true
		}
		for _, f := range m.Fields {
			dst[f.Key] = true
		}
	}
	// An unknown/typo'd auth_method must not make us skip (and thus leave
	// unresolved) every field — resolve everything and let the provider error
	// on the bad method instead of feeding it a raw op:// string.
	if !matched {
		return nil
	}

	var skip []string
	for k := range other {
		if !used[k] {
			skip = append(skip, k)
		}
	}
	return skip
}
