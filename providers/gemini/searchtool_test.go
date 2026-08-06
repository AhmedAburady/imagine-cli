package gemini

import (
	"encoding/json"
	"testing"
)

func TestBuildSearchTool_WireShapes(t *testing.T) {
	cases := []struct {
		grounding, images bool
		want              string
	}{
		{false, false, "null"},
		{true, false, `{}`},
		{false, true, `{"searchTypes":{"imageSearch":{}}}`},
		{true, true, `{"searchTypes":{"webSearch":{},"imageSearch":{}}}`},
	}
	for _, c := range cases {
		raw, err := json.Marshal(buildSearchTool(c.grounding, c.images))
		if err != nil {
			t.Fatalf("marshal(grounding=%v, images=%v): %v", c.grounding, c.images, err)
		}
		if string(raw) != c.want {
			t.Errorf("grounding=%v images=%v marshalled to %s, want %s",
				c.grounding, c.images, raw, c.want)
		}
	}
}

// --grounding on its own must keep sending exactly what it always has.
func TestBuildSearchTool_GroundingOnlyIsUnchanged(t *testing.T) {
	raw, err := json.Marshal([]tool{{GoogleSearch: buildSearchTool(true, false)}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `[{"googleSearch":{}}]`; string(raw) != want {
		t.Errorf("tools = %s, want %s", raw, want)
	}
}
