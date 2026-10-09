package backend

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	pdraw "github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

func corpus(t *testing.T) []string {
	t.Helper()
	files, _ := filepath.Glob("../testdata/corpus/*.agr")
	if len(files) == 0 {
		t.Fatal("no corpus files")
	}
	return files
}

func drawing(t *testing.T, file string) (*agr.Project, *pdraw.Drawing) {
	t.Helper()
	p, err := agr.ParseFile(file)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := render.Render(p)
	return p, d
}

// TestEPSRoundTrip: plot-go's EPS read back (by the reader made for
// gracebat's EPS) is the drawing it was written from.
func TestEPSRoundTrip(t *testing.T) {
	for _, f := range corpus(t) {
		p, d := drawing(t, f)
		var b bytes.Buffer
		if err := WriteEPS(&b, d, p.PageWidth, p.PageHeight, f); err != nil {
			t.Fatal(err)
		}
		back, err := eps.Parse(&b)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(back.Paths) != len(d.Paths) || len(back.Texts) != len(d.Texts) || string(back.Order) != string(d.Order) {
			t.Fatalf("%s: %d paths %d texts back, wrote %d and %d", f, len(back.Paths), len(back.Texts), len(d.Paths), len(d.Texts))
		}
		for i, tx := range d.Texts {
			g := back.Texts[i]
			if g.Str != tx.Str || g.Font != tx.Font || math.Abs(g.At.X-tx.At.X) > 1e-4 || math.Abs(g.At.Y-tx.At.Y) > 1e-4 {
				t.Errorf("%s: text %d back as %+v, wrote %+v", f, i, g, tx)
			}
		}
	}
}

func TestBoundingBoxNearGracebats(t *testing.T) {
	for _, f := range corpus(t) {
		p, d := drawing(t, f)
		bb := BoundingBox(d, math.Min(p.PageWidth, p.PageHeight))
		ref, err := os.ReadFile(filepath.Join("..", "testdata", "ref", strings.TrimSuffix(filepath.Base(f), ".agr")+".eps"))
		if err != nil {
			t.Fatal(err)
		}
		var want [4]float64
		s := string(ref)
		line := s[strings.LastIndex(s, "%%BoundingBox:"):]
		if _, err := fmtSscan(line, &want); err != nil {
			t.Fatal(err)
		}
		got := [4]float64{bb.LLX, bb.LLY, bb.URX, bb.URY}
		for i := range got {
			if math.Abs(got[i]-want[i]) > 3 {
				t.Errorf("%s: box %v, gracebat %v", filepath.Base(f), got, want)
				break
			}
		}
	}
}

// TestPDFPixelsMatchGracebat: the cropped PDF, rasterized by Ghostscript
// like the references, shows the part of gracebat's page its bounding box
// names. Poppler (pdftoppm), which many viewers use, must open it without
// complaint; its pixels are not compared (it snaps thin lines and draws
// fonts its own way).
func TestPDFPixelsMatchGracebat(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("Ghostscript (gs) not installed")
	}
	_, poppler := exec.LookPath("pdftoppm")
	for _, f := range corpus(t) {
		name := strings.TrimSuffix(filepath.Base(f), ".agr")
		t.Run(name, func(t *testing.T) {
			p, d := drawing(t, f)
			dir := t.TempDir()
			var b bytes.Buffer
			if err := WritePDF(&b, d, p.PageWidth, p.PageHeight, name); err != nil {
				t.Fatal(err)
			}
			pdf := filepath.Join(dir, "out.pdf")
			if err := os.WriteFile(pdf, b.Bytes(), 0o644); err != nil {
				t.Fatal(err)
			}
			if poppler == nil {
				out, err := exec.Command("pdftoppm", "-r", "36", "-png", "-singlefile", pdf, filepath.Join(dir, "poppler")).CombinedOutput()
				if err != nil || len(out) > 0 {
					t.Errorf("pdftoppm: %v %s", err, out)
				}
			}
			png := filepath.Join(dir, "out.png")
			if out, err := exec.Command("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r144",
				"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-sOutputFile="+png, pdf).CombinedOutput(); err != nil || len(out) > 0 {
				t.Fatalf("gs: %v %s", err, out)
			}
			got := readPNG(t, png)
			ref := readPNG(t, filepath.Join("..", "testdata", "ref", name+".png"))
			bb := BoundingBox(d, math.Min(p.PageWidth, p.PageHeight))
			// the box in the reference's pixels (144 dpi, y down)
			x0, y0 := int(bb.LLX*2), int((p.PageHeight-bb.URY)*2)
			want := image.NewRGBA(got.Bounds())
			draw.Draw(want, want.Bounds(), ref, image.Pt(x0, y0), draw.Src)
			diff, ink := differing(got, want, 96, 2)
			frac := float64(diff) / float64(ink)
			t.Logf("%d of %d inked pixels differ (%.2f%%)", diff, ink, 100*frac)
			if frac > pixelLimit {
				t.Errorf("%.2f%% of inked pixels differ from gracebat's, limit %.2f%%", 100*frac, 100*pixelLimit)
			}
		})
	}
}

// TestSVGIsWellFormed parses every corpus file's SVG, and checks the
// Symbol font's letters came out Greek.
func TestSVGIsWellFormed(t *testing.T) {
	for _, f := range corpus(t) {
		p, d := drawing(t, f)
		var b bytes.Buffer
		if err := WriteSVG(&b, d, p.PageWidth, p.PageHeight, f); err != nil {
			t.Fatal(err)
		}
		dec := xml.NewDecoder(bytes.NewReader(b.Bytes()))
		for {
			_, err := dec.Token()
			if err != nil {
				if err.Error() != "EOF" {
					t.Fatalf("%s: %v", f, err)
				}
				break
			}
		}
		if strings.Contains(f, "test3-1") && !strings.Contains(b.String(), ">Δ<") {
			t.Errorf("%s: no Greek Delta in the SVG", f)
		}
	}
}

// fmtSscan reads the four numbers of a "%%BoundingBox: a b c d" line.
func fmtSscan(line string, v *[4]float64) (int, error) {
	return fmt.Sscanf(strings.TrimPrefix(strings.SplitN(line, "\n", 2)[0], "%%BoundingBox:"), "%g %g %g %g", &v[0], &v[1], &v[2], &v[3])
}
