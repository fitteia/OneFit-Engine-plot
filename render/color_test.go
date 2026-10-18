package render

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

func TestXYColorMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xycolor", "0", "1", "1", "0", "2 4 2.9\n5 6 4.1")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "color.agr"), filepath.Join(dir, "grace.eps")
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
	got, warnings := Render(p)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if differences := drawcmp.Compare(got, want, 20); len(differences) > 0 {
		t.Fatalf("plot-go differs from gracebat:\n%s", strings.Join(differences, "\n"))
	}
}

func TestXYColorColumnsAndRoundTrip(t *testing.T) {
	if got := agr.Columns["xycolor"]; got != 3 {
		t.Fatalf("xycolor has %d columns; want 3", got)
	}
	input := barProject(t, "xycolor", "0", "1", "1", "0", "5 8 2")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var encoded strings.Builder
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "@type xycolor\n") || !strings.Contains(encoded.String(), "5 8 2") {
		t.Fatalf("xycolor data did not round-trip:\n%s", encoded.String())
	}
}
