// Package figure builds figures: Grace projects of their own, composed from
// the plots OneFit writes for its fits (fit-curves-N.agr), kept apart from
// them so a refit never overwrites one. A figure is self-contained - it
// holds copies of its data, so it draws, and opens in xmgrace, even when
// the fits are gone - and each set remembers where its data came from, in
// its comment ("src: batch-x/fit-y/fit-curves-3.agr#G0.S1"), so Update can
// bring in newer data while keeping everything else.
package figure

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

const srcPrefix = "src: "

// Ref names a set of a project: the project's path (as the caller wants
// it recorded - usually relative to the folder holding the fits and the
// figures folder) and the set's graph and number.
type Ref struct {
	Path       string
	Graph, Set int
}

func (r Ref) String() string { return fmt.Sprintf("%s%s#G%d.S%d", srcPrefix, r.Path, r.Graph, r.Set) }

// SourceOf is the source a set's comment records, if any.
func SourceOf(s *agr.Set) (Ref, bool) {
	c, ok := strings.CutPrefix(s.Comment, srcPrefix)
	if !ok {
		return Ref{}, false
	}
	i := strings.LastIndex(c, "#G")
	if i < 0 {
		return Ref{}, false
	}
	g, st, found := strings.Cut(c[i+2:], ".S")
	if !found {
		return Ref{}, false
	}
	gn, err1 := strconv.Atoi(g)
	sn, err2 := strconv.Atoi(st)
	if err1 != nil || err2 != nil || c[:i] == "" {
		return Ref{}, false
	}
	return Ref{Path: c[:i], Graph: gn, Set: sn}, true
}

func graph(p *agr.Project, id int) *agr.Graph {
	for _, g := range p.Graphs {
		if g.ID == id {
			return g
		}
	}
	return nil
}

// FromPlot makes a figure from a plot: the project itself, each set's
// comment naming it as the source (path as given).
func FromPlot(plot *agr.Project, path string) *agr.Project {
	for _, g := range plot.Graphs {
		for _, s := range g.Sets {
			s.Comment = Ref{Path: path, Graph: g.ID, Set: s.ID}.String()
		}
	}
	plot.Warnings = nil
	return plot
}

// AddSets copies sets - style and data - from graph 0 of src (recorded as
// path) into graph g of the figure, as new sets numbered after the
// figure's last; the colors they use come along when the figure's color
// map lacks them. It returns the new sets' numbers.
func AddSets(fig *agr.Project, g int, src *agr.Project, path string, sets []int) ([]int, error) {
	return addSets(fig, g, src, path, sets, newImporter(fig, src))
}

func addSets(fig *agr.Project, g int, src *agr.Project, path string, sets []int, in *importer) ([]int, error) {
	dst := graph(fig, g)
	if dst == nil {
		return nil, fmt.Errorf("the figure has no graph %d", g)
	}
	from := graph(src, 0)
	if from == nil {
		return nil, fmt.Errorf("%s has no graph 0", path)
	}
	next := 0
	for _, s := range dst.Sets {
		next = max(next, s.ID+1)
	}
	var added []int
	for _, id := range sets {
		var s *agr.Set
		for _, c := range from.Sets {
			if c.ID == id {
				s = c
			}
		}
		if s == nil {
			return added, fmt.Errorf("%s has no set %d", path, id)
		}
		c := *s
		c.Kept = append([]string(nil), s.Kept...)
		c.Data = make([][]float64, len(s.Data))
		for i, row := range s.Data {
			c.Data[i] = append([]float64(nil), row...)
		}
		c.ID = next
		c.Comment = Ref{Path: path, Graph: 0, Set: id}.String()
		in.remap(&c)
		dst.Sets = append(dst.Sets, &c)
		added = append(added, next)
		next++
	}
	return added, nil
}

// importer brings a plot's colors and fonts into a figure by what they
// are, not by number: a color or font the figure maps to the same thing
// keeps its number; one the figure lacks comes in under it; one whose
// number the figure uses for something else comes in under a free number,
// and what is imported is renumbered to it - so a plot keeps its own
// colors, and the figure's sets keep theirs.
type importer struct {
	fig, src      *agr.Project
	colors, fonts map[int]int
}

