package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }

// testdata/gracebat-call holds a real OneFit plot call, recorded on a
// OneFit server (OneFit-Engine test 3, first block): the command line
// (argv), its three input files, and what gracebat wrote
// (gracebat-fit-curves-1.eps and .agr).
const call = "../../testdata/gracebat-call"

// replay runs the recorded command line in a scratch copy of the inputs.
func replay(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"gnu0.da_", "fit-curves-1", "fit1.agr-par"} {
		b, err := os.ReadFile(filepath.Join(call, f))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, f), b, 0o644)
	}
	argv, err := os.ReadFile(filepath.Join(call, "argv"))
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Fields(string(argv))[1:] // without "grace"
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	var out, errs bytes.Buffer
	if code := runGrace(args, &out, &errs); code != 0 || errs.Len() > 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	return dir
}

func TestGracebatCallEPS(t *testing.T) {
	dir := replay(t)
	open := func(name string) *os.File {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		return f
	}
	f := open(filepath.Join(dir, "fit-curves-1.eps"))
	got, err := eps.Parse(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	f = open(filepath.Join(call, "gracebat-fit-curves-1.eps"))
	want, err := eps.Parse(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drawcmp.Compare(got, want, 25) {
		t.Error(d)
	}
}

// TestGracebatCallSaveall: the project plot-go saves draws exactly as the
// project gracebat saved does.
func TestGracebatCallSaveall(t *testing.T) {
	dir := replay(t)
	draw := func(name string) []string {
		p, err := agr.ParseFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range p.Warnings {
			t.Errorf("%s:%d: %s", name, w.Line, w.Msg)
		}
		d, _ := render.Render(p)
		var lines []string
		for _, tx := range d.Texts {
			lines = append(lines, tx.Str)
		}
		for _, pa := range d.Paths {
			for _, s := range pa.Segments {
				if s.Arc != nil {
					lines = append(lines, strings.TrimSpace(strings.Join([]string{"arc", ftoa(s.Arc.X), ftoa(s.Arc.Y)}, " ")))
				}
				for _, q := range s.Points {
					lines = append(lines, ftoa(q.X)+" "+ftoa(q.Y))
				}
			}
		}
		return lines
	}
	saved, err := os.ReadFile(filepath.Join(dir, "fit-curves-1.agr"))
	if err != nil {
		t.Fatal(err)
	}
	// the one-line forms gracebat saves, which the OneFit GUI reads
	for _, line := range []string{"@    world 1e-05, 0, 0.11, 0.11\n", "@    view 0.183247, 0.149994, 1.038397, 0.849964\n"} {
		if !strings.Contains(string(saved), line) {
			t.Errorf("saved project lacks %q", line)
		}
	}
	if strings.Contains(string(saved), "world xmin") || strings.Contains(string(saved), "view xmin") {
		t.Error("saved project still has per-edge world/view lines")
	}
	got, want := draw(filepath.Join(dir, "fit-curves-1.agr")), draw(filepath.Join(call, "gracebat-fit-curves-1.agr"))
	if len(got) != len(want) {
		t.Fatalf("%d drawn items, gracebat's project %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("item %d: %q, gracebat's project %q", i, got[i], want[i])
			break
		}
	}
}
