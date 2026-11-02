package images

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
)

// ConvertToJPEG re-encodes PNG data as JPEG at quality 95 for .jpg/.jpeg
// output. JPEG data passes through untouched, so it is not re-compressed.
func ConvertToJPEG(src []byte) ([]byte, error) {
	if http.DetectContentType(src) == "image/jpeg" {
		return src, nil
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// ConvertToPNG re-encodes JPEG data as PNG for .png output (the Gemini API
// returns JPEG for some models). PNG data and formats the standard library
// cannot decode (WebP) pass through untouched.
func ConvertToPNG(src []byte) ([]byte, error) {
	if http.DetectContentType(src) != "image/jpeg" {
		return src, nil
	}
	img, err := jpeg.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}
	return buf.Bytes(), nil
}
