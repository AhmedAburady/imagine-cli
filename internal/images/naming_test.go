package images

import "testing"

func TestResolveFilename_HonoursWebP(t *testing.T) {
	if got := ResolveFilename(FilenameParams{Custom: "cat.webp", Total: 2, Index: 1}); got != "cat_2.webp" {
		t.Errorf("got %q, want cat_2.webp", got)
	}
}

func TestMatchExt(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n0000")
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	jpeg := []byte("\xff\xd8\xff\xe0")
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"cat.webp", webp, "cat.webp"},
		{"cat.png", webp, "cat.webp"},
		{"cat.webp", png, "cat.png"},
		{"cat.jpeg", jpeg, "cat.jpeg"},
		{"cat.png", png, "cat.png"},
		{"cat.png", []byte("not an image"), "cat.png"},
	}
	for _, c := range cases {
		if got := MatchExt(c.name, c.data); got != c.want {
			t.Errorf("MatchExt(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestConvertToPNG_JPEGBecomesPNG(t *testing.T) {
	src := testJPEG(t)
	out, err := ConvertToPNG(src)
	if err != nil {
		t.Fatal(err)
	}
	if MatchExt("x.png", out) != "x.png" {
		t.Error("JPEG input did not become PNG")
	}
	again, _ := ConvertToJPEG(src)
	if &again[0] != &src[0] {
		t.Error("ConvertToJPEG re-encoded data that was already JPEG")
	}
}
