// Package render draws a Grace project (agr.Project) the way gracebat
// does, into a draw.Drawing. The rules come from measuring gracebat's
// output (docs/grace-behaviour.md), never from Grace's source.
package render

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
)

// Units, in viewport units (docs/grace-behaviour.md).
const (
	lineUnit     = 0.0015 // per unit of linewidth
	minLineWidth = 0.0004 // the thinnest line gracebat draws
	charUnit     = 0.028  // font size per unit of char size
	tickUnit     = 0.02   // tick length per unit of tick size
	symbolUnit   = 0.01   // circle radius per unit of symbol size
	legendUnit   = 0.01   // legend gaps and sample length per unit
	titleGap     = 0.06   // a title's ink above the viewport
	staggerUnit  = 0.02   // a staggered tick label row's height per unit of char size
	subtitleGap  = 0.02   // a subtitle's
	labelOffset  = 0.01   // tick labels from the axis, axis labels from them
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

// strokeWidth is a line width in viewport units. gracebat draws nothing
// thinner than minLineWidth, width 0 included, and scales dashes by the
// width it draws (docs/grace-behaviour.md).
func strokeWidth(linewidth float64) float64 {
	return eps4(max(linewidth*lineUnit, minLineWidth))
}

// eps4 rounds a length as gracebat's EPS writes it, to 4 decimals of the
// viewport: OneFit's PDFs are that EPS, so widths and dashes are drawn at
// these lengths - a dash pattern's rounding adds up along a long line.
func eps4(x float64) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.4f", x), 64)
	return v
}

// patterned strokes a line with a Grace pattern (docs/grace-behaviour.md):
// none for 0, solid for 1, the pattern's cells for 2 to 31; gracebat draws
// any other pattern solid ("invalid pattern number").
func (r *renderer) patterned(st draw.Style, ok bool, pattern int) (draw.Style, bool) {
	switch {
	case !ok || pattern == 0:
		return draw.Style{}, false
	case pattern >= 2 && pattern < 32:
		st.Pattern = pattern
	case pattern != 1:
		r.warn("line pattern %d is not one of Grace's; drawn solid, as gracebat does", pattern)
	}
	return st, true
}

func (r *renderer) style(color int, width float64, linestyle int) (draw.Style, bool) {
	if linestyle == 0 {
		return draw.Style{}, false
	}
	pat, ok := dashes[linestyle]
	if !ok {
		r.warn("line style %d is not supported; solid used", linestyle)
	}
	w := max(width*lineUnit, minLineWidth) // dashes scale with it unrounded
	st := draw.Style{Color: r.color(color), Width: eps4(w)}
	for _, p := range pat {
		st.Dash = append(st.Dash, eps4(p*w))
	}
	return st, true
}

func (r *renderer) path(st draw.Style, fill bool, segs ...draw.Segment) {
	r.d.Paths = append(r.d.Paths, draw.Path{ID: r.id, Segments: segs, Fill: fill, Style: st})
	r.d.Order = append(r.d.Order, 'p')
}

// fill fills an area with a Grace pattern (docs/grace-behaviour.md): none
// for 0, solid for 1; 2 to 31 first fill it with the page's background
// color, then paint the pattern's cells in color. gracebat fills any other
// pattern solid ("invalid pattern number").
func (r *renderer) fill(color, pattern int, segs ...draw.Segment) {
	switch {
	case pattern == 0:
		return
	case pattern >= 2 && pattern < 32:
		r.path(draw.Style{Color: r.color(r.proj.BackgroundColor)}, true, segs...)
		r.path(draw.Style{Color: r.color(color), Pattern: pattern}, true, segs...)
		return
	case pattern != 1:
		r.warn("fill pattern %d is not one of Grace's; drawn solid, as gracebat does", pattern)
	}
	r.path(draw.Style{Color: r.color(color)}, true, segs...)
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
	if ts := p.Timestamp; ts.On {
		// gracebat prints the time it draws at, not the file's text
		r.id = "timestamp"
		t := r.layout(Now().Format("Mon Jan _2 15:04:05 2006"), ts.Font, ts.CharSize*charUnit)
		r.emit(t, ts.X, ts.Y, ts.Rot, ts.Color)
	}
	return r.d, r.warnings
}

// Now is the time a timestamp shows: the current one, as gracebat's, or
// $SOURCE_DATE_EPOCH (seconds, UTC) for output that must be reproducible.
func Now() time.Time {
	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(n, 0).UTC()
		}
	}
	return time.Now()
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

// x and y map world values to the viewport; an inverted axis runs from
// its viewport's far side (gracebat mirrors everything on it: data, ticks,
// labels).
func (t xform) x(v float64) float64 {
	w, vw := t.g.World, t.g.View
	if t.g.X.Invert {
		return scale(v, w.XMin, w.XMax, vw.XMax, vw.XMin, t.logX)
	}
	return scale(v, w.XMin, w.XMax, vw.XMin, vw.XMax, t.logX)
}

func (t xform) y(v float64) float64 {
	w, vw := t.g.World, t.g.View
	if t.g.Y.Invert {
		return scale(v, w.YMin, w.YMax, vw.YMax, vw.YMin, t.logY)
	}
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
	r.frameBackground(g)
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
	r.legend(t)
	r.titles(g)
}

