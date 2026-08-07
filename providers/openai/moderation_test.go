package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AhmedAburady/imagine-cli/internal/transport"
)

func blockingServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func postTo(t *testing.T, url string) error {
	t.Helper()
	_, err := transport.PostJSON[generationsResponse](context.Background(),
		transport.NewClient(5*time.Second), url, transport.NoAuth(), generationsBody{})
	if err == nil {
		t.Fatal("expected an error from the stub server")
	}
	return describeAPIError(err)
}

func TestDescribeAPIError_AddsModerationDetails(t *testing.T) {
	srv := blockingServer(t, `{"error":{"message":"Your request was rejected.","code":"moderation_blocked",
		"moderation_details":{"moderation_stage":"input","categories":["violence","self-harm"]}}}`)

	got := postTo(t, srv.URL).Error()
	for _, want := range []string{"Your request was rejected.", "moderation stage: input", "violence", "self-harm"} {
		if !strings.Contains(got, want) {
			t.Errorf("error %q is missing %q", got, want)
		}
	}
}

func TestDescribeAPIError_UnknownStageStillReported(t *testing.T) {
	srv := blockingServer(t, `{"error":{"message":"nope","code":"moderation_blocked",
		"moderation_details":{"categories":["sexual"]}}}`)

	got := postTo(t, srv.URL).Error()
	if !strings.Contains(got, "moderation stage: unknown") || !strings.Contains(got, "sexual") {
		t.Errorf("error %q should name an unknown stage and the category", got)
	}
}

func TestDescribeAPIError_LeavesOtherErrorsAlone(t *testing.T) {
	for _, body := range []string{
		`{"error":{"message":"bad size","code":"invalid_request_error"}}`,
		`{"error":{"message":"nope","code":"moderation_blocked"}}`,
	} {
		srv := blockingServer(t, body)
		if got := postTo(t, srv.URL).Error(); strings.Contains(got, "moderation stage") {
			t.Errorf("error %q should have passed through unchanged", got)
		}
	}
}
