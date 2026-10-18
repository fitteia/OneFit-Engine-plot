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

func TestXYVMapMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyvmap", "0", "0", "0", "1", "2 3 4 1\n5 6 2 2")
	for old, new := range map[string]string{
		"@    s0 errorbar color 1":           "@    s0 errorbar color 2",
		"@    s0 errorbar riser linewidth 1": "@    s0 errorbar riser linewidth 2",
		"@    s0 errorbar riser linestyle 1": "@    s0 errorbar riser linestyle 2",
	} {
		if !strings.Contains(input, old) {
			t.Fatalf("fixture lacks %q", old)
		}
		input = strings.Replace(input, old, new, 1)
	}
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "vmap.agr"), filepath.Join(dir, "grace.eps")
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
	_ = f.Close()
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

func TestXYVMapNormalizedOffsetsMatchGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyvmap", "0", "0", "0", "0", "4.2 5.3 0.01 0.02")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "vmap.agr"), filepath.Join(dir, "grace.eps")
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
	_ = f.Close()
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

func TestXYVMapArrowheadSizeMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	input := barProject(t, "xyvmap", "0", "0", "0", "0", "4.2 5.3 0.01 0.02")
	input = strings.Replace(input, "@    s0 errorbar size 1.000000", "@    s0 errorbar size 2.000000", 1)
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "vmap.agr"), filepath.Join(dir, "grace.eps")
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
	_ = f.Close()
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

func TestXYVMapColumnsAndRoundTrip(t *testing.T) {
	if got := agr.Columns["xyvmap"]; got != 4 {
		t.Fatalf("xyvmap has %d columns; want 4", got)
	}
	input := barProject(t, "xyvmap", "0", "0", "0", "0", "5 8 .01 -.02")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	var encoded strings.Builder
	if err := agr.Write(&encoded, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), "@type xyvmap\n") || !strings.Contains(encoded.String(), "5 8 0.01 -0.02") {
		t.Fatalf("xyvmap data did not round-trip:\n%s", encoded.String())
	}
}
