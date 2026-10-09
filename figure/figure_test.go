package figure

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

func load(t *testing.T, name string) *agr.Project {
	t.Helper()
	p, err := agr.ParseFile("../testdata/corpus/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestFromPlotRecordsSources(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "batch/fit-a/fit-curves-1.agr")
	for _, s := range fig.Graphs[0].Sets {
		ref, ok := SourceOf(s)
		if !ok || ref != (Ref{"batch/fit-a/fit-curves-1.agr", 0, s.ID}) {
			t.Errorf("set %d: comment %q", s.ID, s.Comment)
		}
	}
}

func TestAddSetsAndUpdate(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	before := len(fig.Graphs[0].Sets)
	src := load(t, "test7-10.agr")
	added, err := AddSets(fig, 0, src, "b/fit-curves-10.agr", []int{0, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 2 || added[0] != before || added[1] != before+1 {
		t.Fatalf("added %v after %d sets", added, before)
	}
	got := fig.Graphs[0].Sets[before+1]
	if got.Line.Color != src.Graphs[0].Sets[2].Line.Color || len(got.Data) != len(src.Graphs[0].Sets[2].Data) {
		t.Errorf("set copied wrongly: %+v", got.Line)
	}
	// a copy, not shared: changing the figure leaves the source alone
	got.Data[0][1] = 999
	if src.Graphs[0].Sets[2].Data[0][1] == 999 {
		t.Error("the figure shares data with its source")
	}
	if _, err := AddSets(fig, 0, src, "b/fit-curves-10.agr", []int{42}); err == nil {
		t.Error("a missing set was added")
	}

	// written and read back, the sources survive
	var b bytes.Buffer
	if err := agr.Write(&b, fig); err != nil {
		t.Fatal(err)
	}
	fig, err = agr.Parse(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if ref, ok := SourceOf(fig.Graphs[0].Sets[before+1]); !ok || ref.Path != "b/fit-curves-10.agr" || ref.Set != 2 {
		t.Fatalf("source after a round trip: %q", fig.Graphs[0].Sets[before+1].Comment)
	}

	// newer data in a source comes in; style edits in the figure stay
	fig.Graphs[0].Sets[before+1].Line.Color = 5
	newer := load(t, "test7-10.agr")
	newer.Graphs[0].Sets[2].Data = [][]float64{{1, 2}, {3, 4}}
	n, warnings := Update(fig, func(path string) (*agr.Project, error) {
		switch path {
		case "b/fit-curves-10.agr":
			return newer, nil
		case "a/fit-curves-1.agr":
			return nil, errors.New("gone")
		}
		return nil, errors.New("unexpected " + path)
	})
	s := fig.Graphs[0].Sets[before+1]
	if len(s.Data) != 2 || s.Data[1][1] != 4 || s.Line.Color != 5 {
		t.Errorf("after Update: data %v, color %d", s.Data, s.Line.Color)
	}
	if n != 2 {
		t.Errorf("%d sets updated, want the two from b", n)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "gone") {
		t.Errorf("warnings %v, want one for the unreadable source", warnings)
	}
}

func TestSourceOf(t *testing.T) {
	for comment, want := range map[string]bool{
		"src: x/y.agr#G0.S1":    true,
		"src: a#b/y.agr#G2.S10": true,
		"sm1-gph.agr":           false,
		"src: #G0.S1":           false,
		"src: y.agr#G0":         false,
		"src: y.agr#Gx.S1":      false,
	} {
		if _, ok := SourceOf(&agr.Set{Comment: comment}); ok != want {
			t.Errorf("SourceOf(%q) ok = %v", comment, ok)
		}
	}
}
