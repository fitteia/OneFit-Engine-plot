package agr

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Write writes the project as a Grace project file, in gracebat's order
// and forms, so Grace reads it too and Parse reads it back to the same
// Project. What the model does not keep (regions, annotated values, fills,
// fixed points - all off in OneFit's files) is written as gracebat writes
// it when off; a file that had them on loses them.
func Write(w io.Writer, p *Project) error {
	b := bufio.NewWriter(w)
	pf := func(format string, a ...any) { fmt.Fprintf(b, format+"\n", a...) }
	pf("# Grace project file")
	pf("#")
	version := p.Version
	if version == 0 {
		version = 50125
	}
	pf("@version %d", version)
	pf("@page size %s, %s", num(p.PageWidth), num(p.PageHeight))
	pf("@page scroll 5%%")
	pf("@page inout 5%%")
	pf("@link page off")
	for _, n := range sortedKeys(p.Fonts) {
		f := p.Fonts[n]
		pf("@map font %d to %s, %s", n, quote(f.Name), quote(f.Fallback))
	}
	for _, n := range sortedKeys(p.Colors) {
		c := p.Colors[n]
		pf("@map color %d to (%d, %d, %d), %s", n, c.R, c.G, c.B, quote(c.Name))
	}
	pf("@reference date 0")
	pf("@date wrap on")
	pf("@date wrap year 1900")
	d := p.Defaults
	pf("@default linewidth %s", num(d.LineWidth))
	pf("@default linestyle %d", d.LineStyle)
	pf("@default color %d", d.Color)
	pf("@default pattern %d", d.Pattern)
	pf("@default font %d", d.Font)
	pf("@default char size %s", size(d.CharSize))
	pf("@default symbol size %s", size(d.SymbolSize))
	sformat := d.SFormat
	if sformat == "" {
		sformat = "%16.8g"
	}
	pf("@default sformat %s", quote(sformat))
	pf("@background color %d", p.BackgroundColor)
	pf("@page background fill %s", onOff(p.PageFill))
	ts := p.Timestamp
	pf("@timestamp %s", onOff(ts.On))
	pf("@timestamp %s, %s", num(ts.X), num(ts.Y))
	pf("@timestamp color %d", ts.Color)
	pf("@timestamp rot %s", num(ts.Rot))
	pf("@timestamp font %d", ts.Font)
	pf("@timestamp char size %s", size(ts.CharSize))
	pf("@timestamp def %s", quote(ts.Text))
	for _, s := range p.Strings {
		pf("@with string")
		pf("@    string %s", onOff(s.On))
		loc := s.LocType
		if loc == "" {
			loc = "view"
		}
		pf("@    string loctype %s", loc)
		if loc == "world" {
			pf("@    string g%d", s.Graph)
		}
		pf("@    string %s, %s", num(s.X), num(s.Y))
		pf("@    string color %d", s.Color)
		pf("@    string rot %s", num(s.Rot))
		pf("@    string font %d", s.Font)
		pf("@    string just %d", s.Just)
		pf("@    string char size %s", size(s.CharSize))
		pf("@    string def %s", quote(s.Text))
	}
	for _, g := range p.Graphs {
		writeGraph(pf, g)
	}
	for _, g := range p.Graphs {
		for _, s := range g.Sets {
			if s.Type == "" {
				continue
			}
			pf("@target G%d.S%d", g.ID, s.ID)
			pf("@type %s", s.Type)
			for _, row := range s.Data {
				parts := make([]string, len(row))
				for i, v := range row {
					parts[i] = fmt.Sprintf("%16.8g", v)
				}
				pf("%s", strings.Join(parts, " "))
			}
			pf("&")
		}
	}
	return b.Flush()
}