func newImporter(fig, src *agr.Project) *importer {
	return &importer{fig: fig, src: src, colors: map[int]int{}, fonts: map[int]int{}}
}

func (in *importer) color(n int) int {
	if m, ok := in.colors[n]; ok {
		return m
	}
	def, ok := in.src.Colors[n]
	if !ok {
		def, ok = DefaultColors[n]
	}
	m := n
	if ok {
		if in.fig.Colors == nil {
			in.fig.Colors = map[int]agr.Color{}
		}
		same := func(c agr.Color) bool { return c.R == def.R && c.G == def.G && c.B == def.B }
		if have, used := in.fig.Colors[n]; !used {
			in.fig.Colors[n] = def
		} else if !same(have) {
			m = -1
			for _, k := range sortedInts(in.fig.Colors) {
				if same(in.fig.Colors[k]) {
					m = k
					break
				}
			}
			if m < 0 {
				m = nextKey(in.fig.Colors)
				in.fig.Colors[m] = def
			}
		}
	}
	in.colors[n] = m
	return m
}

func (in *importer) font(n int) int {
	if m, ok := in.fonts[n]; ok {
		return m
	}
	m := n
	if def, ok := in.src.Fonts[n]; ok {
		if in.fig.Fonts == nil {
			in.fig.Fonts = map[int]agr.Font{}
		}
		if have, used := in.fig.Fonts[n]; !used {
			in.fig.Fonts[n] = def
		} else if have.Name != def.Name {
			m = -1
			for _, k := range sortedInts(in.fig.Fonts) {
				if in.fig.Fonts[k].Name == def.Name {
					m = k
					break
				}
			}
			if m < 0 {
				m = nextKey(in.fig.Fonts)
				in.fig.Fonts[m] = def
			}
		}
	}
	in.fonts[n] = m
	return m
}

// remap renumbers every color and font an imported element uses: each int
// field named ...Color or ...Font, in it and in what it holds (a set; a
// graph's axes, frame, legend and titles; a text).
func (in *importer) remap(v any) {
	var walk func(r reflect.Value)
	walk = func(r reflect.Value) {
		switch r.Kind() {
		case reflect.Pointer:
			if !r.IsNil() {
				walk(r.Elem())
			}
		case reflect.Slice:
			// a set's data rows and kept lines hold no colors
			if k := r.Type().Elem().Kind(); k != reflect.Pointer && k != reflect.Struct {
				return
			}
			for i := 0; i < r.Len(); i++ {
				walk(r.Index(i))
			}
		case reflect.Struct:
			for i := 0; i < r.NumField(); i++ {
				f, name := r.Field(i), r.Type().Field(i).Name
				switch {
				case f.Kind() == reflect.Int && strings.HasSuffix(name, "Color"):
					f.SetInt(int64(in.color(int(f.Int()))))
				case f.Kind() == reflect.Int && strings.HasSuffix(name, "Font"):
					f.SetInt(int64(in.font(int(f.Int()))))
				default:
					walk(f)
				}
			}
		}
	}
	walk(reflect.ValueOf(v))
}

