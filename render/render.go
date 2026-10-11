// Package render draws a Grace project (agr.Project) the way gracebat
// does, into a draw.Drawing. The rules come from measuring gracebat's
// output (docs/grace-behaviour.md), never from Grace's source.
package render

import (
	"fmt"
	"math"
	"strconv"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// Units, in viewport units (docs/grace-behaviour.md).
const (
	lineUnit    = 0.0015 // per unit of linewidth
	charUnit    = 0.028  // font size per unit of char size
	tickUnit    = 0.02   // tick length per unit of tick size
	symbolUnit  = 0.01   // circle radius per unit of symbol size
	labelOffset = 0.01   // tick labels from the axis, axis labels from them
)

// dashes are the line styles' dash patterns, in multiples of the line
// width.
var dashes = map[int][]float64{
	1: nil,
	2: {1, 3},
	3: {5, 3},
	4: {7, 3},
	5: {1, 3, 5, 3},
	6: {1, 3, 7, 3},
	7: {1, 3, 5, 3, 1, 3},
	8: {5, 3, 1, 3, 5, 3},
}

type renderer struct {
	proj     *agr.Project
	d        *draw.Drawing
	warnings []string
	// id names what is being drawn; every path and text is stamped with
	// it. The names, for an editor to map a drawn element to what it
	// edits (agr's names: graph g0, set s3, string by its index in the
	// file):
	//
	//	page                      the page background
	//	g0.s3.line, g0.s3.symbols a set's line and its symbols
	//	g0.x.tick, g0.y.tick      an axis's tick marks (major and minor)
	//	g0.x.ticklabel            an axis's tick labels
	//	g0.x.label                an axis's label
	//	g0.frame                  the frame
	//	string.2                  the third "@with string" in the file
	id string
}

func (r *renderer) warn(format string, a ...any) {
	r.warnings = append(r.warnings, fmt.Sprintf(format, a...))
}

func (r *renderer) color(n int) [3]float64 {
	c, ok := r.proj.Colors[n]
	if !ok {
		if n != 0 && n != 1 {
			r.warn("color %d is not in the color map; black used", n)
		}
		if n == 0 {
			return [3]float64{1, 1, 1}
		}
		return [3]float64{}
	}
	return [3]float64{float64(c.R) / 255, float64(c.G) / 255, float64(c.B) / 255}
}

func (r *renderer) style(color int, width float64, linestyle int) (draw.Style, bool) {
	if linestyle == 0 {
		return draw.Style{}, false
	}
	pat, ok := dashes[linestyle]
	if !ok {
		r.warn("line style %d is not supported; solid used", linestyle)
	}
	w := width * lineUnit
	st := draw.Style{Color: r.color(color), Width: w}
	for _, p := range pat {
		st.Dash = append(st.Dash, p*w)
	}
	return st, true
}

func (r *renderer) path(st draw.Style, fill bool, segs ...draw.Segment) {
	r.d.Paths = append(r.d.Paths, draw.Path{ID: r.id, Segments: segs, Fill: fill, Style: st})
	r.d.Order = append(r.d.Order, 'p')
}

func line(pts ...draw.Point) draw.Segment { return draw.Segment{Points: pts} }

// Render draws the project. The warnings name what it could not draw as
// gracebat would.
func Render(p *agr.Project) (*draw.Drawing, []string) {
	r := &renderer{proj: p, d: &draw.Drawing{}}
	w, h := draw.PageSize(p.PageWidth, p.PageHeight)
	if w != p.PageWidth || h != p.PageHeight {
		r.warn("missing or invalid page size; %gx%g used", w, h)
	}
	unit := math.Min(w, h)
	r.d.Width, r.d.Height = w/unit, h/unit
	if p.PageFill {
		r.id = "page"
		r.path(draw.Style{Color: r.color(p.BackgroundColor)}, true, draw.Segment{
			Points: []draw.Point{{X: 0, Y: 0}, {X: 0, Y: r.d.Height}, {X: r.d.Width, Y: r.d.Height}, {X: r.d.Width, Y: 0}},
			Closed: true,
		})
	}
	for _, g := range p.Graphs {
		if g.On && !g.Hidden {
			r.graph(g)
		}
	}
	// strings in world coordinates were drawn with their graph; the ones
	// placed on the page come last (gracebat's order)
	for i, s := range p.Strings {
		if s.LocType != "world" {
			r.id = fmt.Sprintf("string.%d", i)
			r.text(s)
		}
	}
	if p.Timestamp.On {
		r.warn("the timestamp is not drawn")
	}
	return r.d, r.warnings
}

// xform maps a graph's world coordinates to viewport coordinates.
type xform struct {
	g          *agr.Graph
	logX, logY bool
}

func newXform(g *agr.Graph) xform {
	return xform{g: g, logX: g.X.Scale == "Logarithmic", logY: g.Y.Scale == "Logarithmic"}
}

func scale(v, lo, hi, vlo, vhi float64, log bool) float64 {
	if log {
		v, lo, hi = math.Log10(v), math.Log10(lo), math.Log10(hi)
	}
	return vlo + (v-lo)/(hi-lo)*(vhi-vlo)
}

func (t xform) x(v float64) float64 {
	w, vw := t.g.World, t.g.View
	return scale(v, w.XMin, w.XMax, vw.XMin, vw.XMax, t.logX)
}

func (t xform) y(v float64) float64 {
	w, vw := t.g.World, t.g.View
	return scale(v, w.YMin, w.YMax, vw.YMin, vw.YMax, t.logY)
}

// ok reports whether a point can be placed at all (log axes need > 0).
func (t xform) ok(x, y float64) bool {
	return (!t.logX || x > 0) && (!t.logY || y > 0)
}

func (r *renderer) graph(g *agr.Graph) {
	w, v := g.World, g.View
	if !finite(w.XMin, w.XMax, w.YMin, w.YMax, v.XMin, v.XMax, v.YMin, v.YMax) {
		r.warn("graph %d has a world or view value that is not a finite number; not drawn", g.ID)
		return
	}
	if g.World.XMin == g.World.XMax || g.World.YMin == g.World.YMax {
		r.warn("graph %d has an empty world window; not drawn", g.ID)
		return
	}
	t := newXform(g)
	r.grid(t, true)
	r.grid(t, false)
	for _, s := range g.Sets {
		if !s.Hidden {
			r.set(t, s)
		}
	}
	r.axis(t, true)
	r.axis(t, false)
	r.frame(g)
	for i, s := range r.proj.Strings {
		if s.LocType == "world" && s.Graph == g.ID {
			r.id = fmt.Sprintf("string.%d", i)
			r.text(s)
		}
	}
	if g.Title.Text != "" || g.Subtitle.Text != "" {
		r.warn("graph titles are not drawn")
	}
	if g.Legend.On && g.Legend.BoxPattern != 0 {
		r.warn("the legend box is not drawn")
	}
	for _, s := range g.Sets {
		if g.Legend.On && s.Legend != "" {
			r.warn("legend entries are not drawn")
			break
		}
	}
}

func (r *renderer) set(t xform, s *agr.Set) {
	if s.Line.Type != 0 {
		if s.Line.Type < 1 || s.Line.Type > 5 {
			r.warn("set %d: line type %d is drawn straight", s.ID, s.Line.Type)
		}
		if st, ok := r.style(s.Line.Color, s.Line.LineWidth, s.Line.LineStyle); ok {
			var pts []draw.Point
			var breaks []bool // a point that may not be joined to the one after
			for i, row := range s.Data {
				if !t.ok(row[0], row[1]) {
					if len(pts) > 0 {
						breaks[len(breaks)-1] = true
					}
					continue
				}
				p := draw.Point{X: t.x(row[0]), Y: t.y(row[1])}
				// stairs: a corner between each point and the one before,
				// vertical first (left stairs) or horizontal first (right)
				if n := len(pts); n > 0 && !breaks[n-1] && (s.Line.Type == 2 || s.Line.Type == 3) {
					c := draw.Point{X: pts[n-1].X, Y: p.Y}
					if s.Line.Type == 3 {
						c = draw.Point{X: p.X, Y: pts[n-1].Y}
					}
					pts = append(pts, c)
					breaks = append(breaks, false)
				}
				pts = append(pts, p)
				// segments join the points in twos, 3-segments in threes,
				// counted by their place in the data
				breaks = append(breaks, (s.Line.Type == 4 && i%2 == 1) || (s.Line.Type == 5 && i%3 == 2))
			}
			if segs := clipPolyline(pts, breaks, t.g.View); len(segs) > 0 {
				r.id = fmt.Sprintf("g%d.s%d.line", t.g.ID, s.ID)
				// gracebat strokes each visible piece on its own: each
				// segment of types 4 and 5, each stretch between where a
				// line leaves the viewport and comes back
				for _, sg := range segs {
					r.path(st, false, sg)
				}
			}
		}
	}
	if s.ErrorBar.On && s.Type == "xydy" {
		r.errorBars(t, s)
	}
	if s.Symbol.Type != 0 {
		r.symbols(t, s)
	}
}

// errorBars draws an xydy set's error bars (docs/grace-behaviour.md), after
// its line and before its symbols: for each point inside the world window
// with an error (its size; the sign does not matter, 0 is none), the riser
// from the point to y+dy, then the cap across its end, then the same down
// to y-dy - only up for place "normal", only down for "opposite". A riser
// whose end is out of the viewport has no cap: it runs to the frame, or
// with riser clip on, for the clip length and ends in an arrowhead. All of
// it is cut at the frame.
func (r *renderer) errorBars(t xform, s *agr.Set) {
	e := s.ErrorBar
	if e.Pattern == 0 {
		return
	}
	var dirs []float64
	switch e.Place {
	case "normal":
		dirs = []float64{1}
	case "opposite":
		dirs = []float64{-1}
	default:
		if e.Place != "both" && e.Place != "" {
			r.warn("set %d: error bar place %q is drawn as both", s.ID, e.Place)
		}
		dirs = []float64{1, -1}
	}
	riser, hasRiser := r.style(e.Color, e.RiserWidth, e.RiserStyle)
	capSt, hasCap := r.style(e.Color, e.LineWidth, e.LineStyle)
	arrow := draw.Style{Color: r.color(e.Color), Width: e.LineWidth * lineUnit} // always solid
	half := e.Size * symbolUnit                                                 // the cap's half length; also the arrowhead's half width
	v, w := t.g.View, t.g.World
	r.id = fmt.Sprintf("g%d.s%d.errorbars", t.g.ID, s.ID)
	for _, row := range s.Data {
		if len(row) < 3 {
			continue
		}
		x, y, dy := row[0], row[1], math.Abs(row[2])
		if !t.ok(x, y) || x < min(w.XMin, w.XMax) || x > max(w.XMin, w.XMax) || y < min(w.YMin, w.YMax) || y > max(w.YMin, w.YMax) {
			continue
		}
		if dy == 0 || math.IsNaN(dy) {
			continue
		}
		px, py := t.x(x), t.y(y)
		for _, d := range dirs {
			end := y + d*dy
			// up the page when the error's side and the world's y run the
			// same way
			up := (d > 0) == (w.YMax >= w.YMin)
			sign := -1.0
			if up {
				sign = 1
			}
			// the end in the viewport; out of it when an axis cannot place
			// it (a log axis at or below 0)
			ey := math.Inf(-1)
			if up {
				ey = math.Inf(1)
			}
			if t.ok(x, end) && !math.IsInf(end, 0) {
				ey = t.y(end)
			}
			if ey >= v.YMin && ey <= v.YMax {
				if hasRiser {
					r.clipped(riser, v, draw.Point{X: px, Y: py}, draw.Point{X: px, Y: ey})
				}
				if hasCap {
					// the upper cap is drawn left to right, the lower right to left
					a, b := px-half, px+half
					if !up {
						a, b = b, a
					}
					r.clipped(capSt, v, draw.Point{X: a, Y: ey}, draw.Point{X: b, Y: ey})
				}
				continue
			}
			if !e.RiserClip {
				// to the frame
				if hasRiser {
					r.clipped(riser, v, draw.Point{X: px, Y: py}, draw.Point{X: px, Y: py + sign*2*(v.YMax-v.YMin)})
				}
				continue
			}
			// riser clip: the clip length, then an arrowhead (up: from the
			// right, through the tip, to the left; down the other way),
			// both cut at the frame like everything else
			tip := py + sign*e.RiserClipSize
			if hasRiser {
				r.clipped(riser, v, draw.Point{X: px, Y: py}, draw.Point{X: px, Y: tip})
			}
			a, b := px+half, px-half
			if !up {
				a, b = b, a
			}
			back := tip - sign*2*half
			r.clipped(arrow, v, draw.Point{X: a, Y: back}, draw.Point{X: px, Y: tip}, draw.Point{X: b, Y: back})
		}
	}
}

// clipped strokes a polyline cut to the viewport, each visible piece a
// path of its own (as gracebat strokes them).
func (r *renderer) clipped(st draw.Style, v agr.Rect, pts ...draw.Point) {
	for _, sg := range clipPolyline(pts, make([]bool, len(pts)), v) {
		r.path(st, false, sg)
	}
}

// symbolShapes are the outlines of symbols 2-10 around the point, per unit
// of symbol size (docs/grace-behaviour.md); nil marks the open ones (plus,
// x, star), drawn as strokes and never filled.
var symbolShapes = func() map[int][][]draw.Point {
	const tri, half, d = 0.0115470054, 0.0057735027, 0.0070710678 // 0.02/sqrt(3), its half, 0.01/sqrt(2)
	return map[int][][]draw.Point{
		2:  {{{X: -0.0085, Y: -0.0085}, {X: -0.0085, Y: 0.0085}, {X: 0.0085, Y: 0.0085}, {X: 0.0085, Y: -0.0085}}},
		3:  {{{X: 0, Y: 0.01}, {X: -0.01, Y: 0}, {X: 0, Y: -0.01}, {X: 0.01, Y: 0}}},
		4:  {{{X: 0, Y: tri}, {X: -0.01, Y: -half}, {X: 0.01, Y: -half}}},
		5:  {{{X: -tri, Y: 0}, {X: half, Y: -0.01}, {X: half, Y: 0.01}}},
		6:  {{{X: -0.01, Y: half}, {X: 0, Y: -tri}, {X: 0.01, Y: half}}},
		7:  {{{X: -half, Y: 0.01}, {X: -half, Y: -0.01}, {X: tri, Y: 0}}},
		8:  {{{X: -0.01}, {X: 0.01}}, {{Y: -0.01}, {Y: 0.01}}},
		9:  {{{X: -d, Y: -d}, {X: d, Y: d}}, {{X: -d, Y: d}, {X: d, Y: -d}}},
		10: {{{X: -0.01}, {X: 0.01}}, {{Y: -0.01}, {Y: 0.01}}, {{X: -d, Y: -d}, {X: d, Y: d}}, {{X: -d, Y: d}, {X: d, Y: -d}}},
	}
}()

// symbols draws a set's symbols at its points inside the world window:
// the fill first (fill color; any fill pattern drawn solid), then the
// outline (pattern 0: none), or for symbol 11 the set's character, centred
// on the point by its box as labels are.
func (r *renderer) symbols(t xform, s *agr.Set) {
	sy := s.Symbol
	if sy.Type < 1 || sy.Type > 11 {
		r.warn("set %d: symbol %d is drawn as a circle", s.ID, sy.Type)
		sy.Type = 1
	}
	open := sy.Type >= 8 && sy.Type <= 10
	st, stroke := r.style(sy.Color, sy.LineWidth, sy.LineStyle)
	stroke = stroke && sy.Pattern != 0
	fill := sy.FillPattern != 0 && !open && sy.Type != 11
	if fill && sy.FillPattern != 1 {
		r.warn("set %d: symbol fill pattern %d is drawn solid", s.ID, sy.FillPattern)
	}
	var fst draw.Style
	if fill {
		fst = draw.Style{Color: r.color(sy.FillColor)}
	}
	var ch typeset
	if sy.Type == 11 {
		if sy.Char <= 32 || sy.Char > 255 || sy.Char == '\\' {
			r.warn("set %d: symbol character %d cannot be drawn", s.ID, sy.Char)
			return
		}
		ch = r.layout(string(rune(sy.Char)), sy.CharFont, sy.Size*charUnit)
	}
	r.id = fmt.Sprintf("g%d.s%d.symbols", t.g.ID, s.ID)
	w := t.g.World
	for _, row := range s.Data {
		x, y := row[0], row[1]
		if !t.ok(x, y) || x < min(w.XMin, w.XMax) || x > max(w.XMin, w.XMax) || y < min(w.YMin, w.YMax) || y > max(w.YMin, w.YMax) {
			continue
		}
		px, py := t.x(x), t.y(y)
		switch sy.Type {
		case 1:
			rad := sy.Size * symbolUnit
			arc := draw.Segment{Arc: &draw.Arc{X: px, Y: py, RX: rad, RY: rad, A1: 0, A2: 360}}
			if fill {
				r.path(fst, true, arc)
			}
			if stroke {
				r.path(st, false, arc)
			}
		case 11:
			if !ch.ink.Empty() {
				r.emit(ch, px-(ch.ink.LLX+ch.ink.URX)/2, py-(ch.ink.LLY+ch.ink.URY)/2, 0, sy.Color)
			}
		default:
			for _, shape := range symbolShapes[sy.Type] {
				pts := make([]draw.Point, len(shape), len(shape)+1)
				for i, q := range shape {
					pts[i] = draw.Point{X: px + q.X*sy.Size, Y: py + q.Y*sy.Size}
				}
				if fill {
					r.path(fst, true, draw.Segment{Points: pts, Closed: true})
				}
				if stroke {
					if !open {
						// gracebat's outline returns to its first point
						pts = append(pts, pts[0])
					}
					r.path(st, false, draw.Segment{Points: pts, Closed: !open})
				}
			}
		}
	}
}

// clipPolyline cuts a polyline to the viewport rectangle: one segment per
// visible stretch, ending exactly where it crosses the frame.
func clipPolyline(pts []draw.Point, breaks []bool, v agr.Rect) []draw.Segment {
	var segs []draw.Segment
	var cur []draw.Point
	end := func() {
		if len(cur) > 1 {
			segs = append(segs, draw.Segment{Points: cur})
		}
		cur = nil
	}
	for i := 1; i < len(pts); i++ {
		if breaks[i-1] {
			end()
			continue
		}
		a, b, ok := clipSegment(pts[i-1], pts[i], v)
		if !ok {
			end()
			continue
		}
		if len(cur) == 0 || cur[len(cur)-1] != a {
			end()
			cur = append(cur, a)
		}
		cur = append(cur, b)
		if b != pts[i] {
			end()
		}
	}
	end()
	return segs
}

// clipSegment is Liang-Barsky clipping of a-b to the rectangle.
func clipSegment(a, b draw.Point, v agr.Rect) (draw.Point, draw.Point, bool) {
	t0, t1 := 0.0, 1.0
	dx, dy := b.X-a.X, b.Y-a.Y
	for _, e := range [][2]float64{
		{-dx, a.X - v.XMin}, {dx, v.XMax - a.X},
		{-dy, a.Y - v.YMin}, {dy, v.YMax - a.Y},
	} {
		p, q := e[0], e[1]
		if p == 0 {
			if q < 0 {
				return a, b, false
			}
			continue
		}
		t := q / p
		if p < 0 {
			if t > t1 {
				return a, b, false
			}
			t0 = max(t0, t)
		} else {
			if t < t0 {
				return a, b, false
			}
			t1 = min(t1, t)
		}
	}
	na, nb := a, b
	if t0 > 0 {
		na = draw.Point{X: a.X + t0*dx, Y: a.Y + t0*dy}
	}
	if t1 < 1 {
		nb = draw.Point{X: a.X + t1*dx, Y: a.Y + t1*dy}
	}
	return na, nb, true
}

func (r *renderer) frame(g *agr.Graph) {
	if g.Frame.Type != 0 {
		r.warn("frame type %d is drawn as a box", g.Frame.Type)
	}
	st, ok := r.style(g.Frame.Color, g.Frame.LineWidth, g.Frame.LineStyle)
	if !ok {
		return
	}
	v := g.View
	r.id = fmt.Sprintf("g%d.frame", g.ID)
	r.path(st, false, draw.Segment{Points: []draw.Point{
		{X: v.XMin, Y: v.YMin}, {X: v.XMin, Y: v.YMax}, {X: v.XMax, Y: v.YMax}, {X: v.XMax, Y: v.YMin}, {X: v.XMin, Y: v.YMin},
	}, Closed: true})
}

// label formats a tick value.
func (r *renderer) label(v float64, tl agr.TickLabels) string {
	var s string
	switch tl.Format {
	case "power":
		if v > 0 {
			e := math.Log10(v)
			if r := math.Round(e); math.Abs(e-r) < 1e-9 {
				s = `10\S` + strconv.Itoa(int(r)) + `\N`
				break
			}
		}
		s = strconv.FormatFloat(v, 'g', -1, 64)
	case "general", "":
		// gracebat's default format; also what it keeps when the file's
		// format is one it rejects
		s = strconv.FormatFloat(v, 'g', max(tl.Prec, 1), 64)
	case "exponential":
		s = fmt.Sprintf("%.*e", max(tl.Prec, 0), v)
	case "decimal":
		s = strconv.FormatFloat(v, 'f', max(tl.Prec, 0), 64)
	default:
		r.warn("tick label format %s is drawn as decimal", tl.Format)
		s = strconv.FormatFloat(v, 'f', max(tl.Prec, 0), 64)
		if s[0] == '-' && strconv.FormatFloat(-v, 'f', max(tl.Prec, 0), 64) == s[1:] && -v == 0 {
			s = s[1:]
		}
	}
	return tl.Prepend + s + tl.Append
}

// grid draws an axis's grid lines (docs/grace-behaviour.md): across the
// viewport at each minor, then each major tick, in that tick mark's color,
// width and style - for an axis that is on, whether or not its ticks are.
func (r *renderer) grid(t xform, isX bool) {
	g := t.g
	ax, lo, hi := &g.Y, g.World.YMin, g.World.YMax
	if isX {
		ax, lo, hi = &g.X, g.World.XMin, g.World.XMax
	}
	if !ax.On || !(ax.Tick.MinorMarks.Grid || ax.Tick.MajorMarks.Grid) {
		return
	}
	tk := makeTicks(*ax, lo, hi)
	v := g.View
	name := map[bool]string{true: "x", false: "y"}[isX]
	for _, k := range []struct {
		vals []float64
		m    agr.TickMarks
		id   string
	}{{tk.minors, ax.Tick.MinorMarks, "minorgrid"}, {tk.majors, ax.Tick.MajorMarks, "grid"}} {
		if !k.m.Grid {
			continue
		}
		st, ok := r.style(k.m.Color, k.m.LineWidth, k.m.LineStyle)
		if !ok {
			continue
		}
		r.id = fmt.Sprintf("g%d.%s.%s", g.ID, name, k.id)
		for _, w := range k.vals {
			if isX {
				x := t.x(w)
				r.path(st, false, line(draw.Point{X: x, Y: v.YMin}, draw.Point{X: x, Y: v.YMax}))
			} else {
				y := t.y(w)
				r.path(st, false, line(draw.Point{X: v.XMin, Y: y}, draw.Point{X: v.XMax, Y: y}))
			}
		}
	}
}

func (r *renderer) axis(t xform, isX bool) {
	g := t.g
	ax := &g.Y
	lo, hi := g.World.YMin, g.World.YMax
	if isX {
		ax = &g.X
		lo, hi = g.World.XMin, g.World.XMax
	}
	if !ax.On {
		return
	}
	if ax.Invert {
		r.warn("inverted axes are drawn not inverted")
	}
	v := g.View
	axisName := map[bool]string{true: "x", false: "y"}[isX]
	part := func(name string) string { return fmt.Sprintf("g%d.%s.%s", g.ID, axisName, name) }
	tk := makeTicks(*ax, lo, hi)
	if tk.auto {
		r.warn("too many ticks on the %s axis; tick spacing chosen automatically", map[bool]string{true: "x", false: "y"}[isX])
	}
	if tk.narrow {
		r.warn("the %s axis's window is too narrow for the precision of its values; no ticks", map[bool]string{true: "x", false: "y"}[isX])
	}
	pos := func(w float64) float64 {
		if isX {
			return t.x(w)
		}
		return t.y(w)
	}
	// out: how far ticks reach outside the frame on the normal side.
	out := 0.0
	if ax.Tick.On {
		majorLen := ax.Tick.MajorMarks.Size * tickUnit
		minorLen := ax.Tick.MinorMarks.Size * tickUnit
		if ax.Tick.Direction == "out" || ax.Tick.Direction == "both" {
			out = max(majorLen, minorLen)
		}
		drawTicks := func(vals []float64, length float64, m agr.TickMarks) {
			st, ok := r.style(m.Color, m.LineWidth, m.LineStyle)
			if !ok {
				return
			}
			for _, w := range vals {
				p := pos(w)
				for _, side := range r.tickSides(ax.Tick.Place) {
					a, b := tickEnds(length, ax.Tick.Direction)
					if side == 1 { // opposite side: mirror
						a, b = -a, -b
					}
					if isX {
						base := v.YMin
						if side == 1 {
							base = v.YMax
						}
						r.path(st, false, line(draw.Point{X: p, Y: base + a}, draw.Point{X: p, Y: base + b}))
					} else {
						base := v.XMin
						if side == 1 {
							base = v.XMax
						}
						r.path(st, false, line(draw.Point{X: base + a, Y: p}, draw.Point{X: base + b, Y: p}))
					}
				}
			}
		}
		r.id = part("tick")
		drawTicks(tk.minors, minorLen, ax.Tick.MinorMarks)
		drawTicks(tk.majors, majorLen, ax.Tick.MajorMarks)
	}

	// tick labels, aligned by their ink: x labels hang below the axis,
	// centred; y labels end left of it, centred vertically.
	edge := math.Inf(1) // the outer ink edge of all tick labels
	if isX {
		edge = v.YMin - out
	} else {
		edge = v.XMin - out
	}
	if ax.TickLabel.On {
		if ax.TickLabel.Place != "" && ax.TickLabel.Place != "normal" {
			r.warn("tick labels are drawn on the normal side only")
		}
		size := ax.TickLabel.CharSize * charUnit
		r.id = part("ticklabel")
		for _, w := range tk.majors {
			ts := r.layout(r.label(w, ax.TickLabel), ax.TickLabel.Font, size)
			if ts.ink.Empty() {
				continue
			}
			p := pos(w)
			var ox, oy float64
			if isX {
				ox = p - (ts.ink.LLX+ts.ink.URX)/2
				oy = v.YMin - out - labelOffset - ts.ink.URY
				edge = min(edge, oy+ts.ink.LLY)
			} else {
				ox = v.XMin - out - labelOffset - ts.ink.URX
				oy = p - (ts.ink.LLY+ts.ink.URY)/2
				edge = min(edge, ox+ts.ink.LLX)
			}
			r.emit(ts, ox, oy, ax.TickLabel.Angle, ax.TickLabel.Color)
		}
	}

	// the axis label, beyond the tick labels
	if ax.Label.Text != "" {
		r.id = part("label")
		size := ax.Label.CharSize * charUnit
		ts := r.layout(ax.Label.Text, ax.Label.Font, size)
		if !ts.ink.Empty() {
			if isX {
				ox := (v.XMin+v.XMax)/2 - (ts.ink.LLX+ts.ink.URX)/2
				oy := edge - labelOffset - ts.ink.URY
				r.emit(ts, ox, oy, 0, ax.Label.Color)
			} else {
				// rotated 90 degrees: the text's ink "bottom" (LLY) faces
				// the axis, its length runs up the page
				ox := edge - labelOffset + ts.ink.LLY
				oy := (v.YMin+v.YMax)/2 - (ts.ink.LLX+ts.ink.URX)/2
				r.emit(ts, ox, oy, 90, ax.Label.Color)
			}
		}
	}
}

// tickSides: 0 the normal side (bottom/left), 1 the opposite.
func (r *renderer) tickSides(place string) []int {
	switch place {
	case "normal", "":
		return []int{0}
	case "opposite":
		return []int{1}
	case "both":
		return []int{0, 1}
	}
	r.warn("tick place %q not supported; both used", place)
	return []int{0, 1}
}

// tickEnds: where a tick starts and ends, measured into the graph from the
// normal-side axis.
func tickEnds(length float64, dir string) (float64, float64) {
	switch dir {
	case "out":
		return 0, -length
	case "both":
		return -length, length
	}
	return 0, length
}

func (r *renderer) text(s *agr.Text) {
	if !s.On || s.Text == "" {
		return
	}
	x, y := s.X, s.Y
	if s.LocType == "world" {
		var g *agr.Graph
		for _, gg := range r.proj.Graphs {
			if gg.ID == s.Graph {
				g = gg
			}
		}
		if g == nil {
			r.warn("string %q is on graph %d, which does not exist", s.Text, s.Graph)
			return
		}
		t := newXform(g)
		if !t.ok(x, y) {
			r.warn("string %q is at a point a log axis cannot show", s.Text)
			return
		}
		x, y = t.x(x), t.y(y)
	}
	if s.Just != 0 {
		r.warn("string justification %d is drawn left justified", s.Just)
	}
	ts := r.layout(s.Text, s.Font, s.CharSize*charUnit)
	r.emit(ts, x, y, s.Rot, s.Color)
}
