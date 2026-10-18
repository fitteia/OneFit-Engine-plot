// Package agr reads Grace project files (.agr, .agr-par) - the part of the
// format OneFit writes (see docs/features.md) - into a Project.
//
// Written from the file format and the Grace User's Guide; it contains no
// Grace source code (see AGENTS.md).
package agr

import (
	"math"
	"sort"
)

// Project is one Grace project: a page, its graphs, and the free text
// strings on it.
type Project struct {
	Version         int
	PageWidth       float64 // points
	PageHeight      float64 // points
	PageFill        bool
	BackgroundColor int
	Defaults        Defaults
	Fonts           map[int]Font
	Colors          map[int]Color
	RefDate         float64 // "@reference date": the Julian day dates count from
	Timestamp       Text
	Strings         []*Text
	Graphs          []*Graph
	Warnings        []Warning
	// Kept are the directives the model has no place for (regions, date
	// wrap, ...), as read: Write writes them back, so a project read and
	// written again loses nothing, drawn by plot-go or not. Graphs and
	// sets keep their own.
	Kept []string
	// Objects are Grace's drawing objects (lines, boxes, ellipses): each
	// is the block as read, from "with line" to "line def", without "@".
	// Write keeps untouched blocks as read. For edited Shapes it writes the
	// current settings after the original lines, preserving unknown settings.
	Objects [][]string
	// Shapes are the same objects read (Shapes[i] is Objects[i]): what
	// plot-go draws. Use AddShape and DeleteShape to keep the slices aligned.
	Shapes         []*Shape
	originalShapes []*Shape
}

// AddShape appends a drawing object. Its full settings are written by Write.
func (p *Project) AddShape(s *Shape) {
	p.Shapes = append(p.Shapes, s)
	p.Objects = append(p.Objects, nil)
	p.originalShapes = append(p.originalShapes, nil)
}

// ShapeOrder returns stable object indices from back to front within an
// attachment (one graph's world, or the page) and OnTop layer. Hidden objects
// participate so showing them again preserves their place.
func (p *Project) ShapeOrder(gid int, world, onTop bool, behindData ...bool) []int {
	behind := len(behindData) > 0 && behindData[0]
	var ids []int
	for _, kind := range []string{"box", "ellipse", "line"} {
		for i, s := range p.Shapes {
			if s.Kind == kind && s.OnTop == onTop && s.BehindData == behind && (s.LocType == "world") == world && (!world || max(s.Graph, 0) == gid) {
				ids = append(ids, i)
			}
		}
	}
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := p.Shapes[ids[i]].DrawOrder, p.Shapes[ids[j]].DrawOrder
		if a == 0 {
			return false
		}
		if b == 0 {
			return true
		}
		return a < b
	})
	return ids
}

// MoveShape moves one object a single position within its attachment/layer.
// Indices and raw directives stay unchanged, keeping GUI selections stable.
func (p *Project) MoveShape(i, delta int) bool {
	if i < 0 || i >= len(p.Shapes) || (delta != -1 && delta != 1) {
		return false
	}
	s := p.Shapes[i]
	ids := p.ShapeOrder(max(s.Graph, 0), s.LocType == "world", s.OnTop, s.BehindData)
	for n, id := range ids {
		if id != i {
			continue
		}
		next := n + delta
		if next < 0 || next >= len(ids) {
			return false
		}
		ids[n], ids[next] = ids[next], ids[n]
		for rank, idx := range ids {
			p.Shapes[idx].DrawOrder = rank + 1
		}
		return true
	}
	return false
}

// MoveShapeAcrossLayers moves an object one drawing level forward or backward.
// It reorders within the current layer first, then crosses between BehindData,
// Grace's normal layer and OnTop at that layer's edge.
func (p *Project) MoveShapeAcrossLayers(i, delta int) bool {
	if i < 0 || i >= len(p.Shapes) || (delta != -1 && delta != 1) {
		return false
	}
	s := p.Shapes[i]
	if p.MoveShape(i, delta) {
		return true
	}
	from := 0
	if !s.BehindData {
		from = 1
	}
	if s.OnTop {
		from = 2
	}
	to := from + delta
	if to < 0 || to > 2 {
		return false
	}
	ids := p.ShapeOrder(max(s.Graph, 0), s.LocType == "world", to == 2, to == 0)
	if to == 0 {
		s.BehindData = true
		s.OnTop = false
	} else if to == 2 {
		s.BehindData = false
		s.OnTop = true
	} else {
		s.BehindData = false
		s.OnTop = false
	}
	if delta < 0 {
		ids = append([]int{i}, ids...)
	} else {
		ids = append(ids, i)
	}
	for rank, id := range ids {
		p.Shapes[id].DrawOrder = rank + 1
	}
	return true
}

