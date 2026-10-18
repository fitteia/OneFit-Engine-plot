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

func TestXYHILOMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyhilo", "0", "0", "0", "0", "2 8 2 4 7\n5 9 1 6 3\n8 7 3 4 6")
	// Grace applies symbol styling and symbol size to all three OHLC strokes.
	styled, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := styled.Graphs[0].Sets[0]
	s.Symbol.Size, s.Symbol.Color, s.Symbol.LineWidth, s.Symbol.LineStyle = 2, 2, 2, 3
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, styled); err != nil {
		t.Fatal(err)
	}
	input = encoded.String()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "hilo.agr"), filepath.Join(dir, "grace.eps")
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
		t.Fatalf("plot-go differs from gracebat:\n%s", strings.Join(differences, "\n"))
	}
}

func TestXYHILOGeometryAndSkip(t *testing.T) {
	input := barProject(t, "xyhilo", "0", "0", "0", "1", "2 8 2 4 7\n5 9 1 6 3\n8 7 3 4 6")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	p.Graphs[0].Sets[0].Symbol.Size = 2
	got, warnings := Render(p)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	var strokes int
	for _, path := range got.Paths {
		if path.ID == "g0.s0.hilo" {
			strokes += len(path.Segments)
		}
	}
	if strokes != 6 {
		t.Fatalf("got %d candle strokes with skip 1; want 6 strokes for two rows", strokes)
	}
}

func TestXYHILOColumns(t *testing.T) {
	if got := agr.Columns["xyhilo"]; got != 5 {
		t.Fatalf("xyhilo has %d columns; want 5", got)
	}
	p, err := agr.Parse(strings.NewReader(barProject(t, "xyhilo", "0", "0", "0", "0", "5 8 2 4 7")))
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "@type xyhilo\n") || !strings.Contains(encoded.String(), "5 8 2 4 7") {
		t.Fatalf("xyhilo data did not round-trip:\n%s", encoded.String())
	}
}
