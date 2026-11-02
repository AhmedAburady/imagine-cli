package clierr

import "testing"

func TestCapitalize(t *testing.T) {
	cases := map[string]string{
		"invalid --size \"512\"":                          "Invalid --size \"512\"",
		"--thinking medium is not supported":              "--thinking medium is not supported",
		"openai: unsupported effort \"max\"":              "openai: unsupported effort \"max\"",
		"gemini-nano-banana-2.1 is served only in global": "gemini-nano-banana-2.1 is served only in global",
		"  unknown model":                                 "  Unknown model",
		"":                                                "",
		"Already Capital":                                 "Already Capital",
	}
	for in, want := range cases {
		if got := Capitalize(in); got != want {
			t.Errorf("Capitalize(%q) = %q, want %q", in, got, want)
		}
	}
}
