package render

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

// Tolerances, in viewport units. gracebat writes 4 decimals and rounds its
// text boxes; plot-go's font metrics are Adobe's, not Grace's URW fonts
// (docs/grace-behaviour.md).
// On the corpus the largest differences are 0.0012 for texts and 0.0001
// (the EPS's rounding) for points.
const (
	tolPoint = 0.0002
	tolText  = 0.002
	tolSize  = 0.0002
)

// compare lists how got differs from gracebat's want, at most limit lines.
func compare(got, want *draw.Drawing, limit int) []string {
	var out []string
	add := func(format string, a ...any) {
		if len(out) < limit {
			out = append(out, fmt.Sprintf(format, a...))
		} else if len(out) == limit {
			out = append(out, "...")
		}
	}
	near := func(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

	if len(got.Texts) != len(want.Texts) {
		add("%d texts, gracebat %d", len(got.Texts), len(want.Texts))
	}
	for i := 0; i < min(len(got.Texts), len(want.Texts)); i++ {
		g, w := got.Texts[i], want.Texts[i]
		if g.Str != w.Str || g.Font != w.Font {
			add("text %d: %q %s, gracebat %q %s", i, g.Str, g.Font, w.Str, w.Font)
			continue
		}
		for k := range g.Matrix {
			if !near(g.Matrix[k], w.Matrix[k], tolSize) {
				add("text %d %q: matrix %.4f, gracebat %.4f", i, g.Str, g.Matrix, w.Matrix)
				break
			}
		}
		if !near(g.At.X, w.At.X, tolText) || !near(g.At.Y, w.At.Y, tolText) {
			add("text %d %q: at %.4f,%.4f, gracebat %.4f,%.4f (off %.4f,%.4f)", i, g.Str, g.At.X, g.At.Y, w.At.X, w.At.Y, g.At.X-w.At.X, g.At.Y-w.At.Y)
		}
		if !colorNear(g.Color, w.Color) {
			add("text %d %q: color %v, gracebat %v", i, g.Str, g.Color, w.Color)
		}
	}

	if len(got.Paths) != len(want.Paths) {
		add("%d paths, gracebat %d", len(got.Paths), len(want.Paths))
	}
	for i := 0; i < min(len(got.Paths), len(want.Paths)); i++ {
		g, w := got.Paths[i], want.Paths[i]
		where := fmt.Sprintf("path %d (%s)", i, describe(w))
		if g.Fill != w.Fill {
			add("%s: fill %v, gracebat %v", where, g.Fill, w.Fill)
		}
		if !g.Fill && !near(g.Style.Width, w.Style.Width, 1e-4) {
			add("%s: width %.4f, gracebat %.4f", where, g.Style.Width, w.Style.Width)
		}
		if len(g.Style.Dash) != len(w.Style.Dash) {
			add("%s: dash %v, gracebat %v", where, g.Style.Dash, w.Style.Dash)
		} else {
			for k := range g.Style.Dash {
				if !near(g.Style.Dash[k], w.Style.Dash[k], 1e-4) {
					add("%s: dash %v, gracebat %v", where, g.Style.Dash, w.Style.Dash)
					break
				}
			}
		}
		if !colorNear(g.Style.Color, w.Style.Color) {
			add("%s: color %v, gracebat %v", where, g.Style.Color, w.Style.Color)
		}
		if len(g.Segments) != len(w.Segments) {
			add("%s: %d segments, gracebat %d", where, len(g.Segments), len(w.Segments))
			continue
		}
		for k := range g.Segments {
			gs, ws := g.Segments[k], w.Segments[k]
			if (gs.Arc == nil) != (ws.Arc == nil) {
				add("%s segment %d: arc vs line", where, k)
				continue
			}
			if gs.Arc != nil {
				a, b := gs.Arc, ws.Arc
				if !near(a.X, b.X, tolPoint) || !near(a.Y, b.Y, tolPoint) || !near(a.RX, b.RX, tolPoint) {
					add("%s: arc %.4f,%.4f r %.4f, gracebat %.4f,%.4f r %.4f", where, a.X, a.Y, a.RX, b.X, b.Y, b.RX)
				}
				continue
			}
			if len(gs.Points) != len(ws.Points) {
				add("%s segment %d: %d points, gracebat %d", where, k, len(gs.Points), len(ws.Points))
				continue
			}
			for j := range gs.Points {
				if !near(gs.Points[j].X, ws.Points[j].X, tolPoint) || !near(gs.Points[j].Y, ws.Points[j].Y, tolPoint) {
					add("%s point %d: %.4f,%.4f, gracebat %.4f,%.4f", where, j, gs.Points[j].X, gs.Points[j].Y, ws.Points[j].X, ws.Points[j].Y)
					break
				}
			}
		}
	}
	if len(out) == 0 && string(got.Order) != string(want.Order) {
		add("drawing order differs")
	}
	return out
}

func colorNear(a, b [3]float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > 0.002 {
			return false
		}
	}
	return true
}

func describe(p draw.Path) string {
	if len(p.Segments) == 0 {
		return "empty"
	}
	s := p.Segments[0]
	if s.Arc != nil {
		return fmt.Sprintf("arc at %.4f,%.4f", s.Arc.X, s.Arc.Y)
	}
	return fmt.Sprintf("from %.4f,%.4f, %d points", s.Points[0].X, s.Points[0].Y, len(s.Points))
}

// TestMatchesGracebat renders every corpus file and compares it with
// gracebat's EPS (testdata/ref, made by scripts/make-references.sh).
func TestMatchesGracebat(t *testing.T) {
	files, _ := filepath.Glob("../testdata/corpus/*.agr")
	if len(files) == 0 {
		t.Fatal("no corpus files")
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".agr")
		t.Run(name, func(t *testing.T) {
			p, err := agr.ParseFile(f)
			if err != nil {
				t.Fatal(err)
			}
			got, warnings := Render(p)
			for _, w := range warnings {
				t.Logf("warning: %s", w)
			}
			r, err := os.Open(filepath.Join("..", "testdata", "ref", name+".eps"))
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			want, err := eps.Parse(r)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range compare(got, want, 25) {
				t.Error(d)
			}
		})
	}
}

// TestRenderWholeCorpus draws every .agr under the directories in
// $PLOT_GO_CORPUS (the full set of OneFit outputs, not in the repository)
// and reports the warnings, grouped.
func TestRenderWholeCorpus(t *testing.T) {
	dirs := os.Getenv("PLOT_GO_CORPUS")
	if dirs == "" {
		t.Skip("PLOT_GO_CORPUS not set")
	}
	n := 0
	seen := map[string]int{}
	for _, dir := range filepath.SplitList(dirs) {
		files, _ := filepath.Glob(filepath.Join(dir, "*", "*.agr"))
		more, _ := filepath.Glob(filepath.Join(dir, "*.agr"))
		for _, f := range append(files, more...) {
			p, err := agr.ParseFile(f)
			if err != nil {
				t.Fatal(err)
			}
			d, warnings := Render(p)
			if len(d.Paths) == 0 {
				t.Errorf("%s: nothing drawn", f)
			}
			for _, w := range warnings {
				seen[w]++
			}
			n++
		}
	}
	for w, c := range seen {
		t.Logf("%4d x %s", c, w)
	}
	t.Logf("%d files", n)
}
