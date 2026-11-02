package images

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// FilenameParams captures everything ResolveFilename needs. Keeping this
// neutral (no dependency on api.Config) avoids an import cycle.
type FilenameParams struct {
	Custom       string // -f flag (e.g. "image.png")
	Preserve     bool   // -r flag
	RefInputPath string // used when Preserve is true
	Index        int    // 0-based image index in this batch
	Total        int    // total images requested (n)
}

// ResolveFilename applies the -f → -r → default precedence rule to produce
// the output filename for a single generated image. The returned name is
// just the base (e.g. "sunset_2.png"), not a full path.
//
// Rules:
//   - -f "image.png":   single → "image.png"; multi → "image_1.png", "image_2.png", …
//     .jpg/.jpeg and .webp are honoured; anything else falls back to .png.
//     MatchExt corrects the result once the provider's bytes are known.
//   - -r with RefInputPath "photo.jpg": single → "photo.png"; multi → "photo_1.png", …
//   - Neither: default "generated_{index+1}_{YYYYMMDD_HHMMSS}.png".
func ResolveFilename(p FilenameParams) string {
	switch {
	case p.Custom != "":
		rawExt := strings.ToLower(filepath.Ext(p.Custom))
		stem := strings.TrimSuffix(p.Custom, filepath.Ext(p.Custom))
		ext := ".png"
		switch rawExt {
		case ".jpg", ".jpeg":
			ext = ".jpg"
		case ".webp":
			ext = ".webp"
		}
		if p.Total > 1 {
			return fmt.Sprintf("%s_%d%s", stem, p.Index+1, ext)
		}
		return stem + ext

	case p.Preserve && p.RefInputPath != "":
		base := filepath.Base(p.RefInputPath)
		stem := strings.TrimSuffix(base, filepath.Ext(base))
		if p.Total > 1 {
			return fmt.Sprintf("%s_%d.png", stem, p.Index+1)
		}
		return stem + ".png"

	default:
		return fmt.Sprintf("generated_%d_%s.png", p.Index+1, time.Now().Format("20060102_150405"))
	}
}

// MatchExt swaps a filename's image extension for the one its bytes call for,
// so a provider that cannot honour the requested format (Gemini has no WebP)
// never writes PNG data under .webp, nor WebP data under .png.
func MatchExt(filename string, data []byte) string {
	var want string
	switch http.DetectContentType(data) {
	case "image/png":
		want = ".png"
	case "image/jpeg":
		want = ".jpg"
	case "image/webp":
		want = ".webp"
	default:
		return filename
	}
	ext := filepath.Ext(filename)
	if strings.EqualFold(ext, want) || (want == ".jpg" && strings.EqualFold(ext, ".jpeg")) {
		return filename
	}
	return strings.TrimSuffix(filename, ext) + want
}

// OutputPath places a resolved filename: an absolute -f is used as written,
// anything else lands under the output folder.
func OutputPath(folder, filename string) string {
	if filepath.IsAbs(filename) {
		return filename
	}
	return filepath.Join(folder, filename)
}

// HasJPEGExt reports whether a filename's extension is .jpg or .jpeg.
func HasJPEGExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg"
}
