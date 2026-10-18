// Package glyph gives the outlines of characters in standard and installed fonts, so
// text can be drawn as shapes - the same in every browser, whatever fonts
// it has.
//
// Where the URW base-35 fonts are installed as OpenType (Debian's
// fonts-urw-base35, which Ghostscript needs), it uses them: the very glyphs
// Ghostscript embeds when epstopdf turns OneFit's EPS into its PDF, so the
// preview is the PDF. They are read at run time from the system, never
// shipped. Otherwise it uses the Liberation fonts it carries, whose advance
// widths are those of Helvetica, Times and Courier; the Symbol font's
// letters are then Liberation Serif's Greek. OFE_URW_FONTS names another
// directory holding the URW OpenType files. On macOS the onefite-go
// formula installs them in Homebrew's share/fonts/urw-base35, and the
// font-urw-base35 cask in ~/Library/Fonts; both are searched.
//
// The Liberation fonts are distributed under the SIL Open Font License 1.1
// (see FontLicense).
package glyph

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/fontfile"

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
	xfont "golang.org/x/image/font"
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

// The licenses (BSD) and patent grants of golang.org/x/image, which reads
// and draws the fonts, and golang.org/x/text, which it uses: compiled into
// every plot-go binary, so plot-go -notices prints them too.
var (
	//go:embed x-image-LICENSE
	XImageLicense string
	//go:embed x-image-PATENTS
	XImagePatents string
	//go:embed x-text-LICENSE
	XTextLicense string
	//go:embed x-text-PATENTS
	XTextPatents string
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

// urwFiles: each standard font's URW base-35 OpenType file.
var urwFiles = map[string]string{
	"Helvetica":             "NimbusSans-Regular.otf",
	"Helvetica-Bold":        "NimbusSans-Bold.otf",
	"Helvetica-Oblique":     "NimbusSans-Italic.otf",
	"Helvetica-BoldOblique": "NimbusSans-BoldItalic.otf",
	"Times-Roman":           "NimbusRoman-Regular.otf",
	"Times-Bold":            "NimbusRoman-Bold.otf",
	"Times-Italic":          "NimbusRoman-Italic.otf",
	"Times-BoldItalic":      "NimbusRoman-BoldItalic.otf",
	"Courier":               "NimbusMonoPS-Regular.otf",
	"Courier-Bold":          "NimbusMonoPS-Bold.otf",
	"Courier-Oblique":       "NimbusMonoPS-Italic.otf",
	"Courier-BoldOblique":   "NimbusMonoPS-BoldItalic.otf",
	"Symbol":                "StandardSymbolsPS.otf",
	"ZapfDingbats":          "D050000L.otf",
}

// ownCodes: the URW fonts that map a standard font's own character codes
// ('m' is mu in Standard Symbols, 'A' the a10 dingbat in D050000L), not
// Latin-1.
func ownCodes(font string) bool { return font == "Symbol" || font == "ZapfDingbats" }

// urwDirs are searched in order, after $OFE_URW_FONTS: Debian's and
// Fedora's places, Homebrew's (where the onefite-go formula installs
// them: share/fonts/urw-base35 under its prefix, or next to plot-go's own
// install), then macOS's font folders.
var urwDirs = append(append([]string{
	"/usr/share/fonts/opentype/urw-base35",
	"/usr/local/share/fonts/opentype/urw-base35",
	"/usr/share/fonts/urw-base35",
	"/opt/homebrew/share/fonts/urw-base35",
	"/usr/local/share/fonts/urw-base35",
}, besideExecutable()...), macFontDirs(os.Getenv("HOME"))...)

// besideExecutable is share/fonts/urw-base35 next to the running
// program's bin folder - the program as invoked and as its links resolve
// (Homebrew links bin/plot-go into its prefix from the formula's own
// folder; either holds the fonts).
func besideExecutable() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	paths := []string{exe}
	if real, err := filepath.EvalSymlinks(exe); err == nil && real != exe {
		paths = append(paths, real)
	}
	var dirs []string
	for _, p := range paths {
		dirs = append(dirs, filepath.Join(filepath.Dir(filepath.Dir(p)), "share", "fonts", "urw-base35"))
	}
	return dirs
}

// macFontDirs are the folders macOS installs fonts in - a user's (where
// "brew install --cask font-urw-base35" puts the URW fonts) and the
// machine's.
func macFontDirs(home string) []string {
	var dirs []string
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "Library", "Fonts"))
	}
	return append(dirs, "/Library/Fonts")
}

type font struct {
	kernOnce sync.Once
	kernAFM  *afm.Font
	f        *sfnt.Font
	upm      float64
	urw      bool   // a URW font: Symbol is looked up by its own codes
	path     string // additional font file; empty for Core-14 outlines
	mu       sync.Mutex
	buf      sfnt.Buffer
}

// Source is where outlines come from: the installed font file's path, or
// "Liberation" (the standard-font stand-ins plot-go carries).
func Source(name string) string {
	f, err := load(name)
	if err != nil {
		return ""
	}
	if f.urw {
		return urwPath(name)
	}
	if f.path != "" {
		return f.path
	}
	return "Liberation"
}

