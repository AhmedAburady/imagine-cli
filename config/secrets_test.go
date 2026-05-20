package config

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// withOpRunner swaps the package-level opRunner for the duration of the
// current (sub)test and restores it via t.Cleanup. Tests in this file must
// not call t.Parallel() — opRunner is a package-global and concurrent swaps
// would race.
func withOpRunner(t *testing.T, fn opRunnerFunc) {
	t.Helper()
	prev := opRunner
	opRunner = fn
	t.Cleanup(func() { opRunner = prev })
}

type opRunnerFunc = func(context.Context, string) (string, error)

func TestResolveValue(t *testing.T) {
	t.Setenv("IMAGINE_TEST_KEY", "sk-from-env")
	t.Setenv("IMAGINE_TEST_ITEM", "OpenAI")

	tests := []struct {
		name      string
		in        string
		opReturn  string
		opErr     error
		want      string
		wantOpRef string
		wantErr   string
	}{
		{
			name: "plain literal passthrough",
			in:   "sk-literal",
			want: "sk-literal",
		},
		{
			name: "literal dollar signs pass through",
			in:   "sk-$$special$bare",
			want: "sk-$$special$bare",
		},
		{
			name: "env expansion",
			in:   "${IMAGINE_TEST_KEY}",
			want: "sk-from-env",
		},
		{
			name:    "missing env (single) uses singular wording",
			in:      "${IMAGINE_TEST_MISSING}",
			wantErr: "environment variable IMAGINE_TEST_MISSING is not set",
		},
		{
			name:    "missing env (multiple) uses plural wording",
			in:      "${IMAGINE_TEST_MISSING_A}-${IMAGINE_TEST_MISSING_B}",
			wantErr: "environment variables IMAGINE_TEST_MISSING_A, IMAGINE_TEST_MISSING_B are not set",
		},
		{
			name:      "op ref resolved",
			in:        "op://Personal/OpenAI/api_key",
			opReturn:  "sk-from-op",
			want:      "sk-from-op",
			wantOpRef: "op://Personal/OpenAI/api_key",
		},
		{
			name:      "env interpolated inside op ref",
			in:        "op://Personal/${IMAGINE_TEST_ITEM}/api_key",
			opReturn:  "sk-from-op",
			want:      "sk-from-op",
			wantOpRef: "op://Personal/OpenAI/api_key",
		},
		{
			name:    "op runner error surfaces",
			in:      "op://Personal/Missing/api_key",
			opErr:   errors.New("item not found"),
			wantErr: "item not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotRef string
			withOpRunner(t, func(_ context.Context, ref string) (string, error) {
				gotRef = ref
				if tc.opErr != nil {
					return "", tc.opErr
				}
				return tc.opReturn, nil
			})

			got, err := resolveValue(context.Background(), tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("value: got %q, want %q", got, tc.want)
			}
			if tc.wantOpRef != "" && gotRef != tc.wantOpRef {
				t.Errorf("op ref passed: got %q, want %q", gotRef, tc.wantOpRef)
			}
		})
	}
}

func TestResolveProviderIsLazyAndIsolated(t *testing.T) {
	t.Setenv("IMAGINE_TEST_GEMINI", "gemini-from-env")

	var opCalls int
	withOpRunner(t, func(_ context.Context, ref string) (string, error) {
		opCalls++
		return "resolved:" + ref, nil
	})

	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"gemini": {"api_key": "${IMAGINE_TEST_GEMINI}"},
			"openai": {"api_key": "op://Personal/OpenAI/api_key"},
			"vertex": {"gcp_project": "my-project"},
		},
	}

	gemini, err := cfg.ResolveProvider(context.Background(), "gemini")
	if err != nil {
		t.Fatalf("ResolveProvider(gemini): %v", err)
	}
	if got := gemini["api_key"]; got != "gemini-from-env" {
		t.Errorf("gemini: got %q", got)
	}
	if opCalls != 0 {
		t.Errorf("gemini lookup must not touch op; calls=%d", opCalls)
	}

	if got := cfg.Providers["gemini"]["api_key"]; got != "${IMAGINE_TEST_GEMINI}" {
		t.Errorf("original config mutated: %q", got)
	}

	openai, err := cfg.ResolveProvider(context.Background(), "openai")
	if err != nil {
		t.Fatalf("ResolveProvider(openai): %v", err)
	}
	if got := openai["api_key"]; got != "resolved:op://Personal/OpenAI/api_key" {
		t.Errorf("openai: got %q", got)
	}
	if opCalls != 1 {
		t.Errorf("openai should trigger exactly one op call; calls=%d", opCalls)
	}
}

func TestResolveProviderErrorContextNamesProvider(t *testing.T) {
	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"openai": {"api_key": "${IMAGINE_TEST_NOPE}"},
		},
	}
	_, err := cfg.ResolveProvider(context.Background(), "openai")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "providers.openai.api_key") {
		t.Errorf("error should name provider/key: %v", err)
	}
}

func TestResolveProviderCancelsOpOnContextCancel(t *testing.T) {
	withOpRunner(t, func(ctx context.Context, _ string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &Config{
		Providers: map[string]ProviderConfig{
			"openai": {"api_key": "op://Personal/OpenAI/api_key"},
		},
	}
	_, err := cfg.ResolveProvider(ctx, "openai")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("want context-canceled error, got: %v", err)
	}
}

func TestResolveProviderUnknownReturnsNil(t *testing.T) {
	cfg := &Config{Providers: map[string]ProviderConfig{}}
	got, err := cfg.ResolveProvider(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("want nil for unknown provider, got %v", got)
	}
}
