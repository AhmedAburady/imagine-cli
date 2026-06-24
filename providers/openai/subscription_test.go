package openai

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

var _ providers.Describer = (*Provider)(nil)

func makeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func writeTempStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "store.json")
	if err := saveAuth(path, &storedAuth{}); err != nil {
		t.Fatalf("write temp store: %v", err)
	}
	return path
}

func TestResolveMethod(t *testing.T) {
	cases := []struct {
		name string
		auth providers.Auth
		want authMethod
	}{
		{"explicit subscription", providers.Auth{"auth_method": "subscription"}, methodSubscription},
		{"explicit api_key", providers.Auth{"auth_method": "api_key"}, methodAPIKey},
		{"infer api_key from key", providers.Auth{"api_key": "sk-x"}, methodAPIKey},
		// No key and no token store at the (overridden, nonexistent) path → api_key.
		{"infer none", providers.Auth{"auth_file": filepath.Join(t.TempDir(), "absent.json")}, methodAPIKey},
		// A token store present at the path → subscription.
		{"infer subscription from store", providers.Auth{"auth_file": writeTempStore(t)}, methodSubscription},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveMethod(c.auth); got != c.want {
				t.Errorf("resolveMethod = %q, want %q", got, c.want)
			}
		})
	}
}

func TestNew_APIKeyRequiresKey(t *testing.T) {
	if _, err := New(providers.Auth{"auth_method": "api_key"}); err == nil {
		t.Error("expected error for api_key method without a key")
	}
	if _, err := New(providers.Auth{"auth_method": "api_key", "api_key": "sk-x"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_SubscriptionNoKeyOK(t *testing.T) {
	p, err := New(providers.Auth{"auth_method": "subscription"})
	if err != nil {
		t.Fatalf("subscription New errored without a key: %v", err)
	}
	if p.Info().Capabilities.MaxBatchN != 1 {
		t.Errorf("subscription MaxBatchN = %d, want 1", p.Info().Capabilities.MaxBatchN)
	}
}

func TestInfo_MaxBatchNByMethod(t *testing.T) {
	if (&Provider{method: methodAPIKey}).Info().Capabilities.MaxBatchN != 10 {
		t.Error("api_key MaxBatchN should be 10")
	}
	if (&Provider{method: methodSubscription}).Info().Capabilities.MaxBatchN != 1 {
		t.Error("subscription MaxBatchN should be 1")
	}
}

func TestNewPKCE_ChallengeMatchesVerifier(t *testing.T) {
	pk, err := newPKCE()
	if err != nil {
		t.Fatalf("newPKCE: %v", err)
	}
	if pk.verifier == "" || pk.challenge == "" {
		t.Fatal("empty verifier or challenge")
	}
	sum := sha256.Sum256([]byte(pk.verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); pk.challenge != want {
		t.Errorf("challenge = %q, want %q", pk.challenge, want)
	}
	pk2, _ := newPKCE()
	if pk2.verifier == pk.verifier {
		t.Error("two PKCE verifiers were identical")
	}
}

func TestAccountIDFromIDToken(t *testing.T) {
	tok := makeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_account_id": "acc_123"}})
	if got := accountIDFromIDToken(tok); got != "acc_123" {
		t.Errorf("accountID = %q, want acc_123", got)
	}
	if got := accountIDFromIDToken("not-a-jwt"); got != "" {
		t.Errorf("expected empty for malformed token, got %q", got)
	}
}

func TestJWTExpiry(t *testing.T) {
	exp := time.Now().Add(time.Hour).Unix()
	got, ok := jwtExpiry(makeJWT(t, map[string]any{"exp": float64(exp)}))
	if !ok || got.Unix() != exp {
		t.Fatalf("jwtExpiry = (%v,%v)", got, ok)
	}
	if _, ok := jwtExpiry(makeJWT(t, map[string]any{"sub": "x"})); ok {
		t.Error("expected no exp")
	}
}

func TestNeedsRefresh(t *testing.T) {
	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	fresh := makeJWT(t, map[string]any{"exp": float64(now.Add(time.Hour).Unix())})
	near := makeJWT(t, map[string]any{"exp": float64(now.Add(2 * time.Minute).Unix())})
	noExp := makeJWT(t, map[string]any{"sub": "x"})

	cases := []struct {
		name string
		sa   storedAuth
		want bool
	}{
		{"no token", storedAuth{}, true},
		{"fresh", withAccess(fresh), false},
		{"near expiry", withAccess(near), true},
		{"no exp recent", withAccessRefresh(noExp, now.Add(-time.Hour)), false},
		{"no exp stale", withAccessRefresh(noExp, now.Add(-9*24*time.Hour)), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := needsRefresh(&c.sa, now); got != c.want {
				t.Errorf("needsRefresh = %v, want %v", got, c.want)
			}
		})
	}
}

func withAccess(tok string) storedAuth {
	var sa storedAuth
	sa.Tokens.AccessToken = tok
	return sa
}

func withAccessRefresh(tok string, last time.Time) storedAuth {
	sa := withAccess(tok)
	sa.LastRefresh = last
	return sa
}

func TestApplyRefresh_PartialResponse(t *testing.T) {
	now := time.Now()
	sa := withAccess("old-access")
	sa.Tokens.RefreshToken = "old-refresh"
	sa.Tokens.AccountID = "acc_old"

	applyRefresh(&sa, &tokenResponse{AccessToken: "new-access"}, now)
	if sa.Tokens.AccessToken != "new-access" || sa.Tokens.RefreshToken != "old-refresh" || sa.Tokens.AccountID != "acc_old" {
		t.Errorf("partial refresh clobbered fields: %+v", sa.Tokens)
	}

	newID := makeJWT(t, map[string]any{authClaim: map[string]any{"chatgpt_account_id": "acc_new"}})
	applyRefresh(&sa, &tokenResponse{IDToken: newID, RefreshToken: "r2"}, now)
	if sa.Tokens.AccountID != "acc_new" || sa.Tokens.RefreshToken != "r2" {
		t.Errorf("refresh update wrong: %+v", sa.Tokens)
	}
}

func TestTokenStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if _, err := loadAuth(path); err != errNotSignedIn {
		t.Fatalf("loadAuth(missing) = %v, want errNotSignedIn", err)
	}
	var sa storedAuth
	sa.Tokens.AccessToken = "a"
	sa.Tokens.AccountID = "acc"
	sa.LastRefresh = time.Now().Truncate(time.Second)
	if err := saveAuth(path, &sa); err != nil {
		t.Fatalf("saveAuth: %v", err)
	}
	got, err := loadAuth(path)
	if err != nil {
		t.Fatalf("loadAuth: %v", err)
	}
	if got.Tokens.AccessToken != "a" || got.Tokens.AccountID != "acc" || !got.LastRefresh.Equal(sa.LastRefresh) {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
}

func TestBuildAuthorizeURL(t *testing.T) {
	u := buildAuthorizeURL("CHAL", "STATE", "http://localhost:1455/auth/callback")
	for _, want := range []string{
		"response_type=code", "client_id=" + oauthClientID, "code_challenge=CHAL",
		"code_challenge_method=S256", "state=STATE", "id_token_add_organizations=true",
		"redirect_uri=http%3A%2F%2Flocalhost%3A1455%2Fauth%2Fcallback",
	} {
		if !strings.Contains(u, want) {
			t.Errorf("authorize URL missing %q", want)
		}
	}
}

func TestDoTokenRequest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"access_token":"at","id_token":"it","refresh_token":"rt"}`))
		}))
		defer srv.Close()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, nil)
		tr, err := doTokenRequest(req)
		if err != nil || tr.AccessToken != "at" {
			t.Fatalf("got %+v, %v", tr, err)
		}
	})
	t.Run("error envelope", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"expired"}`))
		}))
		defer srv.Close()
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL, nil)
		if _, err := doTokenRequest(req); err == nil || !strings.Contains(err.Error(), "invalid_grant") {
			t.Fatalf("expected envelope error, got %v", err)
		}
	})
}