func urwPath(name string) string {
	file, ok := urwFiles[name]
	if !ok {
		return ""
	}
	dirs := urwDirs
	if d := os.Getenv("OFE_URW_FONTS"); d != "" {
		dirs = append([]string{d}, dirs...)
	}
	for _, d := range dirs {
		p := filepath.Join(d, file)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			return p
		}
	}
	return ""
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
	if p := urwPath(name); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if f, err := sfnt.Parse(b); err == nil {
				ft := &font{f: f, upm: float64(f.UnitsPerEm()), urw: true}
				cache[name] = ft
				return ft, nil
			}
		}
	}
	ttf, ok := standIns[name]
	if !ok {
		if !afm.Standard(name) {
			face, err := fontfile.Get(name)
			if err != nil {
				return nil, err
			}
			ft := &font{f: face.Font, upm: float64(face.Font.UnitsPerEm()), path: face.Path}
			cache[name] = ft
			return ft, nil
		}
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

// Outline is the outline of byte c in a supported font, in ems. An additional
// font uses its .notdef outline for missing characters; standard fonts retain
// their historical empty-outline behavior for unavailable glyphs.
func Outline(fontName string, c byte) ([]Seg, error) {
	f, err := load(fontName)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r := Rune(fontName, c)
	if f.urw && ownCodes(fontName) {
		r = rune(c)
	}
	i, err := f.f.GlyphIndex(&f.buf, r)
	if err != nil || (i == 0 && f.path == "") {
		return nil, err
	}
	// one unit per font unit: ppem = units per em, in 26.6 fixed point
	segs, err := f.f.LoadGlyph(&f.buf, i, fixed.Int26_6(f.upm*64), nil)
	if err != nil {
		return nil, err
	}
	out := make([]Seg, 0, len(segs))
	for _, s := range segs {
		var n int
		var op Op
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			n, op = 1, MoveTo
		case sfnt.SegmentOpLineTo:
			n, op = 1, LineTo
		case sfnt.SegmentOpQuadTo:
			n, op = 2, QuadTo
		case sfnt.SegmentOpCubeTo:
			n, op = 3, CubeTo
		}
		seg := Seg{Op: op, Pts: make([][2]float64, 0, n)}
		for k := 0; k < n; k++ {
			// font units to ems; sfnt's y grows down, ems' up
			seg.Pts = append(seg.Pts, [2]float64{
				float64(s.Args[k].X) / 64 / f.upm,
				-float64(s.Args[k].Y) / 64 / f.upm,
			})
		}
		out = append(out, seg)
	}
	if fontName == "Symbol" && !f.urw {
		fitSymbol(out, c)
	}
	return out, nil
}

// URWInk is the box of the ink character c leaves in a standard font, in
// ems, from the URW font when it is installed - the glyphs gracebat lays
// its text out with. ok is false without the URW font (or for a character
// it has no glyph for): the caller keeps Adobe's metrics then.
func URWInk(fontName string, c byte) (llx, lly, urx, ury float64, ok bool) {
	f, err := load(fontName)
	if err != nil || !f.urw {
		return 0, 0, 0, 0, false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	r := Rune(fontName, c)
	if ownCodes(fontName) {
		r = rune(c)
	}
	i, err := f.f.GlyphIndex(&f.buf, r)
	if err != nil || i == 0 {
		return 0, 0, 0, 0, false
	}
	b, _, err := f.f.GlyphBounds(&f.buf, i, fixed.Int26_6(f.upm*64), xfont.HintingNone)
	if err != nil || b.Max.X <= b.Min.X {
		return 0, 0, 0, 0, false
	}
	// font units to ems; sfnt's y grows down, ems' up
	u := 64 * f.upm
	return float64(b.Min.X) / u, -float64(b.Max.Y) / u, float64(b.Max.X) / u, -float64(b.Min.Y) / u, true
}

// Ink is the box of the ink str leaves in font name (whose Adobe metrics
// are m), in ems, the pen starting at 0: each character's box from the URW
// font where it is installed - gracebat lays its text out with the URW
// glyphs, whose ink differs from Adobe's by up to 0.02 em - else from
// Adobe's metrics. The advance widths are Adobe's either way (the URW fonts
// are metric compatible).
func Ink(name string, m *afm.Font, str string) afm.Box {
	var b afm.Box
	x := 0.0
	for i := 0; i < len(str); i++ {
		c := str[i : i+1]
		g := m.Ink(c)
		if llx, lly, urx, ury, ok := URWInk(name, str[i]); ok {
			g = afm.Box{LLX: llx, LLY: lly, URX: urx, URY: ury}
		}
		if !g.Empty() {
			g = afm.Box{LLX: x + g.LLX, LLY: g.LLY, URX: x + g.URX, URY: g.URY}
			if b.Empty() {
				b = g
			} else {
				b = b.Union(g)
			}
		}
		x += m.Width(c)
	}
	return b
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

// Kern returns the system URW font's pair adjustment, or the Core-14 AFM
// value when that font is unavailable. No kerning is applied by default.
func Kern(name string, m *afm.Font, a, b byte) float64 {
	f, err := load(name)
	if err != nil || !f.urw {
		return m.Kern(a, b)
	}
	f.kernOnce.Do(func() {
		path := urwPath(name)
		stem := strings.TrimSuffix(filepath.Base(path), ".otf") + ".afm"
		// AFM and OpenType GPOS kerning are not interchangeable: Grace uses AFM.
		dirs := []string{filepath.Dir(path), filepath.Join(filepath.Dir(path), "..", "..", "type1", "urw-base35"), "/usr/share/fonts/type1/urw-base35"}
		for _, dir := range dirs {
			in, err := os.Open(filepath.Join(dir, stem))
			if err != nil {
				continue
			}
			metrics, err := afm.Read(name, in)
			in.Close()
			if err == nil {
				f.kernAFM = metrics
				return
			}
		}
	})
	if f.kernAFM != nil {
		return f.kernAFM.Kern(a, b)
	}
	return m.Kern(a, b)
}
