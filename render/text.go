package render

import (
	"encoding/hex"
	"math"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
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
	str         string
	font        string
	size        float64
	x, y        float64
	under, over bool
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
	baseline := 0.0
	under, over, upper := false, false, false
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
		ink := glyph.Ink(cur, m, str)
		if !ink.Empty() {
			x0, x1 := x+ink.LLX*sz, x+ink.URX*sz
			y0, y1 := shift+ink.LLY*sz, shift+ink.URY*sz
			ts.ink = ts.ink.Union(afm.Box{
				LLX: min(x0, x1), LLY: min(y0, y1),
				URX: max(x0, x1), URY: max(y0, y1),
			})
		}
		ts.runs = append(ts.runs, run{str: str, font: cur, size: sz, x: x, y: shift, under: under, over: over})
		x += m.Width(str) * sz
	}
	// Brace controls are parsed as a whole, including unsupported ones: their
	// arguments must never become visible label text. Values and units below
	// are measured from Grace's EPS, not inferred from its source.
	brace := func(i *int) (string, bool) {
		if *i+1 >= len(s) || s[*i+1] != '{' {
			return "", false
		}
		end := strings.IndexByte(s[*i+2:], '}')
		if end < 0 {
			r.warn("unterminated \\%c{ in %q", s[*i], s)
			*i = len(s)
			return "", false
		}
		arg := s[*i+2 : *i+2+end]
		*i += end + 2
		return arg, true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 == len(s) {
			if upper && c < 128 {
				c += 128
			}
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
			zoom, shift = 1, baseline
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
		case e == 'u' || e == 'U' || e == 'o' || e == 'O':
			flush()
			if e == 'u' || e == 'U' {
				under = e == 'u'
			} else {
				over = e == 'o'
			}
		case e == 'c' || e == 'C':
			upper = e == 'c'
		case e == 'n':
			flush()
			x = 0
			baseline -= size * zoom
			shift = baseline
		case i+1 < len(s) && s[i+1] == '{':
			arg, ok := brace(&i)
			if !ok {
				continue
			}
			switch e {
			case 'f':
				flush()
				switch n, err := strconv.Atoi(arg); {
				case arg == "":
					cur = base
				case err == nil:
					cur = r.fontName(n)
				default:
					cur = arg
				}
			case '#':
				b, err := hex.DecodeString(arg)
				if err != nil {
					r.warn("invalid hexadecimal text escape in %q", s)
				} else {
					buf.Write(b)
				}
			case 'z', 'Z', 'v', 'V', 'h':
				v := 0.0
				var err error
				if arg != "" {
					v, err = strconv.ParseFloat(arg, 64)
				}
				if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
					r.warn("invalid \\%c{%s} in %q", e, arg, s)
					continue
				}
				switch e {
				case 'z', 'Z':
					next := zoom * v
					if arg == "" {
						next = 1
					} else if e == 'Z' {
						next = v
					}
					if math.IsInf(next, 0) || math.IsNaN(next) {
						r.warn("text zoom overflow in %q", s)
						continue
					}
					flush()
					zoom = next
				case 'v':
					next := baseline
					if arg != "" {
						next = shift + v*size*zoom
					}
					if next != shift {
						flush()
						shift = next
					}
				case 'V':
					flush()
					delta := v * size * zoom
					if arg == "" {
						delta = -baseline
					}
					baseline += delta
					shift += delta
				case 'h':
					flush()
					x += v * size * zoom
				}
			default:
				r.warn("text escape \\%c is not supported; ignored in %q", e, s)
			}
		default:
			r.warn("text escape \\%c is not supported; ignored in %q", e, s)
		}
	}
	flush()
	ts.width = x
	for _, rn := range ts.runs {
		pos, thickness := decorationMetrics(rn.font)
		for _, line := range []struct {
			on     bool
			offset float64
		}{{rn.under, pos}, {rn.over, -pos}} {
			if line.on {
				y := rn.y + line.offset*rn.size
				x1 := rn.x + r.metrics(rn.font).Width(rn.str)*rn.size
				half := math.Abs(thickness * rn.size / 2)
				ts.ink = ts.ink.Union(afm.Box{LLX: min(rn.x, x1), LLY: y - half, URX: max(rn.x, x1), URY: y + half})
			}
		}
	}
	// gracebat's box of a string starts at the pen origin, not at the first
	// ink (a leading "1" has a wide left margin); the rest is ink
	if !ts.ink.Empty() {
		ts.ink.LLX = min(ts.ink.LLX, 0)
	}
	ts.ink = r.textBoxGrid(ts, ts.ink)
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
	pageW, pageH := draw.PageSize(r.proj.PageWidth, r.proj.PageHeight)
	origins := ts.origins(deg, 72.0/300/math.Min(pageW, pageH))
	for i, rn := range ts.runs {
		dx, dy := origins[i].X, origins[i].Y
		pos, thickness := decorationMetrics(rn.font)
		for _, line := range []struct {
			on     bool
			offset float64
		}{{rn.under, pos}, {rn.over, -pos}} {
			if !line.on {
				continue
			}
			x0, y0 := rotate(0, line.offset*rn.size, deg)
			x1, y1 := rotate(r.metrics(rn.font).Width(rn.str)*rn.size, line.offset*rn.size, deg)
			r.d.Paths = append(r.d.Paths, draw.Path{ID: r.id,
				Segments: []draw.Segment{{Points: []draw.Point{{X: ox + dx + x0, Y: oy + dy + y0}, {X: ox + dx + x1, Y: oy + dy + y1}}}},
				Style:    draw.Style{Color: rgb, Width: math.Abs(thickness * rn.size)},
			})
			r.d.Order = append(r.d.Order, 'p')
		}
		// Grace advances a whitespace-only run without a show operation.
		if strings.Trim(rn.str, " \t") == "" {
			continue
		}
		r.d.Texts = append(r.d.Texts, draw.Text{
			ID:     r.id,
			At:     draw.Point{X: ox + dx, Y: oy + dy},
			Matrix: [4]float64{rn.size * c, rn.size * s, -rn.size * s, rn.size * c},
			Font:   rn.font,
			Color:  rgb,
			Str:    rn.str,
		})
		r.d.Order = append(r.d.Order, 't')
	}
}

