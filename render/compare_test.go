package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
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
			if name == "style-text-context" || name == "style-text-decorations" || name == "style-text-decoration-rotation" {
				// Grace's positive overline offset crosses many glyphs. plot-go
				// deliberately moves only these decoration paths above their ink;
				// their semantics are asserted in text_test.go. Keep comparing the
				// text, styles, and every non-decoration primitive to Grace.
				alignTextDecorationPaths(got, want)
			}
			if name == "style-text-transforms" || name == "style-text-flow" || name == "style-text-kerning" {
				splitTextGlyphs(got)
				splitTextGlyphs(want)
			}
			for _, d := range drawcmp.Compare(got, want, 25) {
				t.Error(d)
			}
		})
	}
}

func alignTextDecorationPaths(got, want *draw.Drawing) {
	for i := 0; i < min(len(got.Paths), len(want.Paths)); i++ {
		id := got.Paths[i].ID
		if !strings.HasPrefix(id, "string.") && !strings.HasSuffix(id, ".label") && !strings.HasSuffix(id, ".title") && !strings.HasSuffix(id, ".subtitle") && !strings.HasSuffix(id, ".ticklabel") && !strings.HasSuffix(id, ".avalue") {
			continue
		}
		for j := 0; j < min(len(got.Paths[i].Segments), len(want.Paths[i].Segments)); j++ {
			for k := 0; k < min(len(got.Paths[i].Segments[j].Points), len(want.Paths[i].Segments[j].Points)); k++ {
				want.Paths[i].Segments[j].Points[k] = got.Paths[i].Segments[j].Points[k]
			}
		}
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

// Equivalent text can be emitted as a show run or individual kerned glyphs.
// Compare every glyph and its position without requiring identical grouping.
func splitTextGlyphs(d *draw.Drawing) {
	var texts []draw.Text
	var order []byte
	ti := 0
	for _, kind := range d.Order {
		if kind != 't' {
			order = append(order, kind)
			continue
		}
		t := d.Texts[ti]
		ti++
		m, err := afm.Get(t.Font)
		if err != nil {
			panic(err)
		}
		pen := 0.0
		for i := 0; i < len(t.Str); i++ {
			c := t.Str[i : i+1]
			if strings.Trim(c, " \t") != "" {
				g := t
				g.Str = c
				g.At.X += pen * t.Matrix[0]
				g.At.Y += pen * t.Matrix[1]
				texts = append(texts, g)
				order = append(order, 't')
			}
			pen += m.Width(c)
		}
	}
	d.Texts = texts
	d.Order = order
}
