package gvision

import (
	"testing"

	"google.golang.org/genai"
)

func TestThinkingLevel(t *testing.T) {
	cases := map[string]genai.ThinkingLevel{
		"minimal": genai.ThinkingLevelMinimal,
		"low":     genai.ThinkingLevelLow,
		"medium":  genai.ThinkingLevelMedium,
		"high":    genai.ThinkingLevelHigh,
		"HIGH":    genai.ThinkingLevelHigh,
		"":        genai.ThinkingLevelHigh, // empty/unknown → high
		"unknown": genai.ThinkingLevelHigh,
	}
	for in, want := range cases {
		if got := thinkingLevel(in); got != want {
			t.Errorf("thinkingLevel(%q) = %v; want %v", in, got, want)
		}
	}
}
