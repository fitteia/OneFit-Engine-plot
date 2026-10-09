// Package glyph gives the outlines of characters in the standard fonts, so
// text can be drawn as shapes - the same in every browser, whatever fonts
// it has. The standard fonts are stood in for by the Liberation fonts,
// whose advance widths are those of Helvetica, Times and Courier; the
// Symbol font's letters are drawn as the Greek letters of Liberation Serif.
//
// The Liberation fonts are distributed under the SIL Open Font License 1.1
// (see Notice).
package glyph

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/fitteia/OneFit-Engine-plot/internal/afm"

	"github.com/go-fonts/liberation/liberationmonobold"
	"github.com/go-fonts/liberation/liberationmonobolditalic"
	"github.com/go-fonts/liberation/liberationmonoitalic"
	"github.com/go-fonts/liberation/liberationmonoregular"
	"github.com/go-fonts/liberation/liberationsansbold"
	"github.com/go-fonts/liberation/liberationsansbolditalic"
	"github.com/go-fonts/liberation/liberationsansitalic"
	"github.com/go-fonts/liberation/liberationsansregular"
	"github.com/go-fonts/liberation/liberationserifbold"
	"github.com/go-fonts/liberation/liberationserifbolditalic"
	"github.com/go-fonts/liberation/liberationserifitalic"
	"github.com/go-fonts/liberation/liberationserifregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// The Liberation fonts' license (SIL Open Font License 1.1) and that of the
// go-fonts package carrying them (BSD), which must go with the fonts -
// they are embedded in every plot-go binary: plot-go -notices prints these.
var (
	//go:embed OFL.txt
	FontLicense string
	//go:embed go-fonts-LICENSE
	PackageLicense string
)

// standIns: each standard font's Liberation equivalent.
var standIns = map[string][]byte{
	"Helvetica":             liberationsansregular.TTF,
	"Helvetica-Bold":        liberationsansbold.TTF,
	"Helvetica-Oblique":     liberationsansitalic.TTF,
	"Helvetica-BoldOblique": liberationsansbolditalic.TTF,
	"Times-Roman":           liberationserifregular.TTF,
	"Times-Bold":            liberationserifbold.TTF,
	"Times-Italic":          liberationserifitalic.TTF,
	"Times-BoldItalic":      liberationserifbolditalic.TTF,
	"Courier":               liberationmonoregular.TTF,
	"Courier-Bold":          liberationmonobold.TTF,
	"Courier-Oblique":       liberationmonoitalic.TTF,
	"Courier-BoldOblique":   liberationmonobolditalic.TTF,
	"Symbol":                liberationserifregular.TTF,
}

// SymbolGreek is the Symbol font's letters as the Greek characters they
// draw (Adobe's Symbol.afm names them: Alpha, Beta, Chi, ...; theta1,
// sigma1, phi1, omega1 are the variant forms).
var SymbolGreek = map[byte]rune{
	'A': 'Α', 'B': 'Β', 'C': 'Χ', 'D': 'Δ', 'E': 'Ε', 'F': 'Φ', 'G': 'Γ', 'H': 'Η',
	'I': 'Ι', 'J': 'ϑ', 'K': 'Κ', 'L': 'Λ', 'M': 'Μ', 'N': 'Ν', 'O': 'Ο', 'P': 'Π',
	'Q': 'Θ', 'R': 'Ρ', 'S': 'Σ', 'T': 'Τ', 'U': 'Υ', 'V': 'ς', 'W': 'Ω', 'X': 'Ξ',
	'Y': 'Ψ', 'Z': 'Ζ',
	'a': 'α', 'b': 'β', 'c': 'χ', 'd': 'δ', 'e': 'ε', 'f': 'φ', 'g': 'γ', 'h': 'η',
	'i': 'ι', 'j': 'ϕ', 'k': 'κ', 'l': 'λ', 'm': 'μ', 'n': 'ν', 'o': 'ο', 'p': 'π',
	'q': 'θ', 'r': 'ρ', 's': 'σ', 't': 'τ', 'u': 'υ', 'v': 'ϖ', 'w': 'ω', 'x': 'ξ',
	'y': 'ψ', 'z': 'ζ',
}

