package agr

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parse(t *testing.T, name string) *Project {
	t.Helper()
	p, err := ParseFile(filepath.Join("..", "testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseTest7(t *testing.T) {
	p := parse(t, "test7-10.agr")
	if len(p.Warnings) != 0 {
		t.Fatalf("warnings: %+v", p.Warnings)
	}
	if p.Version != 50125 || p.PageWidth != 773 || p.PageHeight != 600 {
		t.Errorf("version %d, page %gx%g", p.Version, p.PageWidth, p.PageHeight)
	}
	if f := p.Fonts[8]; f.Name != "Symbol" {
		t.Errorf("font 8 = %+v, want Symbol", f)
	}
	if c := p.Colors[4]; c != (Color{0, 0, 255, "blue"}) {
		t.Errorf("color 4 = %+v", c)
	}
	if len(p.Strings) != 4 {
		t.Fatalf("%d strings, want 4", len(p.Strings))
	}
	if s := p.Strings[0]; !s.On || s.LocType != "view" || s.X != 0.05 || s.Color != 4 || s.Font != 4 || s.Text != "[300kHz_1]" {
		t.Errorf("string 0 = %+v", s)
	}
	if s := p.Strings[1]; s.LocType != "world" || s.Graph != 0 || s.X != 0.00369713 || s.CharSize != 1.5 || s.Text != `\-\-M\sinf\N` {
		t.Errorf("string 1 = %+v", s)
	}
	if len(p.Graphs) != 1 {
		t.Fatalf("%d graphs, want 1", len(p.Graphs))
	}
	g := p.Graphs[0]
	if !g.On || g.Type != "XY" {
		t.Errorf("graph on %v type %q", g.On, g.Type)
	}
	if g.World != (Rect{0.0005, -2, 10, 3}) {
		t.Errorf("world = %+v", g.World)
	}
	if g.View != (Rect{0.183247, 0.149994, 1.038397, 0.849964}) {
		t.Errorf("view = %+v", g.View)
	}
	if g.X.Scale != "Logarithmic" || g.Y.Scale != "Normal" {
		t.Errorf("scales %q %q", g.X.Scale, g.Y.Scale)
	}
	if g.X.Label.Text != `\xt\4(s) ` || g.X.Tick.Major != 10 || g.X.Tick.MinorTicks != 9 || g.X.Tick.Direction != "in" {
		t.Errorf("x axis = %+v", g.X)
	}
	if g.X.TickLabel.Format != "power" || g.Y.TickLabel.Format != "decimal" || g.Y.TickLabel.Prec != 1 {
		t.Errorf("tick labels %+v / %+v", g.X.TickLabel, g.Y.TickLabel)
	}
	if g.Y.Tick.Major != 0.5 || g.Y.Tick.MajorMarks.Size != 1.5 || g.Y.Tick.MinorMarks.Size != 1 {
		t.Errorf("y ticks = %+v", g.Y.Tick)
	}
	if !g.Legend.On || g.Legend.X != 0.977315227809 || g.Legend.BoxPattern != 0 {
		t.Errorf("legend = %+v", g.Legend)
	}
	if g.Frame.LineWidth != 2 || g.Frame.Color != 1 {
		t.Errorf("frame = %+v", g.Frame)
	}
	if len(g.Sets) != 5 {
		t.Fatalf("%d sets, want 5", len(g.Sets))
	}
	data := g.Sets[0]
	if data.Type != "xydy" || data.Symbol.Type != 1 || data.Line.Type != 0 || data.ErrorBar.On {
		t.Errorf("set 0 = %+v", data)
	}
	if len(data.Data) != 20 || data.Data[0][0] != 0.005 || data.Data[0][1] != 1.85768 || data.Data[0][2] != 1 {
		t.Errorf("set 0 data: %d rows, first %v", len(data.Data), data.Data[0])
	}
	curve := g.Sets[1]
	if curve.Type != "xy" || curve.Symbol.Type != 0 || curve.Line.Type != 1 || curve.Line.LineWidth != 2 || len(curve.Data) != 100 {
		t.Errorf("set 1 = type %s symbol %d line %+v, %d rows", curve.Type, curve.Symbol.Type, curve.Line, len(curve.Data))
	}
}

// TestParseCorpus: every reference file reads with one graph, sets with
// data, and no warning except the known malformed tick label format.
func TestParseCorpus(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "testdata", "corpus", "*.agr"))
	if len(files) == 0 {
		t.Fatal("no corpus files")
	}
	for _, f := range files {
		p, err := ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(f)
		for _, w := range p.Warnings {
			if name == "test1-13.agr" && strings.Contains(w.Msg, `tick label format "unknown"`) {
				continue
			}
			t.Errorf("%s:%d: %s (%s)", name, w.Line, w.Msg, w.Text)
		}
		if len(p.Graphs) != 1 || len(p.Graphs[0].Sets) == 0 {
			t.Errorf("%s: %d graphs", name, len(p.Graphs))
			continue
		}
		for _, s := range p.Graphs[0].Sets {
			if len(s.Data) == 0 {
				t.Errorf("%s: set %d has no data", name, s.ID)
			}
		}
	}
}