// CopyShapeFrom adds a copy of one source object's model and original
// directives. The caller may then change the returned shape (for example,
// to attach it to a different graph); Write keeps any unknown directives.
func (p *Project) CopyShapeFrom(src *Project, i int) *Shape {
	if i < 0 || i >= len(src.Shapes) {
		return nil
	}
	copy := *src.Shapes[i]
	copy.DrawOrder = 0
	p.AddShape(&copy)
	if i < len(src.Objects) {
		p.Objects[len(p.Objects)-1] = append([]string(nil), src.Objects[i]...)
	}
	return &copy
}

// DeleteShape removes the object at index i from the drawing and project.
func (p *Project) DeleteShape(i int) bool {
	if i < 0 || i >= len(p.Shapes) || i >= len(p.Objects) {
		return false
	}
	p.Shapes = append(p.Shapes[:i], p.Shapes[i+1:]...)
	p.Objects = append(p.Objects[:i], p.Objects[i+1:]...)
	if i < len(p.originalShapes) {
		p.originalShapes = append(p.originalShapes[:i], p.originalShapes[i+1:]...)
	}
	// Later blocks may inherit settings from the removed object. Write all
	// remaining shapes explicitly so their appearance does not change.
	p.originalShapes = nil
	return true
}

// Shape is a drawing object: a line (with arrows), a box or an ellipse,
// between two corners (a line's ends), on the page (LocType "view") or in
// a graph's world. What a block leaves out is as the last object of its
// kind had it, "on" included (measured; corners 0, graph -1); for the
// first Grace's default: off, the "@default" line settings, fill color 1,
// fill pattern 0, no arrow.
type Shape struct {
	// OnTop is a plot-go extension stored as a comment before the object.
	// It draws the object after all graphs, legends and text.
	OnTop bool
	// BehindData draws the object below graph data and above the page background.
	// OnTop and BehindData are mutually exclusive.
	BehindData bool
	// DrawOrder is an optional plot-go ordering rank within one attachment/layer.
	// Zero retains Grace ordering; ranked objects precede newly added objects.
	DrawOrder      int
	Kind           string // "line", "box" or "ellipse"
	On             bool
	LocType        string
	Graph          int
	X1, Y1, X2, Y2 float64
	LineStyle      int
	LineWidth      float64
	Color          int
	FillColor      int
	FillPattern    int
	// Arrow: 0 none, 1 at the start, 2 at the end, 3 both; ArrowType 0 a
	// line, 1 filled, 2 opaque (filled with the background, outlined).
	// ArrowLength scales its length, ArrowDL its width (to the length),
	// ArrowLL how far its notch reaches (1: none).
	Arrow, ArrowType              int
	ArrowLength, ArrowDL, ArrowLL float64
}

// NewShape starts an object with Grace's first-object defaults. Callers
// placing it in a figure set On and its coordinates before AddShape.
func NewShape(kind string, d Defaults) *Shape {
	return &Shape{Kind: kind, LocType: "view", Graph: 0,
		LineStyle: d.LineStyle, LineWidth: d.LineWidth, Color: d.Color,
		FillColor: 1, ArrowLength: 1, ArrowDL: 1, ArrowLL: 1}
}

// Defaults are the "@default" settings.
type Defaults struct {
	LineWidth  float64
	LineStyle  int
	Color      int
	Pattern    int
	Font       int
	CharSize   float64
	SymbolSize float64
	SFormat    string
}

// Font is one entry of the font map: "@map font 4 to "Helvetica", ...".
type Font struct {
	Name     string
	Fallback string
}

// Color is one entry of the color map.
type Color struct {
	R, G, B uint8
	Name    string
}

// Text is a string placed on the page: a "@with string" object, or the
// timestamp. Text keeps Grace's escapes (\s, \N, \x, ...) as written.
type Text struct {
	On       bool
	LocType  string // "view" or "world"
	Graph    int    // for LocType "world"
	X, Y     float64
	Color    int
	Rot      float64
	Font     int
	Just     int
	CharSize float64
	Text     string
}

