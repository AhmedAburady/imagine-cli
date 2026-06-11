package openai

// OAuth/PKCE primitives shared by the subscription login flow and the silent
// token-refresh path. These values are a faithful Go port of OpenAI's Codex CLI
// "Sign in with ChatGPT" implementation (codex-rs/login) — we authenticate as
// the same public PKCE client because it is the only one OpenAI sanctions for a
// ChatGPT *subscription* (rather than API-key) credential. Every constant was
// lifted from current openai/codex source, not guessed.

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	oauthIssuer  = "https://auth.openai.com"
	authorizeURL = oauthIssuer + "/oauth/authorize"
	tokenURL     = oauthIssuer + "/oauth/token"

	// oauthClientID is Codex's public PKCE client; the code_verifier is the
	// proof-of-possession. Verified from codex-rs/login/src/auth/manager.rs.
	oauthClientID = "app_EMoamEEZ73f0CkXaXp7hrann"

	// The api.connectors.* scopes unlock the ChatGPT-backend surfaces rather
	// than plain API access.
	oauthScope = "openid profile email offline_access api.connectors.read api.connectors.invoke"

	// Codex binds 1455 (fallback 1457); the redirect_uri host is literally
	// "localhost" though the socket listens on 127.0.0.1. Both must match the
	// server-side allow-list, so these aren't free parameters.
	callbackPort         = 1455
	callbackPortFallback = 1457
	callbackPath         = "/auth/callback"
	successPath          = "/success"

	// authClaim is the id_token claim carrying chatgpt_account_id — the source
	// of the ChatGPT-Account-Id header value.
	authClaim = "https://api.openai.com/auth"

	// We present as the Codex client we authenticated as; a foreign originator
	// risks the backend rejecting the call.
	originator = "codex_cli_rs"
	userAgent  = "imagine-cli (+https://github.com/AhmedAburady/imagine-cli) codex_cli_rs"

	// Refresh within refreshSkew of token expiry; if the access token carries
	// no exp claim, fall back to refreshing after refreshMaxAge.
	refreshSkew   = 5 * time.Minute
	refreshMaxAge = 8 * 24 * time.Hour
)

type pkce struct {
	verifier  string
	challenge string
}

// newPKCE generates a 64-byte verifier and its S256 challenge =
// base64url(SHA256(ascii(verifier))) — hashed over the encoded verifier's
// bytes, per RFC 7636 and Codex.
func newPKCE() (pkce, error) {
	raw := make([]byte, 64)
	if _, err := rand.Read(raw); err != nil {
		return pkce{}, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	return pkce{verifier: verifier, challenge: base64.RawURLEncoding.EncodeToString(sum[:])}, nil
}

func randomState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate OAuth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// buildAuthorizeURL includes the two Codex-specific flags
// (id_token_add_organizations, codex_cli_simplified_flow) the upstream client sends.
func buildAuthorizeURL(challenge, state, redirectURI string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", oauthClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", oauthScope)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("state", state)
	q.Set("originator", originator)
	return authorizeURL + "?" + q.Encode()
}

// tokenResponse is the subset of the /oauth/token response we consume; expiry
// is derived from the JWT instead of expires_in, as Codex does.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

// exchangeCode trades an authorization code for tokens; this leg is
// application/x-www-form-urlencoded.
func exchangeCode(ctx context.Context, code, verifier, redirectURI string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", oauthClientID)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return doTokenRequest(req)
}

type refreshRequest struct {
	ClientID     string `json:"client_id"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// refreshAccessToken exchanges a refresh token for a fresh set. Note the
// asymmetry with exchangeCode: refresh is JSON, code-exchange is form-encoded.
func refreshAccessToken(ctx context.Context, refreshToken string) (*tokenResponse, error) {
	payload, err := json.Marshal(refreshRequest{ClientID: oauthClientID, GrantType: "refresh_token", RefreshToken: refreshToken})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return doTokenRequest(req)
}

func doTokenRequest(req *http.Request) (*tokenResponse, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, oauthError(resp.StatusCode, raw)
	}
	var tr tokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}
	return &tr, nil
}

func oauthError(status int, raw []byte) error {
	var e struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		if e.ErrorDescription != "" {
			return fmt.Errorf("oauth error (%d): %s: %s", status, e.Error, e.ErrorDescription)
		}
		return fmt.Errorf("oauth error (%d): %s", status, e.Error)
	}
	return fmt.Errorf("oauth error (status %d)", status)
}

// jwtClaims base64url-decodes a JWT payload. The signature is NOT verified:
// these tokens arrive over TLS from the token endpoint and are read only for
// self-reported metadata (account id, expiry), never as an authz decision.
func jwtClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("malformed JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse JWT payload: %w", err)
	}
	return claims, nil
}

// accountIDFromIDToken reads id_token → authClaim → chatgpt_account_id ("" when
// absent, as personal accounts may omit it).
func accountIDFromIDToken(idToken string) string {
	claims, err := jwtClaims(idToken)
	if err != nil {
		return ""
	}
	auth, _ := claims[authClaim].(map[string]any)
	id, _ := auth["chatgpt_account_id"].(string)
	return id
}

func jwtExpiry(token string) (time.Time, bool) {
	claims, err := jwtClaims(token)
	if err != nil {
		return time.Time{}, false
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(int64(exp), 0), true
}
