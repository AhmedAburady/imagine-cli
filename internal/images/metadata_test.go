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
	out, ok := EmbedPNGText(makePNG(t), tags)
	if !ok {
		t.Fatal("EmbedPNGText reported nothing embedded")
	}

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

func TestMetadataTags_References(t *testing.T) {
	none := MetadataTags("p", "m", "openai", nil)
	if len(none) != 3 {
		t.Errorf("no refs: want 3 tags, got %d", len(none))
	}
	one := MetadataTags("p", "m", "openai", []string{"ref.png"})
	if one[3].Key != "reference_image" || one[3].Value != "ref.png" {
		t.Errorf("single ref: got %+v", one[3])
	}
	many := MetadataTags("p", "m", "openai", []string{"a.png", "b.png"})
	if many[3].Key != "reference_image" || many[3].Value != `["a.png","b.png"]` {
		t.Errorf("multi ref: want JSON array, got %q", many[3].Value)
	}
}

func TestReadPNGText_RoundTrip(t *testing.T) {
	tags := []TextTag{{"prompt", "a sunset over the 海 🌅\nsecond line"}, {"model", "gpt-image-2"}, {"provider", "openai"}}
	embedded, _ := EmbedPNGText(makePNG(t), tags)
	got, err := ReadPNGText(embedded)
	if err != nil {
		t.Fatalf("ReadPNGText: %v", err)
	}
	if len(got) != len(tags) {
		t.Fatalf("got %d tags, want %d: %+v", len(got), len(tags), got)
	}
	for i, want := range tags {
		if got[i] != want {
			t.Errorf("tag %d: got %+v, want %+v", i, got[i], want)
		}
	}
}

func TestReadPNGText_DistinguishesEmptyFromInvalid(t *testing.T) {
	// Valid PNG, no metadata: nil tags, nil error.
	if got, err := ReadPNGText(makePNG(t)); got != nil || err != nil {
		t.Errorf("plain PNG: want (nil,nil), got (%+v,%v)", got, err)
	}
	// Not a PNG: error.
	if _, err := ReadPNGText([]byte{0xff, 0xd8, 0xff}); err == nil {
		t.Error("non-PNG should error")
	}
	// Bare signature, no chunks: malformed, not valid-empty.
	if _, err := ReadPNGText(append([]byte(nil), pngSignature...)); err == nil {
		t.Error("signature-only PNG should error (no IHDR/IEND)")
	}
	// Valid chunks but IEND stripped: malformed.
	full, _ := EmbedPNGText(makePNG(t), []TextTag{{"prompt", "x"}})
	if _, err := ReadPNGText(full[:len(full)-12]); err == nil {
		t.Error("PNG without IEND should error")
	}
	// Corrupt a chunk byte (after IHDR) → CRC mismatch → error.
	bad, _ := EmbedPNGText(makePNG(t), []TextTag{{"prompt", "x"}})
	bad[40] ^= 0xff
	if _, err := ReadPNGText(bad); err == nil {
		t.Error("corrupted PNG should error on CRC")
	}
}

func TestEmbedPNGText_SkipsInvalidKeys(t *testing.T) {
	out, ok := EmbedPNGText(makePNG(t), []TextTag{{"", "empty key"}, {"ok", "v"}, {"has\x00nul", "v"}})
	if !ok {
		t.Fatal("the one valid key should have embedded")
	}
	tags, err := ReadPNGText(out)
	if err != nil {
		t.Fatalf("ReadPNGText: %v", err)
	}
	if len(tags) != 1 || tags[0].Key != "ok" {
		t.Errorf("only the valid key should survive, got %+v", tags)
	}
}

func TestEmbedPNGText_NonPNGUnchanged(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 1, 2, 3}
	if got, ok := EmbedPNGText(jpeg, []TextTag{{"prompt", "x"}}); ok || !bytes.Equal(got, jpeg) {
		t.Error("non-PNG input was modified or reported embedded")
	}
	png := makePNG(t)
	if got, ok := EmbedPNGText(png, nil); ok || !bytes.Equal(got, png) {
		t.Error("nil tags should return PNG unchanged and not embedded")
	}
}
