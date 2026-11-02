package gemini

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseStore(t *testing.T) {
	for raw, want := range map[string]string{"": "nil", "false": "false", " TRUE ": "true", "False": "false"} {
		got, err := parseStore(raw)
		if err != nil {
			t.Fatalf("parseStore(%q): %v", raw, err)
		}
		s := "nil"
		if got != nil {
			s = map[bool]string{true: "true", false: "false"}[*got]
		}
		if s != want {
			t.Errorf("parseStore(%q) = %s, want %s", raw, s, want)
		}
	}
	if _, err := parseStore("no"); err == nil {
		t.Error("parseStore(\"no\") should fail")
	}
}

// Unset must leave the field off the wire so the project setting applies.
func TestStore_OnTheWire(t *testing.T) {
	off := false
	cases := []struct {
		store *bool
		want  string
		has   bool
	}{{nil, `"store"`, false}, {&off, `"store":false`, true}}
	for _, c := range cases {
		b, err := json.Marshal(geminiRequest{Store: c.store})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), c.want) != c.has {
			t.Errorf("store=%v: body %s, want contains %s = %v", c.store, b, c.want, c.has)
		}
	}
}
