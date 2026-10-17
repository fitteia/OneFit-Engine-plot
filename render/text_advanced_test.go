package render

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
)

// Measure layout rather than trusting the guide's abbreviated descriptions.
// Each case is generated independently and compared with Grace as a black box.
func TestAdvancedTextAlignmentMatchesGrace(t *testing.T) {
	if _, err := exec.LookPath("gracebat"); err != nil {
		t.Skip("gracebat not installed")
	}
	cases := []string{
		`A\t{1 0.5 0.25 1}BC\t{}D`,
		`A\r{30}BC\r{-30}D`,
		`A\r{30}BC\nD`,
		`A\V{1}B\NC`,
		`A\t{2 0 0 3}\sBC\ND`,
		`A\l{0.5}BC\QD`,
		`A\t{2 0 0 3}B\NC`,
		`A\r{30}B\h{1}C\v{1}D\v{}E`,
		`A\m{1}\v{1}BC\M{1}D`,
		`A\v{1}\m{1}B\r{30}\M{1}C\v{}D`,
		`A\m{1}\V{1}B\M{1}C\V{}D`,
		`A\dr\dRBCD\dl\dLEF`,
		`A\R{red}\FkAVATAR\FK\R{}D`,
	}
	settings := [][2]int{{0, 0}, {35, 0}, {35, 1}, {35, 2}, {35, 4}, {35, 8}, {35, 10}, {90, 14}, {225, 6}}
	for ci, text := range cases {
		for _, setting := range settings {
			t.Run(fmt.Sprintf("%d/rot%d-just%d", ci, setting[0], setting[1]), func(t *testing.T) {
				src := fmt.Sprintf("@version 50125\n@page size 773, 600\n@page background fill off\n@map font 0 to \"Helvetica\", \"Helvetica\"\n@map color 1 to (0,0,0), \"black\"\n@map color 2 to (255,0,0), \"red\"\n@g0 hidden true\n@with string\n@string on\n@string loctype view\n@string 0.4, 0.5\n@string font 0\n@string color 1\n@string rot %d\n@string just %d\n@string def \"%s\"\n", setting[0], setting[1], text)
				dir := t.TempDir()
				in, out := filepath.Join(dir, "in.agr"), filepath.Join(dir, "out.eps")
				if err := os.WriteFile(in, []byte(src), 0600); err != nil {
					t.Fatal(err)
				}
				if log, err := exec.Command("gracebat", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
					t.Fatalf("Grace: %v %s", err, log)
				}
				p, err := agr.Parse(strings.NewReader(src))
				if err != nil {
					t.Fatal(err)
				}
				got, w := Render(p)
				if len(w) > 0 {
					t.Fatal(w)
				}
				f, err := os.Open(out)
				if err != nil {
					t.Fatal(err)
				}
				want, err := eps.Parse(f)
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
				splitTextGlyphs(got)
				splitTextGlyphs(want)
				if diff := drawcmp.Compare(got, want, 8); len(diff) > 0 {
					t.Fatal(diff)
				}
			})
		}
	}
}

func TestTextControlStateIsLocalToLabel(t *testing.T) {
	r := &renderer{proj: &agr.Project{Colors: map[int]agr.Color{2: {R: 255, Name: "red"}}}}
	r.layout(`\R{red}\Fk\dr\dR\q\v{1}\m{2}AB`, 0, 0.028)
	ts := r.layout(`CD\M{2}E`, 0, 0.028)
	if len(ts.runs) != 2 || ts.runs[0].str != "CD" || ts.runs[0].matrix != identityText || ts.runs[0].color != -1 || ts.runs[0].kern != nil || ts.runs[0].y != 0 || ts.runs[1].x != 0 {
		t.Fatalf("label leaked state: %+v", ts.runs)
	}
}

func TestOverlineStrokeClearsInkWhenTransformed(t *testing.T) {
	fonts := []string{"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique", "Times-Roman", "Times-Italic", "Times-Bold", "Times-BoldItalic", "Courier", "Courier-Oblique", "Courier-Bold", "Courier-BoldOblique", "Symbol", "ZapfDingbats"}
	for _, font := range fonts {
		for _, control := range []string{"", `\z{2}`, `\t{3 0.5 0 0.5}`, `\r{30}`} {
			t.Run(font+control, func(t *testing.T) {
				r := &renderer{proj: &agr.Project{Fonts: map[int]agr.Font{0: {Name: font}}}, d: &draw.Drawing{}}
				ts := r.layout(control+`\oCO2\O`, 0, 0.028)
				r.emit(ts, 0, 0, 0, 1)
				if len(r.d.Paths) != 1 || len(r.d.Texts) != 1 {
					t.Fatalf("missing decoration: %+v", r.d)
				}
				tx, p := r.d.Texts[0], r.d.Paths[0]
				a := p.Segments[0].Points[0]
				norm := math.Hypot(tx.Matrix[0], tx.Matrix[1])
				nx, ny := -tx.Matrix[1]/norm, tx.Matrix[0]/norm
				height := (tx.Matrix[2]*nx + tx.Matrix[3]*ny)
				ink := glyph.Ink(font, r.metrics(font), "CO2")
				lower := (a.X-tx.At.X)*nx + (a.Y-tx.At.Y)*ny - p.Style.Width/2
				if lower+1e-12 < (ink.URY+0.04)*height {
					t.Errorf("stroke overlaps ink/gap: lower %g, required %g", lower, (ink.URY+0.04)*height)
				}
			})
		}
	}
}

func TestMalformedAdvancedControlsKeepFollowingText(t *testing.T) {
	for _, control := range []string{`\t{bad}`, `\T{1 2 3}`, `\r{NaN}`, `\l{Inf}`, `\R{no-such-colour}`, `\m{-1}`, `\M{huge}`, `\z{1e308}\z{1e308}`} {
		t.Run(control, func(t *testing.T) {
			r := &renderer{proj: &agr.Project{}, d: &draw.Drawing{}}
			ts := r.layout("A"+control+"B", 0, 0.028)
			r.emit(ts, 0, 0, 0, 1)
			var got strings.Builder
			for _, tx := range r.d.Texts {
				got.WriteString(tx.Str)
				for _, v := range tx.Matrix {
					if math.IsNaN(v) || math.IsInf(v, 0) {
						t.Fatalf("nonfinite matrix: %+v", tx)
					}
				}
			}
			if got.String() != "AB" || len(r.warnings) == 0 {
				t.Fatalf("text %q, warnings %v", got.String(), r.warnings)
			}
		})
	}
}
