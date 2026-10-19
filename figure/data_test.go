package figure

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

const onefitData = "# DATA dum = 1\r\n# TAG = RL\r\n# XAXIS Normal 0 10 1 1 decimal 0 f\r\n1 10 0.5\r\n2 20 0.6\r\n# DATA dum = 2\r\n# TAG = YL\r\n1 11 0.5\r\n2 21\r\n3 31 0.7\r\n"

func TestReadData(t *testing.T) {
	b := ReadData(onefitData)
	if len(b) != 2 || b[0].Tag != "RL" || b[1].Tag != "YL" || len(b[0].Rows) != 2 || len(b[1].Rows) != 3 {
		t.Fatalf("blocks %+v", b)
	}
	if b[0].Columns() != 3 || !reflect.DeepEqual(b[1].Rows[1], []float64{2, 21}) {
		t.Errorf("rows %+v", b[1].Rows)
	}
	// a plain table: a header line, commas or semicolons, a label after
	// the numbers - one block, rows of numbers
	for _, text := range []string{
		"time,signal,err\n0,1.5,0.1\n1,2.5,0.1\n",
		"time;signal;err\n0;1.5;0.1\n1;2.5;0.1\n",
		"# comment\n0 1.5 0.1 first\n\n1\t2.5\t0.1\n",
	} {
		b := ReadData(text)
		if len(b) != 1 || b[0].Tag != "" || !reflect.DeepEqual(b[0].Rows, [][]float64{{0, 1.5, 0.1}, {1, 2.5, 0.1}}) {
			t.Errorf("%q: %+v", text, b)
		}
	}
}

func TestDataSourceOf(t *testing.T) {
	r := DataRef{Path: "data/run 7.dat", Block: 2, Cols: []int{1, 2, 3}}
	s := &agr.Set{Comment: r.String()}
	got, ok := DataSourceOf(s)
	if !ok || !reflect.DeepEqual(got, r) {
		t.Errorf("%q read as %+v %v", s.Comment, got, ok)
	}
	if _, ok := SourceOf(s); ok {
		t.Error("a data source read as a plot source")
	}
	if _, ok := DataSourceOf(&agr.Set{Comment: Ref{Path: "a.agr", Graph: 0, Set: 1}.String()}); ok {
		t.Error("a plot source read as a data source")
	}
}

func TestAddDataAndUpdate(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	before := len(fig.Graphs[0].Sets)
	// x, y and dy: one xydy set per block, legends their TAGs
	added, notes, err := AddData(fig, 0, "data/r.dat", onefitData, DataSpec{X: 1, Y: []int{2}, Errors: []int{3}})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 2 || added[0] != fig.Graphs[0].Sets[before-1].ID+1 {
		t.Fatalf("added %v", added)
	}
	s := fig.Graphs[0].Sets[before]
	if s.Type != "xydy" || s.Legend != "RL" || s.Line.Type != 0 || s.Symbol.Type != 1 || len(s.Data) != 2 {
		t.Errorf("set %+v", s)
	}
	// block 2's row without an error is left out, and said so
	if n := len(fig.Graphs[0].Sets[before+1].Data); n != 2 || len(notes) != 1 || !strings.Contains(notes[0], "block 2: 1 row") {
		t.Errorf("block 2: %d rows, notes %q", n, notes)
	}
	// one set per y column; legends name the column
	added, _, err = AddData(fig, 0, "data/t.csv", "x,a,b\n1,2,3\n2,4,6\n", DataSpec{X: 1, Y: []int{2, 3}})
	if err != nil || len(added) != 2 {
		t.Fatal(added, err)
	}
	if l := fig.Graphs[0].Sets[len(fig.Graphs[0].Sets)-1].Legend; l != "t.csv c3" {
		t.Errorf("legend %q", l)
	}
	if _, _, err := AddData(fig, 0, "x", "1 2 3 4", DataSpec{X: 1, Y: []int{2, 3}, Errors: []int{4}}); err == nil {
		t.Error("errors with several y columns accepted")
	}
	// the written figure reads back with its data sets
	var out bytes.Buffer
	if err := agr.Write(&out, fig); err != nil {
		t.Fatal(err)
	}
	back, err := agr.Parse(&out)
	if err != nil || len(back.Graphs[0].Sets) != len(fig.Graphs[0].Sets) {
		t.Fatalf("read back: %v", err)
	}
	if ref, ok := DataSourceOf(back.Graphs[0].Sets[before]); !ok || ref.Block != 1 {
		t.Errorf("source after writing: %q", back.Graphs[0].Sets[before].Comment)
	}
	// Update reads the files again; styles stay
	s.Symbol.Size = 2.5
	changed := strings.Replace(onefitData, "2 20 0.6", "2 25 0.6", 1)
	n, warnings := UpdateFiles(fig, func(string) (*agr.Project, error) { return load(t, "test3-1.agr"), nil },
		func(p string) (string, error) {
			if p == "data/r.dat" {
				return changed, nil
			}
			return "x,a,b\n1,2,3\n", nil
		})
	if len(warnings) != 0 || n != before+4 {
		t.Errorf("updated %d, warnings %q", n, warnings)
	}
	if s.Data[1][1] != 25 || s.Symbol.Size != 2.5 {
		t.Errorf("after update: %v, symbol size %v", s.Data, s.Symbol.Size)
	}
}

