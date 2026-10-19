package figure

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// Overlays: graphs drawn over a base graph, sharing one of its axes (x or
// y). The shared axis is kept linked - SyncLinks gives every graph of the
// group the same range, scale, ticks and numbers - and is drawn once, by
// the base, unless an overlaid graph is shifted away from it (OverlayShift)
// and draws its own copy. The other axis of an overlaid graph, its free
// axis, is drawn on the base's opposite side (right for a y axis, top for
// an x one), each further one on that side moved out beyond the one
// before - by what that one draws, its numbers and label - and AxisGap.
// The links are saved in the project (agr.Link).

// AxisGap is the space, in viewport units, between the drawn extent of one
// free axis and the next on the same side.
const AxisGap = 0.02

// free is the other axis.
func free(shared string) string {
	if shared == "x" {
		return "y"
	}
	return "x"
}

func axisOf(g *agr.Graph, name string) *agr.Axis {
	if name == "x" {
		return &g.X
	}
	return &g.Y
}

// Overlays are the graphs overlaid on base, in the figure's order.
func Overlays(fig *agr.Project, base int) []*agr.Graph {
	var over []*agr.Graph
	for _, g := range fig.Graphs {
		if g.Overlay.Shared != "" && g.Overlay.Base == base {
			over = append(over, g)
		}
	}
	return over
}

// Overlay draws graph over on graph base, sharing axis shared ("x" or
// "y"): over takes base's place and base's shared axis, its frame and
// background are not drawn (base's are), its copy of the shared axis is
// off and its free axis moves to the opposite side, beyond those of the
// graphs overlaid there before it. Base's free axis keeps its ticks on its
// own side. An overlay already linked elsewhere is moved.
func Overlay(fig *agr.Project, base, over int, shared string) error {
	if shared != "x" && shared != "y" {
		return fmt.Errorf("the shared axis is x or y, not %q", shared)
	}
	b, o := graph(fig, base), graph(fig, over)
	switch {
	case b == nil:
		return fmt.Errorf("no graph %d", base)
	case o == nil:
		return fmt.Errorf("no graph %d", over)
	case base == over:
		return fmt.Errorf("a graph cannot be overlaid on itself")
	case b.Overlay.Shared != "":
		return fmt.Errorf("graph %d is overlaid on graph %d; overlay on that one", base, b.Overlay.Base)
	case len(Overlays(fig, over)) > 0:
		return fmt.Errorf("graph %d has graphs overlaid on it", over)
	}
	if o.Overlay.Shared != "" {
		Unlink(fig, over)
	}
	resize(fig, o, b.View)
	o.Overlay = agr.Link{Base: base, Shared: shared}
	o.Frame.LineStyle = 0
	o.Frame.BackgroundPattern = 0
	// the base's free axis: ticks on its own side only, the other side is
	// the overlay's
	fb := axisOf(b, free(shared))
	if fb.Tick.Place == "both" {
		fb.Tick.Place = "normal"
	}
	fa := axisOf(o, free(shared))
	fa.On = true
	fa.Tick.Place, fa.TickLabel.Place, fa.Label.Place = "opposite", "opposite", "opposite"
	fa.Bar.On = true
	fa.Bar.Color, fa.Bar.LineStyle, fa.Bar.LineWidth = b.Frame.Color, max(b.Frame.LineStyle, 1), b.Frame.LineWidth
	sa := axisOf(o, shared)
	sa.On = false
	sa.Tick.Place, sa.TickLabel.Place, sa.Label.Place = "normal", "normal", "normal"
	sa.Bar.On = true
	sa.Bar.Color, sa.Bar.LineStyle, sa.Bar.LineWidth = b.Frame.Color, max(b.Frame.LineStyle, 1), b.Frame.LineWidth
	moveTitles(b, o)
	SyncLinks(fig, base, shared)
	ArrangeOverlays(fig)
	separateLegend(fig, o)
	return nil
}

