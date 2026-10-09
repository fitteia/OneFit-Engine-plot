package eps

import (
	"os"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// testdata/test3-1.eps is gracebat 5.1.25's EPS of
// testdata/corpus/test3-1.agr.
func TestParseGracebatEPS(t *testing.T) {
	f, err := os.Open("testdata/test3-1.eps")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	// 5 x and 6 y tick labels, 2 axis labels (the y one in two fonts:
	// Symbol "D" then "l(m) "), the "(a)" label and the run name
	if len(d.Texts) != 16 {
		t.Errorf("%d texts, want 16", len(d.Texts))
	}
	first := d.Texts[0]
	if first.Str != "0.02" || first.Font != "Helvetica" || first.At != (draw.Point{X: 0.2989, Y: 0.1100}) || first.Matrix != [4]float64{0.042, 0, 0, 0.042} {
		t.Errorf("first text = %+v", first)
	}
	var delta *draw.Text
	for i := range d.Texts {
		if d.Texts[i].Font == "Symbol" {
			delta = &d.Texts[i]
		}
	}
	if delta == nil || delta.Str != "D" || delta.Matrix != [4]float64{0, 0.042, -0.042, 0} {
		t.Errorf("Symbol text = %+v", delta)
	}
	last := d.Texts[len(d.Texts)-1]
	if last.Str != "[1]" || last.Color != [3]float64{0, 0, 1} || last.Matrix[0] != 0.028 {
		t.Errorf("last text = %+v", last)
	}
	circles := 0
	for _, p := range d.Paths {
		for _, s := range p.Segments {
			if s.Arc != nil {
				circles++
				if s.Arc.RX != 0.01 || p.Style.Width != 0.003 {
					t.Errorf("circle %+v width %g", s.Arc, p.Style.Width)
				}
			}
		}
	}
	if circles != 12 {
		t.Errorf("%d circles, want 12", circles)
	}
	// the dashed fitted curve: linestyle 3 at linewidth 2
	found := false
	for _, p := range d.Paths {
		if len(p.Style.Dash) == 2 && p.Style.Dash[0] == 0.015 && p.Style.Dash[1] == 0.009 {
			found = true
		}
	}
	if !found {
		t.Error("no path with dash [0.015 0.009]")
	}
}

func TestParseRejectsUnknownOperators(t *testing.T) {
	_, err := Parse(strings.NewReader("%%EndSetup\n1 2 m 3 4 frobnicate\n%%Trailer\n"))
	if err == nil {
		t.Error("unknown operator accepted")
	}
}
