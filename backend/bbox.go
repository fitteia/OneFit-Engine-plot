// Package backend writes a draw.Drawing as a file: EPS (what OneFit's C
// core asks gracebat for), PDF (cropped as epstopdf crops gracebat's EPS),
// and SVG.
package backend

import (
	"math"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
)

// Box is a rectangle in page points: lower left and upper right.
type Box struct {
	LLX, LLY, URX, URY float64
}

// BoundingBox is the drawing's ink, in whole page points with about a point
// of margin, as gracebat's EPS %%BoundingBox is (the page background is not
// ink). The EPS's box and the PDF's crop are both this box, so a reader
// that maps one onto the other (the OneFit GUI does) stays exact.
func BoundingBox(d *draw.Drawing, unit float64) Box {
	lx, ly, ux, uy := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	grow := func(x, y float64) {
		if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			return
		}
		lx, ly, ux, uy = min(lx, x), min(ly, y), max(ux, x), max(uy, y)
	}
	for _, p := range d.Paths {
		if p.Fill && p.ID == "page" {
			continue // the page background; other fills (a legend's box, filled symbols) are ink
		}
		hw := p.Style.Width / 2
		for _, s := range p.Segments {
			if a := s.Arc; a != nil {
				grow(a.X-a.RX-hw, a.Y-a.RY-hw)
				grow(a.X+a.RX+hw, a.Y+a.RY+hw)
				continue
			}
			for _, q := range s.Points {
				grow(q.X-hw, q.Y-hw)
				grow(q.X+hw, q.Y+hw)
			}
		}
	}
	for _, t := range d.Texts {
		m, err := afm.Get(t.Font)
		if err != nil {
			continue
		}
		b := glyph.Ink(t.Font, m, t.Str) // as the layout measures it
		if b.Empty() {
			continue
		}
		for _, c := range [][2]float64{{b.LLX, b.LLY}, {b.URX, b.URY}, {b.LLX, b.URY}, {b.URX, b.LLY}} {
			grow(t.At.X+t.Matrix[0]*c[0]+t.Matrix[2]*c[1], t.At.Y+t.Matrix[1]*c[0]+t.Matrix[3]*c[1])
		}
	}
	if math.IsInf(lx, 1) {
		return Box{0, 0, d.Width * unit, d.Height * unit}
	}
	// ink far off the page (a text of size 1e300) would make a box no
	// reader can use - an EPS whose box overflows, a raster of one pixel:
	// the box stays within a page's width and height around the page
	w, h := d.Width*unit, d.Height*unit
	return Box{
		LLX: math.Floor(max(lx*unit-1, -w)), LLY: math.Floor(max(ly*unit-1, -h)),
		URX: math.Ceil(min(ux*unit+1, 2*w)), URY: math.Ceil(min(uy*unit+1, 2*h)),
	}
}