// moveTitles puts overlay o's title and subtitle on base b, which draws
// them once for the group: after b's own, " / " between (not again when b
// already has that text, alone or joined), or as they are when b has none.
// o's are cleared. They are plain texts the user edits as wished.
func moveTitles(b, o *agr.Graph) {
	for _, t := range [][2]*agr.Title{{&b.Title, &o.Title}, {&b.Subtitle, &o.Subtitle}} {
		to, from := t[0], t[1]
		switch {
		case from.Text == "" || slices.Contains(strings.Split(to.Text, " / "), from.Text):
		case to.Text == "":
			to.Text = from.Text
		default:
			to.Text += " / " + from.Text
		}
		from.Text = ""
	}
}

// LegendGap is the space, in viewport units, between an overlay's legend
// and the one above it.
const LegendGap = 0.01

// separateLegend moves graph o's legend, when it covers the legend of
// another graph of its overlay, below the lowest of them, its left edge
// with theirs - legends placed apart by hand stay where they are.
func separateLegend(fig *agr.Project, o *agr.Graph) {
	box := func(d *draw.Drawing, g *agr.Graph) (backend.Box, bool) {
		return part(d, fmt.Sprintf("g%d.legend", g.ID))
	}
	d, _ := render.Render(fig)
	mine, ok := box(d, o)
	if !ok {
		return
	}
	var others []backend.Box
	covered := false
	base := graph(fig, o.Overlay.Base)
	for _, g := range append([]*agr.Graph{base}, Overlays(fig, o.Overlay.Base)...) {
		if g == o || g == nil {
			continue
		}
		if b, ok := box(d, g); ok {
			others = append(others, b)
			covered = covered || b.LLX < mine.URX && mine.LLX < b.URX && b.LLY < mine.URY && mine.LLY < b.URY
		}
	}
	if !covered {
		return
	}
	low := others[0]
	for _, b := range others[1:] {
		if b.LLY < low.LLY {
			low = b
		}
	}
	// the legend's place is a fixed step from its box, whatever the
	// legend holds: move the place by what the box is off
	L := &o.Legend
	if L.LocType == "world" {
		L.LocType = "view"
		L.X, L.Y = mine.LLX, mine.URY
		d, _ = render.Render(fig)
		mine, _ = box(d, o)
	}
	L.X += low.LLX - mine.LLX
	L.Y += low.LLY - LegendGap - mine.URY
}

// part is the box, in viewport units, of what the drawing draws with IDs
// id or starting "id."; false when it draws nothing.
func part(d *draw.Drawing, id string) (backend.Box, bool) {
	sub := &draw.Drawing{Width: d.Width, Height: d.Height}
	for _, p := range d.Paths {
		if p.ID == id || strings.HasPrefix(p.ID, id+".") {
			sub.Paths = append(sub.Paths, p)
		}
	}
	for _, t := range d.Texts {
		if t.ID == id || strings.HasPrefix(t.ID, id+".") {
			sub.Texts = append(sub.Texts, t)
		}
	}
	if len(sub.Paths)+len(sub.Texts) == 0 {
		return backend.Box{}, false
	}
	const unit = 10000
	b := backend.BoundingBox(sub, unit)
	// BoundingBox adds a point of margin each side
	return backend.Box{LLX: (b.LLX + 1) / unit, LLY: (b.LLY + 1) / unit, URX: (b.URX - 1) / unit, URY: (b.URY - 1) / unit}, true
}

// OverlayAll overlays every graph of the figure on its first one, sharing
// axis shared: what the Layout choice "overlay" does.
func OverlayAll(fig *agr.Project, shared string) error {
	if len(fig.Graphs) < 2 {
		return fmt.Errorf("an overlay needs two graphs or more")
	}
	base := fig.Graphs[0]
	if base.Overlay.Shared != "" {
		Unlink(fig, base.ID)
	}
	for _, g := range fig.Graphs[1:] {
		Unlink(fig, g.ID)
	}
	for _, g := range fig.Graphs[1:] {
		if err := Overlay(fig, base.ID, g.ID, shared); err != nil {
			return err
		}
	}
	return nil
}

