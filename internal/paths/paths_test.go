package paths

import "testing"

func TestIsBatchFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"batch.yaml", true},
		{"batch.YAML", true},
		{"batch.yml", true},
		{"batch.json", true},
		{"prompt.txt", false},
		{"noext", false},
		{"some.md", false},
	}
	for _, c := range cases {
		if got := IsBatchFile(c.path); got != c.want {
			t.Errorf("IsBatchFile(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}