// Rune is the character byte c draws in a standard font: Symbol letters
// are Greek, 39 is quoteright (gracebat's text encoding), the rest Latin-1.
func Rune(font string, c byte) rune {
	if font == "Symbol" {
		if g, ok := SymbolGreek[c]; ok {
			return g
		}
	}
	if c == '\'' && font != "Symbol" && font != "ZapfDingbats" {
		return '’'
	}
	return rune(c)
}

// Op is a path operator: move, line, quadratic or cubic curve.
type Op byte

const (
	MoveTo Op = 'M'
	LineTo Op = 'L'
	QuadTo Op = 'Q'
	CubeTo Op = 'C'
)

// Seg is one piece of an outline: its operator and its points, in ems,
// y up, from the character's origin on the baseline.
type Seg struct {
	Op  Op
	Pts [][2]float64
}

type font struct {
	f   *sfnt.Font
	upm float64
	mu  sync.Mutex
	buf sfnt.Buffer
}

var (
	mu    sync.Mutex
	cache = map[string]*font{}
)

func load(name string) (*font, error) {
	mu.Lock()
	defer mu.Unlock()
	if f := cache[name]; f != nil {
		return f, nil
	}
	ttf, ok := standIns[name]
	if !ok {
		return nil, fmt.Errorf("no outlines for font %q", name)
	}
	f, err := sfnt.Parse(ttf)
	if err != nil {
		return nil, err
	}
	ft := &font{f: f, upm: float64(f.UnitsPerEm())}
	cache[name] = ft
	return ft, nil
}

// Outline is the outline of character c in a standard font, in ems.
// A character the font has no glyph for has an empty outline.
func Outline(fontName string, c byte) ([]Seg, error) {
	f, err := load(fontName)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	i, err := f.f.GlyphIndex(&f.buf, Rune(fontName, c))
	if err != nil || i == 0 {
		return nil, err
	}
	// one unit per font unit: ppem = units per em, in 26.6 fixed point
	segs, err := f.f.LoadGlyph(&f.buf, i, fixed.Int26_6(f.upm*64), nil)
	if err != nil {
		return nil, err
	}
	out := make([]Seg, 0, len(segs))
	for _, s := range segs {
		n := map[sfnt.SegmentOp]int{sfnt.SegmentOpMoveTo: 1, sfnt.SegmentOpLineTo: 1, sfnt.SegmentOpQuadTo: 2, sfnt.SegmentOpCubeTo: 3}[s.Op]
		op := map[sfnt.SegmentOp]Op{sfnt.SegmentOpMoveTo: MoveTo, sfnt.SegmentOpLineTo: LineTo, sfnt.SegmentOpQuadTo: QuadTo, sfnt.SegmentOpCubeTo: CubeTo}[s.Op]
		seg := Seg{Op: op}
		for k := 0; k < n; k++ {
			// font units to ems; sfnt's y grows down, ems' up
			seg.Pts = append(seg.Pts, [2]float64{
				float64(s.Args[k].X) / 64 / f.upm,
				-float64(s.Args[k].Y) / 64 / f.upm,
			})
		}
		out = append(out, seg)
	}
	if fontName == "Symbol" {
		fitSymbol(out, c)
	}
	return out, nil
}

// fitSymbol stretches a Greek stand-in across the ink box Adobe's Symbol
// metrics give the character: the Symbol font has no metric-compatible
// free stand-in (Liberation Serif's mu is 0.41 em of ink, Symbol's 0.53),
// and the layout gives each character Symbol's room.
func fitSymbol(segs []Seg, c byte) {
	m, err := afm.Get("Symbol")
	if err != nil {
		return
	}
	ink := m.Ink(string(c))
	if ink.Empty() {
		return
	}
	lx, ux := 1e9, -1e9
	for _, s := range segs {
		for _, p := range s.Pts {
			lx, ux = min(lx, p[0]), max(ux, p[0])
		}
	}
	if ux <= lx {
		return
	}
	k := (ink.URX - ink.LLX) / (ux - lx)
	for _, s := range segs {
		for i := range s.Pts {
			s.Pts[i][0] = ink.LLX + (s.Pts[i][0]-lx)*k
		}
	}
}