// Grace advances each text run on its 300 dpi typesetting grid.
// Decorations expose those rounded positions as paths.
// Keep the established continuous positions for undecorated strings.
func (ts typeset) origins(deg, grid float64) []draw.Point {
	decorated := false
	for _, rn := range ts.runs {
		decorated = decorated || rn.under || rn.over
	}
	out := make([]draw.Point, len(ts.runs))
	for i, rn := range ts.runs {
		x, y := rotate(rn.x, rn.y, deg)
		if decorated && i > 0 {
			px, py := rotate(ts.runs[i-1].x, ts.runs[i-1].y, deg)
			x = out[i-1].X + math.Round((x-px)/grid)*grid
			y = out[i-1].Y + math.Round((y-py)/grid)*grid
		}
		out[i] = draw.Point{X: x, Y: y}
	}
	return out
}

// Underline position and thickness, in em, measured from Grace 5.1.25's
// EPS TL operands for each standard font (URW base-35). Overlines use the
// opposite offset. This is also used without locally installed font files.
func decorationMetrics(font string) (float64, float64) {
	switch font {
	case "Times-Roman":
		return -0.107, 0.050
	case "Times-Italic":
		return -0.105, 0.050
	case "Times-Bold", "Times-BoldItalic":
		return -0.109, 0.069
	case "Helvetica":
		return -0.151, 0.050
	case "Helvetica-Oblique":
		return -0.151, 0.050
	case "Helvetica-Bold":
		return -0.155, 0.069
	case "Helvetica-BoldOblique":
		return -0.111, 0.069
	case "Courier", "Courier-Oblique":
		return -0.091, 0.051
	case "Courier-Bold", "Courier-BoldOblique":
		return -0.088, 0.110
	case "Symbol":
		return -0.229, 0.046
	case "ZapfDingbats":
		return -0.072, 0.036
	default:
		return -0.151, 0.050
	}
}
