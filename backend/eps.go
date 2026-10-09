package backend

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// WriteEPS writes the drawing as Encapsulated PostScript on a page of
// pageW x pageH points. It uses the same small vocabulary as gracebat's
// EPS (m, l, s, SLW, SD, CC, FFSF, EARC, ...), so the files read alike.
func WriteEPS(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
	unit := math.Min(pageW, pageH)
	bb := BoundingBox(d, unit)
	b := bufio.NewWriter(w)
	p := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }

	p("%%!PS-Adobe-3.0 EPSF-3.0\n")
	p("%%%%BoundingBox: %d %d %d %d\n", int(bb.LLX), int(bb.LLY), int(bb.URX), int(bb.URY))
	p("%%%%Creator: plot-go\n")
	p("%%%%Title: %s\n", strings.Map(func(r rune) rune {
		if r < ' ' {
			return ' '
		}
		return r
	}, title))
	p("%%%%LanguageLevel: 2\n%%%%Pages: 1\n%%%%EndComments\n%%%%BeginProlog\n")
	p(`/m {moveto} def
/l {lineto} def
/s {stroke} def
/n {newpath} def
/c {closepath} def
/SLW {setlinewidth} def
/GS {gsave} def
/GR {grestore} def
/SC {setcolor} def
/SD {setdash} def
/SLC {setlinecap} def
/SLJ {setlinejoin} def
/SCS {setcolorspace} def
/FFSF {findfont setfont} def
/CC {concat} def
/ellipsedict 8 dict def
ellipsedict /mtrx matrix put
/EARC {
  ellipsedict begin
  /endangle exch def
  /startangle exch def
  /yrad exch def
  /xrad exch def
  /y exch def
  /x exch def
  /savematrix mtrx currentmatrix def
  x y translate
  xrad yrad scale
  0 0 1 startangle endangle arc
  savematrix setmatrix
  end
} def
`)
	// colors as named procedures, like gracebat's Color0, Color1, ...
	colors := map[[3]float64]int{}
	var order [][3]float64
	note := func(c [3]float64) {
		if _, ok := colors[c]; !ok {
			colors[c] = len(order)
			order = append(order, c)
		}
	}
	for _, path := range d.Paths {
		note(path.Style.Color)
	}
	for _, t := range d.Texts {
		note(t.Color)
	}
	for i, c := range order {
		p("/Color%d {%.4f %.4f %.4f} def\n", i, c[0], c[1], c[2])
	}
	// gracebat's text encoding: Latin-1, but 39 is quoteright and 45 the
	// hyphen as in Adobe Standard (ISOLatin1Encoding has quotesingle and
	// minus there, which the layout did not measure)
	p("/TextEncoding ISOLatin1Encoding 256 array copy def\nTextEncoding 39 /quoteright put\nTextEncoding 45 /hyphen put\nTextEncoding 96 /grave put\n")
	p("%%%%EndProlog\n%%%%BeginSetup\n")
	fonts := map[string]int{}
	for _, t := range d.Texts {
		if _, ok := fonts[t.Font]; !ok {
			id := len(fonts)
			fonts[t.Font] = id
			if t.Font == "Symbol" || t.Font == "ZapfDingbats" {
				p("/%s findfont\n/Font%d exch definefont pop\n", t.Font, id)
			} else {
				p("/%s findfont\ndup length dict begin\n {1 index /FID ne {def} {pop pop} ifelse} forall\n /Encoding TextEncoding def\n currentdict\nend\n/Font%d exch definefont pop\n", t.Font, id)
			}
		}
	}
	p("%%%%EndSetup\n%.2f %.2f scale\n", unit, unit)

	var cur draw.Style
	curColor := -1
	setColor := func(c [3]float64) {
		if i := colors[c]; i != curColor {
			p("[/DeviceRGB] SCS\nColor%d SC\n", i)
			curColor = i
		}
	}
	first := true
	setStyle := func(st draw.Style) {
		setColor(st.Color)
		if first || !sameDash(st.Dash, cur.Dash) {
			p("[")
			for _, x := range st.Dash {
				p("%.4f ", x)
			}
			p("] 0 SD\n")
		}
		if first || st.Width != cur.Width {
			p("%.4f SLW\n", st.Width)
		}
		if first || st.Cap != cur.Cap {
			p("%d SLC\n", st.Cap)
		}
		if first || st.Join != cur.Join {
			p("%d SLJ\n", st.Join)
		}
		cur, first = st, false
	}
	pi, ti := 0, 0
	for _, kind := range d.Order {
		if kind == 't' {
			t := d.Texts[ti]
			ti++
			setColor(t.Color)
			p("/Font%d FFSF\n%.4f %.4f m\nGS\n[%.4f %.4f %.4f %.4f 0 0] CC\n(%s) show\nGR\n",
				fonts[t.Font], t.At.X, t.At.Y, t.Matrix[0], t.Matrix[1], t.Matrix[2], t.Matrix[3], psString(t.Str))
			continue
		}
		path := d.Paths[pi]
		pi++
		if path.Fill {
			setColor(path.Style.Color)
		} else {
			setStyle(path.Style)
		}
		p("n\n")
		for _, s := range path.Segments {
			if a := s.Arc; a != nil {
				p("%.4f %.4f %.4f %.4f %g %g EARC\n", a.X, a.Y, a.RX, a.RY, a.A1, a.A2)
				continue
			}
			for i, q := range s.Points {
				op := "l"
				if i == 0 {
					op = "m"
				}
				p("%.4f %.4f %s\n", q.X, q.Y, op)
			}
			if s.Closed {
				p("c\n")
			}
		}
		if path.Fill {
			p("fill\n")
		} else {
			p("s\n")
		}
	}
	p("showpage\n%%%%Trailer\n%%%%EOF\n")
	return b.Flush()
}

func sameDash(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// psString escapes a PostScript string's parentheses and backslashes, and
// writes bytes outside printable ASCII as octal.
func psString(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '(' || c == ')' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c < ' ' || c > '~':
			fmt.Fprintf(&b, "\\%03o", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}
