package backend

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// The fill patterns are checked cell by cell at 10 or more pixels a cell:
// at the corpus's 144 dpi a cell is 2 pixels, and Ghostscript, which drew
// the references, moves pattern tiles by up to a pixel to whole device
// pixels, so there a correct pattern and a shifted one look alike.

// inked reports whether the pixel at (x, y) is mostly ink, over white
// where the image is transparent.
func inked(img image.Image, x, y int) bool {
	r, g, b, a := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y).RGBA()
	return (r+g+b)/3+(0xffff-a) < 0x8000
}

// checkCells compares one 16 by 16 tile of pattern n, drawn with its
// lower left corner at pixel (x0, yUp0) counted up from the image's
// bottom, cell pixels apart, with draw.PatternCells.
func checkCells(t *testing.T, img image.Image, n int, x0, yUp0, cell float64) {
	t.Helper()
	want := map[[2]int]bool{}
	for _, c := range draw.PatternCells(n) {
		want[c] = true
	}
	h := img.Bounds().Dy()
	bad := 0
	for cy := 0; cy < 16; cy++ {
		for cx := 0; cx < 16; cx++ {
			x := int(x0 + (float64(cx)+0.5)*cell)
			y := h - 1 - int(yUp0+(float64(cy)+0.5)*cell)
			if inked(img, x, y) != want[[2]int{cx, cy}] {
				bad++
			}
		}
	}
	if bad > 0 {
		t.Errorf("pattern %d: %d of 256 cells differ", n, bad)
	}
}

// TestPatternCellsMatchGracebat draws gracebat's own pattern definitions
// (the prolog of one of its EPS files) at 10 pixels a cell and checks
// draw.PatternCells against them.
func TestPatternCellsMatchGracebat(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("Ghostscript (gs) not installed")
	}
	b, err := os.ReadFile(filepath.Join("..", "testdata", "ref", "style-patterns-a.eps"))
	if err != nil {
		t.Fatal(err)
	}
	_, prolog, _ := strings.Cut(string(b), "%%BeginProlog")
	prolog, _, ok := strings.Cut(prolog, "%%EndProlog")
	if !ok || !strings.Contains(prolog, "/Pattern31 ") {
		t.Fatal("no pattern definitions in gracebat's prolog")
	}
	var ps strings.Builder
	ps.WriteString("%!PS\n" + prolog + "\n")
	const pitch, cellPx = 200.0, 10.0
	s := cellPx / draw.PatternUnit
	for n := 2; n < 32; n++ {
		i := n - 2
		fmt.Fprintf(&ps, "gsave %g %g translate %g %g scale [/Pattern /DeviceRGB] setcolorspace 0 0 0 Pattern%d setcolor 0 0 %g %g rectfill grestore\n",
			20+pitch*float64(i%6), 20+pitch*float64(i/6), s, s, n, 16*draw.PatternUnit, 16*draw.PatternUnit)
	}
	ps.WriteString("showpage\n")
	dir := t.TempDir()
	src, png := filepath.Join(dir, "p.ps"), filepath.Join(dir, "p.png")
	if err := os.WriteFile(src, []byte(ps.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=pnggray", "-r72",
		"-g1240x1040", "-sOutputFile="+png, src).CombinedOutput(); err != nil || len(out) > 0 {
		t.Fatalf("gs: %v %s", err, out)
	}
	img := readPNG(t, png)
	for n := 2; n < 32; n++ {
		i := n - 2
		checkCells(t, img, n, 20+pitch*float64(i%6), 20+pitch*float64(i/6), cellPx)
	}
}

// TestSVGPatterns: the SVG paints every pattern's cells where gracebat
// does, its tiles starting at the page's origin.
func TestSVGPatterns(t *testing.T) {
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Skip("rsvg-convert not installed")
	}
	const tile = 16 * draw.PatternUnit
	d := &draw.Drawing{Width: 1.3, Height: 1}
	for n := 2; n < 32; n++ {
		i := n - 2
		x, y := 2*tile*float64(i%6+1), 2*tile*float64(i/6+1)
		d.Paths = append(d.Paths, draw.Path{Fill: true, Style: draw.Style{Pattern: n},
			Segments: []draw.Segment{{Points: []draw.Point{{X: x, Y: y}, {X: x + tile, Y: y}, {X: x + tile, Y: y + tile}, {X: x, Y: y + tile}}, Closed: true}}})
		d.Order = append(d.Order, 'p')
	}
	const pageW, pageH, dpi = 780.0, 600.0, 720.0
	var b bytes.Buffer
	if err := WriteSVG(&b, d, pageW, pageH, "patterns"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	svg, png := filepath.Join(dir, "p.svg"), filepath.Join(dir, "p.png")
	if err := os.WriteFile(svg, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("rsvg-convert", "-d", fmt.Sprint(dpi), "-p", fmt.Sprint(dpi), "-o", png, svg).CombinedOutput(); err != nil {
		t.Fatalf("rsvg-convert: %v %s", err, out)
	}
	img := readPNG(t, png)
	bb := BoundingBox(d, pageH)
	px := dpi / 72 // pixels a point
	for n := 2; n < 32; n++ {
		i := n - 2
		x, y := 2*tile*float64(i%6+1), 2*tile*float64(i/6+1)
		checkCells(t, img, n, (x*pageH-bb.LLX)*px, (y*pageH-bb.LLY)*px, draw.PatternUnit*pageH*px)
	}
}