func writeGraph(pf func(string, ...any), g *Graph) {
	pf("@g%d %s", g.ID, onOff(g.On))
	pf("@g%d hidden %s", g.ID, trueFalse(g.Hidden))
	typ := g.Type
	if typ == "" {
		typ = "XY"
	}
	pf("@g%d type %s", g.ID, typ)
	pf("@g%d stacked false", g.ID)
	pf("@g%d bar hgap 0.000000", g.ID)
	pf("@g%d fixedpoint off", g.ID)
	pf("@g%d fixedpoint type 0", g.ID)
	pf("@g%d fixedpoint xy 0.000000, 0.000000", g.ID)
	pf("@g%d fixedpoint format general general", g.ID)
	pf("@g%d fixedpoint prec 6, 6", g.ID)
	pf("@with g%d", g.ID)
	pf("@    world %s, %s, %s, %s", num(g.World.XMin), num(g.World.YMin), num(g.World.XMax), num(g.World.YMax))
	pf("@    stack world 0, 0, 0, 0")
	pf("@    znorm 1")
	pf("@    view %s, %s, %s, %s", num(g.View.XMin), num(g.View.YMin), num(g.View.XMax), num(g.View.YMax))
	for _, t := range []struct {
		name string
		t    Title
	}{{"title", g.Title}, {"subtitle", g.Subtitle}} {
		pf("@    %s %s", t.name, quote(t.t.Text))
		pf("@    %s font %d", t.name, t.t.Font)
		pf("@    %s size %s", t.name, size(t.t.Size))
		pf("@    %s color %d", t.name, t.t.Color)
	}
	pf("@    xaxes scale %s", scaleName(g.X.Scale))
	pf("@    yaxes scale %s", scaleName(g.Y.Scale))
	pf("@    xaxes invert %s", onOff(g.X.Invert))
	pf("@    yaxes invert %s", onOff(g.Y.Invert))
	writeAxis(pf, "xaxis", g.X)
	writeAxis(pf, "yaxis", g.Y)
	pf("@    altxaxis  off")
	pf("@    altyaxis  off")
	l := g.Legend
	pf("@    legend %s", onOff(l.On))
	loc := l.LocType
	if loc == "" {
		loc = "view"
	}
	pf("@    legend loctype %s", loc)
	pf("@    legend %s, %s", num(l.X), num(l.Y))
	pf("@    legend box color %d", l.BoxColor)
	pf("@    legend box pattern %d", l.BoxPattern)
	pf("@    legend box linewidth %s", num(l.BoxWidth))
	pf("@    legend box linestyle %d", l.BoxStyle)
	pf("@    legend box fill color %d", l.FillColor)
	pf("@    legend box fill pattern %d", l.FillPattern)
	pf("@    legend font %d", l.Font)
	pf("@    legend char size %s", size(l.CharSize))
	pf("@    legend color %d", l.Color)
	pf("@    legend length %s", num(l.Length))
	pf("@    legend vgap %s", num(l.VGap))
	pf("@    legend hgap %s", num(l.HGap))
	pf("@    legend invert %s", trueFalse(l.Invert))
	f := g.Frame
	pf("@    frame type %d", f.Type)
	pf("@    frame linestyle %d", f.LineStyle)
	pf("@    frame linewidth %s", num(f.LineWidth))
	pf("@    frame color %d", f.Color)
	pf("@    frame pattern %d", f.Pattern)
	pf("@    frame background color %d", f.BackgroundColor)
	pf("@    frame background pattern %d", f.BackgroundPattern)
	for _, s := range g.Sets {
		writeSet(pf, s)
	}
}

func writeAxis(pf func(string, ...any), name string, a Axis) {
	pf("@    %s  %s", name, onOff(a.On))
	pf("@    %s  type zero false", name)
	pf("@    %s  offset 0.000000 , 0.000000", name)
	pf("@    %s  bar off", name)
	pf("@    %s  bar color 1", name)
	pf("@    %s  bar linestyle 1", name)
	pf("@    %s  bar linewidth 1.0", name)
	pf("@    %s  label %s", name, quote(a.Label.Text))
	pf("@    %s  label layout %s", name, or(a.Label.Layout, "para"))
	pf("@    %s  label place %s", name, or(a.Label.Place, "auto"))
	pf("@    %s  label char size %s", name, size(a.Label.CharSize))
	pf("@    %s  label font %d", name, a.Label.Font)
	pf("@    %s  label color %d", name, a.Label.Color)
	t := a.Tick
	pf("@    %s  tick %s", name, onOff(t.On))
	pf("@    %s  tick major %s", name, num(t.Major))
	pf("@    %s  tick minor ticks %d", name, t.MinorTicks)
	pf("@    %s  tick default %d", name, t.Default)
	pf("@    %s  tick place rounded %s", name, trueFalse(t.Rounded))
	pf("@    %s  tick %s", name, or(t.Direction, "in"))
	for _, m := range []struct {
		kind  string
		marks TickMarks
	}{{"major", t.MajorMarks}, {"minor", t.MinorMarks}} {
		pf("@    %s  tick %s size %s", name, m.kind, size(m.marks.Size))
		pf("@    %s  tick %s color %d", name, m.kind, m.marks.Color)
		pf("@    %s  tick %s linewidth %s", name, m.kind, num(m.marks.LineWidth))
		pf("@    %s  tick %s linestyle %d", name, m.kind, m.marks.LineStyle)
		pf("@    %s  tick %s grid %s", name, m.kind, onOff(m.marks.Grid))
	}
	l := a.TickLabel
	pf("@    %s  ticklabel %s", name, onOff(l.On))
	pf("@    %s  ticklabel format %s", name, or(l.Format, "general"))
	pf("@    %s  ticklabel prec %d", name, l.Prec)
	pf("@    %s  ticklabel formula \"\"", name)
	pf("@    %s  ticklabel append %s", name, quote(l.Append))
	pf("@    %s  ticklabel prepend %s", name, quote(l.Prepend))
	pf("@    %s  ticklabel angle %s", name, num(l.Angle))
	pf("@    %s  ticklabel skip %d", name, l.Skip)
	pf("@    %s  ticklabel stagger %d", name, l.Stagger)
	pf("@    %s  ticklabel place %s", name, or(l.Place, "normal"))
	pf("@    %s  ticklabel offset auto", name)
	pf("@    %s  ticklabel offset 0.000000 , 0.010000", name)
	pf("@    %s  ticklabel start type auto", name)
	pf("@    %s  ticklabel start 0.000000", name)
	pf("@    %s  ticklabel stop type auto", name)
	pf("@    %s  ticklabel stop 0.000000", name)
	pf("@    %s  ticklabel char size %s", name, size(l.CharSize))
	pf("@    %s  ticklabel font %d", name, l.Font)
	pf("@    %s  ticklabel color %d", name, l.Color)
	pf("@    %s  tick place %s", name, or(t.Place, "both"))
	pf("@    %s  tick spec type none", name)
}

