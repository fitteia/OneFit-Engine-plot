package render

import (
	"fmt"
	"math"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

// annotatedValues follows Grace's centre/bottom page-aligned placement:
// offsets are viewport distances, independent of the world and rotation.
func (r *renderer) annotatedValues(t xform, s *agr.Set) {
	a := s.AValue
	if a.Type < 0 || a.Type > 5 {
		r.warn("set %d: annotated value type %d is not drawn, as gracebat does", s.ID, a.Type)
		return
	}
	number := func(v float64) string { return r.label(v, agr.TickLabels{Format: a.Format, Prec: a.Prec}) }
	for i, row := range s.Data {
		if len(row) < 2 || i%(max(s.Symbol.Skip, 0)+1) != 0 || !t.ok(row[0], row[1]) {
			continue
		}
		x, y := t.x(row[0]), t.y(row[1])
		v := t.g.View
		if math.IsNaN(x) || math.IsNaN(y) || x < v.XMin || x > v.XMax || y < v.YMin || y > v.YMax {
			continue
		}
		text := ""
		switch a.Type {
		case 1:
			text = number(row[0])
		case 2:
			text = number(row[1])
		case 3:
			text = number(row[0]) + ", " + number(row[1])
		case 4:
			if i < len(s.Labels) {
				text = s.Labels[i]
			}
		case 5:
			if len(row) > 2 {
				text = number(row[2])
			}
		}
		ts := r.layout(a.Prepend+text+a.Append, a.Font, a.CharSize*charUnit)
		dx, dy, _ := r.justify(ts, 6, a.Rot)
		r.id = fmt.Sprintf("g%d.s%d.avalue", t.g.ID, s.ID)
		r.emit(ts, x+a.Offset[0]+dx, y+a.Offset[1]+dy, a.Rot, a.Color)
	}
}
