package glyph

import (
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
)

// The stand-ins' outlines sit where Adobe's metrics put the ink: a
// character's outline spans about its AFM ink box.
func TestOutlinesMatchAdobeInk(t *testing.T) {
	for _, c := range []struct {
		font string
		ch   byte
	}{{"Helvetica", '0'}, {"Helvetica", 'm'}, {"Times-Roman", 'R'}, {"Symbol", 'D'}, {"Symbol", 'm'}} {
		segs, err := Outline(c.font, c.ch)
		if err != nil || len(segs) == 0 {
			t.Fatalf("%s %q: %v, %d segments", c.font, c.ch, err, len(segs))
		}
		lx, ly, ux, uy := 1e9, 1e9, -1e9, -1e9
		for _, s := range segs {
			for _, p := range s.Pts {
				lx, ly, ux, uy = min(lx, p[0]), min(ly, p[1]), max(ux, p[0]), max(uy, p[1])
			}
		}
		m, _ := afm.Get(c.font)
		ink := m.Ink(string(c.ch))
		for _, d := range []float64{lx - ink.LLX, ly - ink.LLY, ux - ink.URX, uy - ink.URY} {
			if d > 0.06 || d < -0.06 {
				t.Errorf("%s %q: outline box %.3f %.3f %.3f %.3f, Adobe ink %+v", c.font, c.ch, lx, ly, ux, uy, ink)
				break
			}
		}
	}
}

func TestRune(t *testing.T) {
	if Rune("Symbol", 'D') != 'Δ' || Rune("Symbol", 'm') != 'μ' || Rune("Helvetica", '\'') != '’' || Rune("Helvetica", 'a') != 'a' {
		t.Error("wrong characters")
	}
}

func TestUnknownFont(t *testing.T) {
	if _, err := Outline("ZapfDingbats", 'a'); err == nil {
		t.Error("ZapfDingbats has no stand-in, but an outline came back")
	}
}