func sortedInts[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func nextKey[V any](m map[int]V) int {
	n := 0
	for k := range m {
		n = max(n, k+1)
	}
	return n
}

// Update brings each set's data up to date from its source, through load
// (which reads a recorded path), keeping the set's style and everything
// else in the figure. A source that cannot be read, or no longer has the
// set, is a warning and the set keeps its data. It returns how many sets
// were updated.
func Update(fig *agr.Project, load func(path string) (*agr.Project, error)) (int, []string) {
	cache := map[string]*agr.Project{}
	var warnings []string
	updated := 0
	for _, g := range fig.Graphs {
		for _, s := range g.Sets {
			ref, ok := SourceOf(s)
			if !ok {
				continue
			}
			src, seen := cache[ref.Path]
			if !seen {
				p, err := load(ref.Path)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("set %d: %s: %v; its data is kept", s.ID, ref.Path, err))
				}
				cache[ref.Path], src = p, p
			}
			if src == nil {
				continue
			}
			sg := graph(src, ref.Graph)
			var from *agr.Set
			if sg != nil {
				for _, c := range sg.Sets {
					if c.ID == ref.Set {
						from = c
					}
				}
			}
			if from == nil {
				warnings = append(warnings, fmt.Sprintf("set %d: %s has no G%d.S%d; its data is kept", s.ID, ref.Path, ref.Graph, ref.Set))
				continue
			}
			// the data's columns are the source's type's: a set whose
			// source changed type (an xy fit re-run with x errors) follows it
			if s.Type != from.Type {
				warnings = append(warnings, fmt.Sprintf("set %d: its source's type changed from %s to %s; the set follows it", s.ID, s.Type, from.Type))
				s.Type = from.Type
			}
			s.Data = make([][]float64, len(from.Data))
			for i, row := range from.Data {
				s.Data[i] = append([]float64(nil), row...)
			}
			updated++
		}
	}
	return updated, warnings
}

// AddGraph adds graph 0 of a plot (recorded as path) to the figure as a new
// graph - its axes, frame and sets (data copied, each remembering its
// source), and its world-coordinate texts and drawing objects - numbered after
// the figure's last. The page's own texts (the run's name) stay out. The
// new graph keeps the plot's viewport; Layout places the graphs.
func AddGraph(fig *agr.Project, src *agr.Project, path string) (int, error) {
	from := graph(src, 0)
	if from == nil {
		return 0, fmt.Errorf("%s has no graph 0", path)
	}
	id := 0
	for _, g := range fig.Graphs {
		id = max(id, g.ID+1)
	}
	in := newImporter(fig, src)
	g := *from
	g.ID = id
	g.Sets = nil
	in.remap(&g)
	fig.Graphs = append(fig.Graphs, &g)
	sets := make([]int, 0, len(from.Sets))
	for _, s := range from.Sets {
		sets = append(sets, s.ID)
	}
	if _, err := addSets(fig, id, src, path, sets, in); err != nil {
		return id, err
	}
	// AddSets numbers sets after the graph's last; a new graph starts at 0,
	// as the plot's own numbering did
	for _, s := range src.Strings {
		if s.LocType == "world" && s.Graph == 0 {
			c := *s
			c.Graph = id
			in.remap(&c)
			fig.Strings = append(fig.Strings, &c)
		}
	}
	for i, s := range src.Shapes {
		if s.LocType == "world" && (s.Graph == 0 || s.Graph == -1) {
			c := fig.CopyShapeFrom(src, i)
			c.Graph = id
			in.remap(c)
		}
	}
	return id, nil
}

// RemoveGraph takes a graph and its world-coordinate texts and objects out,
// and places the others again: the last one left gets back the place of a
// one-graph plot ("single", OneFit's own, as before graphs were added),
// several keep their arrangement - stacked, side by side or a grid - and
// close the gap.
func RemoveGraph(fig *agr.Project, id int) error {
	layout := arrangement(fig)
	if err := removeGraph(fig, id); err != nil {
		return err
	}
	if len(fig.Graphs) == 1 {
		layout = "single"
	}
	return Layout(fig, layout)
}

// arrangement is how the figure's graphs sit: "stacked" when they share
// their left edge, "side-by-side" when they share their top, else "grid".
func arrangement(fig *agr.Project) string {
	same := func(v func(agr.Rect) float64) bool {
		for _, g := range fig.Graphs {
			if math.Abs(v(g.View)-v(fig.Graphs[0].View)) > 1e-6 {
				return false
			}
		}
		return true
	}
	switch {
	case len(fig.Graphs) < 2:
		return "single"
	case same(func(r agr.Rect) float64 { return r.XMin }):
		return "stacked"
	case same(func(r agr.Rect) float64 { return r.YMax }):
		return "side-by-side"
	}
	return "grid"
}

