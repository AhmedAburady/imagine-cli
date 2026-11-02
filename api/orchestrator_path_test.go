package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmedAburady/imagine-cli/providers"
)

// An absolute -f is used as written, and its directory is created up front.
func TestRunGeneration_AbsoluteFilename(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "leaf.png")
	out := RunGeneration(context.Background(), fakeProvider{data: testPNG(t)}, providers.Request{},
		Params{OutputFolder: t.TempDir(), NumImages: 1, OutputFilename: target})
	if err := out.Results[0].Error; err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("image not written to the absolute -f path: %v", err)
	}
}

// A relative -f with a directory part lands under -o, directory created.
func TestRunGeneration_RelativeNestedFilename(t *testing.T) {
	folder := t.TempDir()
	out := RunGeneration(context.Background(), fakeProvider{data: testPNG(t)}, providers.Request{},
		Params{OutputFolder: folder, NumImages: 2, OutputFilename: "sub/leaf.png"})
	for _, r := range out.Results {
		if r.Error != nil {
			t.Fatalf("image %d: %v", r.Index, r.Error)
		}
	}
	for _, name := range []string{"leaf_1.png", "leaf_2.png"} {
		if _, err := os.Stat(filepath.Join(folder, "sub", name)); err != nil {
			t.Errorf("%s not written under -o/sub: %v", name, err)
		}
	}
}
