package render

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

func TestXYBoxplotMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyboxplot", "0", "0", "1", "0", "2 5 8 2 9 1\n5 6 9 3 10 1\n8 4 7 1 11 -1")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	// Distinct box/whisker settings make the comparison cover the separate
	// Grace symbol and error-bar style controls, including dash patterns.
	s.Symbol.Size = 1.4
	s.Symbol.Color, s.Symbol.FillColor = 2, 3
	s.Symbol.FillPattern = 1
	s.Symbol.LineWidth, s.Symbol.LineStyle, s.Symbol.Pattern = 2, 3, 1
	s.ErrorBar.Color = 4
	s.ErrorBar.Pattern = 1
	s.ErrorBar.RiserWidth, s.ErrorBar.RiserStyle = 3, 2
	s.ErrorBar.LineWidth, s.ErrorBar.LineStyle = 2, 4
	s.Legend = "box plot"
	p.Graphs[0].Legend.On = true
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	input = encoded.String()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "boxplot.agr"), filepath.Join(dir, "grace.eps")
	if err := os.WriteFile(in, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	if log, err := exec.Command(grace, "-nosafe", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
		t.Fatalf("gracebat: %v: %s", err, log)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := eps.Parse(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	p, err = agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got, warnings := Render(p)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if differences := drawcmp.Compare(got, want, 20); len(differences) > 0 {
		t.Fatalf("plot-go differs from gracebat:\n%s", strings.Join(differences, "\n"))
	}
}

func TestXYBoxplotSkipAndErrorBarsOff(t *testing.T) {
	input := barProject(t, "xyboxplot", "0", "0", "1", "1", "2 5 8 2 9 1\n5 6 9 3 10 1\n8 4 7 1 8 0.5")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	s.ErrorBar.On = false
	d, warnings := Render(p)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	counts := map[string]int{}
	for _, path := range d.Paths {
		if strings.HasPrefix(path.ID, "g0.s0.boxplot") {
			counts[path.ID] += len(path.Segments)
		}
	}
	if counts["g0.s0.boxplot.fill"] != 2 || counts["g0.s0.boxplot"] == 0 || counts["g0.s0.boxplot.median"] != 2 {
		t.Fatalf("skip 1 did not draw two filled boxes and medians: %v", counts)
	}
	if counts["g0.s0.boxplot.whisker"] != 0 || counts["g0.s0.boxplot.cap"] != 0 {
		t.Fatalf("errorbar off still drew whiskers: %v", counts)
	}
}

func TestXYBoxplotClipsBoxAtFrameLikeGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyboxplot", "0", "0", "1", "0", "0 5 8 2 11 -1")
	dir := t.TempDir()
	in, out := filepath.Join(dir, "boxplot-edge.agr"), filepath.Join(dir, "grace.eps")
	if err := os.WriteFile(in, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	if log, err := exec.Command(grace, "-nosafe", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
		t.Fatalf("gracebat: %v: %s", err, log)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	want, err := eps.Parse(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got, warnings := Render(p)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if differences := drawcmp.Compare(got, want, 20); len(differences) > 0 {
		t.Fatalf("plot-go differs from gracebat at the graph frame:\n%s", strings.Join(differences, "\n"))
	}
}

func TestXYBoxplotColumnsAndRoundTrip(t *testing.T) {
	if got := agr.Columns["xyboxplot"]; got != 6 {
		t.Fatalf("xyboxplot has %d columns; want 6", got)
	}
	input := barProject(t, "xyboxplot", "0", "0", "1", "0", "5 6 9 3 10 1")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "@type xyboxplot\n") || !strings.Contains(encoded.String(), "5 6 9 3 10 1") {
		t.Fatalf("xyboxplot data did not round-trip:\n%s", encoded.String())
	}
}
