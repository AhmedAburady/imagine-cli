package api

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/AhmedAburady/imagine-cli/internal/images"
	"github.com/AhmedAburady/imagine-cli/providers"
)

// fakeProvider returns fixed bytes so a test can control the saved image's format.
type fakeProvider struct{ data []byte }

func (f fakeProvider) Info() providers.Info {
	return providers.Info{Name: "fake", Capabilities: providers.Capabilities{MaxBatchN: 1}}
}

func (f fakeProvider) Generate(context.Context, providers.Request) (*providers.Response, error) {
	return &providers.Response{Assets: []providers.GeneratedImage{{Data: f.data}}}, nil
}

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
	saveOne(&res, providers.GeneratedAsset{Data: testPNG(t), MimeType: "image/png"}, params)
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

func TestRunGeneration_MetadataSkippedReflectsActualBytes(t *testing.T) {
	meta := []images.TextTag{{Key: "prompt", Value: "x"}}

	// PNG bytes → embedded → not skipped.
	pngOut := RunGeneration(context.Background(), fakeProvider{data: testPNG(t)}, providers.Request{},
		Params{OutputFolder: t.TempDir(), NumImages: 1, Metadata: meta})
	if pngOut.MetadataSkipped {
		t.Error("PNG output: MetadataSkipped should be false")
	}

	// -f x.webp + non-PNG provider bytes → not embeddable → skipped (the OpenAI-webp case).
	webpOut := RunGeneration(context.Background(), fakeProvider{data: []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'}},
		providers.Request{}, Params{OutputFolder: t.TempDir(), NumImages: 1, OutputFilename: "x.webp", Metadata: meta})
	if !webpOut.MetadataSkipped {
		t.Error("non-PNG (webp) output: MetadataSkipped should be true")
	}
}

func TestSaveOne_NoMetadataLeavesPNGUntouched(t *testing.T) {
	dir := t.TempDir()
	src := testPNG(t)
	res := GenerationResult{Index: 0}
	saveOne(&res, providers.GeneratedAsset{Data: src, MimeType: "image/png"}, Params{OutputFolder: dir, NumImages: 1})
	if res.Error != nil {
		t.Fatalf("saveOne: %v", res.Error)
	}
	saved, _ := os.ReadFile(filepath.Join(dir, res.Filename))
	if !bytes.Equal(saved, src) {
		t.Error("PNG bytes changed without --embed-metadata")
	}
}
