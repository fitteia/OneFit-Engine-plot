package backend

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// rasterize renders an EPS over the whole page exactly as
// scripts/make-references.sh renders gracebat's.
func rasterize(t *testing.T, eps []byte) image.Image {
	t.Helper()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.eps"), filepath.Join(dir, "out.png")
	if err := os.WriteFile(in, eps, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r144", "-g1546x1200",
		"-dFIXEDMEDIA", "-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-sOutputFile="+out, in)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gs: %v\n%s", err, b)
	}
	return readPNG(t, out)
}

func readPNG(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// differing counts pixels of either image with no pixel of about the same
// color within shift pixels in the other image, and the pixels not white
// in either. The shift absorbs text placed a fraction of a point apart
// (gracebat rounds its text boxes; its fonts are URW's): what remains is
// something drawn that the other image does not have.
func differing(a, b image.Image, tol, shift int) (diff, ink int) {
	r := a.Bounds()
	at := func(img image.Image, x, y int) [3]int {
		cr, cg, cb, _ := img.At(x, y).RGBA()
		return [3]int{int(cr >> 8), int(cg >> 8), int(cb >> 8)}
	}
	close := func(p, q [3]int) bool {
		for i := range p {
			if d := p[i] - q[i]; d > tol || d < -tol {
				return false
			}
		}
		return true
	}
	white := func(p [3]int) bool { return p[0] >= 250 && p[1] >= 250 && p[2] >= 250 }
	found := func(img image.Image, x, y int, p [3]int) bool {
		for dy := -shift; dy <= shift; dy++ {
			for dx := -shift; dx <= shift; dx++ {
				q := image.Pt(x+dx, y+dy)
				if q.In(r) && close(p, at(img, q.X, q.Y)) {
					return true
				}
			}
		}
		return false
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			pa, pb := at(a, x, y), at(b, x, y)
			if white(pa) && white(pb) {
				continue
			}
			ink++
			if close(pa, pb) {
				continue
			}
			if !found(b, x, y, pa) || !found(a, x, y, pb) {
				diff++
			}
		}
	}
	return diff, ink
}

// TestPixelsMatchGracebat draws every corpus file through the EPS backend
// and Ghostscript and compares the page with gracebat's (testdata/ref).
// Pixels are compared allowing a 2-pixel shift (text may sit up to 0.7 pt
// from gracebat's); anything structural - a missing tick, a wrong dash, a
// misplaced label, a different glyph - is far above the limit.
func TestPixelsMatchGracebat(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("Ghostscript (gs) not installed")
	}
	files, _ := filepath.Glob("../testdata/corpus/*.agr")
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".agr")
		t.Run(name, func(t *testing.T) {
			p, err := agr.ParseFile(f)
			if err != nil {
				t.Fatal(err)
			}
			d, _ := render.Render(p)
			var eps bytes.Buffer
			if err := WriteEPS(&eps, d, p.PageWidth, p.PageHeight, name); err != nil {
				t.Fatal(err)
			}
			got := rasterize(t, eps.Bytes())
			want := readPNG(t, filepath.Join("..", "testdata", "ref", name+".png"))
			if got.Bounds() != want.Bounds() {
				t.Fatalf("size %v, reference %v", got.Bounds(), want.Bounds())
			}
			diff, ink := differing(got, want, 96, 2)
			frac := float64(diff) / float64(ink)
			t.Logf("%d of %d inked pixels differ (%.2f%%)", diff, ink, 100*frac)
			if frac > pixelLimit {
				t.Errorf("%.2f%% of inked pixels differ from gracebat's, limit %.2f%%", 100*frac, 100*pixelLimit)
			}
		})
	}
}

// pixelLimit is the share of inked pixels allowed to differ (all corpus
// files are at 0).
const pixelLimit = 0.002

// TestPixelComparisonCatchesAMissingLabel: the shift tolerance must not
// hide a real difference - test3 without its "[1]" run name fails.
func TestPixelComparisonCatchesAMissingLabel(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("Ghostscript (gs) not installed")
	}
	p, err := agr.ParseFile("../testdata/corpus/test3-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	d, _ := render.Render(p)
	last := d.Texts[len(d.Texts)-1]
	if last.Str != "[1]" {
		t.Fatalf("last text %q, want the run name [1]", last.Str)
	}
	d.Texts = d.Texts[:len(d.Texts)-1]
	d.Order = d.Order[:len(d.Order)-1]
	var eps bytes.Buffer
	if err := WriteEPS(&eps, d, p.PageWidth, p.PageHeight, "test3-1"); err != nil {
		t.Fatal(err)
	}
	diff, ink := differing(rasterize(t, eps.Bytes()), readPNG(t, "../testdata/ref/test3-1.png"), 96, 2)
	if frac := float64(diff) / float64(ink); frac <= pixelLimit {
		t.Errorf("a missing label left %d of %d pixels different (%.3f%%), not above the limit", diff, ink, 100*frac)
	} else {
		t.Logf("a missing label: %d pixels differ (%.2f%%)", diff, 100*frac)
	}
}