// Rect is a rectangle as Grace writes it: x min, y min, x max, y max.
type Rect struct {
	XMin, YMin, XMax, YMax float64
}

// Graph is one graph ("@g0 ...", "@with g0" and what follows).
type Graph struct {
	ID       int
	On       bool
	Hidden   bool
	Type     string
	World    Rect
	View     Rect // viewport units: the page's shorter side is 1
	Title    Title
	Subtitle Title
	X, Y     Axis
	// AltX, AltY are the alternate axes ("altxaxis", "altyaxis"): axes of
	// their own in the same world and scale, drawn after the y axis; Grace's
	// default axis (DefaultAxis) unless the file sets them
	AltX, AltY Axis
	Legend     Legend
	Frame      Frame
	Sets       []*Set
	// Kept are the graph's directives the model has no place for: those
	// in its "@with g" block (alternate axes, axis bars, ...) as read, and
	// in KeptHeader those of its "@gN" header (stacked, fixed point) as
	// read but without "gN". See Project.Kept.
	Kept       []string
	KeptHeader []string
}

// Title is a graph's title or subtitle.
type Title struct {
	Text  string
	Font  int
	Size  float64
	Color int
}

// Axis is one axis with its scale ("@xaxes scale ...") and its ticks,
// labels and axis label ("@xaxis ...").
type Axis struct {
	Scale  string // "Normal" or "Logarithmic"
	Invert bool
	On     bool
	Offset [2]float64 // the axis moved out from the frame: normal side, opposite
	// Zero ("type zero true"): the axis is drawn at the other axis's 0
	// instead of at the frame - not at all when 0 is outside its window
	Zero      bool
	Bar       AxisBar // a line along the axis, on its tick sides
	Label     AxisLabel
	Tick      Ticks
	TickLabel TickLabels
}

// DefaultAxis is Grace's axis before a file sets it - an alternate axis
// switched on with nothing else (measured: the axis gracebat saves for a
// graph it makes itself), off.
func DefaultAxis() Axis {
	marks := func(size float64) TickMarks { return TickMarks{Size: size, Color: 1, LineWidth: 1, LineStyle: 1} }
	return Axis{
		Scale: "Normal",
		Bar:   AxisBar{On: true, Color: 1, LineStyle: 1, LineWidth: 1},
		Label: AxisLabel{Layout: "para", Place: "normal", CharSize: 1, Color: 1},
		Tick: Ticks{On: true, Major: 0.2, MinorTicks: 1, Default: 6, Rounded: true, Direction: "in", Place: "both",
			MajorMarks: marks(1), MinorMarks: marks(0.5), SpecType: "none"},
		TickLabel: TickLabels{On: true, Format: "general", Prec: 5, Place: "normal", CharSize: 1, Color: 1, Offset: [2]float64{0, 0.01}},
	}
}

// AxisBar is the line drawn along an axis ("bar on"), on the sides its
// ticks are placed on.
type AxisBar struct {
	On        bool
	Color     int
	LineStyle int
	LineWidth float64
}

// AxisLabel is the axis title.
type AxisLabel struct {
	Text     string
	Layout   string
	Place    string     // the side(s): "normal", "opposite", "both"
	Spec     bool       // "label place spec": at SpecAt, not beyond the tick labels
	SpecAt   [2]float64 // along the axis from its middle, out from it
	CharSize float64
	Font     int
	Color    int
}

// TickMarks describes the major or the minor tick marks.
type TickMarks struct {
	Size      float64
	Color     int
	LineWidth float64
	LineStyle int
	Grid      bool
}

// Ticks are an axis's tick marks: the spacing of major ticks (in world
// units, or a factor on a log axis) and the number of minor ticks between
// two major ones.
type Ticks struct {
	On         bool
	Major      float64
	MinorTicks int
	Default    int
	Rounded    bool
	Direction  string // "in", "out" or "both"
	Place      string // "both", "normal" or "opposite"
	MajorMarks TickMarks
	MinorMarks TickMarks
	// SpecType is "tick spec type": "none" (ticks every Major), "ticks"
	// (at Spec's places, labelled as usual) or "both" (labelled with
	// Spec's own strings). Only the first SpecCount entries count.
	SpecType  string
	SpecCount int
	Spec      []SpecTick
}

