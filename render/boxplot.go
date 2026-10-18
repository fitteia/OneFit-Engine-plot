package render

import (
	"fmt"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// boxplot draws Grace's XYBOXPLOT rows: x, median, upper/lower box limits,
// then upper/lower whisker endpoints. gracebat uses symbol size for the
// half-width of both the box and whisker caps, symbol fill for the box,
// symbol stroke for its outline and median, and error-bar riser/cap styles
// for the whiskers (checked against gracebat and the Grace User's Guide).
func (r *renderer) boxplot(t xform, s *agr.Set) {
	if s.ErrorBar.On {
		r.boxWhiskers(t, s)
	}

	half := max(0, s.Symbol.Size) * symbolUnit
	if half == 0 {
		return
	}
	step := s.Symbol.Skip + 1
	for i, row := range s.Data {
		if i%step != 0 {
			continue
		}
		x, median, upper, lower := row[0], row[1], row[2], row[3]
		if !t.ok(x, median) || !t.ok(x, upper) || !t.ok(x, lower) {
			continue
		}
		left, right := t.x(x)-half, t.x(x)+half
		top, bottom := t.y(upper), t.y(lower)
		corners := []draw.Point{{X: left, Y: top}, {X: left, Y: bottom}, {X: right, Y: bottom}, {X: right, Y: top}}
		if s.Symbol.FillPattern != 0 {
			if clipped := clipPolygon(corners, t.g.View); len(clipped) >= 3 {
				r.id = fmt.Sprintf("g%d.s%d.boxplot.fill", t.g.ID, s.ID)
				r.fill(s.Symbol.FillColor, s.Symbol.FillPattern, draw.Segment{Points: clipped, Closed: true})
			}
		}
		st, ok := r.style(s.Symbol.Color, s.Symbol.LineWidth, s.Symbol.LineStyle)
		st, ok = r.patterned(st, ok, s.Symbol.Pattern)
		if ok {
			inside := true
			for _, point := range corners {
				inside = inside && point.X >= t.g.View.XMin && point.X <= t.g.View.XMax && point.Y >= t.g.View.YMin && point.Y <= t.g.View.YMax
			}
			if inside {
				r.id = fmt.Sprintf("g%d.s%d.boxplot", t.g.ID, s.ID)
				r.path(st, false, draw.Segment{Points: append(append([]draw.Point(nil), corners...), corners[0]), Closed: true})
			} else {
				outline := append(append([]draw.Point(nil), corners...), corners[0])
				for _, visible := range clipPolyline(outline, make([]bool, len(outline)), t.g.View) {
					r.id = fmt.Sprintf("g%d.s%d.boxplot", t.g.ID, s.ID)
					r.path(st, false, visible)
				}
			}
			if t.ok(x, median) {
				r.id = fmt.Sprintf("g%d.s%d.boxplot.median", t.g.ID, s.ID)
				r.clipped(st, t.g.View, draw.Point{X: left, Y: t.y(median)}, draw.Point{X: right, Y: t.y(median)})
			}
		}
	}
}

// boxWhiskers draws each whisker from its corresponding box limit to the
// endpoint, with a horizontal cap at the endpoint. Grace uses riser settings
// for the stem and error-bar line settings for the cap.
func (r *renderer) boxWhiskers(t xform, s *agr.Set) {
	e := s.ErrorBar
	if e.Pattern == 0 {
		return
	}
	pat, _ := r.patterned(draw.Style{}, true, e.Pattern)
	riser, hasRiser := r.style(e.Color, e.RiserWidth, e.RiserStyle)
	riser.Pattern = pat.Pattern
	cap, hasCap := r.style(e.Color, e.LineWidth, e.LineStyle)
	cap.Pattern = pat.Pattern
	half := max(0, s.Symbol.Size) * symbolUnit
	step := s.Symbol.Skip + 1
	for i, row := range s.Data {
		if i%step != 0 {
			continue
		}
		x := row[0]
		if !t.ok(x, row[2]) || !t.ok(x, row[3]) || !t.ok(x, row[4]) || !t.ok(x, row[5]) {
			continue
		}
		px := t.x(x)
		for _, pair := range [][2]float64{{row[2], row[4]}, {row[3], row[5]}} {
			limit, whisker := t.y(pair[0]), t.y(pair[1])
			if hasRiser {
				r.id = fmt.Sprintf("g%d.s%d.boxplot.whisker", t.g.ID, s.ID)
				r.clipped(riser, t.g.View, draw.Point{X: px, Y: limit}, draw.Point{X: px, Y: whisker})
			}
			if hasCap && half > 0 && finite(whisker) && whisker >= t.g.View.YMin && whisker <= t.g.View.YMax {
				r.id = fmt.Sprintf("g%d.s%d.boxplot.cap", t.g.ID, s.ID)
				r.clipped(cap, t.g.View, draw.Point{X: px - half, Y: whisker}, draw.Point{X: px + half, Y: whisker})
			}
		}
	}
}
