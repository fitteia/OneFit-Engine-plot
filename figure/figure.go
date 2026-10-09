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
