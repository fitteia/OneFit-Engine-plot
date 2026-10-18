package backend

import (
	"bytes"
	"image"
	"image/draw"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/internal/eps"

	pdraw "github.com/fitteia/OneFit-Engine-plot/draw"
)

// The PDF text operator advances degree by 400/1000 em in Helvetica.
// Outlined SVG must place the following C at the same position.
func TestSVGOutlinedDegreeSpacing(t *testing.T) {
	coord := func(x float64) string { return num(x) }
	text := pdraw.Text{Font: "Helvetica", Str: "\xb0C", Matrix: [4]float64{1, 0, 0, 1}}
	got, ok := textPath(text, coord, coord)
	if !ok {
		t.Fatal("degree+C has no outlines")
	}
	text.Str = "C"
	text.At.X = .400
	c, ok := textPath(text, coord, coord)
	if !ok || !strings.HasSuffix(got, c) {
		t.Fatal("outlined C is not positioned 0.400 em after degree")
	}
}

// glyphLimit: with text as Liberation outlines, rasterized by rsvg, the
// share of inked pixels allowed to differ from gracebat's page (URW fonts
// through Ghostscript). Every line, tick and text position agrees; what
// differs is glyph shapes at their edges - 0.4% to 1.6% on the corpus.
const glyphLimit = 0.025

// TestSVGTextAsPathsMatchesGracebat: the editor preview's SVG - text drawn
// as outlines, no <text> left - shows gracebat's page.
func TestSVGTextAsPathsMatchesGracebat(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("rsvg-convert not installed")
	}
	for _, f := range corpus(t) {
		name := strings.TrimSuffix(filepath.Base(f), ".agr")
		t.Run(name, func(t *testing.T) {
			sameTime(t, name)
			p, d := drawing(t, f)
			if overlineFiles[name] {
				gracebatDecorations(t, d, name)
			}
			for _, path := range d.Paths {
				if path.Style.Pattern != 0 {
					// gracebat's page was drawn by Ghostscript, which moves
					// pattern tiles to whole pixels: TestSVGPatterns checks
					// the SVG's patterns cell by cell instead
					t.Skip("fill patterns: see TestSVGPatterns")
				}
			}
			var b bytes.Buffer
			if err := WriteSVGWith(&b, d, p.PageWidth, p.PageHeight, name, SVGOptions{TextAsPaths: true}); err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(b.String(), "<text"); n != 0 {
				t.Errorf("%d texts left as <text>", n)
			}
			dir := t.TempDir()
			svg, png := filepath.Join(dir, "a.svg"), filepath.Join(dir, "a.png")
			if err := os.WriteFile(svg, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("rsvg-convert", "-d", "144", "-p", "144", "-o", png, svg).CombinedOutput(); err != nil {
				t.Fatalf("rsvg-convert: %v %s", err, out)
			}
			got := readPNG(t, png)
			ref := readPNG(t, filepath.Join("..", "testdata", "ref", name+".png"))
			bb := BoundingBox(d, math.Min(p.PageWidth, p.PageHeight))
			want := image.NewRGBA(got.Bounds())
			draw.Draw(want, want.Bounds(), ref, image.Pt(int(bb.LLX*2), int((p.PageHeight-bb.URY)*2)), draw.Src)
			diff, ink := differing(got, want, 96, 2)
			frac := float64(diff) / float64(ink)
			t.Logf("%d of %d inked pixels differ (%.2f%%)", diff, ink, 100*frac)
			if frac > glyphLimit {
				t.Errorf("%.2f%% differ from gracebat's page, limit %.1f%%", 100*frac, 100*glyphLimit)
			}
		})
	}
}

// overlineFiles draw \o overlines, which plot-go deliberately puts above
// the glyphs where Grace's stroke crosses them (docs/grace-behaviour.md,
// Text); render's text tests check where plot-go puts them.
var overlineFiles = map[string]bool{
	"style-text-context": true, "style-text-decorations": true, "style-text-decoration-rotation": true,
}

// gracebatDecorations moves the text decoration strokes (underlines and
// overlines, drawn with their text's ID) to where gracebat's EPS has them,
// as render's TestMatchesGracebat does, so the pixels compare everything
// else.
func gracebatDecorations(t *testing.T, d *pdraw.Drawing, name string) {
	t.Helper()
	r, err := os.Open(filepath.Join("..", "testdata", "ref", name+".eps"))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	want, err := eps.Parse(r)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < min(len(d.Paths), len(want.Paths)); i++ {
		id := d.Paths[i].ID
		if !strings.HasPrefix(id, "string.") && !strings.HasSuffix(id, ".label") && !strings.HasSuffix(id, ".title") &&
			!strings.HasSuffix(id, ".subtitle") && !strings.HasSuffix(id, ".ticklabel") && !strings.HasSuffix(id, ".avalue") {
			continue
		}
		for j := 0; j < min(len(d.Paths[i].Segments), len(want.Paths[i].Segments)); j++ {
			gs, ws := d.Paths[i].Segments[j].Points, want.Paths[i].Segments[j].Points
			for k := 0; k < min(len(gs), len(ws)); k++ {
				gs[k] = ws[k]
			}
		}
	}
}
