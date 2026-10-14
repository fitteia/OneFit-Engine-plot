package agr

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Write writes the project as a Grace project file, in gracebat's order
// and forms, so Grace reads it too and Parse reads it back to the same
// Project. What the model has no place for (regions, annotated values,
// fixed points, ...) is written as Parse kept it (Project.Kept, Objects and
// the graphs' and sets'), so nothing is lost; what it has not kept, as
// gracebat writes it when off.
func Write(w io.Writer, p *Project) error {
	b := bufio.NewWriter(w)
	out := func(format string, a ...any) { fmt.Fprintf(b, format+"\n", a...) }
	k := &kept{out: out}
	k.keep("", p.Kept, func(d string) string { return "@" + d })
	pf := k.pf
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
	pf("@reference date %s", num(p.RefDate))
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
	k.rest()
	for _, o := range p.Objects {
		for i, d := range o {
			if i == 0 || i == len(o)-1 && strings.HasSuffix(d, " def") {
				pf("@%s", d)
			} else {
				pf("@    %s", d)
			}
		}
	}
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
		writeGraph(out, g)
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

// kept writes kept directives (see Project.Kept): each in place of the
// directive of the same kind the writer would write, the others by rest.
type kept struct {
	out   func(string, ...any)
	lines map[string]string // directiveKey -> the line to write
	order []string          // keys, in the order kept
	done  map[string]bool
}

// keep adds directives kept without prefix, written as line(k).
func (k *kept) keep(prefix string, dirs []string, line func(string) string) {
	if k.lines == nil {
		k.lines, k.done = map[string]string{}, map[string]bool{}
	}
	for _, d := range dirs {
		key := directiveKey(prefix + d)
		k.lines[key] = line(d)
		k.order = append(k.order, key)
	}
}

// pf writes a directive, or the kept one that replaces it.
func (k *kept) pf(format string, a ...any) {
	line := fmt.Sprintf(format, a...)
	if d, ok := strings.CutPrefix(line, "@"); ok && k.lines != nil {
		key := directiveKey(strings.TrimSpace(d))
		if l, ok := k.lines[key]; ok {
			if !k.done[key] {
				k.out("%s", l)
				k.done[key] = true
			}
			return
		}
	}
	k.out("%s", line)
}

// rest writes the kept directives not written in place.
func (k *kept) rest() {
	for _, key := range k.order {
		if !k.done[key] {
			k.out("%s", k.lines[key])
			k.done[key] = true
		}
	}
}

func writeGraph(out func(string, ...any), g *Graph) {
	header := &kept{out: out}
	header.keep("g0 ", g.KeptHeader, func(d string) string { return fmt.Sprintf("@g%d %s", g.ID, d) })
	pf := header.pf
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
	header.rest()
	pf("@with g%d", g.ID)
	body := &kept{out: out}
	body.keep("", g.Kept, func(d string) string { return "@    " + d })
	pf = body.pf
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
	// an alternate axis at Grace's defaults and off is one line, as Grace
	// writes it; one that is on, or off with settings of its own (hidden
	// in the Figures editor, to come back as it was), is written whole -
	// Grace itself writes them "off", and nothing else, even when on
	for _, alt := range []struct {
		name string
		a    Axis
	}{{"altxaxis", g.AltX}, {"altyaxis", g.AltY}} {
		if alt.a.On || !reflect.DeepEqual(alt.a, DefaultAxis()) {
			writeAxis(pf, alt.name, alt.a)
		} else {
			pf("@    %s  off", alt.name)
		}
	}
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
	body.rest()
	for _, s := range g.Sets {
		writeSet(out, s)
	}
}

func writeAxis(pf func(string, ...any), name string, a Axis) {
	pf("@    %s  %s", name, onOff(a.On))
	pf("@    %s  type zero %s", name, map[bool]string{true: "true", false: "false"}[a.Zero])
	pf("@    %s  offset %s , %s", name, size(a.Offset[0]), size(a.Offset[1]))
	pf("@    %s  bar %s", name, onOff(a.Bar.On))
	pf("@    %s  bar color %d", name, a.Bar.Color)
	pf("@    %s  bar linestyle %d", name, a.Bar.LineStyle)
	pf("@    %s  bar linewidth %s", name, num(a.Bar.LineWidth))
	pf("@    %s  label %s", name, quote(a.Label.Text))
	pf("@    %s  label layout %s", name, or(a.Label.Layout, "para"))
	// how ("auto"/"spec", and where), then the side - Grace's own order
	if a.Label.Spec {
		pf("@    %s  label place spec", name)
		pf("@    %s  label place %s, %s", name, size(a.Label.SpecAt[0]), size(a.Label.SpecAt[1]))
	} else {
		pf("@    %s  label place auto", name)
	}
	pf("@    %s  label place %s", name, or(a.Label.Place, "normal"))
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
	pf("@    %s  ticklabel formula %s", name, quote(l.Formula))
	pf("@    %s  ticklabel append %s", name, quote(l.Append))
	pf("@    %s  ticklabel prepend %s", name, quote(l.Prepend))
	pf("@    %s  ticklabel angle %s", name, num(l.Angle))
	pf("@    %s  ticklabel skip %d", name, l.Skip)
	pf("@    %s  ticklabel stagger %d", name, l.Stagger)
	pf("@    %s  ticklabel place %s", name, or(l.Place, "normal"))
	if l.OffsetSpec {
		pf("@    %s  ticklabel offset spec", name)
		pf("@    %s  ticklabel offset %s , %s", name, size(l.Offset[0]), size(l.Offset[1]))
	} else {
		pf("@    %s  ticklabel offset auto", name)
		pf("@    %s  ticklabel offset 0.000000 , 0.010000", name)
	}
	pf("@    %s  ticklabel start type %s", name, map[bool]string{true: "spec", false: "auto"}[l.StartSpec])
	pf("@    %s  ticklabel start %s", name, size(l.Start))
	pf("@    %s  ticklabel stop type %s", name, map[bool]string{true: "spec", false: "auto"}[l.StopSpec])
	pf("@    %s  ticklabel stop %s", name, size(l.Stop))
	pf("@    %s  ticklabel char size %s", name, size(l.CharSize))
	pf("@    %s  ticklabel font %d", name, l.Font)
	pf("@    %s  ticklabel color %d", name, l.Color)
	pf("@    %s  tick place %s", name, or(t.Place, "both"))
	pf("@    %s  tick spec type %s", name, or(t.SpecType, "none"))
	if t.SpecCount > 0 || len(t.Spec) > 0 {
		pf("@    %s  tick spec %d", name, t.SpecCount)
	}
	for i, st := range t.Spec {
		if st.Kind != "" {
			pf("@    %s  tick %s %d, %s", name, st.Kind, i, num(st.Value))
		}
		if st.HasLabel {
			pf("@    %s  ticklabel %d, %s", name, i, quote(st.Label))
		}
	}
}

func writeSet(out func(string, ...any), s *Set) {
	k := &kept{out: out}
	k.keep("s0 ", s.Kept, func(d string) string { return fmt.Sprintf("@    s%d %s", s.ID, d) })
	pf := k.pf
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
	pf("%s baseline type %d", n, s.Baseline.Type)
	pf("%s baseline %s", n, onOff(s.Baseline.On))
	pf("%s dropline %s", n, onOff(s.Dropline))
	pf("%s fill type %d", n, s.Fill.Type)
	pf("%s fill rule %d", n, s.Fill.Rule)
	pf("%s fill color %d", n, s.Fill.Color)
	pf("%s fill pattern %d", n, s.Fill.Pattern)
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
	k.rest()
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
