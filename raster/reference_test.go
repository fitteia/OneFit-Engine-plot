package raster

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// needConverter skips a PNG test where neither converter is installed.
func needConverter(t *testing.T) {
	t.Helper()
	if Available() {
		return
	}
	t.Skip("no PNG converter installed")
}

// pngOf renders a corpus file to PNG at dpi with every converter there
// is, each alone.
func pngOf(t *testing.T, name string, dpi int) map[string]image.Image {
	t.Helper()
	needConverter(t)
	p, err := agr.ParseFile(filepath.Join("../testdata/corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := render.Render(p)
	all := converters()
	out := map[string]image.Image{}
	for _, c := range all {
		var b bytes.Buffer
		if err := writeWith(context.Background(), &b, d, p.PageWidth, p.PageHeight, name, Options{Format: "png", DPI: dpi}, []converter{c}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		img, err := png.Decode(&b)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		out[c.name] = img
	}
	return out
}

// TestPNGIsTheWholePage: gracebat's PNG is the page, its points times
// dpi/72 pixels, rounded (measured: 773x600 points -> 773x600 at 72 dpi,
// 1546x1200 at 144, 3221x2500 at 300 - plot-go's own default).
func TestPNGIsTheWholePage(t *testing.T) {
	for _, c := range []struct{ dpi, w, h int }{{72, 773, 600}, {144, 1546, 1200}, {DefaultDPI, 3221, 2500}} {
		for conv, img := range pngOf(t, "p96-10-1.agr", c.dpi) {
			if b := img.Bounds(); b.Dx() != c.w || b.Dy() != c.h {
				t.Errorf("%s at %d dpi: %dx%d, gracebat %dx%d", conv, c.dpi, b.Dx(), b.Dy(), c.w, c.h)
			}
		}
	}
}

// TestPNGMatchesTheReference: at 144 dpi the PNG is the page gracebat's
// EPS makes (testdata/ref, rasterized the same way), within antialiasing:
// pdftoppm's to 0.5% of its ink, Ghostscript's (gs, ImageMagick) to 5% -
// it strokes the PDF's lines a little differently from the EPS's, at
// their edges.
func TestPNGMatchesTheReference(t *testing.T) {
	for _, name := range []string{"p96-10-1", "style-defaults", "style-errorbars", "style-legend"} {
		ref, err := os.Open(filepath.Join("../testdata/ref", name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		want, err := png.Decode(ref)
		ref.Close()
		if err != nil {
			t.Fatal(err)
		}
		for conv, img := range pngOf(t, name+".agr", 144) {
			diff, ink := unmatched(img, want, 48, 2)
			t.Logf("%s %s: %d of %d ink pixels unmatched", name, conv, diff, ink)
			limit := 20 // 5%
			if conv == "pdftoppm" {
				limit = 200 // 0.5%
			}
			if diff*limit > ink {
				t.Errorf("%s %s: %d of %d ink pixels have no match in gracebat's", name, conv, diff, ink)
			}
		}
	}
}

// TestPNGFillsEvenOdd: gracebat's raster devices fill even-odd only (the
// Grace User's Guide says so; its PNG leaves the middle of
// style-set-fill's lower star, filled non-zero, empty), the PDF and EPS
// fill it.
func TestPNGFillsEvenOdd(t *testing.T) {
	for conv, img := range pngOf(t, "style-set-fill.agr", 72) {
		in := func(x, y int) [3]uint32 {
			r, g, b, _ := img.At(x, y).RGBA()
			return [3]uint32{r >> 8, g >> 8, b >> 8}
		}
		if c := in(483, 400); c != [3]uint32{255, 255, 255} {
			t.Errorf("%s: the lower star's middle is %v, gracebat leaves it white", conv, c)
		}
		if c := in(483, 360); c != [3]uint32{0, 0, 255} {
			t.Errorf("%s: the lower star's top arm is %v, gracebat's is blue", conv, c)
		}
	}
}

// unmatched counts the ink pixels (not white) of a with no pixel of about
// the same colour within shift pixels in b, and all ink pixels of a.
func unmatched(a, b image.Image, tol, shift int) (diff, ink int) {
	at := func(img image.Image, x, y int) [3]int {
		r, g, bl, _ := img.At(x, y).RGBA()
		return [3]int{int(r >> 8), int(g >> 8), int(bl >> 8)}
	}
	near := func(p, q [3]int) bool {
		for i := range p {
			if d := p[i] - q[i]; d > tol || d < -tol {
				return false
			}
		}
		return true
	}
	r := a.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			p := at(a, x, y)
			if p[0] >= 250 && p[1] >= 250 && p[2] >= 250 {
				continue
			}
			ink++
			found := false
			for dy := -shift; dy <= shift && !found; dy++ {
				for dx := -shift; dx <= shift && !found; dx++ {
					q := image.Pt(x+dx, y+dy)
					found = q.In(b.Bounds()) && near(p, at(b, q.X, q.Y))
				}
			}
			if !found {
				diff++
			}
		}
	}
	return diff, ink
}
