package render

import (
	"reflect"
	"sort"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

// Every element names what it draws, so an editor can select it; test3
// draws exactly these things.
func TestElementIDs(t *testing.T) {
	p, err := agr.ParseFile("../testdata/corpus/test3-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	d, _ := Render(p)
	seen := map[string]bool{}
	for _, x := range d.Paths {
		if x.ID == "" {
			t.Fatalf("a path without an ID: %+v", x)
		}
		seen[x.ID] = true
	}
	for _, x := range d.Texts {
		if x.ID == "" {
			t.Fatalf("a text without an ID: %+v", x)
		}
		seen[x.ID] = true
	}
	var got []string
	for id := range seen {
		got = append(got, id)
	}
	sort.Strings(got)
	// string.0 is the run name "[1]" (view), string.1 the "(a)" label
	want := []string{"g0.frame", "g0.s0.symbols", "g0.s1.line", "g0.s2.line", "g0.x.label", "g0.x.tick", "g0.x.ticklabel",
		"g0.y.label", "g0.y.tick", "g0.y.ticklabel", "page", "string.0", "string.1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("IDs %v\nwant %v", got, want)
	}
	for _, x := range d.Texts {
		if x.Str == "[1]" && x.ID != "string.0" || x.Str == "(a)" && x.ID != "string.1" {
			t.Errorf("text %q has ID %q", x.Str, x.ID)
		}
	}
}