// Unlink makes an overlaid graph a graph of its own again, where it is,
// with its frame, its shared axis (numbers and label) and its other axis
// on its own side; unlinking a base unlinks the graphs overlaid on it. A
// graph not linked is left as it is.
func Unlink(fig *agr.Project, id int) {
	g := graph(fig, id)
	if g == nil {
		return
	}
	if g.Overlay.Shared == "" {
		for _, o := range Overlays(fig, id) {
			Unlink(fig, o.ID)
		}
		return
	}
	shared := g.Overlay.Shared
	// the group's label of the shared axis - only one copy shows it - is
	// this graph's own again, and the base's if this one had it
	label := ""
	for _, c := range group(fig, id, shared) {
		if t := axisOf(c, shared).Label.Text; t != "" && label == "" {
			label = t
		}
	}
	if base := graph(fig, g.Overlay.Base); base != nil && axisOf(base, shared).Label.Text == "" {
		axisOf(base, shared).Label.Text = label
	}
	g.Overlay = agr.Link{}
	g.Frame.LineStyle = 1
	a := axisOf(g, shared)
	a.On, a.Offset = true, [2]float64{}
	a.Bar.On = false
	a.Tick.On, a.TickLabel.On = true, true
	if a.Label.Text == "" {
		a.Label.Text = label
	}
	// its free axis back on its own side, at the frame
	fa := axisOf(g, free(shared))
	fa.Tick.Place, fa.TickLabel.Place, fa.Label.Place = "both", "normal", "normal"
	fa.Offset, fa.Bar.On = [2]float64{}, false
	ArrangeOverlays(fig)
}

// OverlayShift moves an overlaid graph away from its base by d viewport
// units at right angles to the shared axis - down for a shared x, left for
// a shared y - so it draws its own copy of the shared axis, still linked;
// 0 puts it back exactly over the base, the copy off.
func OverlayShift(fig *agr.Project, id int, d float64) error {
	g := graph(fig, id)
	if g == nil {
		return fmt.Errorf("no graph %d", id)
	}
	if g.Overlay.Shared == "" {
		return fmt.Errorf("graph %d is not overlaid on another", id)
	}
	if math.IsNaN(d) || d < 0 || d > 0.2 {
		return fmt.Errorf("the shift is 0 to 0.2")
	}
	b := graph(fig, g.Overlay.Base)
	v := b.View
	if g.Overlay.Shared == "x" {
		v.YMin, v.YMax = v.YMin-d, v.YMax-d
	} else {
		v.XMin, v.XMax = v.XMin-d, v.XMax-d
	}
	g.View = v
	axisOf(g, g.Overlay.Shared).On = d > 0
	ArrangeOverlays(fig)
	return nil
}

// ArrangeOverlays places what the overlays of every group draw outside the
// base: each free axis beyond the one before it on its side (by what that
// one draws, its numbers and label, and AxisGap), and the numbers and label
// of a shared axis on its outermost copy - the graph shifted furthest, or
// the base - the other copies drawing their line and ticks only. Edits to
// the axes of a group (sizes, labels, formats) call for it again.
func ArrangeOverlays(fig *agr.Project) {
	for _, b := range fig.Graphs {
		over := Overlays(fig, b.ID)
		if b.Overlay.Shared != "" || len(over) == 0 {
			continue
		}
		for _, shared := range []string{"x", "y"} {
			sharedNumbers(fig, b, shared)
		}
		// what each free axis draws, measured where it is drawn now, before
		// any moves: an axis's reach does not depend on its offset
		d, _ := render.Render(fig)
		for _, shared := range []string{"x", "y"} {
			out := 0.0 // how far the last free axis on the side reaches
			first := true
			for _, o := range over {
				if o.Overlay.Shared != shared {
					continue
				}
				a := axisOf(o, free(shared))
				r := reach(d, o, free(shared), a.Offset[1])
				if first {
					a.Offset[1], first = 0, false
				} else {
					a.Offset[1] = out + AxisGap
				}
				out = a.Offset[1] + r
			}
		}
	}
}

