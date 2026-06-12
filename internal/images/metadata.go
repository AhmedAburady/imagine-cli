package images

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
)

// TextTag is one keyword/value pair embedded into a PNG.
type TextTag struct{ Key, Value string }

var pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

// EmbedPNGText splices UTF-8 iTXt chunks (one per tag) immediately after the
// IHDR chunk. It re-encodes nothing — just one allocation and a copy — and
// returns data unchanged if it isn't a PNG, so callers can invoke it blindly.
func EmbedPNGText(data []byte, tags []TextTag) []byte {
	if len(tags) == 0 || len(data) < 33 || !bytes.HasPrefix(data, pngSignature) || string(data[12:16]) != "IHDR" {
		return data
	}
	ihdrLen := binary.BigEndian.Uint32(data[8:12])
	insert := 8 + 8 + int(ihdrLen) + 4 // signature + (len+type) + data + crc
	if insert > len(data) {
		return data
	}

	var chunks []byte
	for _, t := range tags {
		chunks = append(chunks, itxtChunk(t.Key, t.Value)...)
	}

	out := make([]byte, 0, len(data)+len(chunks))
	out = append(out, data[:insert]...)
	out = append(out, chunks...)
	out = append(out, data[insert:]...)
	return out
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
