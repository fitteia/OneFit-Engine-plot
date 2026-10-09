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
)

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
			p, d := drawing(t, f)
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
