// Package drawcmp compares two drawings - plot-go's and gracebat's (read
// from its EPS by internal/eps) - primitive by primitive.
package drawcmp

import (
	"fmt"
	"math"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// Tolerances, in viewport units. gracebat writes 4 decimals, and measures
// text on its 300 dpi glyph rasters, rounded and hinted (a box edge up to
// 0.0004 off the outlines); plot-go measures the URW outlines exactly
// (docs/grace-behaviour.md). On the corpus the largest differences are
// 0.0006 for texts and 0.0001 (the EPS's rounding) for points. A legend
// is placed by its texts' boxes, entry after entry: its shapes may be off
// by up to 0.0007.
const (
	tolPoint  = 0.0002
	tolText   = 0.002
	tolSize   = 0.0002
	tolLegend = 0.001
)

// Compare lists how got differs from gracebat's want, at most limit lines.
func Compare(got, want *draw.Drawing, limit int) []string {
	var out []string
	add := func(format string, a ...any) {
		if len(out) < limit {
			out = append(out, fmt.Sprintf(format, a...))
		} else if len(out) == limit {
			out = append(out, "...")
		}
	}
	near := func(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

	if len(got.Texts) != len(want.Texts) {
		add("%d texts, gracebat %d", len(got.Texts), len(want.Texts))
	}
	for i := 0; i < min(len(got.Texts), len(want.Texts)); i++ {
		g, w := got.Texts[i], want.Texts[i]
		// a timestamp shows the time each program drew at
		if (g.Str != w.Str && g.ID != "timestamp") || g.Font != w.Font {
			add("text %d: %q %s, gracebat %q %s", i, g.Str, g.Font, w.Str, w.Font)
			continue
		}
		for k := range g.Matrix {
			if !near(g.Matrix[k], w.Matrix[k], tolSize) {
				add("text %d %q: matrix %.4f, gracebat %.4f", i, g.Str, g.Matrix, w.Matrix)
				break
			}
		}
		if !near(g.At.X, w.At.X, tolText) || !near(g.At.Y, w.At.Y, tolText) {
			add("text %d %q: at %.4f,%.4f, gracebat %.4f,%.4f (off %.4f,%.4f)", i, g.Str, g.At.X, g.At.Y, w.At.X, w.At.Y, g.At.X-w.At.X, g.At.Y-w.At.Y)
		}
		if !colorNear(g.Color, w.Color) {
			add("text %d %q: color %v, gracebat %v", i, g.Str, g.Color, w.Color)
		}
	}

	if len(got.Paths) != len(want.Paths) {
		add("%d paths, gracebat %d", len(got.Paths), len(want.Paths))
	}
	for i := 0; i < min(len(got.Paths), len(want.Paths)); i++ {
		g, w := got.Paths[i], want.Paths[i]
		where := fmt.Sprintf("path %d (%s)", i, describe(w))
		if g.Fill != w.Fill {
			add("%s: fill %v, gracebat %v", where, g.Fill, w.Fill)
		}
		if !g.Fill && !near(g.Style.Width, w.Style.Width, 1e-4) {
			add("%s: width %.4f, gracebat %.4f", where, g.Style.Width, w.Style.Width)
		}
		// a fill has no dash; gracebat's EPS just leaves the last stroke's
		// in the graphics state
		if !(g.Fill && w.Fill) && !dashNear(g.Style.Dash, w.Style.Dash) {
			add("%s: dash %v, gracebat %v", where, g.Style.Dash, w.Style.Dash)
		}
		if g.Style.Pattern != w.Style.Pattern {
			add("%s: pattern %d, gracebat %d", where, g.Style.Pattern, w.Style.Pattern)
		}
		if !colorNear(g.Style.Color, w.Style.Color) {
			add("%s: color %v, gracebat %v", where, g.Style.Color, w.Style.Color)
		}
		if len(g.Segments) != len(w.Segments) {
			add("%s: %d segments, gracebat %d", where, len(g.Segments), len(w.Segments))
			continue
		}
		tp := tolPoint
		if strings.HasSuffix(g.ID, ".legend") {
			tp = tolLegend // placed by text boxes
		}
		for k := range g.Segments {
			gs, ws := g.Segments[k], w.Segments[k]
			if (gs.Arc == nil) != (ws.Arc == nil) {
				add("%s segment %d: arc vs line", where, k)
				continue
			}
			if gs.Arc != nil {
				a, b := gs.Arc, ws.Arc
				if !near(a.X, b.X, tp) || !near(a.Y, b.Y, tp) || !near(a.RX, b.RX, tolPoint) {
					add("%s: arc %.4f,%.4f r %.4f, gracebat %.4f,%.4f r %.4f", where, a.X, a.Y, a.RX, b.X, b.Y, b.RX)
				}
				continue
			}
			if len(gs.Points) != len(ws.Points) {
				add("%s segment %d: %d points, gracebat %d", where, k, len(gs.Points), len(ws.Points))
				continue
			}
			for j := range gs.Points {
				if !near(gs.Points[j].X, ws.Points[j].X, tp) || !near(gs.Points[j].Y, ws.Points[j].Y, tp) {
					add("%s point %d: %.4f,%.4f, gracebat %.4f,%.4f", where, j, gs.Points[j].X, gs.Points[j].Y, ws.Points[j].X, ws.Points[j].Y)
					break
				}
			}
		}
	}
	if len(out) == 0 && string(got.Order) != string(want.Order) {
		add("drawing order differs")
	}
	return out
}

func colorNear(a, b [3]float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > 0.002 {
			return false
		}
	}
	return true
}

func describe(p draw.Path) string {
	if len(p.Segments) == 0 {
		return "empty"
	}
	s := p.Segments[0]
	if s.Arc != nil {
		return fmt.Sprintf("arc at %.4f,%.4f", s.Arc.X, s.Arc.Y)
	}
	return fmt.Sprintf("from %.4f,%.4f, %d points", s.Points[0].X, s.Points[0].Y, len(s.Points))
}

func dashNear(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if math.Abs(a[k]-b[k]) > 1e-4 {
			return false
		}
	}
	return true
}
