package api

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmedAburady/imagine-cli/internal/images"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestSaveOne_EmbedsMetadataIntoPNG(t *testing.T) {
	dir := t.TempDir()
	params := Params{
		OutputFolder: dir,
		NumImages:    1,
		Metadata:     []images.TextTag{{Key: "prompt", Value: "a teal cube"}, {Key: "provider", Value: "openai"}},
	}
	res := GenerationResult{Index: 0}
	saveOne(&res, testPNG(t), params)
	if res.Error != nil {
		t.Fatalf("saveOne: %v", res.Error)
	}

	saved, err := os.ReadFile(filepath.Join(dir, res.Filename))
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(saved)); err != nil {
		t.Fatalf("saved PNG invalid: %v", err)
	}
	if !bytes.Contains(saved, []byte("a teal cube")) || !bytes.Contains(saved, []byte("iTXt")) {
		t.Error("metadata not embedded in saved PNG")
	}
}

func TestSaveOne_NoMetadataLeavesPNGUntouched(t *testing.T) {
	dir := t.TempDir()
	src := testPNG(t)
	res := GenerationResult{Index: 0}
	saveOne(&res, src, Params{OutputFolder: dir, NumImages: 1})
	if res.Error != nil {
		t.Fatalf("saveOne: %v", res.Error)
	}
	saved, _ := os.ReadFile(filepath.Join(dir, res.Filename))
	if !bytes.Equal(saved, src) {
		t.Error("PNG bytes changed without --embed-metadata")
	}
}
