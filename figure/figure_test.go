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

// TestRemoveGraphPlacesTheOthers: removing the graph added beside the first
// gives the first its shape back - its place and its sizes, as before the
// grid; several left keep their arrangement and close the gap.
func TestRemoveGraphPlacesTheOthers(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	view, size := fig.Graphs[0].View, fig.Graphs[0].X.TickLabel.CharSize
	if _, err := AddGraph(fig, load(t, "test7-10.agr"), "b/fit-curves-10.agr"); err != nil {
		t.Fatal(err)
	}
	if err := Layout(fig, "grid"); err != nil { // as the GUI's Add graph does
		t.Fatal(err)
	}
	if fig.Graphs[0].View == view {
		t.Fatal("the grid did not move the first graph")
	}
	if err := RemoveGraph(fig, 1); err != nil {
		t.Fatal(err)
	}
	g := fig.Graphs[0]
	for _, d := range []float64{g.View.XMin - view.XMin, g.View.YMin - view.YMin, g.View.XMax - view.XMax, g.View.YMax - view.YMax} {
		if math.Abs(d) > 1e-6 {
			t.Errorf("view %+v after the remove, want the plot's own %+v", g.View, view)
			break
		}
	}
	if math.Abs(g.X.TickLabel.CharSize-size) > 1e-9 {
		t.Errorf("tick label size %g after the remove, want %g", g.X.TickLabel.CharSize, size)
	}

	for _, layout := range []string{"stacked", "side-by-side"} {
		fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
		for i := 0; i < 2; i++ {
			if _, err := AddGraph(fig, load(t, "test7-10.agr"), "b/fit-curves-10.agr"); err != nil {
				t.Fatal(err)
			}
		}
		if err := Layout(fig, layout); err != nil {
			t.Fatal(err)
		}
		if err := RemoveGraph(fig, 1); err != nil {
			t.Fatal(err)
		}
		a, b := fig.Graphs[0].View, fig.Graphs[1].View
		ok := layout == "stacked" && a.XMin == b.XMin && b.YMax < a.YMin && a.YMax > 0.9 && b.YMin < 0.2 ||
			layout == "side-by-side" && a.YMax == b.YMax && a.XMax < b.XMin && a.XMin < 0.2 && b.XMax > 1.1
		if !ok {
			t.Errorf("%s, a graph removed: %+v %+v - not %s over the whole page", layout, a, b, layout)
		}
	}
}

func TestGraphObjectsFollowAddedAndRemovedGraph(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	fig.AddShape(&agr.Shape{Kind: "box", On: true, LocType: "view", Graph: -1,
		X1: 0.1, Y1: 0.1, X2: 0.2, Y2: 0.2, LineStyle: 1, LineWidth: 1, Color: 1})
	src := load(t, "test7-10.agr")
	src.AddShape(&agr.Shape{Kind: "line", On: true, LocType: "world", Graph: 0,
		X1: 1, Y1: 2, X2: 3, Y2: 4, LineStyle: 1, LineWidth: 1, Color: 2, Arrow: 2})
	id, err := AddGraph(fig, src, "b/fit-curves-10.agr")
	if err != nil || len(fig.Shapes) != 2 || fig.Shapes[1].Graph != id {
		t.Fatalf("added graph %d: %v; objects %+v", id, err, fig.Shapes)
	}
	var b bytes.Buffer
	if err := agr.Write(&b, fig); err != nil {
		t.Fatal(err)
	}
	q, err := agr.Parse(strings.NewReader(b.String()))
	if err != nil || len(q.Shapes) != 2 || q.Shapes[1].Graph != id {
		t.Fatalf("reopened graph object: %v; %+v", err, q.Shapes)
	}
	if err := RemoveGraph(fig, id); err != nil {
		t.Fatal(err)
	}
	if len(fig.Shapes) != 1 || fig.Shapes[0].Kind != "box" || len(fig.Objects) != 1 {
		t.Fatalf("removed graph retained its object: %+v", fig.Shapes)
	}
}

