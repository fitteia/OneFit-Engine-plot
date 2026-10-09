package figure

import (
	"bytes"
	"errors"
	"math"
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

func TestAddGraphAndLayout(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	src := load(t, "test7-10.agr")
	id, err := AddGraph(fig, src, "b/fit-curves-10.agr")
	if err != nil || id != 1 || len(fig.Graphs) != 2 {
		t.Fatalf("AddGraph: id %d, %d graphs, %v", id, len(fig.Graphs), err)
	}
	g1 := fig.Graphs[1]
	if len(g1.Sets) != len(src.Graphs[0].Sets) || g1.Sets[0].ID != 0 {
		t.Errorf("graph 1 sets: %d, first %d", len(g1.Sets), g1.Sets[0].ID)
	}
	if ref, ok := SourceOf(g1.Sets[2]); !ok || ref != (Ref{"b/fit-curves-10.agr", 0, 2}) {
		t.Errorf("source %q", g1.Sets[2].Comment)
	}
	labels := 0
	for _, s := range fig.Strings {
		if s.LocType == "world" && s.Graph == 1 {
			labels++
		}
	}
	if labels != 3 {
		t.Errorf("%d curve labels moved to graph 1, want test7's 3", labels)
	}

	for _, l := range Layouts {
		if err := Layout(fig, l); err != nil {
			t.Fatalf("%s: %v", l, err)
		}
		a, b := fig.Graphs[0].View, fig.Graphs[1].View
		switch l {
		case "side-by-side":
			if !(a.XMax < b.XMin && a.YMin == b.YMin) {
				t.Errorf("side by side: %+v %+v", a, b)
			}
		case "stacked":
			if !(b.YMax < a.YMin && a.XMin == b.XMin) {
				t.Errorf("stacked: %+v %+v", a, b)
			}
		}
		for _, g := range fig.Graphs {
			v := g.View
			if v.XMin <= 0 || v.YMin <= 0 || v.XMax > 773.0/600 || v.YMax > 1 || v.XMin >= v.XMax || v.YMin >= v.YMax {
				t.Errorf("%s: graph %d view %+v leaves the page", l, g.ID, v)
			}
		}
	}
	// sizes follow the panel and come back with it
	if err := Layout(fig, "single"); err != nil {
		t.Fatal(err)
	}
	full := fig.Graphs[0].X.TickLabel.CharSize
	Layout(fig, "grid")
	small := fig.Graphs[0].X.TickLabel.CharSize
	Layout(fig, "single")
	if !(small < full) || math.Abs(fig.Graphs[0].X.TickLabel.CharSize-full) > 1e-9 {
		t.Errorf("tick label size: single %g, grid %g, single again %g", full, small, fig.Graphs[0].X.TickLabel.CharSize)
	}
	if err := Layout(fig, "diagonal"); err == nil {
		t.Error("an unknown layout was accepted")
	}

	if err := RemoveGraph(fig, 1); err != nil || len(fig.Graphs) != 1 {
		t.Fatalf("RemoveGraph: %v, %d graphs", err, len(fig.Graphs))
	}
	for _, s := range fig.Strings {
		if s.LocType == "world" && s.Graph == 1 {
			t.Error("a text of the removed graph is left")
		}
	}
	if err := RemoveGraph(fig, 0); err == nil {
		t.Error("the last graph was removed")
	}
}
