// Package images holds image utilities shared across providers: MIME detection,
// reference-image loading (files and directories), and filename resolution.
//
// A Reference carries raw bytes + MIME type. Each provider is responsible for
// whatever encoding its API demands (base64 for Gemini's inline_data, multipart
// for OpenAI's edits endpoint, genai.Blob for Vertex).
package images

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

// Reference is a provider-agnostic handle to a reference image: raw file bytes
// plus its MIME type. Providers do their own encoding.
type Reference struct {
	MimeType string
	Data     []byte
}

// supportedMediaExts is the broader image+video+audio table used by the loader.
// Callers classify a loaded Reference by its MIME prefix (image/ video/ audio/).
var supportedMediaExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".mp3":  "audio/mpeg",
	".wav":  "audio/wav",
}

// IsSupportedMedia reports whether path has a supported image/video/audio extension.
func IsSupportedMedia(path string) bool {
	_, ok := supportedMediaExts[strings.ToLower(filepath.Ext(path))]
	return ok
}

// MediaMimeType returns the MIME type for any supported image/video/audio
// extension (with or without dot). Returns ok=false for unsupported extensions.
func MediaMimeType(ext string) (string, bool) {
	mt, ok := supportedMediaExts[strings.ToLower(ext)]
	return mt, ok
}

// KindOf classifies a MIME type into its top-level prefix ("image", "video",
// "audio"), or "" when there is no recognizable prefix.
func KindOf(mimeType string) string {
	if i := strings.IndexByte(mimeType, '/'); i > 0 {
		return mimeType[:i]
	}
	return ""
}

// Load reads a single file or a directory of references, filtered to the
// accepted media classes (default image-only). Directory entries load in
// parallel with order preserved.
func Load(path string, accept ...string) ([]Reference, error) {
	if len(accept) == 0 {
		accept = []string{"image"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return loadDir(path, accept)
	}
	return loadFile(path, accept)
}

// accepts reports whether class is one of the accepted media classes.
func accepts(accept []string, class string) bool {
	return slices.Contains(accept, class)
}

// CountMediaInDir returns how many supported image/video/audio files live
// directly in dirPath (non-recursive).
func CountMediaInDir(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := supportedMediaExts[strings.ToLower(filepath.Ext(entry.Name()))]; ok {
			count++
		}
	}
	return count, nil
}

func loadFile(filePath string, accept []string) ([]Reference, error) {
	mt, ok := supportedMediaExts[strings.ToLower(filepath.Ext(filePath))]
	if !ok {
		return nil, fmt.Errorf("unsupported media format: %s", filepath.Ext(filePath))
	}
	if !accepts(accept, KindOf(mt)) {
		return nil, fmt.Errorf("unsupported reference %q: %s files are not accepted here (accepts: %s)", filePath, KindOf(mt), strings.Join(accept, ", "))
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read media: %v", err)
	}
	return []Reference{{MimeType: mt, Data: data}}, nil
}

func loadDir(dirPath string, accept []string) ([]Reference, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	type candidate struct {
		index    int
		filePath string
		mimeType string
	}
	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		mt, ok := supportedMediaExts[strings.ToLower(filepath.Ext(entry.Name()))]
		if !ok || !accepts(accept, KindOf(mt)) {
			continue
		}
		candidates = append(candidates, candidate{
			index:    len(candidates),
			filePath: filepath.Join(dirPath, entry.Name()),
			mimeType: mt,
		})
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	type loadResult struct {
		index int
		ref   Reference
		err   error
	}

	var wg sync.WaitGroup
	resultsChan := make(chan loadResult, len(candidates))
	for _, c := range candidates {
		wg.Add(1)
		go func(c candidate) {
			defer wg.Done()
			data, err := os.ReadFile(c.filePath)
			if err != nil {
				resultsChan <- loadResult{index: c.index, err: fmt.Errorf("failed to read %s: %v", filepath.Base(c.filePath), err)}
				return
			}
			resultsChan <- loadResult{index: c.index, ref: Reference{MimeType: c.mimeType, Data: data}}
		}(c)
	}
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	results := make([]Reference, len(candidates))
	for r := range resultsChan {
		if r.err != nil {
			return nil, r.err
		}
		results[r.index] = r.ref
	}
	return results, nil
}
