package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

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
			for _, d := range drawcmp.Compare(got, want, 25) {
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