func TestDistinctColors(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	seen := map[int]bool{}
	for _, s := range fig.Graphs[0].Sets {
		seen[s.Line.Color], seen[s.Symbol.Color] = true, true
	}
	src := load(t, "test3-1.agr")
	var sets []int
	for _, s := range src.Graphs[0].Sets {
		sets = append(sets, s.ID)
	}
	// three more copies of the same plot: each its own color, never one
	// already on the graph
	for k := range 3 {
		added, err := AddSets(fig, 0, src, "b/fit-curves-1.agr", sets)
		if err != nil {
			t.Fatal(err)
		}
		c, err := Distinct(fig, 0, added)
		if err != nil || c < 0 {
			t.Fatalf("copy %d: color %d, %v", k, c, err)
		}
		if seen[c] {
			t.Errorf("copy %d: color %d is already used", k, c)
		}
		seen[c] = true
		for _, s := range fig.Graphs[0].Sets[len(fig.Graphs[0].Sets)-len(added):] {
			if s.Line.Color != c || s.Symbol.Color != c || s.ErrorBar.Color != c {
				t.Errorf("copy %d set %d not all in color %d: %+v %+v", k, s.ID, c, s.Line, s.Symbol)
			}
		}
		if _, ok := fig.Colors[c]; !ok {
			t.Errorf("color %d not in the figure's map", c)
		}
	}
	if _, err := Distinct(fig, 7, nil); err == nil {
		t.Error("graph 7 accepted")
	}
}

func TestExtend(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr") // linear, 1e-5..0.11 x 0..0.11
	var all []int
	for _, s := range fig.Graphs[0].Sets {
		all = append(all, s.ID)
	}
	before := fig.Graphs[0].World
	if changed, _ := Extend(fig, 0, all); changed || fig.Graphs[0].World != before {
		t.Errorf("its own data changed the range: %+v -> %+v", before, fig.Graphs[0].World)
	}
	g := fig.Graphs[0]
	ticks := (g.World.YMax - g.World.YMin) / g.Y.Tick.Major
	g.Sets = append(g.Sets, &agr.Set{ID: 99, Type: "xydy", ErrorBar: agr.ErrorBar{On: true},
		Data: [][]float64{{0.05, 0.3, 0.05}, {-0.02, -0.1, 0}, {math.NaN(), 5, 0}}})
	if changed, err := Extend(fig, 0, []int{99}); !changed || err != nil {
		t.Fatalf("no change: %v", err)
	}
	w := g.World
	if !(w.XMin <= -0.02 && w.XMax >= 0.11 && w.YMin <= -0.1 && w.YMax >= 0.35 && w.YMax < 5) {
		t.Errorf("world %+v does not hold the data (and only it)", w)
	}
	if n := (w.YMax - w.YMin) / g.Y.Tick.Major; n < ticks/2.5 || n > ticks*2.5 {
		t.Errorf("y ticks %g -> %g", ticks, n)
	}
	for _, v := range []float64{w.YMin, w.YMax} {
		if r := v / g.Y.Tick.Major; math.Abs(r-math.Round(r)) > 1e-9 {
			t.Errorf("y end %g not on a tick of %g", v, g.Y.Tick.Major)
		}
	}

	// logarithmic: decades, non-positive x left out
	lf := FromPlot(load(t, "test7-10.agr"), "a/fit-curves-10.agr") // x 5e-4..10 log
	lg := lf.Graphs[0]
	lg.Sets = append(lg.Sets, &agr.Set{ID: 99, Type: "xy", Data: [][]float64{{2e-6, 0}, {300, 0}, {-1, 0}, {0, 0}, {100, 0}}})
	Extend(lf, 0, []int{99})
	if lg.World.XMin != 1e-6 || lg.World.XMax != 1000 {
		t.Errorf("log x = %g..%g, want 1e-06..1000", lg.World.XMin, lg.World.XMax)
	}
	// data ending on a decade ends the axis there
	lg.Sets = append(lg.Sets, &agr.Set{ID: 100, Type: "xy", Data: [][]float64{{1e4, 0}}})
	Extend(lf, 0, []int{100})
	if lg.World.XMax != 1e4 {
		t.Errorf("log x max = %g, want 10000", lg.World.XMax)
	}
	// x errors, each side its own: the right one widens the log axis, the
	// left one reaching below 0 is drawn to the frame and leaves it be
	lg.Sets = append(lg.Sets, &agr.Set{ID: 101, Type: "xydxdx", ErrorBar: agr.ErrorBar{On: true},
		Data: [][]float64{{1e4, 0, 9e4, 2e4}}})
	Extend(lf, 0, []int{101})
	if lg.World.XMin != 1e-6 || lg.World.XMax != 1e5 {
		t.Errorf("log x with errors = %g..%g, want 1e-06..1e+05", lg.World.XMin, lg.World.XMax)
	}
}

