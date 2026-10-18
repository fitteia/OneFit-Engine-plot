package render

import (
	"encoding/hex"
	"math"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/fontfile"
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
	matrix      textMatrix // dimensionless, including inline zoom
	color       int        // -1 keeps the label colour
	kern        []float64  // pair adjustments, in ems; nil disables kerning
}

// typeset is a laid-out string: its runs, the box labels are aligned by
// (ink, but starting at the pen origin), and the advance of the whole.
type typeset struct {
	runs     []run
	ink      afm.Box
	width    float64
	advanceY float64 // inline rotations can advance the pen vertically
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

// textMatrix follows PostScript's column order: a b c d.
type textMatrix [4]float64

var identityText = textMatrix{1, 0, 0, 1}

func (m textMatrix) point(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y, m[1]*x + m[3]*y
}
func (m textMatrix) mul(n textMatrix) textMatrix {
	return textMatrix{m[0]*n[0] + m[2]*n[1], m[1]*n[0] + m[3]*n[1], m[0]*n[2] + m[2]*n[3], m[1]*n[2] + m[3]*n[3]}
}
func (m textMatrix) scale(v float64) textMatrix {
	for i := range m {
		m[i] *= v
	}
	return m
}

// Grace's absolute zoom is the height perpendicular to the baseline.
func (m textMatrix) height() float64 {
	w := math.Hypot(m[0], m[1])
	if w == 0 {
		return math.Hypot(m[2], m[3])
	}
	return math.Abs((m[0]/w)*m[3] - (m[1]/w)*m[2])
}
func (m textMatrix) finite() bool {
	for _, v := range m {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}
func (rn run) point(x, y float64) (float64, float64) {
	dx, dy := rn.matrix.point(x*rn.size, y*rn.size)
	return rn.x + dx, rn.y + dy
}
func (rn run) advance(m *afm.Font) float64 {
	w := m.Width(rn.str)
	for _, k := range rn.kern {
		w += k
	}
	return w
}

// layout typesets Grace controls using independent pen, baseline and style state.
func (r *renderer) layout(s string, font int, size float64) typeset {
	var ts typeset
	base := r.fontName(font)
	cur := base
	matrix := identityText
	shift, baseline, x, y := 0.0, 0.0, 0.0, 0.0
	under, over, upper, kern := false, false, false, false
	reverse, backward := false, false
	color := -1
	type mark struct{ x, y float64 }
	marks := make(map[int]mark)
	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		str := buf.String()
		buf.Reset()
		if reverse {
			b := []byte(str)
			for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
				b[i], b[j] = b[j], b[i]
			}
			str = string(b)
		}
		rn := run{str: str, font: cur, size: size, matrix: matrix, color: color, under: under, over: over}
		m := r.metrics(cur)
		rn.font = m.Name
		for i := 0; i < len(str); i++ {
			if m.Missing(str[i]) {
				r.warn("font %q has no glyph for byte 0x%02x; missing-character glyph used", m.Name, str[i])
			}
		}
		if kern && len(str) > 1 {
			rn.kern = make([]float64, len(str)-1)
			for i := range rn.kern {
				rn.kern[i] = glyph.Kern(cur, m, str[i], str[i+1])
			}
		}
		dx, dy := matrix.point(rn.advance(m)*size, 0)
		if backward {
			x -= dx
			y -= dy
		}
		h := matrix.height()
		sx, sy := 0.0, 0.0
		if h > 0 {
			sx, sy = matrix.point(0, shift/h)
		}
		rn.x, rn.y = x+sx, y+sy
		ts.runs = append(ts.runs, rn)
		ts.ink = ts.ink.Union(r.runGlyphBox(rn, 0))
		if !backward {
			x += dx
			y += dy
		}
	}
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
	transform := func(next textMatrix) {
		if !next.finite() || math.IsNaN(next.height()) || math.IsInf(next.height(), 0) {
			r.warn("text transformation overflow in %q", s)
			return
		}
		if next.height() == 0 {
			r.warn("singular text transformation in %q; ignored", s)
			return
		}
		if next != matrix {
			flush()
			matrix = next
		}
	}
	normalZoom := func(v float64) textMatrix {
		h := matrix.height()
		if h == 0 {
			return identityText.scale(v)
		}
		return matrix.scale(v / h)
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
		case e == 's' || e == 'S':
			flush()
			v := -0.4
			if e == 'S' {
				v = 0.6
			}
			shift += v * size * matrix.height()
			matrix = matrix.scale(script)
		case e == 'N':
			flush()
			matrix = normalZoom(1)
			shift = baseline
		case e == '-' || e == '+':
			v := smaller
			if e == '+' {
				v = larger
			}
			transform(matrix.scale(v))
		case e == 'q' || e == 'Q':
			v := 0.25
			if e == 'Q' {
				v = -v
			}
			transform(textMatrix{1, 0, v, 1}.mul(matrix))
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
			x, y = 0, 0
			baseline -= size * matrix.height()
			shift = baseline
		case e == 'F' && i+1 < len(s) && strings.ContainsRune("kKlL", rune(s[i+1])):
			flush()
			i++
			switch s[i] {
			case 'k':
				kern = true
			case 'K':
				kern = false
			}
			// Core-14/URW Latin-1 fonts have no encodable ligature substitutions.
			// Fl/FL are accepted state boundaries, as in Grace with these fonts.
		case e == 'd' && i+1 < len(s) && strings.ContainsRune("lrLR", rune(s[i+1])):
			flush()
			i++
			switch s[i] {
			case 'l':
				reverse = false
			case 'r':
				reverse = true
			case 'L':
				backward = false
			case 'R':
				backward = true
			}
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
			case 'R':
				flush()
				color = -1
				if arg != "" {
					if n, err := strconv.Atoi(arg); err == nil {
						color = n
					} else {
						found := false
						for n, c := range r.proj.Colors {
							if c.Name == arg {
								color = n
								found = true
								break
							}
						}
						if !found {
							r.warn("unknown text colour %q; original colour used", arg)
						}
					}
				}
			case '#':
				b, err := hex.DecodeString(arg)
				if err != nil {
					r.warn("invalid hexadecimal text escape in %q", s)
				} else {
					buf.Write(b)
				}
			case 'm', 'M':
				n, err := strconv.Atoi(arg)
				if err != nil || n < 0 || n > 31 {
					r.warn("invalid text mark %q; expected 0 to 31", arg)
					continue
				}
				flush()
				if e == 'm' {
					marks[n] = mark{x, y}
				} else {
					p := marks[n]
					x, y, shift = p.x, p.y, baseline
				}
			case 't', 'T':
				next := identityText
				if arg != "" {
					fields := strings.Fields(arg)
					valid := len(fields) == 4
					var v [4]float64
					if valid {
						for j := range v {
							var err error
							v[j], err = strconv.ParseFloat(fields[j], 64)
							valid = valid && err == nil && !math.IsNaN(v[j]) && !math.IsInf(v[j], 0)
						}
					}
					if !valid {
						r.warn("invalid text matrix %q", arg)
						continue
					}
					next = textMatrix{v[0], v[2], v[1], v[3]}
					if e == 't' {
						next = next.mul(matrix)
					}
				}
				transform(next)
			case 'z', 'Z', 'v', 'V', 'h', 'r', 'l':
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
					next := matrix.scale(v)
					if arg == "" {
						next = normalZoom(1)
					} else if e == 'Z' {
						next = normalZoom(v)
					}
					transform(next)
				case 'r':
					sn, cs := math.Sincos(v * math.Pi / 180)
					transform(textMatrix{cs, sn, -sn, cs}.mul(matrix))
				case 'l':
					transform(textMatrix{1, 0, v, 1}.mul(matrix))
				case 'v':
					next := baseline
					if arg != "" {
						next = shift + v*size*matrix.height()
					}
					if math.IsNaN(next) || math.IsInf(next, 0) {
						r.warn("text shift overflow in %q", s)
						continue
					}
					if next != shift {
						flush()
						shift = next
					}
				case 'V':
					delta := v * size * matrix.height()
					if arg == "" {
						delta = -baseline
					}
					if math.IsNaN(delta) || math.IsInf(delta, 0) {
						r.warn("text baseline overflow in %q", s)
						continue
					}
					flush()
					baseline += delta
					shift += delta
				case 'h':
					dx, dy := matrix.point(v*size, 0)
					if math.IsNaN(dx) || math.IsInf(dx, 0) || math.IsNaN(dy) || math.IsInf(dy, 0) {
						r.warn("text shift overflow in %q", s)
						continue
					}
					flush()
					x += dx
					y += dy
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
	ts.advanceY = y
	for _, rn := range ts.runs {
		pos, thickness := decorationMetrics(rn.font)
		if rn.under {
			ts.ink = ts.ink.Union(r.decorationBox(rn, pos, thickness))
		}
		if rn.over {
			ts.ink = ts.ink.Union(r.decorationBox(rn, r.overlineOffset(rn), thickness))
		}
	}
	if !ts.ink.Empty() {
		ts.ink.LLX = min(ts.ink.LLX, 0)
	}
	ts.ink = r.textBoxGrid(ts, ts.ink)
	return ts
}

// runGlyphBox measures transformed glyphs, including explicit pair kerning.
func (r *renderer) runGlyphBox(rn run, deg float64) afm.Box {
	m := r.metrics(rn.font)
	if rn.matrix == identityText && deg == 0 && rn.kern == nil {
		b := glyph.Ink(rn.font, m, rn.str)
		if b.Empty() {
			return b
		}
		return afm.Box{LLX: rn.x + b.LLX*rn.size, LLY: rn.y + b.LLY*rn.size, URX: rn.x + b.URX*rn.size, URY: rn.y + b.URY*rn.size}
	}
	var b afm.Box
	set := false
	add := func(x, y float64) {
		x, y = rn.point(x, y)
		x, y = rotate(x, y, deg)
		if !set {
			b = afm.Box{LLX: x, LLY: y, URX: x, URY: y}
			set = true
		} else {
			b.LLX = min(b.LLX, x)
			b.LLY = min(b.LLY, y)
			b.URX = max(b.URX, x)
			b.URY = max(b.URY, y)
		}
	}
	pen := 0.0
	for i := 0; i < len(rn.str); i++ {
		segs, _ := glyph.Outline(rn.font, rn.str[i])
		var cur [2]float64
		for _, sg := range segs {
			switch sg.Op {
			case glyph.MoveTo, glyph.LineTo:
				cur = sg.Pts[0]
				add(pen+cur[0], cur[1])
			default:
				ctl := append([][2]float64{cur}, sg.Pts...)
				for k := 1; k <= 16; k++ {
					pt := bezier(ctl, float64(k)/16)
					add(pen+pt[0], pt[1])
				}
				cur = sg.Pts[len(sg.Pts)-1]
			}
		}
		if len(segs) == 0 {
			ink := m.Ink(rn.str[i : i+1])
			if !ink.Empty() {
				for _, pt := range [][2]float64{{ink.LLX, ink.LLY}, {ink.URX, ink.LLY}, {ink.LLX, ink.URY}, {ink.URX, ink.URY}} {
					add(pen+pt[0], pt[1])
				}
			}
		}
		pen += m.Width(rn.str[i : i+1])
		if i < len(rn.kern) {
			pen += rn.kern[i]
		}
	}
	return b
}

// overlineOffset puts an overline above the run's actual glyphs. Grace's EPS
// emits it only a small distance above the baseline, through most capital
// letters; plot-go intentionally fixes that behavior for Figures and exports.
func (r *renderer) overlineOffset(rn run) float64 {
	m := r.metrics(rn.font)
	ink := glyph.Ink(rn.font, m, rn.str)
	if ink.Empty() {
		ink = glyph.Ink(rn.font, m, "H")
	}
	_, thickness := decorationMetrics(rn.font)
	h := rn.matrix.height()
	strokeHalf := 0.0
	if h > 0 {
		strokeHalf = math.Abs(thickness) * math.Hypot(rn.matrix[0], rn.matrix[1]) / (2 * h)
	}
	return ink.URY + 0.04 + strokeHalf
}

func (r *renderer) decorationBox(rn run, offset, thickness float64) afm.Box {
	x0, y0 := rn.point(0, offset)
	x1, y1 := rn.point(rn.advance(r.metrics(rn.font)), offset)
	half := math.Abs(thickness * rn.size * math.Hypot(rn.matrix[0], rn.matrix[1]) / 2)
	return afm.Box{LLX: min(x0, x1) - half, LLY: min(y0, y1) - half, URX: max(x0, x1) + half, URY: max(y0, y1) + half}
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
	sn, cs := math.Sincos(deg * math.Pi / 180)
	if deg == 0 {
		sn, cs = 0, 1
	}
	rotation := textMatrix{cs, sn, -sn, cs}
	pageW, pageH := draw.PageSize(r.proj.PageWidth, r.proj.PageHeight)
	origins := ts.origins(deg, 72.0/300/math.Min(pageW, pageH))
	for i, rn := range ts.runs {
		dx, dy := origins[i].X, origins[i].Y
		rgb := r.color(color)
		if rn.color >= 0 {
			rgb = r.color(rn.color)
		}
		mat := rotation.mul(rn.matrix).scale(rn.size)
		pos, thickness := decorationMetrics(rn.font)
		for _, line := range []struct {
			on     bool
			offset float64
		}{{rn.under, pos}, {rn.over, r.overlineOffset(rn)}} {
			if !line.on {
				continue
			}
			x0, y0 := mat.point(0, line.offset)
			x1, y1 := mat.point(rn.advance(r.metrics(rn.font)), line.offset)
			r.d.Paths = append(r.d.Paths, draw.Path{ID: r.id, Segments: []draw.Segment{{Points: []draw.Point{{X: ox + dx + x0, Y: oy + dy + y0}, {X: ox + dx + x1, Y: oy + dy + y1}}}}, Style: draw.Style{Color: rgb, Width: math.Abs(thickness) * math.Hypot(mat[0], mat[1])}})
			r.d.Order = append(r.d.Order, 'p')
		}
		emit := func(str string, pen float64) {
			if strings.Trim(str, " \t") == "" {
				return
			}
			x, y := mat.point(pen, 0)
			r.d.Texts = append(r.d.Texts, draw.Text{ID: r.id, At: draw.Point{X: ox + dx + x, Y: oy + dy + y}, Matrix: [4]float64(mat), Font: rn.font, Color: rgb, Str: str})
			r.d.Order = append(r.d.Order, 't')
		}
		if rn.kern == nil {
			emit(rn.str, 0)
		} else {
			pen := 0.0
			m := r.metrics(rn.font)
			for j := 0; j < len(rn.str); j++ {
				emit(rn.str[j:j+1], pen)
				pen += m.Width(rn.str[j : j+1])
				if j < len(rn.kern) {
					pen += rn.kern[j]
				}
			}
		}
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
// EPS TL operands for each standard font (URW base-35). Overlines use
// glyph-aware placement instead. Also used without installed font files.
func decorationMetrics(font string) (float64, float64) {
	if !afm.Standard(font) {
		if f, err := fontfile.Get(font); err == nil {
			return f.UnderlinePosition, f.UnderlineThickness
		}
	}
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
