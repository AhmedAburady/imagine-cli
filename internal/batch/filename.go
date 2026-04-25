package batch

import (
	"regexp"
	"strings"
)

// reservedRE matches the union of POSIX path separators and Windows
// reserved characters. Replacing them with underscore lets us turn
// arbitrary entry keys ("hero shot", "dir/file", "x:y") into safe
// filename stems on every OS we ship for.
var reservedRE = regexp.MustCompile(`[\\/:*?"<>|]+`)

// whitespaceRE collapses any run of whitespace into a single
// underscore, so "hero  shot" → "hero_shot".
var whitespaceRE = regexp.MustCompile(`\s+`)

// sanitizeStem turns a raw entry key into a filesystem-safe filename
// stem. It does not add an extension — ResolveFilename handles that
// downstream. Empty results fall back to "entry" so we always produce
// a usable name.
func sanitizeStem(s string) string {
	s = reservedRE.ReplaceAllString(s, "_")
	s = whitespaceRE.ReplaceAllString(s, "_")
	s = strings.Trim(s, "._")
	if s == "" {
		return "entry"
	}
	return s
}
