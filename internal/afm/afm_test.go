package afm

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestHelvetica(t *testing.T) {
	f, err := Get("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	if w := f.Width("0.02"); !near(w, 0.556*3+0.278) {
		t.Errorf("width(0.02) = %g", w)
	}
	// "0.02": the first zero's left ink edge, the last two's right edge
	want := Box{0.037, -0.019, 0.556*2 + 0.278 + 0.507, 0.703}
	got := f.Ink("0.02")
	if !near(got.LLX, want.LLX) || !near(got.LLY, want.LLY) || !near(got.URX, want.URX) || !near(got.URY, want.URY) {
		t.Errorf("ink(0.02) = %+v, want %+v", got, want)
	}
	if w := f.Width("'"); w != 0.191 {
		t.Errorf("apostrophe width %g, want quotesingle's 0.191", w)
	}
}

func TestSymbol(t *testing.T) {
	f, err := Get("Symbol")
	if err != nil {
		t.Fatal(err)
	}
	// "D" is Delta in the Symbol font, "m" is mu
	if w := f.Width("D"); w != 0.612 {
		t.Errorf("Delta width %g", w)
	}
	if w := f.Width("m"); w != 0.576 {
		t.Errorf("mu width %g", w)
	}
}

func TestEveryFontLoads(t *testing.T) {
	for _, n := range []string{"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique",
		"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic",
		"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique", "Symbol", "ZapfDingbats"} {
		if _, err := Get(n); err != nil {
			t.Error(err)
		}
	}
	if _, err := Get("Palatino"); err == nil {
		t.Error("a non-standard font loaded")
	}
}