func writeSet(pf func(string, ...any), s *Set) {
	n := fmt.Sprintf("@    s%d", s.ID)
	pf("%s hidden %s", n, trueFalse(s.Hidden))
	if s.Type != "" {
		pf("%s type %s", n, s.Type)
	}
	y := s.Symbol
	pf("%s symbol %d", n, y.Type)
	pf("%s symbol size %s", n, size(y.Size))
	pf("%s symbol color %d", n, y.Color)
	pf("%s symbol pattern %d", n, y.Pattern)
	pf("%s symbol fill color %d", n, y.FillColor)
	pf("%s symbol fill pattern %d", n, y.FillPattern)
	pf("%s symbol linewidth %s", n, num(y.LineWidth))
	pf("%s symbol linestyle %d", n, y.LineStyle)
	pf("%s symbol char %d", n, y.Char)
	pf("%s symbol char font %d", n, y.CharFont)
	pf("%s symbol skip %d", n, y.Skip)
	l := s.Line
	pf("%s line type %d", n, l.Type)
	pf("%s line linestyle %d", n, l.LineStyle)
	pf("%s line linewidth %s", n, num(l.LineWidth))
	pf("%s line color %d", n, l.Color)
	pf("%s line pattern %d", n, l.Pattern)
	pf("%s baseline type 0", n)
	pf("%s baseline off", n)
	pf("%s dropline off", n)
	pf("%s fill type 0", n)
	pf("%s fill rule 0", n)
	pf("%s fill color 1", n)
	pf("%s fill pattern 1", n)
	pf("%s avalue off", n)
	e := s.ErrorBar
	pf("%s errorbar %s", n, onOff(e.On))
	pf("%s errorbar place %s", n, or(e.Place, "both"))
	pf("%s errorbar color %d", n, e.Color)
	pf("%s errorbar pattern %d", n, e.Pattern)
	pf("%s errorbar size %s", n, size(e.Size))
	pf("%s errorbar linewidth %s", n, num(e.LineWidth))
	pf("%s errorbar linestyle %d", n, e.LineStyle)
	pf("%s errorbar riser linewidth %s", n, num(e.RiserWidth))
	pf("%s errorbar riser linestyle %d", n, e.RiserStyle)
	pf("%s errorbar riser clip %s", n, onOff(e.RiserClip))
	pf("%s errorbar riser clip length %s", n, size(e.RiserClipSize))
	pf("%s comment %s", n, quote(s.Comment))
	pf("%s legend  %s", n, quote(s.Legend))
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// num writes a number exactly (shortest form that reads back the same).
func num(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// size writes a size as gracebat does ("1.500000"), exactly when that
// would lose digits.
func size(v float64) string {
	s := strconv.FormatFloat(v, 'f', 6, 64)
	if f, _ := strconv.ParseFloat(s, 64); f != v {
		return num(v)
	}
	return s
}

// quote makes a Grace string: one line, and a double quote becomes a
// single one (Grace strings have no escape for it).
func quote(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return `"` + strings.ReplaceAll(s, `"`, `'`) + `"`
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func trueFalse(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func or(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func scaleName(s string) string { return or(s, "Normal") }