func TestAddDataGraph(t *testing.T) {
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	id, added, _, err := AddDataGraph(fig, "data/t.dat", "# TAG = run\n100 3 0.1\n300 4 0.2\n", DataSpec{X: 1, Y: []int{2}, Errors: []int{3}})
	if err != nil || len(added) != 1 {
		t.Fatal(id, added, err)
	}
	g := fig.Graphs[len(fig.Graphs)-1]
	if g.ID != id || len(g.Sets) != 1 || g.Title.Text != "" || g.Sets[0].Legend != "run" {
		t.Fatalf("graph %+v", g)
	}
	w := g.World
	lo, hi := min(w.XMin, w.XMax), max(w.XMin, w.XMax)
	if !(lo <= 100 && hi >= 300 && lo < hi && w.YMin <= 2.9 && w.YMax >= 4.2) {
		t.Errorf("window %+v does not hold the data", w)
	}
	// the first graph is as it was
	if len(fig.Graphs[0].Sets) == 0 || fig.Graphs[0].ID == id {
		t.Error("first graph changed")
	}
	if _, _, _, err := AddDataGraph(fig, "x", "no numbers here\n", DataSpec{X: 1, Y: []int{2}}); err == nil || len(fig.Graphs) != 2 {
		t.Errorf("a file without data: err %v, %d graphs", err, len(fig.Graphs))
	}
}

func TestAddDataGraphAxesAndLegend(t *testing.T) {
	fig := &agr.Project{}
	g := &agr.Graph{View: agr.Rect{XMin: 0.15, YMin: 0.15, XMax: 1.15, YMax: 0.85},
		World: agr.Rect{XMin: 1, XMax: 1e4, YMin: 0, YMax: 1}}
	g.X.Scale, g.X.Invert, g.Y.Scale = "Logarithmic", true, "Normal"
	g.X.Tick.Major, g.Y.Tick.Major = 10, 0.2
	g.X.Tick.MinorTicks, g.X.TickLabel.Format, g.X.TickLabel.Prec = 9, "power", 0
	g.Legend.LocType, g.Legend.X, g.Legend.Y = "world", 100, 0.5
	fig.Graphs = []*agr.Graph{g}
	id, added, _, err := AddDataGraph(fig, "t.csv", "0,3\n7,41\n", DataSpec{X: 1, Y: []int{2}})
	if err != nil || len(added) != 1 {
		t.Fatal(id, added, err)
	}
	n := fig.Graphs[1]
	if n.X.Scale != "Normal" || n.X.Invert || n.Y.Scale != "Normal" {
		t.Errorf("axes %s %v %s, want linear", n.X.Scale, n.X.Invert, n.Y.Scale)
	}
	if n.X.Tick.MinorTicks != 1 || n.X.TickLabel.Format != "general" || n.X.TickLabel.Prec != 5 {
		t.Errorf("x ticks %d minor, labels %s %d, want 1 general 5", n.X.Tick.MinorTicks, n.X.TickLabel.Format, n.X.TickLabel.Prec)
	}
	if w := n.World; w.XMin != 0 || w.XMax != 8 || w.YMin != 0 || w.YMax != 50 || n.X.Tick.Major != 2 || n.Y.Tick.Major != 10 {
		t.Errorf("world %+v majors %g %g", w, n.X.Tick.Major, n.Y.Tick.Major)
	}
	if L := n.Legend; L.LocType != "view" || math.Abs(L.X-0.65) > 1e-9 || math.Abs(L.Y-0.5) > 1e-9 {
		t.Errorf("legend %s %g %g, want view 0.65 0.5", L.LocType, L.X, L.Y)
	}
	if g.Legend.LocType != "world" {
		t.Error("the first graph's legend changed")
	}
	// side by side: each legend stays at its place in its own graph
	if err := Layout(fig, "grid"); err != nil {
		t.Fatal(err)
	}
	for _, x := range fig.Graphs[1:] {
		v := x.View
		if fx := (x.Legend.X - v.XMin) / (v.XMax - v.XMin); math.Abs(fx-0.5) > 1e-9 {
			t.Errorf("graph %d legend at %g of its width, want 0.5", x.ID, fx)
		}
	}
}
