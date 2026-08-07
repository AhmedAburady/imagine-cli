package openai

import "testing"

func TestValidateImageSize(t *testing.T) {
	cases := []struct {
		size string
		ok   bool
		why  string
	}{
		{"", true, "empty means auto"},
		{"auto", true, "auto"},
		{"1024x1024", true, "1K shorthand target"},
		{"2048x2048", true, "2K shorthand target"},
		{"3840x2160", true, "4K shorthand target, exactly on the pixel ceiling"},
		{"1536x1024", true, "landscape"},
		{"1024x1536", true, "portrait"},
		{"1024x640", true, "exactly on the pixel floor"},
		{"1000x1000", false, "edges not multiples of 16"},
		{"3856x2160", false, "longest edge over 3840"},
		{"3840x1024", false, "ratio wider than 3:1"},
		{"1024x3840", false, "ratio taller than 1:3"},
		{"640x640", false, "under the pixel floor"},
		{"3840x2176", false, "over the pixel ceiling"},
		{"huge", false, "not a WxH string"},
		{"1024x", false, "missing an edge"},
		{"-16x1024", false, "negative edge"},
	}
	for _, c := range cases {
		err := validateImageSize(c.size)
		if c.ok && err != nil {
			t.Errorf("validateImageSize(%q) = %v, want nil (%s)", c.size, err, c.why)
		}
		if !c.ok && err == nil {
			t.Errorf("validateImageSize(%q) = nil, want an error (%s)", c.size, c.why)
		}
	}
}

func TestCanonicalSizesAllValidate(t *testing.T) {
	for _, shorthand := range []string{"1K", "2K", "4K", "auto", ""} {
		size := canonicalSize(shorthand)
		if err := validateImageSize(size); err != nil {
			t.Errorf("shorthand %q canonicalised to %q which fails validation: %v", shorthand, size, err)
		}
	}
}
