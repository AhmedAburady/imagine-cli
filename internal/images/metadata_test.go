package images

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func makePNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func TestEmbedPNGText_ValidAndReadable(t *testing.T) {
	tags := []TextTag{{"prompt", "a sunset over the 海 🌅"}, {"model", "gpt-image-2"}, {"provider", "openai"}}
	out := EmbedPNGText(makePNG(t), tags)

	// Still a valid PNG (png.Decode verifies every chunk CRC).
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("embedded PNG no longer decodes: %v", err)
	}
	// Each tag's keyword and UTF-8 value survives in the bytes.
	for _, tag := range tags {
		if !bytes.Contains(out, []byte(tag.Key)) || !bytes.Contains(out, []byte(tag.Value)) {
			t.Errorf("tag %q=%q not found in output", tag.Key, tag.Value)
		}
	}
	if !bytes.Contains(out, []byte("iTXt")) {
		t.Error("no iTXt chunk written")
	}
}

func TestEmbedPNGText_NonPNGUnchanged(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 1, 2, 3}
	if got := EmbedPNGText(jpeg, []TextTag{{"prompt", "x"}}); !bytes.Equal(got, jpeg) {
		t.Error("non-PNG input was modified")
	}
	png := makePNG(t)
	if got := EmbedPNGText(png, nil); !bytes.Equal(got, png) {
		t.Error("nil tags should return PNG unchanged")
	}
}
