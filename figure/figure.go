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
		c.Data = make([][]float64, len(s.Data))
		for i, row := range s.Data {
			c.Data[i] = append([]float64(nil), row...)
		}
		c.ID = next
		c.Comment = Ref{Path: path, Graph: 0, Set: id}.String()
		dst.Sets = append(dst.Sets, &c)
		added = append(added, next)
		next++
		for _, n := range []int{c.Line.Color, c.Symbol.Color, c.Symbol.FillColor, c.ErrorBar.Color} {
			if _, ok := fig.Colors[n]; !ok {
				if col, ok := src.Colors[n]; ok {
					if fig.Colors == nil {
						fig.Colors = map[int]agr.Color{}
					}
					fig.Colors[n] = col
				}
			}
		}
	}
	return added, nil
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
// source) and the texts placed in its world coordinates - numbered after
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
	g := *from
	g.ID = id
	g.Sets = nil
	fig.Graphs = append(fig.Graphs, &g)
	sets := make([]int, 0, len(from.Sets))
	for _, s := range from.Sets {
		sets = append(sets, s.ID)
	}
	if _, err := AddSets(fig, id, src, path, sets); err != nil {
		return id, err
	}
	// AddSets numbers sets after the graph's last; a new graph starts at 0,
	// as the plot's own numbering did
	for _, s := range src.Strings {
		if s.LocType == "world" && s.Graph == 0 {
			c := *s
			c.Graph = id
			fig.Strings = append(fig.Strings, &c)
		}
	}
	for n, c := range src.Fonts {
		if _, ok := fig.Fonts[n]; !ok {
			if fig.Fonts == nil {
				fig.Fonts = map[int]agr.Font{}
			}
			fig.Fonts[n] = c
		}
	}
	return id, nil
}

// RemoveGraph takes a graph and the texts placed in it out of the figure.
func RemoveGraph(fig *agr.Project, id int) error {
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
