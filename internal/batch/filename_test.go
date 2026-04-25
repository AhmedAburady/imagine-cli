package batch

import "testing"

func TestSanitizeStem(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hero", "hero"},
		{"hero shot", "hero_shot"},
		{"hero  shot", "hero_shot"},
		{"dir/file", "dir_file"},
		{`a\b`, "a_b"},
		{"x:y*z?", "x_y_z"},
		{`<bad>"name"|`, "bad_name"},
		{"...dots...", "dots"},
		{"___under___", "under"},
		{"", "entry"},
		{"   ", "entry"},
		{".///|", "entry"},
	}
	for _, c := range cases {
		if got := sanitizeStem(c.in); got != c.want {
			t.Errorf("sanitizeStem(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
