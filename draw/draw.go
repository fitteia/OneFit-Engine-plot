// Package draw is the drawing plot-go makes of a page: paths and texts in
// viewport units (the page's shorter side is 1, origin at the bottom
// left), in the order they are drawn. The renderer produces one, the
// backends (PDF, SVG, PNG) turn it into a file, and internal/eps reads
// gracebat's output into one for comparison.
package draw

// Point is a position in viewport units (the page's shorter side is 1).
type Point struct{ X, Y float64 }

// Style is the graphics state a primitive was drawn with.
type Style struct {
	Color [3]float64
	Width float64   // line width, viewport units
	Dash  []float64 // dash pattern, viewport units; empty is solid
	Cap   int
	Join  int
}

// Segment is one piece of a path: a polyline, or an elliptical arc.
type Segment struct {
	Points []Point // polyline (m, l, ...)
	Closed bool
	Arc    *Arc
}

// Arc is gracebat's EARC: an ellipse arc around (X, Y), angles in degrees.
type Arc struct {
	X, Y, RX, RY, A1, A2 float64
}

// Path is a stroked or filled path.
type Path struct {
	Segments []Segment
	Fill     bool
	Style    Style
}

// Text is a string shown at a point: Matrix is the font matrix (a b c d)
// in viewport units, so Matrix[0] is the font size for unrotated text.
type Text struct {
	At     Point
	Matrix [4]float64
	Font   string // PostScript font name: Helvetica, Symbol, ...
	Color  [3]float64
	Str    string
}

// Drawing is everything a page draws, in order.
type Drawing struct {
	// Width and Height are the page's size in viewport units (one of them
	// is 1).
	Width, Height float64
	Paths         []Path
	Texts         []Text
	// Order lists primitives as drawn: 'p' for the next path, 't' for the
	// next text.
	Order []byte
}