func TestApiSizeQuality(t *testing.T) {
	if apiSize("auto") != "" || apiSize("1024x1024") != "1024x1024" {
		t.Error("apiSize wrong")
	}
	if apiQuality("auto") != "" || apiQuality("high") != "high" {
		t.Error("apiQuality wrong")
	}
}

func TestBuildTool(t *testing.T) {
	raw, _ := json.Marshal(buildTool(&Options{Model: "gpt-image-2", Size: "auto", Quality: "auto", OutputFormat: "png", Compression: 100}))
	if s := string(raw); strings.Contains(s, "\"size\"") || strings.Contains(s, "output_compression") || !strings.Contains(s, `"output_format":"png"`) {
		t.Errorf("tool omit/keep wrong: %s", s)
	}
	raw2, _ := json.Marshal(buildTool(&Options{Model: "gpt-image-1.5", Size: "1536x1024", Quality: "high", OutputFormat: "jpeg", Compression: 80, Background: "opaque"}))
	for _, want := range []string{`"size":"1536x1024"`, `"quality":"high"`, `"output_compression":80`, `"background":"opaque"`} {
		if !strings.Contains(string(raw2), want) {
			t.Errorf("tool missing %s", want)
		}
	}
}

func TestResponsesContent_EditAttachesImages(t *testing.T) {
	content := responsesContent("make it winter", []images.Reference{{MimeType: "image/png", Data: []byte("foo")}})
	if len(content) != 2 || content[0].Type != "input_text" || content[1].Type != "input_image" {
		t.Fatalf("content shape wrong: %+v", content)
	}
	if content[1].ImageURL != "data:image/png;base64,Zm9v" {
		t.Errorf("image url = %q", content[1].ImageURL)
	}
}

