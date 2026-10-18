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

func TestXYRFixedGraphMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyr", "0", "1", "1", "0", "2 5 1\n5 6 2\n8 4 3")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	p.Graphs[0].Type = "Fixed"
	s := p.Graphs[0].Sets[0]
	s.Symbol.Color, s.Symbol.FillColor = 2, 3
	s.Symbol.LineWidth = 1.5
	s.Line.Pattern = 4
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	input = encoded.String()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "xyr.agr"), filepath.Join(dir, "grace.eps")
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
		t.Fatalf("plot-go differs from gracebat for xyr: %s", strings.Join(differences, "\n"))
	}
}

func TestXYRColumnsAndWrongGraphType(t *testing.T) {
	if got := agr.Columns["xyr"]; got != 3 {
		t.Fatalf("xyr has %d columns; want 3", got)
	}
	input := barProject(t, "xyr", "0", "1", "1", "0", "5 6 2")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "@type xyr\n") || !strings.Contains(encoded.String(), "5 6 2") {
		t.Fatalf("xyr data did not round-trip:\n%s", encoded.String())
	}
	_, warnings := Render(p)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "only drawn in Fixed graphs") {
		t.Fatalf("XY graph with xyr warnings = %v; want an applicability warning", warnings)
	}
}
