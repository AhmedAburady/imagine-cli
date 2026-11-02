package vertex

import (
	"strings"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers/gemini"
)

func TestCheckLocation_GlobalOnlyModel(t *testing.T) {
	if err := checkLocation(gemini.ModelFlash, "global"); err != nil {
		t.Errorf("flash on global should pass: %v", err)
	}
	err := checkLocation(gemini.ModelFlash, "us-central1")
	if err == nil || !strings.Contains(err.Error(), "global") {
		t.Errorf("flash on us-central1 should name the global location, got %v", err)
	}
	for _, model := range []string{gemini.ModelPro, gemini.ModelFlashLite} {
		if err := checkLocation(model, "us-central1"); err != nil {
			t.Errorf("%s is regional and should pass: %v", model, err)
		}
	}
}