// TestParseWholeCorpus reads every .agr/.agr-par under the directories in
// $PLOT_GO_CORPUS (separated by the path list separator) - the full set of
// OneFit outputs, which is not in the repository. Only the few files with
// values Grace itself does not know (a tick label format, a precision
// without a value) may warn.
func TestParseWholeCorpus(t *testing.T) {
	dirs := os.Getenv("PLOT_GO_CORPUS")
	if dirs == "" {
		t.Skip("PLOT_GO_CORPUS not set")
	}
	n, warned := 0, 0
	for _, dir := range filepath.SplitList(dirs) {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !(strings.HasSuffix(path, ".agr") || strings.HasSuffix(path, ".agr-par")) {
				return err
			}
			p, err := ParseFile(path)
			if err != nil {
				return err
			}
			n++
			for _, w := range p.Warnings {
				known := strings.Contains(w.Msg, "unknown tick label format") ||
					strings.Contains(w.Msg, "precision has no value")
				if !known {
					t.Errorf("%s:%d: %s (%s)", path, w.Line, w.Msg, w.Text)
				} else {
					warned++
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("%d files, %d known warnings", n, warned)
}

func TestTokenize(t *testing.T) {
	toks, err := tokenize(`map color 1 to (0, 0, 0), "black"`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tk := range toks {
		switch {
		case tk.isStr:
			got = append(got, "S:"+tk.word)
		case tk.isNum:
			got = append(got, "N:"+tk.word)
		default:
			got = append(got, tk.word)
		}
	}
	if strings.Join(got, " ") != "map color N:1 to N:0 N:0 N:0 S:black" {
		t.Errorf("tokens = %v", got)
	}
	if _, err := tokenize(`string def "open`); err == nil {
		t.Error("unterminated string accepted")
	}
}

func TestWarnings(t *testing.T) {
	p, err := Parse(strings.NewReader(`@version 50125
@with g0
@    s0 avalue on
@    xaxes scale Reciprocal
@    frobnicate 3
@target G0.S0
@type xy
1 2
3
&
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"annotated values is not drawn", "axis scale Reciprocal", "unknown directive", "row has 1 numbers"}
	if len(p.Warnings) != len(want) {
		t.Fatalf("warnings = %+v", p.Warnings)
	}
	for i, w := range want {
		if !strings.Contains(p.Warnings[i].Msg, w) {
			t.Errorf("warning %d = %q, want it to mention %q", i, p.Warnings[i].Msg, w)
		}
	}
	if d := p.Graphs[0].Sets[0].Data; len(d) != 1 || d[0][1] != 2 {
		t.Errorf("data = %v", d)
	}
}
