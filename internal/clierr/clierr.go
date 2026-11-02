// Package clierr renders command errors for fang.
package clierr

import (
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/fang/v2"
)

// Handler is fang's default error rendering with a safer first-word rule.
// fang title-cases the first word of every error, which turns flag names
// into "--Thinking" and provider prefixes into "Openai:". Handler
// capitalises only a plain lowercase word and leaves the rest as written.
func Handler(w io.Writer, styles fang.Styles, err error) {
	styles.ErrorText = styles.ErrorText.Transform(Capitalize)
	fang.DefaultErrorHandler(w, styles, err)
}

// Capitalize upper-cases the first letter when the first word is made of
// lowercase letters only; anything else (flags, IDs, "name:" prefixes) is
// returned unchanged.
func Capitalize(s string) string {
	body := strings.TrimLeftFunc(s, unicode.IsSpace)
	word, _, _ := strings.Cut(body, " ")
	if word == "" || strings.IndexFunc(word, func(r rune) bool { return !unicode.IsLower(r) }) >= 0 {
		return s
	}
	r, size := utf8.DecodeRuneInString(body)
	return s[:len(s)-len(body)] + string(unicode.ToUpper(r)) + body[size:]
}