// Reach is how far beyond the base's frame the free axes of its overlays
// on the side of the overlays sharing axis shared are drawn, in viewport
// units; 0 when none is.
func Reach(fig *agr.Project, base int, shared string) float64 {
	var r float64
	var d *draw.Drawing
	for _, o := range Overlays(fig, base) {
		if o.Overlay.Shared == shared {
			if d == nil {
				d, _ = render.Render(fig)
			}
			a := axisOf(o, free(shared))
			r = max(r, a.Offset[1]+reach(d, o, free(shared), a.Offset[1]))
		}
	}
	return r
}

// reach is how far out from where it is drawn (its frame edge, offset out
// by off) graph g's axis name, on the opposite side, reaches with all it
// draws there.
func reach(d *draw.Drawing, g *agr.Graph, name string, off float64) float64 {
	box, ok := part(d, fmt.Sprintf("g%d.%s", g.ID, name))
	if !ok {
		return 0
	}
	if name == "y" {
		return max(0, box.URX-g.View.XMax-off)
	}
	return max(0, box.URY-g.View.YMax-off)
}

// carrier is the copy of base b's shared axis that draws its numbers and
// label: the overlay shifted furthest whose copy is on, else the base.
func carrier(fig *agr.Project, b *agr.Graph, shared string) *agr.Graph {
	outer, far := b, 0.0
	for _, o := range Overlays(fig, b.ID) {
		if o.Overlay.Shared == shared && axisOf(o, shared).On && Shift(fig, o.ID) > far {
			outer, far = o, Shift(fig, o.ID)
		}
	}
	return outer
}

// sharedNumbers puts the numbers and label of the group's shared axis on
// its outermost copy; the label text moves with them, never lost. Numbers
// switched off by the user stay off.
func sharedNumbers(fig *agr.Project, b *agr.Graph, shared string) {
	copies := []*agr.Graph{b}
	for _, o := range Overlays(fig, b.ID) {
		if o.Overlay.Shared == shared {
			copies = append(copies, o)
		}
	}
	if len(copies) < 2 {
		return
	}
	outer := carrier(fig, b, shared)
	numbers, label := false, ""
	for _, g := range copies {
		a := axisOf(g, shared)
		numbers = numbers || a.TickLabel.On
		if label == "" {
			label = a.Label.Text // all the same after SyncLinks
		}
	}
	for _, g := range copies {
		a := axisOf(g, shared)
		if g == outer {
			a.TickLabel.On, a.Label.Text = numbers, label
		} else {
			a.TickLabel.On, a.Label.Text = false, ""
		}
	}
}

// Shift is how far an overlaid graph is moved off its base (OverlayShift),
// 0 for one exactly over it or not overlaid.
func Shift(fig *agr.Project, id int) float64 {
	g := graph(fig, id)
	if g == nil || g.Overlay.Shared == "" {
		return 0
	}
	b := graph(fig, g.Overlay.Base)
	if b == nil {
		return 0
	}
	if g.Overlay.Shared == "x" {
		return b.View.YMin - g.View.YMin
	}
	return b.View.XMin - g.View.XMin
}

