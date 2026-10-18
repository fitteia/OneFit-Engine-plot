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

func TestXYZAnnotationsMatchGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyz", "1", "1", "0", "0", "2 5 1\n5 6 3\n8 4 7")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	s.AValue.On = true
	s.AValue.Type = 5 // third numeric column (Z)
	s.AValue.Prec = 1
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	input = encoded.String()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "xyz.agr"), filepath.Join(dir, "grace.eps")
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
		t.Fatalf("plot-go differs from gracebat for xyz with Z labels:\n%s", strings.Join(differences, "\n"))
	}
}

func TestXYZColumnsAndRoundTrip(t *testing.T) {
	if got := agr.Columns["xyz"]; got != 3 {
		t.Fatalf("xyz has %d columns; want 3", got)
	}
	input := barProject(t, "xyz", "0", "0", "0", "0", "5 6 9.25")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "@type xyz\n") || !strings.Contains(encoded.String(), "5 6 9.25") {
		t.Fatalf("xyz data did not round-trip:\n%s", encoded.String())
	}
}