func TestUseColor(t *testing.T) {
	fig := &agr.Project{}
	if err := UseColor(fig, 11); err != nil || fig.Colors[11].Name != "orange" {
		t.Errorf("orange: %v %+v", err, fig.Colors)
	}
	fig.Colors[20] = agr.Color{R: 1, Name: "mine"}
	if err := UseColor(fig, 20); err != nil {
		t.Errorf("the figure's own color 20: %v", err)
	}
	if err := UseColor(fig, 21); err == nil {
		t.Error("color 21 accepted")
	}
	if len(DefaultColors) != 16 {
		t.Errorf("%d default colors", len(DefaultColors))
	}
}

// TestUpdateFollowsTheSourceType: a source whose set changed type (an xy
// fit re-run with x errors) brings its type with its data, so the new
// columns are read as what they are.
func TestUpdateFollowsTheSourceType(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	s := fig.Graphs[0].Sets[0]
	s.Type, s.Data = "xy", [][]float64{{1, 2}}
	s.Line.Color = 5
	newer := load(t, "test3-1.agr")
	newer.Graphs[0].Sets[0].Type = "xydx"
	newer.Graphs[0].Sets[0].Data = [][]float64{{1, 2, 0.1}, {2, 3, 0.2}}
	_, warnings := Update(fig, func(string) (*agr.Project, error) { return newer, nil })
	if s.Type != "xydx" || len(s.Data) != 2 || len(s.Data[1]) != 3 || s.Line.Color != 5 {
		t.Errorf("after Update: type %s, data %v, color %d", s.Type, s.Data, s.Line.Color)
	}
	if !strings.Contains(strings.Join(warnings, "\n"), "xy to xydx") {
		t.Errorf("no note of the type change: %v", warnings)
	}
}

