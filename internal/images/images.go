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
	"strings"
	"sync"
)

// Reference is a provider-agnostic handle to a reference image: raw file bytes
// plus its MIME type. Providers do their own encoding.
type Reference struct {
	MimeType string
	Data     []byte
}

// supportedExts maps lowercase file extensions (with dot) to MIME types.
var supportedExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// IsSupported reports whether the given path has a supported image extension.
func IsSupported(path string) bool {
	_, ok := supportedExts[strings.ToLower(filepath.Ext(path))]
	return ok
}

// MimeType returns the MIME type for a supported extension (with or without dot).
// Returns ok=false for unsupported extensions.
func MimeType(ext string) (string, bool) {
	mt, ok := supportedExts[strings.ToLower(ext)]
	return mt, ok
}

// Load reads a single image file or a directory of images and returns a slice
// of References. Directory entries load in parallel with order preserved.
func Load(path string) ([]Reference, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return loadDir(path)
	}
	return loadFile(path)
}

// CountInDir returns how many supported image files live directly in dirPath
// (non-recursive).
func CountInDir(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := supportedExts[strings.ToLower(filepath.Ext(entry.Name()))]; ok {
			count++
		}
	}
	return count, nil
}

func loadFile(filePath string) ([]Reference, error) {
	mt, ok := supportedExts[strings.ToLower(filepath.Ext(filePath))]
	if !ok {
		return nil, fmt.Errorf("unsupported image format: %s", filepath.Ext(filePath))
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %v", err)
	}
	return []Reference{{MimeType: mt, Data: data}}, nil
}

func loadDir(dirPath string) ([]Reference, error) {
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
		mt, ok := supportedExts[strings.ToLower(filepath.Ext(entry.Name()))]
		if !ok {
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
