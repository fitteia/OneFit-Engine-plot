package render

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

// twoPanels is test3 drawn twice on one page, side by side - g0 on the
// left, a copy as g1 on the right - with a curve label on each panel.
func twoPanels(t *testing.T) *agr.Project {
	t.Helper()
	p, err := agr.ParseFile("../testdata/corpus/test3-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	g0 := p.Graphs[0]
	g1 := *g0
	g1.ID = 1
	g1.Sets = nil
	for _, s := range g0.Sets {
		c := *s
		g1.Sets = append(g1.Sets, &c)
	}
	g0.View = agr.Rect{XMin: 0.12, YMin: 0.15, XMax: 0.6, YMax: 0.85}
	g1.View = agr.Rect{XMin: 0.75, YMin: 0.15, XMax: 1.23, YMax: 0.85}
	p.Graphs = append(p.Graphs, &g1)
	for _, s := range p.Strings {
		if s.LocType == "world" {
			c := *s
			c.Graph = 1
			c.Text = "(b)"
			p.Strings = append(p.Strings, &c)
			break
		}
	}
	return p
}

func TestMultiGraphMatchesGracebat(t *testing.T) {
	if _, err := exec.LookPath("gracebat"); err != nil {
		t.Skip("gracebat not installed")
	}
	p := twoPanels(t)
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "two.agr"), filepath.Join(dir, "two.eps")
	os.WriteFile(in, b.Bytes(), 0o644)
	if o, err := exec.Command("gracebat", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
		t.Fatalf("gracebat: %v %s", err, o)
	}
	r, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	want, err := eps.Parse(r)
	if err != nil {
		t.Fatal(err)
	}
	got, warnings := Render(p)
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}
	for _, d := range drawcmp.Compare(got, want, 30) {
		t.Error(d)
	}
}
