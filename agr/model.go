// Package agr reads Grace project files (.agr, .agr-par) - the part of the
// format OneFit writes (see docs/features.md) - into a Project.
//
// Written from the file format and the Grace User's Guide; it contains no
// Grace source code (see AGENTS.md).
package agr

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
	Timestamp       Text
	Strings         []*Text
	Graphs          []*Graph
	Warnings        []Warning
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
	Legend   Legend
	Frame    Frame
	Sets     []*Set
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
	Scale     string // "Normal" or "Logarithmic"
	Invert    bool
	On        bool
	Label     AxisLabel
	Tick      Ticks
	TickLabel TickLabels
}

// AxisLabel is the axis title.
type AxisLabel struct {
	Text     string
	Layout   string
	Place    string
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
}

// TickLabels are the numbers written at major ticks.
type TickLabels struct {
	On       bool
	Format   string // "decimal", "power", ...
	Prec     int
	Append   string
	Prepend  string
	Angle    float64
	Skip     int
	Stagger  int
	Place    string
	CharSize float64
	Font     int
	Color    int
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
	Type     string // "xy" or "xydy"
	Symbol   Symbol
	Line     Line
	ErrorBar ErrorBar
	Comment  string
	Legend   string
	// Data holds one row per point: x, y, and dy for an xydy set.
	Data [][]float64
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

// ErrorBar is how an xydy set's dy is drawn.
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

// Warning is a line plot-go could not use: unknown, malformed, or a
// feature it does not draw. Like gracebat, reading continues.
type Warning struct {
	Line int
	Text string
	Msg  string
}
