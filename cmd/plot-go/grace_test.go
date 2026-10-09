package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// Each testdata/gracebat-call* directory is a real OneFit plot call,
// recorded on a OneFit server with a logging stand-in for grace on the
// PATH: the command line (argv), its input files, and what gracebat wrote,
// named gracebat-OUTPUT (its -printfile EPS and -saveall project).
//
//	gracebat-call            OneFit-Engine test 3, block 1
//	gracebat-call-format60   test 8, block 13: "ticklabel format 60" and a
//	                         "ticklabel prec" without a value
func calls(t *testing.T) []string {
	dirs, _ := filepath.Glob("../../testdata/gracebat-call*")
	if len(dirs) == 0 {
		t.Fatal("no recorded calls")
	}
	return dirs
}

// replay runs a recorded command line in a scratch copy of its inputs and
// returns the scratch directory and the arguments.
func replay(t *testing.T, call string) (string, []string) {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(call)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "argv" || strings.HasPrefix(e.Name(), "gracebat-") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(call, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644)
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
	if code := runGrace(args, &out, &errs); code != 0 {
		t.Fatalf("exit %d: %s", code, errs.String())
	}
	t.Logf("plot-go said: %s", strings.TrimSpace(errs.String()))
	return dir, args
}

// option is the value after a flag in args.
func option(args []string, flag string) string {
	for i := range args[:len(args)-1] {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func parseEPS(t *testing.T, name string) *draw.Drawing {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := eps.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGracebatCallEPS(t *testing.T) {
	for _, call := range calls(t) {
		t.Run(filepath.Base(call), func(t *testing.T) {
			dir, args := replay(t, call)
			out := option(args, "-printfile")
			got := parseEPS(t, filepath.Join(dir, out))
			want := parseEPS(t, filepath.Join(call, "gracebat-"+out))
			for _, d := range drawcmp.Compare(got, want, 25) {
				t.Error(d)
			}
		})
	}
}

// TestGracebatCallSaveall: the project plot-go saves redraws what gracebat
// printed, and has world and view in the one-line form the OneFit GUI
// reads. (gracebat's own saved project need not: for "ticklabel format 60"
// it saves "unknown", which it rejects when reading the project back.)
func TestGracebatCallSaveall(t *testing.T) {
	for _, call := range calls(t) {
		t.Run(filepath.Base(call), func(t *testing.T) {
			dir, args := replay(t, call)
			out := option(args, "-saveall")
			saved, err := os.ReadFile(filepath.Join(dir, out))
			if err != nil {
				t.Fatal(err)
			}
			for _, form := range []string{"\n@    world ", "\n@    view "} {
				if !strings.Contains(string(saved), form) {
					t.Errorf("saved project has no %q line", strings.TrimSpace(form))
				}
			}
			if strings.Contains(string(saved), "world xmin") || strings.Contains(string(saved), "view xmin") {
				t.Error("saved project still has per-edge world/view lines")
			}
			p, err := agr.ParseFile(filepath.Join(dir, out))
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range p.Warnings {
				t.Errorf("saved project, line %d: %s", w.Line, w.Msg)
			}
			got, _ := render.Render(p)
			want := parseEPS(t, filepath.Join(call, "gracebat-"+option(args, "-printfile")))
			for _, d := range drawcmp.Compare(got, want, 25) {
				t.Error(d)
			}
		})
	}
}
