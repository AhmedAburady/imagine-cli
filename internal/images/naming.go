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
	AssetMime    string // asset MIME type; drives the default extension (empty → image/png)
}

// ExtForMime maps a MIME type to its output file extension. Unknown or empty
// input returns ".png" to preserve the historical image default.
func ExtForMime(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	default:
		return ".png"
	}
}

// ResolveFilename applies the -f → -r → default precedence rule to produce
// the output filename for a single generated image. The returned name is
// just the base (e.g. "sunset_2.png"), not a full path.
//
// The extension tracks the asset's actual bytes (ExtForMime(AssetMime); empty
// → .png). Rules:
//   - -f "image.png":   single → "image.png"; multi → "image_1.png", "image_2.png", …
//     The only -f extension that changes the result is .jpg/.jpeg on an image,
//     which forces .jpg and drives saveOne's PNG→JPEG conversion. Other -f
//     extensions are stem-only — the format follows the bytes, not the name.
//   - -r with RefInputPath "photo.jpg": single → "photo<defaultExt>"; multi → "photo_1<defaultExt>", …
//   - Neither: default "generated_{index+1}_{YYYYMMDD_HHMMSS}<defaultExt>".
func ResolveFilename(p FilenameParams) string {
	defaultExt := ExtForMime(p.AssetMime)
	switch {
	case p.Custom != "":
		rawExt := strings.ToLower(filepath.Ext(p.Custom))
		stem := strings.TrimSuffix(p.Custom, filepath.Ext(p.Custom))
		ext := defaultExt
		// JPEG conversion is the one format the save path can produce on demand,
		// so -f *.jpg/.jpeg on an image forces .jpg. Empty AssetMime = image.
		assetKind := KindOf(p.AssetMime)
		if assetKind == "" {
			assetKind = "image"
		}
		if assetKind == "image" && (rawExt == ".jpg" || rawExt == ".jpeg") {
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
			return fmt.Sprintf("%s_%d%s", stem, p.Index+1, defaultExt)
		}
		return stem + defaultExt

	default:
		return fmt.Sprintf("generated_%d_%s%s", p.Index+1, time.Now().Format("20060102_150405"), defaultExt)
	}
}

// HasJPEGExt reports whether a filename's extension is .jpg or .jpeg.
func HasJPEGExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jpg" || ext == ".jpeg"
}