// MaxSpecTicks is the number of special ticks gracebat has room for:
// entries 0 to 255.
const MaxSpecTicks = 256

// SpecTick is one special tick: "tick major i, v" or "tick minor i, v",
// and "ticklabel i, \"text\"".
type SpecTick struct {
	Kind     string // "major", "minor", or "" (no tick)
	Value    float64
	Label    string
	HasLabel bool
}

// TickLabels are the numbers written at major ticks.
type TickLabels struct {
	On         bool
	Format     string // "decimal", "power", ...; empty is Grace's default, general
	Prec       int
	Append     string
	Prepend    string
	Angle      float64
	Skip       int
	Stagger    int
	Place      string
	CharSize   float64
	Font       int
	Color      int
	OffsetSpec bool       // "ticklabel offset spec": Offset, not the default 0, 0.01
	Offset     [2]float64 // along the axis, out from it
	// Formula ("ticklabel formula"): each label is this expression of $t,
	// the tick's value, instead of the value (render/formula.go)
	Formula string
	// "ticklabel start/stop type spec": labels only for the ticks whose
	// values are from Start to Stop, before any formula
	StartSpec, StopSpec bool
	Start, Stop         float64
}

// Legend is a graph's legend box.
type Legend struct {
	On          bool
	LocType     string
	X, Y        float64
	BoxColor    int
	BoxPattern  int
	BoxWidth    float64
	BoxStyle    int
	FillColor   int
	FillPattern int
	Font        int
	CharSize    float64
	Color       int
	Length      float64
	VGap, HGap  float64
	Invert      bool
}

// Frame is the box around a graph's viewport.
type Frame struct {
	Type              int
	LineStyle         int
	LineWidth         float64
	Color             int
	Pattern           int
	BackgroundColor   int
	BackgroundPattern int
}

// Set is one data set: how it is drawn and its data.
type Set struct {
	ID       int
	Hidden   bool
	Type     string // "xy", "xydy", ... (see Columns)
	Symbol   Symbol
	Line     Line
	Baseline Baseline
	Dropline bool // a line from the baseline to each point
	Fill     Fill
	ErrorBar ErrorBar
	AValue   AnnotatedValues
	Comment  string
	Legend   string
	// Data holds one row per point: x, y and the errors its type has
	// (see Columns).
	Data [][]float64
	// Labels holds optional per-point quoted text, indexed like Data.
	Labels []string
	// Kept are directives the model has no place for, as read but without
	// "sN", so they stay with the set when renumbered. See Project.Kept.
	Kept []string
}

// AnnotatedValues controls labels next to a set's points. Type is 0 (none),
// 1 (x), 2 (y), 3 (x, y), 4 (point text), or 5 (third numeric column).
type AnnotatedValues struct {
	On              bool
	Type            int
	CharSize        float64
	Font, Color     int
	Rot             float64
	Format          string
	Prec            int
	Prepend, Append string
	Offset          [2]float64
}

// Symbol is how a set's points are marked; Type 0 is none, 1 a circle.
type Symbol struct {
	Type        int
	Size        float64
	Color       int
	Pattern     int
	FillColor   int
	FillPattern int
	LineWidth   float64
	LineStyle   int
	Char        int
	CharFont    int
	Skip        int
}

// Line is how a set's points are joined; Type 0 is none, 1 straight.
type Line struct {
	Type      int
	LineStyle int
	LineWidth float64
	Color     int
	Pattern   int
}

// Baseline is the level a set's fill of type 2 and its droplines start
// from, drawn as a line when On. Type: 0 y = 0, 1 the set's smallest y,
// 2 its largest, 3 the graph's world bottom, 4 its top, 5 the set's mean
// y.
type Baseline struct {
	On   bool
	Type int
}

// Fill is the area a set's line encloses: Type 0 none, 1 the polygon of
// its points, 2 down (or up) to its baseline. Rule 1 is even-odd, 0
// non-zero winding.
type Fill struct {
	Type    int
	Rule    int
	Color   int
	Pattern int
}

// ErrorBar is how a set's errors are drawn (see Set.Errors).
type ErrorBar struct {
	On            bool
	Place         string
	Color         int
	Pattern       int
	Size          float64
	LineWidth     float64
	LineStyle     int
	RiserWidth    float64
	RiserStyle    int
	RiserClip     bool
	RiserClipSize float64
}

