package openai

// On-disk token store for the subscription route plus the silent-refresh path
// used on every Generate/Describe call. Kept separate from config.yaml: OAuth
// tokens rotate, are large, and must never be hand-edited, so they live in
// their own 0600 file.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AhmedAburady/imagine-cli/config"
)

const authFileName = "openai-subscription-auth.json"

var errNotSignedIn = errors.New("no ChatGPT subscription session — run `imagine providers add openai` and choose Subscription")

// storedAuth mirrors a subset of Codex's auth.json shape (familiar, but our
// own file).
type storedAuth struct {
	Tokens struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AccountID    string `json:"account_id"`
	} `json:"tokens"`
	LastRefresh time.Time `json:"last_refresh"`
}

// defaultAuthPath is ~/.config/imagine/<authFileName> (platform-appropriate);
// "" when the config dir can't be resolved.
func defaultAuthPath() string {
	dir := config.DefaultConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, authFileName)
}

// subscriptionAuthFile resolves the token-store path: an explicit override, else
// the default location.
func subscriptionAuthFile(override string) string {
	if override != "" {
		return override
	}
	return defaultAuthPath()
}

// loginStorePath is where the interactive login writes tokens. The login flow
// has no Provider/Auth in hand, so it reads providers.openai.auth_file straight
// from config to stay in sync with what New() → ensureFreshToken later loads
// (the value is a plain path, so no secret resolution is needed).
func loginStorePath() string {
	if cfg, err := config.Load(); err == nil {
		if af := cfg.Providers["openai"]["auth_file"]; af != "" {
			return af
		}
	}
	return defaultAuthPath()
}

// subscriptionConfigured reports whether a token store exists at the resolved
// path — used to infer the auth method for configs predating auth_method.
func subscriptionConfigured(override string) bool {
	path := subscriptionAuthFile(override)
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// loadAuth reads the token store; a missing file is errNotSignedIn.
func loadAuth(path string) (*storedAuth, error) {
	if path == "" {
		return nil, errNotSignedIn
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errNotSignedIn
		}
		return nil, fmt.Errorf("read token store %s: %w", path, err)
	}
	var sa storedAuth
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, fmt.Errorf("parse token store %s: %w", path, err)
	}
	return &sa, nil
}

// saveAuth writes the store via a temp-file rename with 0600 perms.
func saveAuth(path string, sa *storedAuth) error {
	if path == "" {
		return errors.New("cannot determine token store path (no home/config dir)")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(sa, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write token store: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("finalize token store: %w", err)
	}
	return nil
}

// needsRefresh refreshes within refreshSkew of the access token's exp; with no
// exp claim it falls back to last_refresh age. A missing access token forces a
// refresh so a held refresh token gets a chance to mint one.
func needsRefresh(sa *storedAuth, now time.Time) bool {
	if sa.Tokens.AccessToken == "" {
		return true
	}
	if exp, ok := jwtExpiry(sa.Tokens.AccessToken); ok {
		return now.After(exp.Add(-refreshSkew))
	}
	return sa.LastRefresh.Before(now.Add(-refreshMaxAge))
}

// applyRefresh overwrites only the fields the server returned, re-deriving the
// account id when a new id_token arrives.
func applyRefresh(sa *storedAuth, tr *tokenResponse, now time.Time) {
	if tr.AccessToken != "" {
		sa.Tokens.AccessToken = tr.AccessToken
	}
	if tr.RefreshToken != "" {
		sa.Tokens.RefreshToken = tr.RefreshToken
	}
	if tr.IDToken != "" {
		sa.Tokens.IDToken = tr.IDToken
		if id := accountIDFromIDToken(tr.IDToken); id != "" {
			sa.Tokens.AccountID = id
		}
	}
	sa.LastRefresh = now
}

// ensureFreshToken loads (then caches) the store, refreshes when due, persists
// any change, and returns the bearer token + account id. The whole sequence
// runs under p.mu so concurrent Generate goroutines share one refresh rather
// than racing to rotate the refresh token.
func (p *Provider) ensureFreshToken(ctx context.Context) (accessToken, accountID string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cached == nil {
		sa, lerr := loadAuth(p.authFile)
		if lerr != nil {
			return "", "", lerr
		}
		p.cached = sa
	}
	sa := p.cached

	if needsRefresh(sa, time.Now()) {
		if sa.Tokens.RefreshToken == "" {
			return "", "", errNotSignedIn
		}
		tr, rerr := refreshAccessToken(ctx, sa.Tokens.RefreshToken)
		if rerr != nil {
			// A concurrent process may have refreshed first, rotating (and
			// invalidating) our refresh token. Re-read the store; if it now
			// holds a usable token, adopt it instead of forcing a re-login.
			if fresh, lerr := loadAuth(p.authFile); lerr == nil && !needsRefresh(fresh, time.Now()) {
				p.cached = fresh
				return fresh.Tokens.AccessToken, fresh.Tokens.AccountID, nil
			}
			return "", "", fmt.Errorf("session expired and refresh failed — re-run `imagine providers add openai`: %w", rerr)
		}
		applyRefresh(sa, tr, time.Now())
		if serr := saveAuth(p.authFile, sa); serr != nil {
			return "", "", serr
		}
	}

	if sa.Tokens.AccessToken == "" {
		return "", "", errNotSignedIn
	}
	return sa.Tokens.AccessToken, sa.Tokens.AccountID, nil
}
