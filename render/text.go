package render

import (
	"math"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
)

// Escape factors, measured from gracebat (the User's Guide rounds them to
// 0.84, 0.71 and 1.19): see docs/grace-behaviour.md.
var (
	smaller = math.Pow(2, -0.25) // \-
	larger  = math.Pow(2, 0.25)  // \+
	script  = math.Pow(2, -0.5)  // \s, \S
)

// run is one piece of a typeset string drawn in a single font and size, at
// (x, y) from the string's origin, before rotation.
type run struct {
	str  string
	font string
	size float64
	x, y float64
}

// typeset is a laid-out string: its runs, the box labels are aligned by
// (ink, but starting at the pen origin), and the advance of the whole.
type typeset struct {
	runs  []run
	ink   afm.Box
	width float64
}

// fontName is the PostScript name of font number n in the project's font
// map, or Helvetica when the map has no such font.
func (r *renderer) fontName(n int) string {
	if f, ok := r.proj.Fonts[n]; ok && f.Name != "" {
		return f.Name
	}
	return "Helvetica"
}

func (r *renderer) metrics(name string) *afm.Font {
	f, err := afm.Get(name)
	if err != nil {
		r.warn("%v; Helvetica's metrics used", err)
		f, _ = afm.Get("Helvetica")
	}
	return f
}

// layout typesets s, which may hold Grace escapes, starting in font number
// font at the given size (viewport units).
func (r *renderer) layout(s string, font int, size float64) typeset {
	var ts typeset
	base := r.fontName(font)
	cur := base
	zoom, shift := 1.0, 0.0
	x := 0.0
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		str := buf.String()
		buf.Reset()
		sz := size * zoom
		m := r.metrics(cur)
		ink := m.Ink(str)
		if !ink.Empty() {
			ts.ink = ts.ink.Union(afm.Box{
				LLX: x + ink.LLX*sz, LLY: shift + ink.LLY*sz,
				URX: x + ink.URX*sz, URY: shift + ink.URY*sz,
			})
		}
		ts.runs = append(ts.runs, run{str: str, font: cur, size: sz, x: x, y: shift})
		x += m.Width(str) * sz
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			buf.WriteByte(c)
			continue
		}
		i++
		e := s[i]
		switch {
		case e == '\\':
			buf.WriteByte('\\')
			continue
		case e == 's' || e == 'S':
			flush()
			if e == 's' {
				shift -= 0.4 * size * zoom
			} else {
				shift += 0.6 * size * zoom
			}
			zoom *= script
		case e == 'N':
			flush()
			zoom, shift = 1, 0
		case e == '-':
			flush()
			zoom *= smaller
		case e == '+':
			flush()
			zoom *= larger
		case e == 'x':
			flush()
			cur = "Symbol"
		case e >= '0' && e <= '9':
			flush()
			cur = r.fontName(int(e - '0'))
		case e == 'f' && i+1 < len(s) && s[i+1] == '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				r.warn("unterminated \\f{ in %q", s)
				i = len(s)
				continue
			}
			arg := s[i+2 : i+end]
			i += end
			flush()
			switch n, err := strconv.Atoi(arg); {
			case arg == "":
				cur = base
			case err == nil:
				cur = r.fontName(n)
			default:
				cur = arg
			}
		default:
			r.warn("text escape \\%c is not supported; ignored in %q", e, s)
		}
	}
	flush()
	ts.width = x
	// gracebat's box of a string starts at the pen origin, not at the first
	// ink (a leading "1" has a wide left margin); the rest is ink
	if !ts.ink.Empty() {
		ts.ink.LLX = min(ts.ink.LLX, 0)
	}
	return ts
}

// rotate turns (x, y) by deg degrees.
func rotate(x, y, deg float64) (float64, float64) {
	if deg == 0 {
		return x, y
	}
	s, c := math.Sincos(deg * math.Pi / 180)
	return x*c - y*s, x*s + y*c
}

// emit draws a typeset string with its origin at (ox, oy), rotated by deg.
func (r *renderer) emit(ts typeset, ox, oy, deg float64, color int) {
	s, c := math.Sincos(deg * math.Pi / 180)
	if deg == 0 {
		s, c = 0, 1
	}
	rgb := r.color(color)
	for _, rn := range ts.runs {
		dx, dy := rotate(rn.x, rn.y, deg)
		r.d.Texts = append(r.d.Texts, draw.Text{
			At:     draw.Point{X: ox + dx, Y: oy + dy},
			Matrix: [4]float64{rn.size * c, rn.size * s, -rn.size * s, rn.size * c},
			Font:   rn.font,
			Color:  rgb,
			Str:    rn.str,
		})
		r.d.Order = append(r.d.Order, 't')
	}
}
