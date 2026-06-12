package images

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
)

// TextTag is one keyword/value pair embedded into a PNG.
type TextTag struct{ Key, Value string }

// MetadataTags is the canonical --embed-metadata field set. refs are the edit-mode
// reference paths: omitted when none, a bare path for one, a JSON array for several.
func MetadataTags(prompt, model, provider string, refs []string) []TextTag {
	tags := []TextTag{
		{Key: "prompt", Value: prompt},
		{Key: "model", Value: model},
		{Key: "provider", Value: provider},
	}
	switch {
	case len(refs) == 1:
		tags = append(tags, TextTag{Key: "reference_image", Value: refs[0]})
	case len(refs) > 1:
		b, _ := json.Marshal(refs)
		tags = append(tags, TextTag{Key: "reference_image", Value: string(b)})
	}
	return tags
}

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// EmbedPNGText splices UTF-8 iTXt chunks (one per valid tag) immediately after
// the IHDR chunk and reports whether anything was embedded. It re-encodes nothing
// (one allocation + copy) and returns (data, false) for non-PNG input or no valid tags.
func EmbedPNGText(data []byte, tags []TextTag) ([]byte, bool) {
	if len(tags) == 0 || len(data) < 33 || !bytes.HasPrefix(data, pngSignature) || string(data[12:16]) != "IHDR" {
		return data, false
	}
	if binary.BigEndian.Uint32(data[8:12]) != 13 {
		return data, false // IHDR is spec-fixed at 13 bytes; bail rather than splice at a bad offset
	}
	const insert = 33 // 8 (signature) + 8 (length+type) + 13 (IHDR data) + 4 (CRC)

	var chunks []byte
	for _, t := range tags {
		if !validKeyword(t.Key) {
			continue // skip keys that would make an invalid PNG chunk
		}
		chunks = append(chunks, itxtChunk(t.Key, t.Value)...)
	}
	if len(chunks) == 0 {
		return data, false
	}

	out := make([]byte, 0, len(data)+len(chunks))
	out = append(out, data[:insert]...)
	out = append(out, chunks...)
	out = append(out, data[insert:]...)
	return out, true
}

// ReadPNGText returns the uncompressed iTXt tags in order. nil+nil = valid PNG with no metadata; a non-nil error = not-a-PNG or malformed (bad CRC / truncated).
func ReadPNGText(data []byte) ([]TextTag, error) {
	if len(data) < 8 || !bytes.HasPrefix(data, pngSignature) {
		return nil, errors.New("not a PNG file")
	}
	var tags []TextTag
	first, sawIEND := true, false
	for i := 8; i < len(data); {
		if i+8 > len(data) {
			return nil, errors.New("malformed PNG: truncated chunk header")
		}
		ln := int(binary.BigEndian.Uint32(data[i : i+4]))
		typ := string(data[i+4 : i+8])
		body := i + 8
		if ln < 0 || body+ln+4 > len(data) {
			return nil, errors.New("malformed PNG: truncated chunk data")
		}
		if first {
			if typ != "IHDR" || ln != 13 {
				return nil, errors.New("malformed PNG: missing IHDR chunk")
			}
			first = false
		}
		got := crc32.ChecksumIEEE(data[i+4 : body+ln]) // CRC covers type + data
		if got != binary.BigEndian.Uint32(data[body+ln:body+ln+4]) {
			return nil, fmt.Errorf("malformed PNG: bad CRC in %s chunk", typ)
		}
		if typ == "iTXt" {
			if tag, ok := parseITXt(data[body : body+ln]); ok {
				tags = append(tags, tag)
			}
		}
		if typ == "IEND" {
			sawIEND = true
			break
		}
		i = body + ln + 4
	}
	if !sawIEND {
		return nil, errors.New("malformed PNG: no IEND chunk")
	}
	return tags, nil
}

// validKeyword rejects keys PNG forbids outright: empty, over 79 bytes, or containing NUL.
func validKeyword(key string) bool {
	return len(key) >= 1 && len(key) <= 79 && !strings.Contains(key, "\x00")
}

// parseITXt decodes keyword \0 compFlag compMethod langTag \0 transKeyword \0 text.
// Only uncompressed (compFlag 0) chunks are returned.
func parseITXt(b []byte) (TextTag, bool) {
	nul := bytes.IndexByte(b, 0)
	if nul < 0 || nul+2 >= len(b) || b[nul+1] != 0 {
		return TextTag{}, false // not us / compressed
	}
	rest := b[nul+3:] // skip compFlag + compMethod
	langEnd := bytes.IndexByte(rest, 0)
	if langEnd < 0 {
		return TextTag{}, false
	}
	rest = rest[langEnd+1:]
	_, after, ok := bytes.Cut(rest, []byte{0})
	if !ok {
		return TextTag{}, false
	}
	return TextTag{Key: string(b[:nul]), Value: string(after)}, true
}

// itxtChunk builds one uncompressed iTXt chunk: keyword \0 compFlag compMethod langTag \0 transKeyword \0 utf8text.
func itxtChunk(key, val string) []byte {
	var body bytes.Buffer
	body.WriteString(key)
	body.Write([]byte{0, 0, 0, 0, 0}) // null sep, compFlag, compMethod, empty langTag\0, empty transKeyword\0
	body.WriteString(val)

	payload := body.Bytes()
	chunk := make([]byte, 0, 12+len(payload))
	chunk = binary.BigEndian.AppendUint32(chunk, uint32(len(payload)))
	chunk = append(chunk, "iTXt"...)
	chunk = append(chunk, payload...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	return chunk
}