func removeGraph(fig *agr.Project, id int) error {
	if len(fig.Graphs) == 1 {
		return fmt.Errorf("a figure keeps at least one graph")
	}
	i := -1
	for k, g := range fig.Graphs {
		if g.ID == id {
			i = k
		}
	}
	if i < 0 {
		return fmt.Errorf("no graph %d", id)
	}
	fig.Graphs = append(fig.Graphs[:i], fig.Graphs[i+1:]...)
	kept := fig.Strings[:0]
	for _, s := range fig.Strings {
		if !(s.LocType == "world" && s.Graph == id) {
			kept = append(kept, s)
		}
	}
	fig.Strings = kept
	for k := len(fig.Shapes) - 1; k >= 0; k-- {
		s := fig.Shapes[k]
		if s.LocType == "world" && (s.Graph == id || id == 0 && s.Graph == -1) {
			fig.DeleteShape(k)
		}
	}
	return nil
}

// Layouts are the arrangements Layout knows.
var Layouts = []string{"single", "side-by-side", "stacked", "grid"}

// Layout places the figure's graphs on the page, in order: one per row of a
// column ("stacked"), one per column of a row ("side-by-side"), in a grid
// as square as it can be ("grid"), or each over the whole page ("single",
// what OneFit's one-graph plots use). Margins leave room for the axes'
// numbers and labels.
func Layout(fig *agr.Project, layout string) error {
	n := len(fig.Graphs)
	if n == 0 {
		return fmt.Errorf("the figure has no graphs")
	}
	w, h := fig.PageWidth, fig.PageHeight
	if w <= 0 || h <= 0 {
		w, h = 773, 600
	}
	unit := min(w, h)
	pw, ph := w/unit, h/unit // the page in viewport units
	var cols, rows int
	switch layout {
	case "single":
		for _, g := range fig.Graphs {
			// OneFit's own one-graph viewport, on a page its shape
			resize(fig, g, agr.Rect{XMin: 0.183247 * pw / (773.0 / 600), YMin: 0.149994 * ph, XMax: 1.038397 * pw / (773.0 / 600), YMax: 0.849964 * ph})
		}
		return nil
	case "side-by-side":
		cols, rows = n, 1
	case "stacked":
		cols, rows = 1, n
	case "grid":
		cols = 1
		for cols*cols < n {
			cols++
		}
		rows = (n + cols - 1) / cols
	default:
		return fmt.Errorf("unknown layout %q (%s)", layout, strings.Join(Layouts, ", "))
	}
	// left for the y numbers and label, bottom for the x ones and below
	// them the run's name OneFit puts at (0.05, 0.05) - the outer margins
	// are OneFit's own (its viewport starts at 0.18, 0.15); the gaps between
	// panels hold the next panel's numbers and labels
	const left, right, bottom, top, gapX, gapY = 0.18, 0.04, 0.15, 0.05, 0.13, 0.11
	cw := (pw - left - right - gapX*float64(cols-1)) / float64(cols)
	ch := (ph - bottom - top - gapY*float64(rows-1)) / float64(rows)
	if cw <= 0.05 || ch <= 0.05 {
		return fmt.Errorf("%d graphs do not fit %s on this page", n, layout)
	}
	for i, g := range fig.Graphs {
		c, r := i%cols, i/cols
		x0 := left + float64(c)*(cw+gapX)
		y1 := ph - top - float64(r)*(ch+gapY)
		resize(fig, g, agr.Rect{XMin: x0, YMin: y1 - ch, XMax: x0 + cw, YMax: y1})
	}
	return nil
}

