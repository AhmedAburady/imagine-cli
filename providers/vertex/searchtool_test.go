package vertex

import "testing"

func TestSearchTool(t *testing.T) {
	if searchTool(false, false) != nil {
		t.Error("no flags: want no tool")
	}
	if gs := searchTool(true, false); gs == nil || gs.SearchTypes != nil {
		t.Errorf("grounding only: want a bare googleSearch, got %+v", gs)
	}
	gs := searchTool(false, true)
	if gs == nil || gs.SearchTypes == nil || gs.SearchTypes.ImageSearch == nil || gs.SearchTypes.WebSearch != nil {
		t.Errorf("image search only: want searchTypes{imageSearch}, got %+v", gs)
	}
	gs = searchTool(true, true)
	if gs == nil || gs.SearchTypes == nil || gs.SearchTypes.ImageSearch == nil || gs.SearchTypes.WebSearch == nil {
		t.Errorf("both: want searchTypes{webSearch, imageSearch}, got %+v", gs)
	}
}