// titles draws a graph's title and subtitle (docs/grace-behaviour.md):
// centred on the viewport by their boxes, as labels are, their ink 0.06
// (title) and 0.02 (subtitle) above its top - fixed, neither making room
// for the other.
func (r *renderer) titles(g *agr.Graph) {
	for _, tt := range []struct {
		t   agr.Title
		gap float64
		id  string
	}{{g.Title, titleGap, "title"}, {g.Subtitle, subtitleGap, "subtitle"}} {
		if tt.t.Text == "" {
			continue
		}
		ts := r.layout(tt.t.Text, tt.t.Font, tt.t.Size*charUnit)
		if ts.ink.Empty() {
			continue
		}
		r.id = fmt.Sprintf("g%d.%s", g.ID, tt.id)
		x := (g.View.XMin+g.View.XMax)/2 - (ts.ink.LLX+ts.ink.URX)/2
		r.emit(ts, x, g.View.YMax+tt.gap-ts.ink.LLY, 0, tt.t.Color)
	}
}

// legend draws a graph's legend (docs/grace-behaviour.md): an entry per
// visible set with a legend text, in set order (or reversed: invert) - a
// sample and the text. The entries are laid out on their own, then moved so
// that the box of all they draw, widened by the gaps, has its top left
// corner at the legend's place; that box is the legend box.
func (r *renderer) legend(t xform) {
	g := t.g
	L := g.Legend
	if !L.On {
		return
	}
	var sets []*agr.Set
	maxSym := 0.0
	for _, s := range g.Sets {
		if s.Hidden {
			continue
		}
		maxSym = max(maxSym, s.Symbol.Size) // every visible set's, whether drawn or not
		if s.Legend != "" {
			sets = append(sets, s)
		}
	}
	if len(sets) == 0 {
		return
	}
	if L.Invert {
		for i, j := 0, len(sets)-1; i < j; i, j = i+1, j-1 {
			sets[i], sets[j] = sets[j], sets[i]
		}
	}
	h, v := L.HGap*legendUnit, L.VGap*legendUnit
	length := L.Length * legendUnit
	textX := length + h + maxSym*symbolUnit
	type entry struct {
		s       *agr.Set
		ts      typeset
		base, y float64 // the text's baseline, the sample's line
		line    bool
		lst     draw.Style
		pen     symbolPen
	}
	var ents []entry
	var box afm.Box
	add := func(b afm.Box) {
		if box.Empty() {
			box = b
		} else {
			box = box.Union(b)
		}
	}
	top := 0.0 // the next entry's ink top
	for _, s := range sets {
		e := entry{s: s, ts: r.layout(s.Legend, L.Font, L.CharSize*charUnit)}
		ink := e.ts.ink
		e.base = top - ink.URY
		e.y = e.base + (ink.LLY+ink.URY)/2
		top = e.base + ink.LLY - v
		add(afm.Box{LLX: textX + ink.LLX, LLY: e.base + ink.LLY, URX: textX + ink.URX, URY: e.base + ink.URY})
		if s.Line.Type != 0 {
			st, ok := r.style(s.Line.Color, s.Line.LineWidth, s.Line.LineStyle)
			e.lst, e.line = r.patterned(st, ok, s.Line.Pattern)
		}
		if e.line {
			w := e.lst.Width / 2
			add(afm.Box{LLX: -w, LLY: e.y - w, URX: length + w, URY: e.y + w})
		}
		e.pen = r.pen(s)
		if e.pen.ok {
			ext := e.pen.extent()
			xs := []float64{length / 2}
			if e.line {
				xs = []float64{0, length} // with a line: at both its ends
			}
			for _, x := range xs {
				add(afm.Box{LLX: x + ext.LLX, LLY: e.y + ext.LLY, URX: x + ext.URX, URY: e.y + ext.URY})
			}
		}
		ents = append(ents, e)
	}
	x, y := L.X, L.Y
	if L.LocType == "world" {
		if !t.ok(x, y) {
			r.warn("the legend is at a point a log axis cannot show; not drawn")
			return
		}
		x, y = t.x(x), t.y(y)
	}
	dx, dy := x+h-box.LLX, y-v-box.URY
	left, right, upper, lower := x, box.URX+dx+h, y, box.LLY+dy-v
	r.id = fmt.Sprintf("g%d.legend", g.ID)
	corners := []draw.Point{{X: left, Y: upper}, {X: left, Y: lower}, {X: right, Y: lower}, {X: right, Y: upper}}
	r.fill(L.FillColor, L.FillPattern, draw.Segment{Points: corners, Closed: true})
	if st, ok := r.style(L.BoxColor, L.BoxWidth, L.BoxStyle); ok {
		if st, ok = r.patterned(st, ok, L.BoxPattern); ok {
			r.path(st, false, draw.Segment{Points: append(corners, corners[0]), Closed: true})
		}
	}
	for _, e := range ents {
		r.emit(e.ts, textX+dx, e.base+dy, 0, L.Color)
		y := e.y + dy
		if e.line {
			r.path(e.lst, false, line(draw.Point{X: dx, Y: y}, draw.Point{X: length + dx, Y: y}))
		}
		if e.pen.ok {
			if e.line {
				e.pen.draw(r, dx, y)
				e.pen.draw(r, length+dx, y)
			} else {
				e.pen.draw(r, length/2+dx, y)
			}
		}
	}
}

