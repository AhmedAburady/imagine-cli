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

// expandEnv runs os.Expand and returns an error if any referenced variable is
// unset. Treats `$$` as a literal `$` (standard os.Expand behaviour) and
// leaves values without `$` untouched.
func expandEnv(s string) (string, error) {
	if !strings.Contains(s, "$") {
		return s, nil
	}
	var missing []string
	expanded := os.Expand(s, func(name string) string {
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
			return ""
		}
		return v
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("environment variable %s is not set", strings.Join(missing, ", "))
	}
	return expanded, nil
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
		return "", fmt.Errorf("op read %s failed: %s", ref, msg)
	}
	return stdout.String(), nil
}