// group is the graphs that share axis name with graph id: its base and the
// base's overlays sharing that axis; nil when the axis is not shared.
func group(fig *agr.Project, id int, name string) []*agr.Graph {
	g := graph(fig, id)
	if g == nil {
		return nil
	}
	base := g
	if g.Overlay.Shared != "" {
		if g.Overlay.Shared != name {
			return nil
		}
		base = graph(fig, g.Overlay.Base)
		if base == nil {
			return nil
		}
	}
	out := []*agr.Graph{base}
	for _, o := range Overlays(fig, base.ID) {
		if o.Overlay.Shared == name {
			out = append(out, o)
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// Linked are the graphs whose axis name is linked with graph id's, id's
// own included; nil when it is not linked.
func Linked(fig *agr.Project, id int, name string) []int {
	var ids []int
	for _, g := range group(fig, id, name) {
		ids = append(ids, g.ID)
	}
	return ids
}

// SyncLinks gives every graph sharing axis name with graph id what id's
// axis has: its range, scale, inversion, ticks, numbers, color and label.
// What each graph keeps of its own: whether its copy is drawn (On), where
// (the axis offset and the sides of its ticks, numbers and label), and
// whether it draws ticks, a line and the numbers. The label's text is the
// group's, set by the copy edited: by the copy that draws it (carrier)
// always - cleared there, it is cleared - and by another copy when it has
// a text, so a label typed on a copy that does not draw it is the label
// too. ArrangeOverlays then leaves the text on the copy that draws it only.
// Nothing is done when the axis is not shared.
func SyncLinks(fig *agr.Project, id int, name string) {
	from := graph(fig, id)
	members := group(fig, id, name)
	if from == nil || members == nil {
		return
	}
	// the label: what the copy edited has - when it is the copy that draws
	// it, even nothing (cleared); else what that copy has, unless it has
	// none (an edit of something else on a copy without the label)
	label := axisOf(from, name).Label.Text
	if c := carrier(fig, members[0], name); from != c && label == "" {
		label = axisOf(c, name).Label.Text
	}
	for _, g := range members {
		axisOf(g, name).Label.Text = label
		if g == from {
			continue
		}
		if name == "x" {
			g.World.XMin, g.World.XMax = from.World.XMin, from.World.XMax
		} else {
			g.World.YMin, g.World.YMax = from.World.YMin, from.World.YMax
		}
		src, dst := axisOf(from, name), axisOf(g, name)
		keep := *dst
		*dst = *src
		dst.On, dst.Offset, dst.Zero = keep.On, keep.Offset, keep.Zero
		dst.Bar.On, dst.Bar.LineStyle, dst.Bar.LineWidth = keep.Bar.On, keep.Bar.LineStyle, keep.Bar.LineWidth
		dst.Label.Place, dst.Label.Spec, dst.Label.SpecAt = keep.Label.Place, keep.Label.Spec, keep.Label.SpecAt
		dst.Tick.Place, dst.TickLabel.Place = keep.Tick.Place, keep.TickLabel.Place
		dst.TickLabel.On = keep.TickLabel.On // which copy has the numbers: sharedNumbers
		dst.Tick.On = keep.Tick.On           // a copy drawn without ticks stays so
		dst.Tick.Spec = append([]agr.SpecTick(nil), src.Tick.Spec...)
	}
}

// ShareLabel is SyncLinks after an edit of the label of graph id's axis
// name: its text, even none (cleared), becomes the label of every graph
// sharing the axis, whichever copy was edited - where SyncLinks, after an
// edit of something else, keeps the label when the copy edited has none.
// ArrangeOverlays then leaves it on the copy that draws it. Nothing is done
// when the axis is not shared.
func ShareLabel(fig *agr.Project, id int, name string) {
	from := graph(fig, id)
	if from == nil {
		return
	}
	text := axisOf(from, name).Label.Text
	for _, g := range group(fig, id, name) {
		axisOf(g, name).Label.Text = text
	}
	SyncLinks(fig, id, name)
}

// AxisColor colors an axis whole - its line, ticks, numbers and label -
// and, when it is shared, the same axis of the graphs it is linked with.
func AxisColor(fig *agr.Project, id int, name string, color int) error {
	g := graph(fig, id)
	if g == nil {
		return fmt.Errorf("no graph %d", id)
	}
	if err := UseColor(fig, color); err != nil {
		return err
	}
	a := axisOf(g, name)
	a.Bar.Color, a.Tick.MajorMarks.Color, a.Tick.MinorMarks.Color = color, color, color
	a.TickLabel.Color, a.Label.Color = color, color
	SyncLinks(fig, id, name)
	return nil
}

// Candidate is a graph that looks overlaid on another without a link - an
// overlay made in Grace: Over sits exactly over Base and has its range and
// scale on axis Shared.
type Candidate struct {
	Over   int    `json:"over"`
	Base   int    `json:"base"`
	Shared string `json:"shared"`
}

// Candidates are the figure's unlinked overlays, for an editor to offer
// linking (Link): each graph without a link over an earlier one that is
// not itself overlaid, at the same place, sharing the range and scale of
// an axis. When both axes match, the shared one is the axis whose other
// axis the graph draws on the opposite side, else x.
func Candidates(fig *agr.Project) []Candidate {
	var out []Candidate
	near := func(a, b float64) bool { return math.Abs(a-b) <= 1e-6*max(1, math.Abs(a), math.Abs(b)) }
	sameView := func(a, b agr.Rect) bool {
		return near(a.XMin, b.XMin) && near(a.XMax, b.XMax) && near(a.YMin, b.YMin) && near(a.YMax, b.YMax)
	}
	same := func(a, b *agr.Graph, name string) bool {
		if name == "x" {
			return near(a.World.XMin, b.World.XMin) && near(a.World.XMax, b.World.XMax) && a.X.Scale == b.X.Scale && a.X.Invert == b.X.Invert
		}
		return near(a.World.YMin, b.World.YMin) && near(a.World.YMax, b.World.YMax) && a.Y.Scale == b.Y.Scale && a.Y.Invert == b.Y.Invert
	}
	linked := map[int]bool{} // a graph a candidate overlays: not a base too
	for i, o := range fig.Graphs {
		if o.Overlay.Shared != "" || len(Overlays(fig, o.ID)) > 0 || o.Hidden {
			continue
		}
		for _, b := range fig.Graphs[:i] {
			if b.Overlay.Shared != "" || linked[b.ID] || b.Hidden || !sameView(o.View, b.View) {
				continue
			}
			sx, sy := same(o, b, "x"), same(o, b, "y")
			shared := ""
			switch {
			case sx && sy:
				shared = "x"
				if o.X.TickLabel.Place == "opposite" && o.Y.TickLabel.Place != "opposite" {
					shared = "y"
				}
			case sx:
				shared = "x"
			case sy:
				shared = "y"
			default:
				continue
			}
			out = append(out, Candidate{Over: o.ID, Base: b.ID, Shared: shared})
			linked[o.ID] = true
			break
		}
	}
	return out
}

// Link links graph over to graph base, sharing axis shared, as an overlay
// already drawn as wanted (one made in Grace): the link is added, over
// takes base's shared axis (SyncLinks) and the axis's numbers and label are
// left to the base's copy, where the two coincide; frames, the other axes
// and the places of everything stay as they are.
func Link(fig *agr.Project, base, over int, shared string) error {
	if shared != "x" && shared != "y" {
		return fmt.Errorf("the shared axis is x or y, not %q", shared)
	}
	b, o := graph(fig, base), graph(fig, over)
	switch {
	case b == nil:
		return fmt.Errorf("no graph %d", base)
	case o == nil:
		return fmt.Errorf("no graph %d", over)
	case base == over:
		return fmt.Errorf("a graph cannot be overlaid on itself")
	case b.Overlay.Shared != "":
		return fmt.Errorf("graph %d is overlaid on graph %d; link to that one", base, b.Overlay.Base)
	case o.Overlay.Shared != "":
		return fmt.Errorf("graph %d is already overlaid on graph %d", over, o.Overlay.Base)
	case len(Overlays(fig, over)) > 0:
		return fmt.Errorf("graph %d has graphs overlaid on it", over)
	}
	o.Overlay = agr.Link{Base: base, Shared: shared}
	if ba, oa := axisOf(b, shared), axisOf(o, shared); ba.Label.Text == "" {
		ba.Label.Text = oa.Label.Text // the overlay's label, if only it has one
	}
	SyncLinks(fig, base, shared)
	sharedNumbers(fig, b, shared)
	return nil
}