// set draws a set (docs/grace-behaviour.md): its fill, line, droplines,
// baseline, error bars and symbols, in that order.
func (r *renderer) set(t xform, s *agr.Set) {
	pts, breaks := r.linePoints(t, s)
	if s.Line.Type != 0 && (s.Fill.Type == 1 || s.Fill.Type == 2) {
		r.setFill(t, s, pts)
	} else if s.Line.Type != 0 && s.Fill.Type != 0 {
		r.warn("set %d: fill type %d is not one of Grace's; not drawn, as gracebat does", s.ID, s.Fill.Type)
	}
	st, ok := r.style(s.Line.Color, s.Line.LineWidth, s.Line.LineStyle)
	st, ok = r.patterned(st, ok, s.Line.Pattern)
	if s.Line.Type != 0 && ok {
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
	if ok && (s.Dropline || s.Baseline.On) {
		r.droplines(t, s, st)
	}
	if s.ErrorBar.On && s.Type == "xydy" {
		r.errorBars(t, s)
	}
	if s.Symbol.Type != 0 {
		r.symbols(t, s)
	}
}

// linePoints are the viewport points a set's line joins: its points that
// the axes can place, with stair corners for line types 2 and 3; breaks
// marks a point not joined to the next.
func (r *renderer) linePoints(t xform, s *agr.Set) (pts []draw.Point, breaks []bool) {
	if s.Line.Type < 0 || s.Line.Type > 5 {
		r.warn("set %d: line type %d is drawn straight", s.ID, s.Line.Type)
	}
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
	return pts, breaks
}

// baseline is where a set's baseline is, in world y, and the x extent of
// its points (docs/grace-behaviour.md); ok is false for a set without a
// point the axes can place.
func (r *renderer) baseline(t xform, s *agr.Set) (y, xmin, xmax float64, ok bool) {
	ymin, ymax, sum, n := math.Inf(1), math.Inf(-1), 0.0, 0
	xmin, xmax = math.Inf(1), math.Inf(-1)
	for _, row := range s.Data {
		if !t.ok(row[0], row[1]) {
			continue
		}
		xmin, xmax = min(xmin, row[0]), max(xmax, row[0])
		ymin, ymax = min(ymin, row[1]), max(ymax, row[1])
		sum += row[1]
		n++
	}
	if n == 0 {
		return 0, 0, 0, false
	}
	switch s.Baseline.Type {
	case 0:
		y = 0
	case 1:
		y = ymin
	case 2:
		y = ymax
	case 3:
		y = t.g.World.YMin
	case 4:
		y = t.g.World.YMax
	case 5:
		y = sum / float64(n)
	default:
		r.warn("set %d: baseline type %d is not one of Grace's; y = 0 used, as gracebat does (\"Wrong type of baseline\")", s.ID, s.Baseline.Type)
	}
	return y, xmin, xmax, true
}

// baseY is a baseline's viewport y: on a log axis a baseline at or below
// 0 is infinitely far below the world, so beyond the frame on that side.
func baseY(t xform, y float64) float64 {
	if t.g.Y.Scale == "Logarithmic" && y <= 0 {
		y = math.SmallestNonzeroFloat64
	}
	return t.y(y)
}

// setFill fills the area of a set's line (docs/grace-behaviour.md): type 1
// the polygon of its points (stair corners included), type 2 that and
// then along the baseline from the point furthest right in x to the one
// furthest left, cut to the frame.
func (r *renderer) setFill(t xform, s *agr.Set, pts []draw.Point) {
	poly := append([]draw.Point(nil), pts...)
	if s.Fill.Type == 2 {
		y, xmin, xmax, ok := r.baseline(t, s)
		if !ok {
			return
		}
		b := baseY(t, y)
		poly = append(poly, draw.Point{X: t.x(xmax), Y: b}, draw.Point{X: t.x(xmin), Y: b})
	}
	poly = clipPolygon(poly, t.g.View)
	if len(poly) < 3 {
		return
	}
	r.id = fmt.Sprintf("g%d.s%d.fill", t.g.ID, s.ID)
	n := len(r.d.Paths)
	r.fill(s.Fill.Color, s.Fill.Pattern, draw.Segment{Points: poly, Closed: true})
	for i := n; i < len(r.d.Paths); i++ {
		r.d.Paths[i].EvenOdd = s.Fill.Rule == 1
	}
}

// droplines draws a set's droplines, from its baseline up (or down) to
// each point, and its baseline, from its smallest x to its largest, in
// the line's style, cut to the frame (docs/grace-behaviour.md).
func (r *renderer) droplines(t xform, s *agr.Set, st draw.Style) {
	y, xmin, xmax, ok := r.baseline(t, s)
	if !ok {
		return
	}
	b := baseY(t, y)
	v := t.g.View
	if s.Dropline {
		r.id = fmt.Sprintf("g%d.s%d.dropline", t.g.ID, s.ID)
		for _, row := range s.Data {
			if !t.ok(row[0], row[1]) {
				continue
			}
			x := t.x(row[0])
			if a, c, ok := clipSegment(draw.Point{X: x, Y: b}, draw.Point{X: x, Y: t.y(row[1])}, v); ok {
				r.path(st, false, line(a, c))
			}
		}
	}
	if s.Baseline.On {
		r.id = fmt.Sprintf("g%d.s%d.baseline", t.g.ID, s.ID)
		if a, c, ok := clipSegment(draw.Point{X: t.x(xmin), Y: b}, draw.Point{X: t.x(xmax), Y: b}, v); ok {
			r.path(st, false, line(a, c))
		}
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
	// the error bar's pattern is the risers', caps' and arrowheads'
	pat, _ := r.patterned(draw.Style{}, true, e.Pattern)
	riser, hasRiser := r.style(e.Color, e.RiserWidth, e.RiserStyle)
	capSt, hasCap := r.style(e.Color, e.LineWidth, e.LineStyle)
	arrow := draw.Style{Color: r.color(e.Color), Width: strokeWidth(e.LineWidth)} // solid whatever the line style
	riser.Pattern, capSt.Pattern, arrow.Pattern = pat.Pattern, pat.Pattern, pat.Pattern
	half := e.Size * symbolUnit // the cap's half length; also the arrowhead's half width
	v, w := t.g.View, t.g.World
	r.id = fmt.Sprintf("g%d.s%d.errorbars", t.g.ID, s.ID)
	for i, row := range s.Data {
		if i%(s.Symbol.Skip+1) != 0 {
			continue // symbol skip leaves out the error bars too
		}
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
			// same way (an inverted axis turns it round)
			up := ((d > 0) == (w.YMax >= w.YMin)) != t.g.Y.Invert
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

// symbolPen is how a set's symbols are drawn: style, fill, and for symbol
// 11 its typeset character. ok is false when there is nothing to draw.
type symbolPen struct {
	sy           agr.Symbol
	st           draw.Style
	stroke, fill bool
	open         bool // plus, x, star: arms only, never filled
	ch           typeset
	ok           bool
}

// pen prepares a set's symbols (docs/grace-behaviour.md): the fill first
// (fill color and pattern), then the outline (pattern 0:
// none), or for symbol 11 the set's character.
func (r *renderer) pen(s *agr.Set) symbolPen {
	sy := s.Symbol
	if sy.Type == 0 {
		return symbolPen{}
	}
	if sy.Type < 1 || sy.Type > 11 {
		r.warn("set %d: symbol %d is drawn as a circle", s.ID, sy.Type)
		sy.Type = 1
	}
	p := symbolPen{sy: sy, open: sy.Type >= 8 && sy.Type <= 10, ok: true}
	p.st, p.stroke = r.style(sy.Color, sy.LineWidth, sy.LineStyle)
	p.st, p.stroke = r.patterned(p.st, p.stroke, sy.Pattern)
	p.fill = sy.FillPattern != 0 && !p.open && sy.Type != 11
	if sy.Type == 11 {
		if sy.Char <= 32 || sy.Char > 255 || sy.Char == '\\' {
			r.warn("set %d: symbol character %d cannot be drawn", s.ID, sy.Char)
			return symbolPen{}
		}
		p.ch = r.layout(string(rune(sy.Char)), sy.CharFont, sy.Size*charUnit)
	}
	return p
}

// draw draws one symbol centred on (px, py).
func (p symbolPen) draw(r *renderer, px, py float64) {
	sy := p.sy
	switch sy.Type {
	case 1:
		rad := sy.Size * symbolUnit
		arc := draw.Segment{Arc: &draw.Arc{X: px, Y: py, RX: rad, RY: rad, A1: 0, A2: 360}}
		if p.fill {
			r.fill(p.sy.FillColor, p.sy.FillPattern, arc)
		}
		if p.stroke {
			r.path(p.st, false, arc)
		}
	case 11:
		if !p.ch.ink.Empty() {
			r.emit(p.ch, px-(p.ch.ink.LLX+p.ch.ink.URX)/2, py-(p.ch.ink.LLY+p.ch.ink.URY)/2, 0, sy.Color)
		}
	default:
		for _, shape := range symbolShapes[sy.Type] {
			pts := make([]draw.Point, len(shape), len(shape)+1)
			for i, q := range shape {
				pts[i] = draw.Point{X: px + q.X*sy.Size, Y: py + q.Y*sy.Size}
			}
			if p.fill {
				r.fill(p.sy.FillColor, p.sy.FillPattern, draw.Segment{Points: pts, Closed: true})
			}
			if p.stroke {
				if !p.open {
					// gracebat's outline returns to its first point
					pts = append(pts, pts[0])
				}
				r.path(p.st, false, draw.Segment{Points: pts, Closed: !p.open})
			}
		}
	}
}

// extent is the box of what draw draws, around the point: the shape, plus
// half the outline's width when there is one; a character's box.
func (p symbolPen) extent() afm.Box {
	var b afm.Box
	switch p.sy.Type {
	case 1:
		rad := p.sy.Size * symbolUnit
		b = afm.Box{LLX: -rad, LLY: -rad, URX: rad, URY: rad}
	case 11:
		w, h := p.ch.ink.URX-p.ch.ink.LLX, p.ch.ink.URY-p.ch.ink.LLY
		return afm.Box{LLX: -w / 2, LLY: -h / 2, URX: w / 2, URY: h / 2}
	default:
		first := true
		for _, shape := range symbolShapes[p.sy.Type] {
			for _, q := range shape {
				x, y := q.X*p.sy.Size, q.Y*p.sy.Size
				if first {
					b, first = afm.Box{LLX: x, LLY: y, URX: x, URY: y}, false
				}
				b.LLX, b.LLY, b.URX, b.URY = min(b.LLX, x), min(b.LLY, y), max(b.URX, x), max(b.URY, y)
			}
		}
	}
	if p.stroke {
		h := p.st.Width / 2
		b.LLX, b.LLY, b.URX, b.URY = b.LLX-h, b.LLY-h, b.URX+h, b.URY+h
	}
	return b
}

// symbols draws a set's symbols at its points inside the world window.
func (r *renderer) symbols(t xform, s *agr.Set) {
	p := r.pen(s)
	if !p.ok {
		return
	}
	r.id = fmt.Sprintf("g%d.s%d.symbols", t.g.ID, s.ID)
	w := t.g.World
	// symbol skip k draws every (k+1)-th row, counting the rows out of
	// view too (docs/grace-behaviour.md)
	step := s.Symbol.Skip + 1
	for i, row := range s.Data {
		if i%step != 0 {
			continue
		}
		x, y := row[0], row[1]
		if !t.ok(x, y) || x < min(w.XMin, w.XMax) || x > max(w.XMin, w.XMax) || y < min(w.YMin, w.YMax) || y > max(w.YMin, w.YMax) {
			continue
		}
		p.draw(r, t.x(x), t.y(y))
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

// clipMargin is how far outside the frame gracebat cuts lines: 0.0001 of
// the viewport on every side, whatever the world (docs/grace-behaviour.md);
// fills it cuts at the frame.
const clipMargin = 0.0001

// clipSegment is Liang-Barsky clipping of a-b to the rectangle, widened by
// clipMargin. A segment with both ends outside starts, as gracebat draws
// it, on the frame's bottom, else right, else top, else left edge.
func clipSegment(a, b draw.Point, v agr.Rect) (draw.Point, draw.Point, bool) {
	v = agr.Rect{XMin: v.XMin - clipMargin, YMin: v.YMin - clipMargin, XMax: v.XMax + clipMargin, YMax: v.YMax + clipMargin}
	t0, t1 := 0.0, 1.0
	e0, e1 := -1, -1 // the edges a and b are cut at, as ranked below
	dx, dy := b.X-a.X, b.Y-a.Y
	for _, e := range []struct {
		p, q float64
		rank int
	}{
		{-dx, a.X - v.XMin, 3}, {dx, v.XMax - a.X, 1},
		{-dy, a.Y - v.YMin, 0}, {dy, v.YMax - a.Y, 2},
	} {
		p, q := e.p, e.q
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
			if t > t0 {
				t0, e0 = t, e.rank
			}
		} else {
			if t < t0 {
				return a, b, false
			}
			if t < t1 {
				t1, e1 = t, e.rank
			}
		}
	}
	na, nb := a, b
	if t0 > 0 {
		na = draw.Point{X: a.X + t0*dx, Y: a.Y + t0*dy}
	}
	if t1 < 1 {
		nb = draw.Point{X: a.X + t1*dx, Y: a.Y + t1*dy}
	}
	if t0 > 0 && t1 < 1 && e1 < e0 {
		na, nb = nb, na
	}
	return na, nb, true
}

// clipPolygon cuts a closed outline to the viewport rectangle
// (Sutherland-Hodgman), edge by edge.
func clipPolygon(pts []draw.Point, v agr.Rect) []draw.Point {
	type edge struct {
		in  func(draw.Point) bool
		cut func(a, b draw.Point) draw.Point
	}
	at := func(a, b draw.Point, t float64) draw.Point {
		return draw.Point{X: a.X + t*(b.X-a.X), Y: a.Y + t*(b.Y-a.Y)}
	}
	xAt := func(x float64) func(a, b draw.Point) draw.Point {
		return func(a, b draw.Point) draw.Point { p := at(a, b, (x-a.X)/(b.X-a.X)); p.X = x; return p }
	}
	yAt := func(y float64) func(a, b draw.Point) draw.Point {
		return func(a, b draw.Point) draw.Point { p := at(a, b, (y-a.Y)/(b.Y-a.Y)); p.Y = y; return p }
	}
	for _, e := range []edge{
		{func(p draw.Point) bool { return p.X >= v.XMin }, xAt(v.XMin)},
		{func(p draw.Point) bool { return p.X <= v.XMax }, xAt(v.XMax)},
		{func(p draw.Point) bool { return p.Y >= v.YMin }, yAt(v.YMin)},
		{func(p draw.Point) bool { return p.Y <= v.YMax }, yAt(v.YMax)},
	} {
		var out []draw.Point
		for i, b := range pts {
			a := pts[(i+len(pts)-1)%len(pts)]
			switch {
			case e.in(b) && !e.in(a):
				out = append(out, e.cut(a, b), b)
			case e.in(b):
				out = append(out, b)
			case e.in(a):
				out = append(out, e.cut(a, b))
			}
		}
		pts = out
		if len(pts) == 0 {
			break
		}
	}
	return pts
}

// frameBackground fills the viewport with the frame's background
// (docs/grace-behaviour.md): before anything else of the graph, for any
// frame type, in its pattern.
func (r *renderer) frameBackground(g *agr.Graph) {
	v := g.View
	r.id = fmt.Sprintf("g%d.frame", g.ID)
	r.fill(g.Frame.BackgroundColor, g.Frame.BackgroundPattern, draw.Segment{Points: []draw.Point{
		{X: v.XMin, Y: v.YMin}, {X: v.XMin, Y: v.YMax}, {X: v.XMax, Y: v.YMax}, {X: v.XMax, Y: v.YMin},
	}, Closed: true})
}

// frame draws the frame as gracebat does (docs/grace-behaviour.md): type 0
// a closed box, 1 the left and bottom sides, 2-5 the box without its top,
// bottom, left or right side, each one path from the corner after the gap;
// any other type, pattern 0 or line style 0 draws nothing.
func (r *renderer) frame(g *agr.Graph) {
	f := g.Frame
	if f.Pattern == 0 {
		return
	}
	v := g.View
	bl, tl := draw.Point{X: v.XMin, Y: v.YMin}, draw.Point{X: v.XMin, Y: v.YMax}
	tr, br := draw.Point{X: v.XMax, Y: v.YMax}, draw.Point{X: v.XMax, Y: v.YMin}
	var seg draw.Segment
	switch f.Type {
	case 0:
		seg = draw.Segment{Points: []draw.Point{bl, tl, tr, br, bl}, Closed: true}
	case 1:
		seg = line(tl, bl, br)
	case 2:
		seg = line(tl, bl, br, tr)
	case 3:
		seg = line(bl, tl, tr, br)
	case 4:
		seg = line(bl, br, tr, tl)
	case 5:
		seg = line(br, bl, tl, tr)
	default:
		r.warn("frame type %d is not one of Grace's; not drawn, as gracebat does", f.Type)
		return
	}
	st, ok := r.style(f.Color, f.LineWidth, f.LineStyle)
	if st, ok = r.patterned(st, ok, f.Pattern); !ok {
		return
	}
	r.id = fmt.Sprintf("g%d.frame", g.ID)
	r.path(st, false, seg)
}

// label formats a tick value.
func (r *renderer) label(v float64, tl agr.TickLabels) string {
	s := formatNumber(v, tl.Format, tl.Prec)
	if t, ok := formatDate(v, r.proj.RefDate, tl.Format); ok {
		s = t
	} else if t, ok := formatAngle(v, tl.Format, tl.Prec); ok {
		s = t
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
		// from the other axis's low end to its high end, which an inverted
		// axis puts on the far side
		y0, y1, x0, x1 := v.YMin, v.YMax, v.XMin, v.XMax
		if g.Y.Invert {
			y0, y1 = y1, y0
		}
		if g.X.Invert {
			x0, x1 = x1, x0
		}
		for _, w := range k.vals {
			if isX {
				x := t.x(w)
				r.path(st, false, line(draw.Point{X: x, Y: y0}, draw.Point{X: x, Y: y1}))
			} else {
				y := t.y(w)
				r.path(st, false, line(draw.Point{X: x0, Y: y}, draw.Point{X: x1, Y: y}))
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
	if ax.Tick.On {
		majorLen := ax.Tick.MajorMarks.Size * tickUnit
		minorLen := ax.Tick.MinorMarks.Size * tickUnit
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
					// at the axis, which "offset" moves out from the frame
					if isX {
						base := v.YMin - ax.Offset[0]
						if side == 1 {
							base = v.YMax + ax.Offset[1]
						}
						r.path(st, false, line(draw.Point{X: p, Y: base + a}, draw.Point{X: p, Y: base + b}))
					} else {
						base := v.XMin - ax.Offset[0]
						if side == 1 {
							base = v.XMax + ax.Offset[1]
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

	// per side (0 normal: bottom/left; 1 opposite: top/right): where the
	// axis is, which way is out, and how far the ticks drawn there reach
	dir := func(sd int) float64 { return float64(2*sd - 1) }
	base := func(sd int) float64 { // the axis, moved out by its offset
		f := map[bool][2]float64{true: {v.YMin, v.YMax}, false: {v.XMin, v.XMax}}[isX][sd]
		return f + dir(sd)*ax.Offset[sd]
	}
	ticksOn := func(sd int) bool {
		if !ax.Tick.On {
			return false
		}
		for _, x := range r.tickSides(ax.Tick.Place) {
			if x == sd {
				return true
			}
		}
		return false
	}
	reach := func(sd int) float64 { // the ticks' length outward
		if ticksOn(sd) && (ax.Tick.Direction == "out" || ax.Tick.Direction == "both") {
			return max(ax.Tick.MajorMarks.Size, ax.Tick.MinorMarks.Size) * tickUnit
		}
		return 0
	}
	// edge: the outer side of all drawn on a side so far - the ticks' box,
	// their line's half width included (gracebat measures it so) - which the
	// axis label goes beyond
	var edge [2]float64
	for sd := range 2 {
		edge[sd] = base(sd)
		if ticksOn(sd) {
			hw := max(ax.Tick.MajorMarks.LineWidth, ax.Tick.MinorMarks.LineWidth) * lineUnit / 2
			edge[sd] += dir(sd) * (reach(sd) + hw)
		}
	}
	far := func(sd int, x float64) {
		if sd == 0 {
			edge[sd] = min(edge[sd], x)
		} else {
			edge[sd] = max(edge[sd], x)
		}
	}

	// tick labels (docs/grace-behaviour.md): on the sides "ticklabel place"
	// says, 0.01 beyond the ticks reaching out there; centred on their tick
	// by their box (their turned glyphs' when at an angle); every
	// (skip+1)th tick; staggered over stagger+1 rows, the label of tick k in
	// row k%(stagger+1), moved out by row x (0.02 x char size + 0.01)
	if ax.TickLabel.On {
		size := ax.TickLabel.CharSize * charUnit
		step := max(ax.TickLabel.Skip, 0) + 1
		rows := max(ax.TickLabel.Stagger, 0) + 1
		type placed struct {
			ts     typeset
			ox, oy float64
			ok     bool
		}
		at := make([][2]placed, len(tk.majors))
		along, out := 0.0, labelOffset // "ticklabel offset spec": the file's
		if ax.TickLabel.OffsetSpec {
			along, out = ax.TickLabel.Offset[0], ax.TickLabel.Offset[1]
		}
		for _, sd := range r.sides(ax.TickLabel.Place, "tick labels") {
			start0 := base(sd) + dir(sd)*(reach(sd)+out)
			{
				rowFar := start0
				for k, w := range tk.majors {
					if k%step != 0 {
						continue
					}
					text := r.label(w, ax.TickLabel)
					if tk.labels != nil { // special ticks with their own labels ("" if none)
						text = ax.TickLabel.Prepend + tk.labels[k] + ax.TickLabel.Append
					}
					ts := r.layout(text, ax.TickLabel.Font, size)
					if ts.ink.Empty() {
						continue
					}
					b := ts.ink
					if math.Mod(ax.TickLabel.Angle, 360) != 0 {
						b = r.turnedBox(ts, ax.TickLabel.Angle)
					}
					p := pos(w) + along
					var pl placed
					pl.ts, pl.ok = ts, true
					// gracebat's row height: 0.02 per unit of char size, on
					// either axis, whatever the label (superscripts included)
					start := start0 + dir(sd)*float64(k%rows)*(ax.TickLabel.CharSize*staggerUnit+labelOffset)
					if isX {
						pl.ox = p - (b.LLX+b.URX)/2
						if sd == 0 {
							pl.oy = start - b.URY
							rowFar = min(rowFar, pl.oy+b.LLY)
						} else {
							pl.oy = start - b.LLY
							rowFar = max(rowFar, pl.oy+b.URY)
						}
						far(sd, rowFar)
					} else {
						pl.oy = p - (b.LLY+b.URY)/2
						if sd == 0 {
							pl.ox = start - b.URX
							rowFar = min(rowFar, pl.ox+b.LLX)
						} else {
							pl.ox = start - b.LLX
							rowFar = max(rowFar, pl.ox+b.URX)
						}
						far(sd, rowFar)
					}
					at[k][sd] = pl
				}
			}
		}
		// drawn tick by tick, the sides of each together (gracebat's order)
		r.id = part("ticklabel")
		for k := range at {
			for sd := range 2 {
				if pl := at[k][sd]; pl.ok {
					r.emit(pl.ts, pl.ox, pl.oy, ax.TickLabel.Angle, ax.TickLabel.Color)
				}
			}
		}
	}

	// the axis label, 0.01 beyond all on its side(s): parallel to the axis,
	// or perpendicular ("label layout perp"), centred on the axis by its ink
	if ax.Label.Text != "" {
		r.id = part("label")
		size := ax.Label.CharSize * charUnit
		ts := r.layout(ax.Label.Text, ax.Label.Font, size)
		perp := ax.Label.Layout == "perp"
		if !ts.ink.Empty() {
			i := ts.ink
			for _, sd := range r.sides(ax.Label.Place, "the axis label") {
				if ax.Label.Spec {
					// "label place spec": its ink centred at SpecAt - along
					// the axis from its middle, out from it
					a, o := ax.Label.SpecAt[0], base(sd)+dir(sd)*ax.Label.SpecAt[1]
					cx, cy := (v.XMin+v.XMax)/2+a, o
					if !isX {
						cx, cy = o, (v.YMin+v.YMax)/2+a
					}
					if isX != perp { // across the page: unturned
						r.emit(ts, cx-(i.LLX+i.URX)/2, cy-(i.LLY+i.URY)/2, 0, ax.Label.Color)
					} else { // turned 90 degrees
						r.emit(ts, cx+(i.LLY+i.URY)/2, cy-(i.LLX+i.URX)/2, 90, ax.Label.Color)
					}
					continue
				}
				e := edge[sd] + dir(sd)*labelOffset
				switch {
				case isX && !perp: // across, below/above
					ox := (v.XMin+v.XMax)/2 - (i.LLX+i.URX)/2
					oy := e - i.URY
					if sd == 1 {
						oy = e - i.LLY
					}
					r.emit(ts, ox, oy, 0, ax.Label.Color)
				case isX: // turned 90 degrees: its length runs up the page
					ox := (v.XMin+v.XMax)/2 + (i.LLY+i.URY)/2
					oy := e - i.URX
					if sd == 1 {
						oy = e - i.LLX
					}
					r.emit(ts, ox, oy, 90, ax.Label.Color)
				case !perp: // turned 90 degrees: its ink "bottom" faces the axis
					oy := (v.YMin+v.YMax)/2 - (i.LLX+i.URX)/2
					ox := e + i.LLY
					if sd == 1 {
						ox = e + i.URY
					}
					r.emit(ts, ox, oy, 90, ax.Label.Color)
				default: // across, left/right
					oy := (v.YMin+v.YMax)/2 - (i.LLY+i.URY)/2
					ox := e - i.URX
					if sd == 1 {
						ox = e - i.LLX
					}
					r.emit(ts, ox, oy, 0, ax.Label.Color)
				}
			}
		}
	}
}

// sides: the sides "place" names - 0 normal (bottom/left), 1 opposite.
func (r *renderer) sides(place, what string) []int {
	switch place {
	case "normal", "", "auto":
		return []int{0}
	case "opposite":
		return []int{1}
	case "both":
		return []int{0, 1}
	}
	r.warn("%s: place %q not supported; normal used", what, place)
	return []int{0}
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
	ts := r.layout(s.Text, s.Font, s.CharSize*charUnit)
	dx, dy, ok := r.justify(ts, s.Just, s.Rot)
	if !ok {
		r.warn("string %q: justification %d is not one of Grace's; not drawn, as gracebat does", s.Text, s.Just)
		return
	}
	r.emit(ts, x+dx, y+dy, s.Rot, s.Color)
}

// justify is where a string's pen origin goes relative to its point, for
// Grace's justification (docs/grace-behaviour.md): its low two bits
// horizontal - 0 left, 1 right, 2 centre (3 draws nothing) - the next two
// vertical - 0 the baseline, 1 bottom, 2 top, 3 middle. On the baseline
// the string moves along its own direction by its advance width; aligned
// vertically, by the page's box of its rotated glyphs and pen origin.
func (r *renderer) justify(ts typeset, just int, deg float64) (dx, dy float64, ok bool) {
	h, v := just&3, (just>>2)&3
	if h == 3 || just < 0 || just > 15 {
		return 0, 0, false
	}
	if v == 0 {
		along := map[int]float64{0: 0, 1: -ts.width, 2: -ts.width / 2}[h]
		dx, dy = rotate(along, 0, deg)
		return dx, dy, true
	}
	b := r.turnedBox(ts, deg)
	dx = map[int]float64{0: -b.LLX, 1: -b.URX, 2: -(b.LLX + b.URX) / 2}[h]
	dy = map[int]float64{1: -b.LLY, 2: -b.URY, 3: -(b.LLY + b.URY) / 2}[v]
	return dx, dy, true
}

// turnedBox is the box, on the page, of a typeset string's glyphs turned
// by deg about its pen origin, the pen origin included: unturned, its ink
// box; turned, the box of its glyphs' turned outlines (tighter than the
// turned ink box, as gracebat measures it).
func (r *renderer) turnedBox(ts typeset, deg float64) afm.Box {
	b := afm.Box{}
	if math.Mod(deg, 360) == 0 {
		if !ts.ink.Empty() {
			b = afm.Box{LLX: min(ts.ink.LLX, 0), LLY: min(ts.ink.LLY, 0), URX: max(ts.ink.URX, 0), URY: max(ts.ink.URY, 0)}
		}
		return b
	}
	add := func(x, y float64) {
		x, y = rotate(x, y, deg)
		b.LLX, b.LLY, b.URX, b.URY = min(b.LLX, x), min(b.LLY, y), max(b.URX, x), max(b.URY, y)
	}
	for _, rn := range ts.runs {
		m := r.metrics(rn.font)
		px := rn.x
		for i := 0; i < len(rn.str); i++ {
			segs, err := glyph.Outline(rn.font, rn.str[i])
			if err != nil {
				segs = nil
			}
			at := func(p [2]float64) (float64, float64) { return px + p[0]*rn.size, rn.y + p[1]*rn.size }
			var cur [2]float64
			for _, sg := range segs {
				switch sg.Op {
				case glyph.MoveTo, glyph.LineTo:
					cur = sg.Pts[0]
					add(at(cur))
				default:
					// sample the curve: its control points may lie outside it
					ctl := append([][2]float64{cur}, sg.Pts...)
					for k := 1; k <= 16; k++ {
						add(at(bezier(ctl, float64(k)/16)))
					}
					cur = sg.Pts[len(sg.Pts)-1]
				}
			}
			px += m.Width(rn.str[i:i+1]) * rn.size
		}
	}
	return b
}

// bezier is the point at t of the Bezier curve with control points pts.
func bezier(pts [][2]float64, t float64) [2]float64 {
	p := append([][2]float64(nil), pts...)
	for n := len(p) - 1; n > 0; n-- {
		for i := 0; i < n; i++ {
			p[i] = [2]float64{p[i][0] + (p[i+1][0]-p[i][0])*t, p[i][1] + (p[i+1][1]-p[i][1])*t}
		}
	}
	return p[0]
}
