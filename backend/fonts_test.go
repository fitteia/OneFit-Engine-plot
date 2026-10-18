package backend

import (
	"bytes"
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
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
	"github.com/fitteia/OneFit-Engine-plot/render"
	"github.com/go-fonts/liberation/liberationsansregular"
	"github.com/go-fonts/liberation/liberationserifregular"
)

func TestAdditionalFontExports(t *testing.T) {
	dir := t.TempDir()
	font := filepath.Join(dir, "test.ttf")
	if err := os.WriteFile(font, liberationsansregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	serif := filepath.Join(dir, "serif.ttf")
	if err := os.WriteFile(serif, liberationserifregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFE_FONT_PATH", dir)
	src := `@version 50125
@page size 773, 600
@g0 hidden true
@map font 20 to "LiberationSans", "Helvetica"
@with string
@string on
@string 0.2, 0.5
@string font 20
@string char size 2
@string def "AV \#{b0}C \#{e9} \uunder\U \oover\O \s2\N \f{Helvetica}standard \f{LiberationSerif}serif\f{}"
`
	p, err := agr.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	d, warnings := render.Render(p)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	if len(d.Texts) == 0 || d.Texts[0].Font != "LiberationSans" {
		t.Fatalf("font fell back: %+v", d.Texts)
	}
	if got := glyph.Source("LiberationSans"); got != font {
		t.Fatalf("source %s", got)
	}
	m, err := afm.Get("LiberationSans")
	if err != nil {
		t.Fatal(err)
	}
	if m.Width("\xb0C") <= m.Width("C") {
		t.Fatal("degree has no advance")
	}
	if m.Kern('A', 'V') >= 0 {
		t.Fatal("font's AV kerning not loaded")
	}
	missingProject, err := agr.Parse(strings.NewReader(strings.Replace(src, "AV ", "\\#{80} ", 1)))
	if err != nil {
		t.Fatal(err)
	}
	missingDrawing, missingWarnings := render.Render(missingProject)
	if len(missingWarnings) == 0 || len(missingDrawing.Texts) == 0 || !m.Missing(0x80) {
		t.Fatal("unavailable character did not produce a visible, warned missing glyph")
	}
	// Mixing embedded fonts, a standard font and a pattern exercises all PDF
	// resource references, rather than just the one-font special case.
	d.Paths = append(d.Paths, pdraw.Path{Fill: true, Style: pdraw.Style{Pattern: 1}, Segments: []pdraw.Segment{{Closed: true, Points: []pdraw.Point{{X: .2, Y: .1}, {X: .3, Y: .1}, {X: .3, Y: .2}}}}})
	d.Order = append(d.Order, 'p')
	var pdf, eps, svg bytes.Buffer
	if err := WritePDF(&pdf, d, p.PageWidth, p.PageHeight, "custom"); err != nil {
		t.Fatal(err)
	}
	if err := WriteEPS(&eps, d, p.PageWidth, p.PageHeight, "custom"); err != nil {
		t.Fatal(err)
	}
	if err := WriteSVG(&svg, d, p.PageWidth, p.PageHeight, "custom"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pdf.String(), "/Subtype /Type3") || !strings.Contains(pdf.String(), "/ToUnicode") {
		t.Fatal("PDF lacks embedded glyphs/Unicode mapping")
	}
	if !strings.Contains(eps.String(), "/FontType 3") {
		t.Fatal("EPS does not embed additional font")
	}
	if strings.Count(svg.String(), "<text") != 1 || !strings.Contains(svg.String(), ">standard </text>") {
		t.Fatal("additional SVG font depends on browser installation or standard font changed")
	}
	if err := os.Remove(font); err != nil {
		t.Fatal(err)
	} // exports must be self-contained
	if err := os.Remove(serif); err != nil {
		t.Fatal(err)
	}
	for ext, data := range map[string][]byte{"pdf": pdf.Bytes(), "eps": eps.Bytes()} {
		path := filepath.Join(dir, "out."+ext)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := exec.LookPath("gs"); err == nil {
			if out, err := exec.Command("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=nullpage", path).CombinedOutput(); err != nil {
				t.Fatalf("%s: %v %s", ext, err, out)
			}
		}
		if ext == "pdf" {
			if _, err := exec.LookPath("pdftotext"); err == nil {
				out, err := exec.Command("pdftotext", path, "-").CombinedOutput()
				if err != nil || !strings.Contains(string(out), "AV °C é") {
					t.Fatalf("PDF text extraction: %v %s", err, out)
				}
			}
		}
	}
}

// OpenType/CFF outlines and their metrics are compared against the Grace
// executable using only public font maps and AGR directives.
func TestAdditionalURWFontsMatchGrace(t *testing.T) {
	for _, command := range []string{"gs", "gracebat"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skip(command + " not installed")
		}
	}
	fonts := []string{"URWBookman-Light", "URWBookman-DemiItalic", "URWGothic-Book", "URWGothic-DemiOblique", "P052-Roman", "P052-BoldItalic", "C059-Roman", "NimbusSansNarrow-Regular", "Z003-MediumItalic"}
	var src strings.Builder
	src.WriteString("@version 50125\n@page size 773,600\n@page background fill on\n@g0 hidden true\n")
	for i, name := range fonts {
		if _, err := afm.Get(name); err != nil {
			t.Skip(name + " not installed")
		}
		fmt.Fprintf(&src, "@map font %d to %q, %q\n@with string\n@string on\n@string loctype view\n@string 0.6, %g\n@string font %d\n@string char size 1.5\n@string just 2\n@string def \"\\FkAVATAR\\FK \\#{b0}C \\#{e9} \\uunder\\U \\s2\\N\"\n", 20+i, name, name, .9-float64(i)*.09, 20+i)
	}
	p, err := agr.Parse(strings.NewReader(src.String()))
	if err != nil {
		t.Fatal(err)
	}
	d, w := render.Render(p)
	if len(w) > 0 {
		t.Fatal(w)
	}
	dir := t.TempDir()
	agrPath := filepath.Join(dir, "in.agr")
	graceEPS := filepath.Join(dir, "grace.eps")
	if err := os.WriteFile(agrPath, []byte(src.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("gracebat", agrPath, "-hdevice", "EPS", "-printfile", graceEPS).CombinedOutput(); err != nil {
		t.Fatalf("Grace: %v %s", err, out)
	}
	bb := BoundingBox(d, math.Min(p.PageWidth, p.PageHeight))
	ref := filepath.Join(dir, "ref.png")
	// Grace writes BoundingBox at end. Crop to the same measured coordinates
	// explicitly so Ghostscript does not have to interpret an atend box.
	args := []string{"-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r144", "-dTextAlphaBits=4", "-dGraphicsAlphaBits=4"}
	refArgs := append(append([]string{}, args...), fmt.Sprintf("-dDEVICEWIDTHPOINTS=%g", bb.URX-bb.LLX), fmt.Sprintf("-dDEVICEHEIGHTPOINTS=%g", bb.URY-bb.LLY), "-dFIXEDMEDIA", "-sOutputFile="+ref, "-c", fmt.Sprintf("%g %g translate", -bb.LLX, -bb.LLY), "-f", graceEPS)
	if out, err := exec.Command("gs", refArgs...).CombinedOutput(); err != nil {
		t.Fatalf("Grace raster: %v %s", err, out)
	}
	for _, format := range []string{"pdf", "eps"} {
		var b bytes.Buffer
		if format == "pdf" {
			err = WritePDF(&b, d, p.PageWidth, p.PageHeight, "fonts")
		} else {
			err = WriteEPS(&b, d, p.PageWidth, p.PageHeight, "fonts")
		}
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "out."+format)
		png := path + ".png"
		if err := os.WriteFile(path, b.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		if format == "pdf" {
			out, err := exec.Command("gs", append(append([]string{}, args...), "-sOutputFile="+png, path)...).CombinedOutput()
			if err != nil {
				t.Fatalf("PDF raster: %v %s", err, out)
			}
		} else {
			epsArgs := append(append([]string{}, args...), fmt.Sprintf("-dDEVICEWIDTHPOINTS=%g", bb.URX-bb.LLX), fmt.Sprintf("-dDEVICEHEIGHTPOINTS=%g", bb.URY-bb.LLY), "-dFIXEDMEDIA", "-sOutputFile="+png, "-c", fmt.Sprintf("%g %g translate", -bb.LLX, -bb.LLY), "-f", path)
			out, err := exec.Command("gs", epsArgs...).CombinedOutput()
			if err != nil {
				t.Fatalf("EPS raster: %v %s", err, out)
			}
		}
		got, want := readPNG(t, png), readPNG(t, ref)
		if got.Bounds() != want.Bounds() {
			t.Fatalf("%s dimensions %v want %v", format, got.Bounds(), want.Bounds())
		}
		white := image.NewRGBA(got.Bounds())
		draw.Draw(white, white.Bounds(), want, image.Point{}, draw.Src)
		diff, ink := differing(got, white, 96, 2)
		frac := float64(diff) / float64(ink)
		t.Logf("%s: %.3f%% of ink differs from Grace", format, 100*frac)
		if frac > glyphLimit {
			t.Errorf("%s differs %.3f%%, limit %.1f%%", format, 100*frac, 100*glyphLimit)
		}
	}
}

func TestMissingFontUsesSameFallbackEverywhere(t *testing.T) {
	p, err := agr.Parse(strings.NewReader("@g0 hidden true\n@map font 20 to \"NoSuchFont-12345\", \"Helvetica\"\n@with string\n@string on\n@string 0.2,0.5\n@string font 20\n@string def \"test\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	d, w := render.Render(p)
	if len(w) == 0 {
		t.Fatal("missing-font fallback was silent")
	}
	if len(d.Texts) == 0 || d.Texts[0].Font != "Helvetica" {
		t.Fatalf("metrics and output fonts disagree: %+v", d.Texts)
	}
}
