package afm

import (
	"math"
	"strings"
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
	if w := f.Width("'"); w != 0.222 {
		t.Errorf("apostrophe width %g, want quoteright's 0.222", w)
	}
	if w := f.Width("-"); w != 0.333 {
		t.Errorf("hyphen width %g, want 0.333 (not minus, 0.584)", w)
	}
}

func TestHelveticaLatin1(t *testing.T) {
	f, err := Get("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	for code, want := range map[byte]float64{
		0xb0: .400, 0xb1: .584, 0xb2: .333, 0xb3: .333,
		0xb5: .556, 0xb7: .278, 0xd7: .584, 0xf7: .584,
		0xa0: .278, 0xad: .333, 0xe9: .556,
	} {
		if got := f.Width(string([]byte{code})); !near(got, want) {
			t.Errorf("Latin-1 %02x width %g, want %g", code, got, want)
		}
	}
	if got := f.Ink("\xb0C"); !near(got.URX, .400+.681) {
		t.Errorf("degree+C right edge %g, want 1.081", got.URX)
	}
}

func TestLatin1Coverage(t *testing.T) {
	for _, name := range []string{"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique",
		"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic",
		"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique"} {
		f, err := Get(name)
		if err != nil {
			t.Fatal(err)
		}
		for code := 0xa0; code <= 0xff; code++ {
			if f.Width(string([]byte{byte(code)})) <= 0 {
				t.Errorf("%s missing Latin-1 %02x", name, code)
			}
		}
		if f.Width("\x80\x9f") != 0 {
			t.Errorf("%s retains non-Latin-1 upper-half codes", name)
		}
	}
}

func TestReencodedKerningAliases(t *testing.T) {
	f, err := Read("Helvetica", strings.NewReader(`C 32 ; WX 278 ; N space ; B 0 0 0 0 ;
C 45 ; WX 333 ; N hyphen ; B 0 0 300 10 ;
C 65 ; WX 667 ; N A ; B 0 0 667 700 ;
C -1 ; WX 667 ; N Aacute ; B 0 0 667 900 ;
KPX space A -20
KPX A hyphen -30
KPX Aacute A -40
`))
	if err != nil {
		t.Fatal(err)
	}
	for pair, want := range map[[2]byte]float64{
		{' ', 'A'}: -.020, {0xa0, 'A'}: -.020,
		{'A', '-'}: -.030, {'A', 0xad}: -.030, {0xc1, 'A'}: -.040,
	} {
		if got := f.Kern(pair[0], pair[1]); !near(got, want) {
			t.Errorf("pair %x: %g, want %g", pair, got, want)
		}
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

func TestCore14PairKerning(t *testing.T) {
	f, err := Get("Helvetica")
	if err != nil {
		t.Fatal(err)
	}
	for pair, want := range map[[2]byte]float64{{'A', 'V'}: -0.070, {'T', 'o'}: -0.120, {'0', '0'}: 0} {
		if got := f.Kern(pair[0], pair[1]); !near(got, want) {
			t.Errorf("pair %q: %g, want %g", pair, got, want)
		}
	}
}
