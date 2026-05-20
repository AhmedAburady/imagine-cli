package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// opRunner reads a 1Password secret reference and returns its plaintext value.
// Overridable in tests; the default runner shells out to the `op` CLI.
var opRunner = defaultOpRunner

const opTimeout = 5 * time.Second

// ResolveProvider returns a copy of providers.<name>'s config with every
// value resolved through expandEnv + 1Password lookup. The original config
// map is left untouched so subsequent calls (or `providers show`-style
// metadata reads) don't see resolved secrets.
//
// Two reference styles are supported:
//
//	${VAR}            — environment variable expansion (os.Expand semantics)
//	op://vault/...    — 1Password CLI lookup via `op read`
//
// Env vars are expanded first so a reference like op://Vault/${ITEM}/field
// composes naturally. Missing env vars are an error — silently expanding to
// "" produces opaque 401s downstream.
//
// Resolution is lazy and per-provider: only the active provider pays the
// `op` round-trip on any given invocation, so commands like `providers show`
// or `--help` don't trigger a 1Password auth prompt.
func (c *Config) ResolveProvider(name string) (ProviderConfig, error) {
	if c == nil {
		return nil, nil
	}
	pc, ok := c.Providers[name]
	if !ok {
		return nil, nil
	}
	out := make(ProviderConfig, len(pc))
	for key, val := range pc {
		resolved, err := resolveValue(val)
		if err != nil {
			return nil, fmt.Errorf("providers.%s.%s: %w", name, key, err)
		}
		out[key] = resolved
	}
	return out, nil
}

// resolveValue applies env expansion then 1Password resolution to a single
// scalar. Plain literals pass through untouched.
func resolveValue(v string) (string, error) {
	expanded, err := expandEnv(v)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(expanded, "op://") {
		ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
		defer cancel()
		return opRunner(ctx, expanded)
	}
	return expanded, nil
}

// expandEnv replaces every ${NAME} occurrence with os.Getenv(NAME) and errors
// if any referenced variable is unset. Only ${...} is special — bare `$`,
// `$VAR` (no braces), and `$$` all pass through verbatim, so a literal API
// token containing `$` characters survives unchanged. Missing-variable errors
// are loud by design; a silent empty expansion would surface later as an
// opaque 401.
func expandEnv(s string) (string, error) {
	if !strings.Contains(s, "${") {
		return s, nil
	}
	var missing []string
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '$' && s[i+1] == '{' {
			end := strings.IndexByte(s[i+2:], '}')
			if end < 0 {
				// Unterminated ${ — leave the rest literal so the user sees
				// their config value verbatim in any downstream error.
				b.WriteString(s[i:])
				break
			}
			name := s[i+2 : i+2+end]
			if v, ok := os.LookupEnv(name); ok {
				b.WriteString(v)
			} else {
				missing = append(missing, name)
			}
			i += 2 + end + 1
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable %s is not set", strings.Join(missing, ", "))
	}
	return b.String(), nil
}

// defaultOpRunner invokes the 1Password CLI to read a secret reference.
// Returns a clear, actionable error if `op` is not installed.
func defaultOpRunner(ctx context.Context, ref string) (string, error) {
	if _, err := exec.LookPath("op"); err != nil {
		return "", errors.New("1Password CLI (`op`) not found in PATH — install from https://developer.1password.com/docs/cli/ or replace the op:// reference with a literal value")
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "op", "read", "--no-newline", ref)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("op read failed: %s", msg)
	}
	return stdout.String(), nil
}
