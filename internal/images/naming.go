package images

import (
	"fmt"
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
// Rules (identical to banana-cli's behaviour):
//   - -f "image.png":   single → "image.png"; multi → "image_1.png", "image_2.png", …
//     Only .jpg/.jpeg extensions are honoured; anything else falls back to .png.
//   - -r with RefInputPath "photo.jpg": single → "photo.png"; multi → "photo_1.png", …
//   - Neither: default "generated_{index+1}_{YYYYMMDD_HHMMSS}.png".
func ResolveFilename(p FilenameParams) string {
	switch {
	case p.Custom != "":
		rawExt := strings.ToLower(filepath.Ext(p.Custom))
		stem := strings.TrimSuffix(p.Custom, filepath.Ext(p.Custom))
		ext := ".png"
		if rawExt == ".jpg" || rawExt == ".jpeg" {
			ext = ".jpg"
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

// HasJPEGExt reports whether a filename's extension is .jpg or .jpeg.
func HasJPEGExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg"
}