func TestParseImageStream(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"response.created"}`,
		`data: {"type":"response.output_item.done","item":{"type":"image_generation_call","output_format":"jpeg","result":"Zm9v"}}`,
		`data: [DONE]`,
	}, "\n")
	out, err := parseImageStream(strings.NewReader(stream), "png")
	if err != nil {
		t.Fatalf("parseImageStream: %v", err)
	}
	if len(out.Assets) != 1 || string(out.Assets[0].Data) != "foo" || out.Assets[0].MimeType != "image/jpeg" {
		t.Errorf("bad image: %+v", out.Assets)
	}
}

func TestParseImageStream_Errors(t *testing.T) {
	fail := `data: {"type":"response.failed","response":{"error":{"message":"blocked"}}}` + "\n"
	if _, err := parseImageStream(strings.NewReader(fail), "png"); err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected blocked error, got %v", err)
	}
	none := `data: {"type":"response.completed","response":{}}` + "\n" + "data: [DONE]\n"
	if _, err := parseImageStream(strings.NewReader(none), "png"); err == nil {
		t.Error("expected no-image error")
	}
}

func TestParseImageStream_IncompleteSurfacesReason(t *testing.T) {
	stream := `data: {"type":"response.incomplete","response":{"incomplete_details":{"reason":"content_filter"}}}` + "\n"
	_, err := parseImageStream(strings.NewReader(stream), "png")
	if err == nil || !strings.Contains(err.Error(), "content_filter") {
		t.Fatalf("expected incomplete reason surfaced, got %v", err)
	}
}

func TestStallReader_AbortsOnSilence(t *testing.T) {
	pr, _ := io.Pipe() // never written; the read blocks until the stall watchdog closes it
	sr := newStallReader(pr, 30*time.Millisecond)
	if _, err := sr.Read(make([]byte, 8)); !errors.Is(err, errStalled) {
		t.Fatalf("expected errStalled, got %v", err)
	}
}

func TestStallReader_StallsOnCustomIdleAfterData(t *testing.T) {
	pr, pw := io.Pipe()
	go func() { _, _ = pw.Write([]byte("data: {}\n")) }() // bytes, then permanent silence (no close)
	sr := newStallReader(pr, 30*time.Millisecond)
	if _, err := io.ReadAll(sr); !errors.Is(err, errStalled) {
		t.Fatalf("expected errStalled on the custom idle, got %v", err)
	}
}

func TestStallReader_PassesDataThrough(t *testing.T) {
	sr := newStallReader(io.NopCloser(strings.NewReader("hello")), time.Second)
	defer sr.Close()
	if got, err := io.ReadAll(sr); err != nil || string(got) != "hello" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestParseTextStream(t *testing.T) {
	stream := `data: {"type":"response.output_text.done","text":"A dark style."}` + "\n" + "data: [DONE]\n"
	got, err := parseTextStream(strings.NewReader(stream))
	if err != nil || got != "A dark style." {
		t.Fatalf("got %q, %v", got, err)
	}
	fail := `data: {"type":"response.failed","response":{"error":{"message":"nope"}}}` + "\n"
	if _, err := parseTextStream(strings.NewReader(fail)); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("expected error, got %v", err)
	}
}

func TestStripCodeFence(t *testing.T) {
	for in, want := range map[string]string{
		`{"a":1}`:                 `{"a":1}`,
		"```json\n{\"a\":1}\n```": `{"a":1}`,
		"```\n{\"a\":1}\n```":     `{"a":1}`,
	} {
		if got := stripCodeFence(in); got != want {
			t.Errorf("stripCodeFence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMimeTypeFor(t *testing.T) {
	for in, want := range map[string]string{"png": "image/png", "jpeg": "image/jpeg", "webp": "image/webp", "": "image/png"} {
		if got := mimeTypeFor(in); got != want {
			t.Errorf("mimeTypeFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOptionsValidateNormalize(t *testing.T) {
	info := (&Provider{}).Info()
	if err := (&Options{Model: "gpt-image-2", Size: "huge"}).Validate(info); err == nil {
		t.Error("expected bad-size error")
	}
	o := &Options{Size: "1K", Background: "auto"}
	o.Normalize()
	if o.Size != "1024x1024" || o.Background != "" {
		t.Errorf("normalize wrong: %+v", o)
	}
	if err := finalizeOptions(&Options{Model: "gpt-image-2", Background: "transparent", OutputFormat: "png"}); err == nil {
		t.Error("expected transparent+gpt-image-2 error")
	}
}