// resize gives a graph a new viewport and scales what is drawn at a size of
// its own - axis numbers and labels, ticks, symbols, the texts placed in it
// - by the square root of the change in area (kept between 1/2 and 2), so a
// smaller panel is not crowded. It is relative to the graph's size before,
// so going back to a bigger layout restores the sizes.
func resize(fig *agr.Project, g *agr.Graph, v agr.Rect) {
	old := (g.View.XMax - g.View.XMin) * (g.View.YMax - g.View.YMin)
	area := (v.XMax - v.XMin) * (v.YMax - v.YMin)
	g.View = v
	if old <= 0 || area <= 0 {
		return
	}
	f := math.Sqrt(area / old)
	f = min(max(f, 0.5), 2)
	if math.Abs(f-1) < 1e-9 {
		return
	}
	for _, a := range []*agr.Axis{&g.X, &g.Y} {
		a.Label.CharSize *= f
		a.TickLabel.CharSize *= f
		a.Tick.MajorMarks.Size *= f
		a.Tick.MinorMarks.Size *= f
	}
	for _, s := range g.Sets {
		s.Symbol.Size *= f
	}
	for _, s := range fig.Strings {
		if s.LocType == "world" && s.Graph == g.ID {
			s.CharSize *= f
		}
	}
}

// palette is the order Distinct takes new colors in: Grace's default map
// without white, yellow and grey (too light on white paper) and black (what
// OneFit draws the first fit in).
var palette = []int{2, 4, 15, 8, 11, 10, 13, 14, 12, 6, 3, 9}

// DefaultColors is Grace's default color map, the one OneFit's plots carry:
// what a figure may use without defining colors of its own.
var DefaultColors = map[int]agr.Color{
	0: {R: 255, G: 255, B: 255, Name: "white"}, 1: {Name: "black"},
	2: {R: 255, Name: "red"}, 3: {G: 255, Name: "green"}, 4: {B: 255, Name: "blue"},
	5: {R: 255, G: 255, Name: "yellow"}, 6: {R: 188, G: 143, B: 143, Name: "brown"},
	7: {R: 220, G: 220, B: 220, Name: "grey"}, 8: {R: 148, B: 211, Name: "violet"},
	9: {G: 255, B: 255, Name: "cyan"}, 10: {R: 255, B: 255, Name: "magenta"},
	11: {R: 255, G: 165, Name: "orange"}, 12: {R: 114, G: 33, B: 188, Name: "indigo"},
	13: {R: 103, G: 7, B: 72, Name: "maroon"}, 14: {R: 64, G: 224, B: 208, Name: "turquoise"},
	15: {G: 139, Name: "green4"},
}

// UseColor makes sure color n is in the figure's map, taking it from
// DefaultColors when the figure lacks it; a color in neither is an error.
func UseColor(fig *agr.Project, n int) error {
	if _, ok := fig.Colors[n]; ok {
		return nil
	}
	c, ok := DefaultColors[n]
	if !ok {
		return fmt.Errorf("no color %d (Grace's are 0 to 15)", n)
	}
	if fig.Colors == nil {
		fig.Colors = map[int]agr.Color{}
	}
	fig.Colors[n] = c
	return nil
}

// Distinct draws sets of graph g - the ones a plot just brought in - in one
// color no other set of the graph uses, symbols, lines and error bars alike,
// so each fit reads as one color. It returns the color (-1 when every
// palette color is taken, and nothing changes).
func Distinct(fig *agr.Project, g int, sets []int) (int, error) {
	gr := graph(fig, g)
	if gr == nil {
		return -1, fmt.Errorf("the figure has no graph %d", g)
	}
	mine := map[int]bool{}
	for _, id := range sets {
		mine[id] = true
	}
	used := map[int]bool{}
	for _, s := range gr.Sets {
		if !mine[s.ID] {
			for _, c := range []int{s.Line.Color, s.Symbol.Color, s.Symbol.FillColor, s.ErrorBar.Color} {
				used[c] = true
			}
		}
	}
	for _, c := range palette {
		if used[c] {
			continue
		}
		for _, s := range gr.Sets {
			if mine[s.ID] {
				s.Line.Color, s.Symbol.Color, s.Symbol.FillColor, s.ErrorBar.Color = c, c, c, c
			}
		}
		if err := UseColor(fig, c); err != nil {
			return -1, err
		}
		return c, nil
	}
	return -1, nil
}