// NewSet is set id as Grace makes it when a file says nothing more about
// it (measured from gracebat): a straight line, no symbol, error bars on
// (they show for xydy data), the widths, line styles, symbol size and
// character font from the project's defaults, and a color by its number -
// 1, 2, ... through the color map's colors but white (0), then 1 again.
// colors is how many the map has (Grace's default map: 16).
func NewSet(id int, d Defaults, colors int) *Set {
	if colors < 2 {
		colors = 16
	}
	c := id%(colors-1) + 1
	return &Set{
		ID:     id,
		AValue: AnnotatedValues{Type: 2, CharSize: d.CharSize, Font: d.Font, Color: d.Color, Format: "general", Prec: 3},
		Type:   "xy",
		Line:   Line{Type: 1, LineStyle: d.LineStyle, LineWidth: d.LineWidth, Color: c, Pattern: 1},
		Symbol: Symbol{Size: d.SymbolSize, Color: c, Pattern: 1, FillColor: c, LineWidth: d.LineWidth, LineStyle: d.LineStyle, Char: 'A', CharFont: d.Font},
		Fill:   Fill{Color: 1, Pattern: 1},
		ErrorBar: ErrorBar{On: true, Place: "both", Color: c, Pattern: 1, Size: 1,
			LineWidth: d.LineWidth, LineStyle: d.LineStyle, RiserWidth: d.LineWidth, RiserStyle: d.LineStyle, RiserClipSize: 0.1},
	}
}

// Warning is a line plot-go could not use: unknown, malformed, or a
// feature it does not draw. Like gracebat, reading continues.
type Warning struct {
	Line int
	Text string
	Msg  string
}

// Columns is how many numbers a row of each set type plot-go draws holds:
// x, y, then the errors - one per axis for both sides (dx, dy), or one
// per side, right then left (dxdx) and up then down (dydy). For xysize,
// the third column is symbol size, not an error; xycolor uses it as the
// symbol fill color. xyhilo uses high, low, open and close after x. xyvmap
// uses the last two columns as viewport-space vector offsets. xyboxplot uses
// median, upper/lower box limits and upper/lower whiskers after x.
var Columns = map[string]int{
	"xy": 2, "xydx": 3, "xydy": 3, "xydxdx": 4, "xydydy": 4, "xydxdy": 4, "xydxdxdydy": 6,
	"bar": 2, "bardy": 3, "bardydy": 4, "xysize": 3, "xyhilo": 5, "xycolor": 3, "xyvmap": 4, "xyboxplot": 6,
}

// Errors are a point's error bars, by side, as drawn: world lengths, 0
// where there is none.
type Errors struct{ Right, Left, Up, Down float64 }

// Errors gives the error bars of a row of the set's data, by its type and
// its error bar place (measured from gracebat, see
// docs/grace-behaviour.md): an error given once for an axis is drawn on
// both sides of the point, only the right/upper one with place normal,
// only the left/lower one with opposite; errors given per side are drawn
// as given, swapped with opposite. Their signs do not matter.
func (s *Set) Errors(row []float64) Errors {
	col := func(i int) float64 {
		if i < len(row) {
			return math.Abs(row[i])
		}
		return 0
	}
	one := func(v float64) [2]float64 {
		switch s.ErrorBar.Place {
		case "normal":
			return [2]float64{v, 0}
		case "opposite":
			return [2]float64{0, v}
		}
		return [2]float64{v, v}
	}
	two := func(plus, minus float64) [2]float64 {
		if s.ErrorBar.Place == "opposite" {
			return [2]float64{minus, plus}
		}
		return [2]float64{plus, minus}
	}
	var x, y [2]float64
	switch s.Type {
	case "xydx":
		x = one(col(2))
	case "xydy":
		y = one(col(2))
	case "xydxdy":
		x, y = one(col(2)), one(col(3))
	case "xydxdx":
		x = two(col(2), col(3))
	case "xydydy":
		y = two(col(2), col(3))
	case "xydxdxdydy":
		x, y = two(col(2), col(3)), two(col(4), col(5))
	case "bardy":
		y = one(col(2))
	case "bardydy":
		y = two(col(2), col(3))
	}
	return Errors{Right: x[0], Left: x[1], Up: y[0], Down: y[1]}
}