// TestAddKeepsTheSourceColors: a plot's color 2 that is not the figure's
// color 2 comes in as its own color, under a free number - the figure's
// sets keep theirs; a color both map alike keeps its number. Fonts alike.
func TestAddKeepsTheSourceColors(t *testing.T) {
	red, green := agr.Color{R: 255, Name: "red"}, agr.Color{G: 255, Name: "green"}
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	fig.Colors = map[int]agr.Color{0: {R: 255, G: 255, B: 255, Name: "white"}, 1: {}, 2: red}
	fig.Fonts = map[int]agr.Font{0: {Name: "Times-Roman"}, 4: {Name: "Helvetica"}}
	mine := fig.Graphs[0].Sets[0]
	mine.Line.Color = 2
	src := load(t, "test3-1.agr")
	src.Colors = map[int]agr.Color{0: {R: 255, G: 255, B: 255, Name: "white"}, 1: {}, 2: green}
	src.Fonts = map[int]agr.Font{0: {Name: "Times-Roman"}, 4: {Name: "Courier"}}
	in := src.Graphs[0].Sets[0]
	in.Line.Color, in.Symbol.Color, in.ErrorBar.Color, in.Symbol.FillColor, in.Fill.Color = 2, 2, 1, 2, 0
	in.Symbol.CharFont = 4
	in.AValue.Font, in.AValue.Color = 4, 2
	added, err := AddSets(fig, 0, src, "b/fit-curves-1.agr", []int{0})
	if err != nil {
		t.Fatal(err)
	}
	var got *agr.Set
	for _, s := range fig.Graphs[0].Sets {
		if s.ID == added[0] {
			got = s
		}
	}
	if c := got.Line.Color; c == 2 || fig.Colors[c] != green || got.Symbol.Color != c || got.Symbol.FillColor != c {
		t.Errorf("imported green is color %d (%+v), symbol %d, fill %d", c, fig.Colors[c], got.Symbol.Color, got.Symbol.FillColor)
	}
	if got.AValue.Font != got.Symbol.CharFont || got.AValue.Color != got.Line.Color {
		t.Errorf("annotation font/color not imported: %+v", got.AValue)
	}
	if got.ErrorBar.Color != 1 || got.Fill.Color != 0 {
		t.Errorf("colors both map alike were renumbered: error bars %d, fill %d", got.ErrorBar.Color, got.Fill.Color)
	}
	if fig.Colors[2] != red || mine.Line.Color != 2 {
		t.Errorf("the figure's own red changed: %+v, its set uses %d", fig.Colors[2], mine.Line.Color)
	}
	if f := got.Symbol.CharFont; f == 4 || fig.Fonts[f].Name != "Courier" || fig.Fonts[4].Name != "Helvetica" {
		t.Errorf("imported Courier is font %d (%+v); the figure's 4 is %+v", f, fig.Fonts[f], fig.Fonts[4])
	}

	// a whole graph: its axes, frame and texts follow its colors too
	src.Graphs[0].Frame.Color = 2
	src.Strings = append(src.Strings, &agr.Text{LocType: "world", Graph: 0, Color: 2, Font: 4, Text: "label"})
	id, err := AddGraph(fig, src, "b/fit-curves-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	g := graph(fig, id)
	if fig.Colors[g.Frame.Color] != green {
		t.Errorf("the added graph's frame is color %d (%+v), not green", g.Frame.Color, fig.Colors[g.Frame.Color])
	}
	last := fig.Strings[len(fig.Strings)-1]
	if fig.Colors[last.Color] != green || fig.Fonts[last.Font].Name != "Courier" {
		t.Errorf("the added text: color %d %+v, font %d %+v", last.Color, fig.Colors[last.Color], last.Font, fig.Fonts[last.Font])
	}
}

func TestPointLabelsCopyAndRefresh(t *testing.T) {
	src := load(t, "style-avalue-4.agr")
	fig := FromPlot(load(t, "style-avalue-4.agr"), "source.agr")
	fig.Graphs[0].Sets = nil
	if _, err := AddSets(fig, 0, src, "source.agr", []int{0}); err != nil {
		t.Fatal(err)
	}
	s := fig.Graphs[0].Sets[0]
	if len(s.Labels) != 2 || s.AValue.Type != 4 {
		t.Fatalf("labels not imported: %+v", s)
	}
	s.Labels[0] = "edited"
	if src.Graphs[0].Sets[0].Labels[0] == "edited" {
		t.Fatal("labels share source storage")
	}
	s.AValue.Color = 2
	src.Graphs[0].Sets[0].Labels = []string{"new", "labels"}
	n, w := Update(fig, func(string) (*agr.Project, error) { return src, nil })
	if n != 1 || len(w) != 0 || s.Labels[0] != "new" || s.AValue.Color != 2 {
		t.Fatalf("refresh %d %v: %+v", n, w, s)
	}
	s.Labels[0] = "again"
	if src.Graphs[0].Sets[0].Labels[0] != "new" {
		t.Fatal("refreshed labels share storage")
	}
}