// Extend widens graph g's axes, when needed, to show all the data of the
// given sets (error bars included), with a margin of 5% of the new span -
// on a logarithmic axis, out to whole decades instead, values that are not
// positive left out. A linear axis that grows gets a new major tick step keeping
// about as many ticks as before. It reports whether anything changed.
func Extend(fig *agr.Project, g int, sets []int) (bool, error) {
	gr := graph(fig, g)
	if gr == nil {
		return false, fmt.Errorf("the figure has no graph %d", g)
	}
	mine := map[int]bool{}
	for _, id := range sets {
		mine[id] = true
	}
	xlog, ylog := gr.X.Scale == "Logarithmic", gr.Y.Scale == "Logarithmic"
	xlo, xhi, ylo, yhi := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, s := range gr.Sets {
		if !mine[s.ID] {
			continue
		}
		for _, row := range s.Data {
			if len(row) < 2 {
				continue
			}
			x, y := row[0], row[1]
			var e agr.Errors
			if s.ErrorBar.On {
				e = s.Errors(row)
			}
			// the span from v-minus to v+plus; on a log axis an end at or
			// below 0 (drawn to the frame) is left out
			span := func(v, minus, plus float64, log bool) (float64, float64, bool) {
				lo, hi := v-minus, v+plus
				if log && lo <= 0 {
					lo = v
				}
				ok := !math.IsNaN(lo) && !math.IsInf(lo, 0) && !math.IsNaN(hi) && !math.IsInf(hi, 0) && (!log || lo > 0)
				return lo, hi, ok
			}
			x0, x1, okx := span(x, e.Left, e.Right, xlog)
			y0, y1, oky := span(y, e.Down, e.Up, ylog)
			if !okx || !oky {
				continue
			}
			xlo, xhi = min(xlo, x0), max(xhi, x1)
			ylo, yhi = min(ylo, y0), max(yhi, y1)
		}
	}
	if xlo > xhi {
		return false, nil
	}
	w := &gr.World
	cx := grow(&w.XMin, &w.XMax, xlo, xhi, xlog, &gr.X)
	cy := grow(&w.YMin, &w.YMax, ylo, yhi, ylog, &gr.Y)
	return cx || cy, nil
}

// grow widens [*a, *b] (in either order - an inverted range keeps its
// order) to hold [lo, hi].
func grow(a, b *float64, lo, hi float64, log bool, ax *agr.Axis) bool {
	wlo, whi := min(*a, *b), max(*a, *b)
	if lo >= wlo && hi <= whi {
		return false
	}
	nlo, nhi := min(wlo, lo), max(whi, hi)
	if log {
		if wlo <= 0 {
			nlo = lo
		}
		l, h := math.Log10(nlo), math.Log10(nhi)
		// out to whole decades, as OneFit's own plots end; data on a decade
		// ends there
		if lo < wlo || wlo <= 0 {
			nlo = math.Pow(10, math.Floor(l+1e-9))
		}
		if hi > whi {
			nhi = math.Pow(10, math.Ceil(h-1e-9))
		}
	} else {
		m := 0.05 * (nhi - nlo)
		if m == 0 {
			m = math.Max(math.Abs(nhi), 1) * 0.05
		}
		if lo < wlo {
			nlo -= m
		}
		if hi > whi {
			nhi += m
		}
		if ax.Tick.Major > 0 {
			n := max(math.Round((whi-wlo)/ax.Tick.Major), 2)
			ax.Tick.Major = niceStep((nhi - nlo) / n)
			// whole steps: the ends land on ticks, as OneFit's own do
			nlo = math.Floor(nlo/ax.Tick.Major) * ax.Tick.Major
			nhi = math.Ceil(nhi/ax.Tick.Major) * ax.Tick.Major
		}
	}
	if *a <= *b {
		*a, *b = nlo, nhi
	} else {
		*a, *b = nhi, nlo
	}
	return true
}

// niceStep is the smallest of 1, 2, 5 x 10^n at least d.
func niceStep(d float64) float64 {
	if !(d > 0) || math.IsInf(d, 0) {
		return 1
	}
	p := math.Pow(10, math.Floor(math.Log10(d)))
	for _, m := range []float64{1, 2, 5, 10} {
		if m*p >= d*(1-1e-9) {
			return m * p
		}
	}
	return 10 * p
}
