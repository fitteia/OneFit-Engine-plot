package agr

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// token is one word of a directive: a bare word, a number, or a quoted
// string (Str holds what is between the quotes, escapes untouched).
type token struct {
	word  string
	num   float64
	isNum bool
	isStr bool
}

// tokenize splits a directive (without its "@") into tokens. Whitespace,
// commas and parentheses separate words; a quoted string is one token.
func tokenize(s string) ([]token, error) {
	var toks []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == ',' || c == '(' || c == ')' || c == '\r':
			i++
		case c == '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			if j == len(s) {
				return nil, fmt.Errorf("unterminated string")
			}
			toks = append(toks, token{word: s[i+1 : j], isStr: true})
			i = j + 1
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \t,()\r\"", rune(s[j])) {
				j++
			}
			w := s[i:j]
			if f, err := strconv.ParseFloat(w, 64); err == nil {
				toks = append(toks, token{word: w, num: f, isNum: true})
			} else {
				toks = append(toks, token{word: w})
			}
			i = j
		}
	}
	return toks, nil
}

// A pattern is a directive's shape, word by word: a literal word, or
//
//	N     a number           S     a quoted string     W  any bare word
//	G# S# R#  graph, set, region number (g0, s12, r3)
//	AXIS  xaxis or yaxis     AXES  xaxes or yaxes
//
// Everything but literal words is passed to the handler, in order, as text.
type rule struct {
	pattern []string
	handle  func(p *parser, a []string)
}

func matchIndex(prefix, w string) bool {
	if len(w) < 2 || !strings.EqualFold(w[:1], prefix) {
		return false
	}
	_, err := strconv.Atoi(w[1:])
	return err == nil
}

func (r rule) match(toks []token) ([]string, bool) {
	if len(r.pattern) != len(toks) {
		return nil, false
	}
	var args []string
	for i, p := range r.pattern {
		t := toks[i]
		ok := false
		switch p {
		case "N":
			ok = t.isNum
		case "S":
			ok = t.isStr
		case "W":
			ok = !t.isNum && !t.isStr
		case "G#", "S#", "R#":
			ok = !t.isStr && matchIndex(p[:1], t.word)
		case "AXIS":
			ok = !t.isStr && (t.word == "xaxis" || t.word == "yaxis")
		case "AXES":
			ok = !t.isStr && (t.word == "xaxes" || t.word == "yaxes")
		default:
			if !t.isStr && strings.EqualFold(p, t.word) {
				continue
			}
		}
		if !ok {
			return nil, false
		}
		args = append(args, t.word)
	}
	return args, true
}

type parser struct {
	proj   *Project
	line   int
	text   string
	graph  *Graph // the graph "@with gN" selected
	str    *Text  // the string "@with string" started
	target *Set   // the set "@target" selected, receiving data
	inData bool
}

func (p *parser) warn(format string, a ...any) {
	p.proj.Warnings = append(p.proj.Warnings, Warning{Line: p.line, Text: p.text, Msg: fmt.Sprintf(format, a...)})
}

func (p *parser) num(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		p.warn("not a number: %q", s)
	}
	return f
}

func (p *parser) int(s string) int {
	f := p.num(s)
	if f != float64(int(f)) {
		p.warn("not a whole number: %q", s)
	}
	return int(f)
}

func (p *parser) bool(s string) bool {
	switch strings.ToLower(s) {
	case "on", "true":
		return true
	case "off", "false":
		return false
	}
	p.warn("expected on/off or true/false, not %q", s)
	return false
}

// index is the number in "g0", "s12", "r3".
func (p *parser) index(s string) int {
	n, _ := strconv.Atoi(s[1:])
	return n
}

func (p *parser) graphByID(id int) *Graph {
	for _, g := range p.proj.Graphs {
		if g.ID == id {
			return g
		}
	}
	g := &Graph{ID: id}
	p.proj.Graphs = append(p.proj.Graphs, g)
	return g
}

func (p *parser) currentGraph() *Graph {
	if p.graph == nil {
		p.warn("graph setting before any \"@with g...\"; applied to g0")
		p.graph = p.graphByID(0)
	}
	return p.graph
}

func (g *Graph) setByID(id int) *Set {
	for _, s := range g.Sets {
		if s.ID == id {
			return s
		}
	}
	s := &Set{ID: id}
	g.Sets = append(g.Sets, s)
	return s
}

func (p *parser) set(a []string) *Set { return p.currentGraph().setByID(p.index(a[0])) }

func (p *parser) axis(name string) *Axis {
	if strings.HasPrefix(name, "x") {
		return &p.currentGraph().X
	}
	return &p.currentGraph().Y
}

func (p *parser) currentString() *Text {
	if p.str == nil {
		p.warn("string setting before any \"@with string\"")
		p.str = &Text{}
		p.proj.Strings = append(p.proj.Strings, p.str)
	}
	return p.str
}

// unsupportedWhenOn accepts a feature plot-go does not draw while it is
// off, and warns when a file turns it on.
func unsupportedWhenOn(what string, arg int) func(p *parser, a []string) {
	return func(p *parser, a []string) {
		if p.bool(a[arg]) {
			p.warn("%s is not drawn by plot-go", what)
		}
	}
}

var ignore = func(*parser, []string) {}

var tickLabelFormats = map[string]bool{
	"decimal": true, "power": true, "general": true, "exponential": true,
	"scientific": true, "engineering": true, "computing": true,
}

func r(pattern string, h func(p *parser, a []string)) rule {
	return rule{pattern: strings.Fields(pattern), handle: h}
}

var rules []rule

func init() {
	rules = []rule{
		// page and project
		r("version N", func(p *parser, a []string) { p.proj.Version = p.int(a[0]) }),
		r("page size N N", func(p *parser, a []string) {
			p.proj.PageWidth, p.proj.PageHeight = p.num(a[0]), p.num(a[1])
		}),
		r("page scroll W", ignore),
		r("page inout W", ignore),
		r("page background fill W", func(p *parser, a []string) { p.proj.PageFill = p.bool(a[0]) }),
		r("background color N", func(p *parser, a []string) { p.proj.BackgroundColor = p.int(a[0]) }),
		r("link page W", ignore),
		r("reference date N", ignore),
		r("date wrap W", ignore),
		r("date wrap year N", ignore),
		r("map font N to S S", func(p *parser, a []string) {
			p.proj.Fonts[p.int(a[0])] = Font{Name: a[1], Fallback: a[2]}
		}),
		r("map color N to N N N S", func(p *parser, a []string) {
			p.proj.Colors[p.int(a[0])] = Color{R: uint8(p.int(a[1])), G: uint8(p.int(a[2])), B: uint8(p.int(a[3])), Name: a[4]}
		}),
		r("default linewidth N", func(p *parser, a []string) { p.proj.Defaults.LineWidth = p.num(a[0]) }),
		r("default linestyle N", func(p *parser, a []string) { p.proj.Defaults.LineStyle = p.int(a[0]) }),
		r("default color N", func(p *parser, a []string) { p.proj.Defaults.Color = p.int(a[0]) }),
		r("default pattern N", func(p *parser, a []string) { p.proj.Defaults.Pattern = p.int(a[0]) }),
		r("default font N", func(p *parser, a []string) { p.proj.Defaults.Font = p.int(a[0]) }),
		r("default char size N", func(p *parser, a []string) { p.proj.Defaults.CharSize = p.num(a[0]) }),
		r("default symbol size N", func(p *parser, a []string) { p.proj.Defaults.SymbolSize = p.num(a[0]) }),
		r("default sformat S", func(p *parser, a []string) { p.proj.Defaults.SFormat = a[0] }),

		// timestamp and strings
		r("timestamp W", func(p *parser, a []string) { p.proj.Timestamp.On = p.bool(a[0]) }),
		r("timestamp N N", func(p *parser, a []string) { p.proj.Timestamp.X, p.proj.Timestamp.Y = p.num(a[0]), p.num(a[1]) }),
		r("timestamp color N", func(p *parser, a []string) { p.proj.Timestamp.Color = p.int(a[0]) }),
		r("timestamp rot N", func(p *parser, a []string) { p.proj.Timestamp.Rot = p.num(a[0]) }),
		r("timestamp font N", func(p *parser, a []string) { p.proj.Timestamp.Font = p.int(a[0]) }),
		r("timestamp char size N", func(p *parser, a []string) { p.proj.Timestamp.CharSize = p.num(a[0]) }),
		r("timestamp def S", func(p *parser, a []string) { p.proj.Timestamp.Text = a[0] }),
		r("with string", func(p *parser, a []string) {
			p.str = &Text{}
			p.proj.Strings = append(p.proj.Strings, p.str)
		}),
		// "string G#" before "string W": W matches any word, g0 included
		r("string G#", func(p *parser, a []string) { p.currentString().Graph = p.index(a[0]) }),
		r("string W", func(p *parser, a []string) { p.currentString().On = p.bool(a[0]) }),
		r("string loctype W", func(p *parser, a []string) { p.currentString().LocType = a[0] }),
		r("string N N", func(p *parser, a []string) { s := p.currentString(); s.X, s.Y = p.num(a[0]), p.num(a[1]) }),
		r("string color N", func(p *parser, a []string) { p.currentString().Color = p.int(a[0]) }),
		r("string rot N", func(p *parser, a []string) { p.currentString().Rot = p.num(a[0]) }),
		r("string font N", func(p *parser, a []string) { p.currentString().Font = p.int(a[0]) }),
		r("string just N", func(p *parser, a []string) { p.currentString().Just = p.int(a[0]) }),
		r("string char size N", func(p *parser, a []string) { p.currentString().CharSize = p.num(a[0]) }),
		r("string def S", func(p *parser, a []string) { p.currentString().Text = a[0] }),

		// regions: accepted while off
		r("R# W", unsupportedWhenOn("a region", 1)),
		r("link R# to G#", ignore),
		r("R# type W", ignore),
		r("R# linestyle N", ignore),
		r("R# linewidth N", ignore),
		r("R# color N", ignore),
		r("R# line N N N N", ignore),

		// graph header ("@g0 ...") and selection
		r("G# W", func(p *parser, a []string) { p.graphByID(p.index(a[0])).On = p.bool(a[1]) }),
		r("G# hidden W", func(p *parser, a []string) { p.graphByID(p.index(a[0])).Hidden = p.bool(a[1]) }),
		r("G# type W", func(p *parser, a []string) {
			p.graphByID(p.index(a[0])).Type = a[1]
			if !strings.EqualFold(a[1], "XY") {
				p.warn("graph type %s is not drawn by plot-go (only XY)", a[1])
			}
		}),
		r("G# stacked W", unsupportedWhenOn("a stacked graph", 1)),
		r("G# bar hgap N", ignore),
		r("G# fixedpoint W", unsupportedWhenOn("a fixed point", 1)),
		r("G# fixedpoint type N", ignore),
		r("G# fixedpoint xy N N", ignore),
		r("G# fixedpoint format W W", ignore),
		r("G# fixedpoint prec N N", ignore),
		r("with G#", func(p *parser, a []string) { p.graph = p.graphByID(p.index(a[0])) }),

		// graph contents
		r("world N N N N", func(p *parser, a []string) {
			p.currentGraph().World = Rect{p.num(a[0]), p.num(a[1]), p.num(a[2]), p.num(a[3])}
		}),
		r("world xmin N", func(p *parser, a []string) { p.currentGraph().World.XMin = p.num(a[0]) }),
		r("world xmax N", func(p *parser, a []string) { p.currentGraph().World.XMax = p.num(a[0]) }),
		r("world ymin N", func(p *parser, a []string) { p.currentGraph().World.YMin = p.num(a[0]) }),
		r("world ymax N", func(p *parser, a []string) { p.currentGraph().World.YMax = p.num(a[0]) }),
		r("view N N N N", func(p *parser, a []string) {
			p.currentGraph().View = Rect{p.num(a[0]), p.num(a[1]), p.num(a[2]), p.num(a[3])}
		}),
		r("view xmin N", func(p *parser, a []string) { p.currentGraph().View.XMin = p.num(a[0]) }),
		r("view xmax N", func(p *parser, a []string) { p.currentGraph().View.XMax = p.num(a[0]) }),
		r("view ymin N", func(p *parser, a []string) { p.currentGraph().View.YMin = p.num(a[0]) }),
		r("view ymax N", func(p *parser, a []string) { p.currentGraph().View.YMax = p.num(a[0]) }),
		r("stack world N N N N", ignore),
		r("znorm N", ignore),
		r("title S", func(p *parser, a []string) { p.currentGraph().Title.Text = a[0] }),
		r("title font N", func(p *parser, a []string) { p.currentGraph().Title.Font = p.int(a[0]) }),
		r("title size N", func(p *parser, a []string) { p.currentGraph().Title.Size = p.num(a[0]) }),
		r("title color N", func(p *parser, a []string) { p.currentGraph().Title.Color = p.int(a[0]) }),
		r("subtitle S", func(p *parser, a []string) { p.currentGraph().Subtitle.Text = a[0] }),
		r("subtitle font N", func(p *parser, a []string) { p.currentGraph().Subtitle.Font = p.int(a[0]) }),
		r("subtitle size N", func(p *parser, a []string) { p.currentGraph().Subtitle.Size = p.num(a[0]) }),
		r("subtitle color N", func(p *parser, a []string) { p.currentGraph().Subtitle.Color = p.int(a[0]) }),

		// axes
		r("AXES scale W", func(p *parser, a []string) {
			switch a[1] {
			case "Normal", "Logarithmic":
			default:
				p.warn("axis scale %s is not drawn by plot-go", a[1])
			}
			p.axis(a[0]).Scale = a[1]
		}),
		r("AXES invert W", func(p *parser, a []string) { p.axis(a[0]).Invert = p.bool(a[1]) }),
		r("altxaxis W", unsupportedWhenOn("an alternate x axis", 0)),
		r("altyaxis W", unsupportedWhenOn("an alternate y axis", 0)),
		r("AXIS W", func(p *parser, a []string) { p.axis(a[0]).On = p.bool(a[1]) }),
		r("AXIS type zero W", unsupportedWhenOn("an axis at zero", 1)),
		r("AXIS offset N N", ignore),
		r("AXIS bar W", unsupportedWhenOn("an axis bar", 1)),
		r("AXIS bar color N", ignore),
		r("AXIS bar linestyle N", ignore),
		r("AXIS bar linewidth N", ignore),
		r("AXIS label S", func(p *parser, a []string) { p.axis(a[0]).Label.Text = a[1] }),
		r("AXIS label layout W", func(p *parser, a []string) { p.axis(a[0]).Label.Layout = a[1] }),
		r("AXIS label place W", func(p *parser, a []string) { p.axis(a[0]).Label.Place = a[1] }),
		r("AXIS label char size N", func(p *parser, a []string) { p.axis(a[0]).Label.CharSize = p.num(a[1]) }),
		r("AXIS label font N", func(p *parser, a []string) { p.axis(a[0]).Label.Font = p.int(a[1]) }),
		r("AXIS label color N", func(p *parser, a []string) { p.axis(a[0]).Label.Color = p.int(a[1]) }),
		r("AXIS tick W", func(p *parser, a []string) {
			switch a[1] {
			case "in", "out", "both":
				p.axis(a[0]).Tick.Direction = a[1]
			default:
				p.axis(a[0]).Tick.On = p.bool(a[1])
			}
		}),
		r("AXIS tick major N", func(p *parser, a []string) { p.axis(a[0]).Tick.Major = p.num(a[1]) }),
		r("AXIS tick minor ticks N", func(p *parser, a []string) { p.axis(a[0]).Tick.MinorTicks = p.int(a[1]) }),
		r("AXIS tick default N", func(p *parser, a []string) { p.axis(a[0]).Tick.Default = p.int(a[1]) }),
		r("AXIS tick place rounded W", func(p *parser, a []string) { p.axis(a[0]).Tick.Rounded = p.bool(a[1]) }),
		r("AXIS tick place W", func(p *parser, a []string) { p.axis(a[0]).Tick.Place = a[1] }),
		r("AXIS tick spec type W", func(p *parser, a []string) {
			if a[1] != "none" {
				p.warn("special ticks are not drawn by plot-go")
			}
		}),
		r("AXIS tick major size N", func(p *parser, a []string) { p.axis(a[0]).Tick.MajorMarks.Size = p.num(a[1]) }),
		r("AXIS tick major color N", func(p *parser, a []string) { p.axis(a[0]).Tick.MajorMarks.Color = p.int(a[1]) }),
		r("AXIS tick major linewidth N", func(p *parser, a []string) { p.axis(a[0]).Tick.MajorMarks.LineWidth = p.num(a[1]) }),
		r("AXIS tick major linestyle N", func(p *parser, a []string) { p.axis(a[0]).Tick.MajorMarks.LineStyle = p.int(a[1]) }),
		r("AXIS tick major grid W", func(p *parser, a []string) { p.axis(a[0]).Tick.MajorMarks.Grid = p.bool(a[1]) }),
		r("AXIS tick minor size N", func(p *parser, a []string) { p.axis(a[0]).Tick.MinorMarks.Size = p.num(a[1]) }),
		r("AXIS tick minor color N", func(p *parser, a []string) { p.axis(a[0]).Tick.MinorMarks.Color = p.int(a[1]) }),
		r("AXIS tick minor linewidth N", func(p *parser, a []string) { p.axis(a[0]).Tick.MinorMarks.LineWidth = p.num(a[1]) }),
		r("AXIS tick minor linestyle N", func(p *parser, a []string) { p.axis(a[0]).Tick.MinorMarks.LineStyle = p.int(a[1]) }),
		r("AXIS tick minor grid W", func(p *parser, a []string) { p.axis(a[0]).Tick.MinorMarks.Grid = p.bool(a[1]) }),
		r("AXIS ticklabel prec", func(p *parser, a []string) { p.warn("tick label precision has no value; ignored") }),
		r("AXIS ticklabel W", func(p *parser, a []string) { p.axis(a[0]).TickLabel.On = p.bool(a[1]) }),
		r("AXIS ticklabel format W", func(p *parser, a []string) {
			if !tickLabelFormats[a[1]] {
				p.warn("unknown tick label format %q; decimal used", a[1])
				a[1] = "decimal"
			}
			p.axis(a[0]).TickLabel.Format = a[1]
		}),
		r("AXIS ticklabel format N", func(p *parser, a []string) {
			p.warn("unknown tick label format %q; decimal used", a[1])
			p.axis(a[0]).TickLabel.Format = "decimal"
		}),
		r("AXIS ticklabel prec N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Prec = p.int(a[1]) }),
		r("AXIS ticklabel formula S", func(p *parser, a []string) {
			if a[1] != "" {
				p.warn("tick label formulas are not applied by plot-go")
			}
		}),
		r("AXIS ticklabel append S", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Append = a[1] }),
		r("AXIS ticklabel prepend S", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Prepend = a[1] }),
		r("AXIS ticklabel angle N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Angle = p.num(a[1]) }),
		r("AXIS ticklabel skip N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Skip = p.int(a[1]) }),
		r("AXIS ticklabel stagger N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Stagger = p.int(a[1]) }),
		r("AXIS ticklabel place W", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Place = a[1] }),
		r("AXIS ticklabel offset W", ignore),
		r("AXIS ticklabel offset N N", ignore),
		r("AXIS ticklabel start type W", func(p *parser, a []string) {
			if a[1] != "auto" {
				p.warn("tick label start/stop are not applied by plot-go")
			}
		}),
		r("AXIS ticklabel start N", ignore),
		r("AXIS ticklabel stop type W", func(p *parser, a []string) {
			if a[1] != "auto" {
				p.warn("tick label start/stop are not applied by plot-go")
			}
		}),
		r("AXIS ticklabel stop N", ignore),
		r("AXIS ticklabel char size N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.CharSize = p.num(a[1]) }),
		r("AXIS ticklabel font N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Font = p.int(a[1]) }),
		r("AXIS ticklabel color N", func(p *parser, a []string) { p.axis(a[0]).TickLabel.Color = p.int(a[1]) }),

		// legend and frame
		r("legend W", func(p *parser, a []string) { p.currentGraph().Legend.On = p.bool(a[0]) }),
		r("legend loctype W", func(p *parser, a []string) { p.currentGraph().Legend.LocType = a[0] }),
		r("legend N N", func(p *parser, a []string) {
			l := &p.currentGraph().Legend
			l.X, l.Y = p.num(a[0]), p.num(a[1])
		}),
		r("legend box color N", func(p *parser, a []string) { p.currentGraph().Legend.BoxColor = p.int(a[0]) }),
		r("legend box pattern N", func(p *parser, a []string) { p.currentGraph().Legend.BoxPattern = p.int(a[0]) }),
		r("legend box linewidth N", func(p *parser, a []string) { p.currentGraph().Legend.BoxWidth = p.num(a[0]) }),
		r("legend box linestyle N", func(p *parser, a []string) { p.currentGraph().Legend.BoxStyle = p.int(a[0]) }),
		r("legend box fill color N", func(p *parser, a []string) { p.currentGraph().Legend.FillColor = p.int(a[0]) }),
		r("legend box fill pattern N", func(p *parser, a []string) { p.currentGraph().Legend.FillPattern = p.int(a[0]) }),
		r("legend font N", func(p *parser, a []string) { p.currentGraph().Legend.Font = p.int(a[0]) }),
		r("legend char size N", func(p *parser, a []string) { p.currentGraph().Legend.CharSize = p.num(a[0]) }),
		r("legend color N", func(p *parser, a []string) { p.currentGraph().Legend.Color = p.int(a[0]) }),
		r("legend length N", func(p *parser, a []string) { p.currentGraph().Legend.Length = p.num(a[0]) }),
		r("legend vgap N", func(p *parser, a []string) { p.currentGraph().Legend.VGap = p.num(a[0]) }),
		r("legend hgap N", func(p *parser, a []string) { p.currentGraph().Legend.HGap = p.num(a[0]) }),
		r("legend invert W", func(p *parser, a []string) { p.currentGraph().Legend.Invert = p.bool(a[0]) }),
		r("frame type N", func(p *parser, a []string) { p.currentGraph().Frame.Type = p.int(a[0]) }),
		r("frame linestyle N", func(p *parser, a []string) { p.currentGraph().Frame.LineStyle = p.int(a[0]) }),
		r("frame linewidth N", func(p *parser, a []string) { p.currentGraph().Frame.LineWidth = p.num(a[0]) }),
		r("frame color N", func(p *parser, a []string) { p.currentGraph().Frame.Color = p.int(a[0]) }),
		r("frame pattern N", func(p *parser, a []string) { p.currentGraph().Frame.Pattern = p.int(a[0]) }),
		r("frame background color N", func(p *parser, a []string) { p.currentGraph().Frame.BackgroundColor = p.int(a[0]) }),
		r("frame background pattern N", func(p *parser, a []string) { p.currentGraph().Frame.BackgroundPattern = p.int(a[0]) }),

		// sets
		r("S# hidden W", func(p *parser, a []string) { p.set(a).Hidden = p.bool(a[1]) }),
		r("S# type W", func(p *parser, a []string) { p.setType(p.set(a), a[1]) }),
		r("S# symbol N", func(p *parser, a []string) { p.set(a).Symbol.Type = p.int(a[1]) }),
		r("S# symbol size N", func(p *parser, a []string) { p.set(a).Symbol.Size = p.num(a[1]) }),
		r("S# symbol color N", func(p *parser, a []string) { p.set(a).Symbol.Color = p.int(a[1]) }),
		r("S# symbol pattern N", func(p *parser, a []string) { p.set(a).Symbol.Pattern = p.int(a[1]) }),
		r("S# symbol fill color N", func(p *parser, a []string) { p.set(a).Symbol.FillColor = p.int(a[1]) }),
		r("S# symbol fill pattern N", func(p *parser, a []string) { p.set(a).Symbol.FillPattern = p.int(a[1]) }),
		r("S# symbol linewidth N", func(p *parser, a []string) { p.set(a).Symbol.LineWidth = p.num(a[1]) }),
		r("S# symbol linestyle N", func(p *parser, a []string) { p.set(a).Symbol.LineStyle = p.int(a[1]) }),
		r("S# symbol char N", func(p *parser, a []string) { p.set(a).Symbol.Char = p.int(a[1]) }),
		r("S# symbol char font N", func(p *parser, a []string) { p.set(a).Symbol.CharFont = p.int(a[1]) }),
		r("S# symbol skip N", func(p *parser, a []string) { p.set(a).Symbol.Skip = p.int(a[1]) }),
		r("S# line type N", func(p *parser, a []string) { p.set(a).Line.Type = p.int(a[1]) }),
		r("S# line linestyle N", func(p *parser, a []string) { p.set(a).Line.LineStyle = p.int(a[1]) }),
		r("S# line linewidth N", func(p *parser, a []string) { p.set(a).Line.LineWidth = p.num(a[1]) }),
		r("S# line color N", func(p *parser, a []string) { p.set(a).Line.Color = p.int(a[1]) }),
		r("S# line pattern N", func(p *parser, a []string) { p.set(a).Line.Pattern = p.int(a[1]) }),
		r("S# baseline type N", ignore),
		r("S# baseline W", unsupportedWhenOn("a set baseline", 1)),
		r("S# dropline W", unsupportedWhenOn("set droplines", 1)),
		r("S# fill type N", func(p *parser, a []string) {
			if p.int(a[1]) != 0 {
				p.warn("set fills are not drawn by plot-go")
			}
		}),
		r("S# fill rule N", ignore),
		r("S# fill color N", ignore),
		r("S# fill pattern N", ignore),
		r("S# avalue W", unsupportedWhenOn("annotated values", 1)),
		r("S# avalue type N", ignore),
		r("S# avalue char size N", ignore),
		r("S# avalue font N", ignore),
		r("S# avalue color N", ignore),
		r("S# avalue rot N", ignore),
		r("S# avalue format W", ignore),
		r("S# avalue prec N", ignore),
		r("S# avalue prepend S", ignore),
		r("S# avalue append S", ignore),
		r("S# avalue offset N N", ignore),
		r("S# errorbar W", func(p *parser, a []string) { p.set(a).ErrorBar.On = p.bool(a[1]) }),
		r("S# errorbar place W", func(p *parser, a []string) { p.set(a).ErrorBar.Place = a[1] }),
		r("S# errorbar color N", func(p *parser, a []string) { p.set(a).ErrorBar.Color = p.int(a[1]) }),
		r("S# errorbar pattern N", func(p *parser, a []string) { p.set(a).ErrorBar.Pattern = p.int(a[1]) }),
		r("S# errorbar size N", func(p *parser, a []string) { p.set(a).ErrorBar.Size = p.num(a[1]) }),
		r("S# errorbar linewidth N", func(p *parser, a []string) { p.set(a).ErrorBar.LineWidth = p.num(a[1]) }),
		r("S# errorbar linestyle N", func(p *parser, a []string) { p.set(a).ErrorBar.LineStyle = p.int(a[1]) }),
		r("S# errorbar riser linewidth N", func(p *parser, a []string) { p.set(a).ErrorBar.RiserWidth = p.num(a[1]) }),
		r("S# errorbar riser linestyle N", func(p *parser, a []string) { p.set(a).ErrorBar.RiserStyle = p.int(a[1]) }),
		r("S# errorbar riser clip W", func(p *parser, a []string) { p.set(a).ErrorBar.RiserClip = p.bool(a[1]) }),
		r("S# errorbar riser clip length N", func(p *parser, a []string) { p.set(a).ErrorBar.RiserClipSize = p.num(a[1]) }),
		r("S# comment S", func(p *parser, a []string) { p.set(a).Comment = a[1] }),
		r("S# legend S", func(p *parser, a []string) { p.set(a).Legend = a[1] }),

		// data: "@target G0.S1", "@type xy", rows, "&"
		r("target W", func(p *parser, a []string) {
			g, s, ok := parseTarget(a[0])
			if !ok {
				p.warn("bad target %q", a[0])
				return
			}
			p.target = p.graphByID(g).setByID(s)
		}),
		r("type W", func(p *parser, a []string) {
			if p.target == nil {
				p.warn("\"@type\" without a \"@target\"")
				return
			}
			p.setType(p.target, a[0])
			p.target.Data = nil
			p.inData = true
		}),
	}
}

// columns is how many numbers one row of each supported set type holds.
var columns = map[string]int{"xy": 2, "xydy": 3}

func (p *parser) setType(s *Set, t string) {
	t = strings.ToLower(t)
	if columns[t] == 0 {
		p.warn("set type %s is not drawn by plot-go (only xy and xydy)", t)
	}
	s.Type = t
}

// parseTarget reads "G0.S1".
func parseTarget(s string) (graph, set int, ok bool) {
	g, st, found := strings.Cut(strings.ToUpper(s), ".")
	if !found || !matchIndex("G", g) || !matchIndex("S", st) {
		return 0, 0, false
	}
	graph, _ = strconv.Atoi(g[1:])
	set, _ = strconv.Atoi(st[1:])
	return graph, set, true
}

func (p *parser) dataRow(line string) {
	fields := strings.Fields(line)
	row := make([]float64, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			p.warn("not a number in data: %q", f)
			return
		}
		row = append(row, v)
	}
	if want := columns[p.target.Type]; want != 0 && len(row) != want {
		p.warn("%s row has %d numbers, not %d", p.target.Type, len(row), want)
		return
	}
	p.target.Data = append(p.target.Data, row)
}

func (p *parser) directive(line string) {
	toks, err := tokenize(line)
	if err != nil {
		p.warn("%v", err)
		return
	}
	if len(toks) == 0 {
		return
	}
	for _, r := range rules {
		if args, ok := r.match(toks); ok {
			r.handle(p, args)
			return
		}
	}
	p.warn("unknown directive")
}

// Parse reads a Grace project. It fails only on a read error; anything it
// cannot use is a Warning in the Project, and reading goes on.
func Parse(rd io.Reader) (*Project, error) {
	p := &parser{proj: &Project{Fonts: map[int]Font{}, Colors: map[int]Color{}}}
	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		p.line++
		p.text = sc.Text()
		line := strings.TrimSpace(p.text)
		if p.inData {
			switch {
			case line == "&":
				p.inData = false
				continue
			case strings.HasPrefix(line, "@"):
				p.warn("data of %s ended without \"&\"", "a set")
				p.inData = false
			case line == "":
				continue
			default:
				p.dataRow(line)
				continue
			}
		}
		// Outside data every line is a directive; parameter files (.agr-par)
		// write them without the "@".
		if line != "" && !strings.HasPrefix(line, "#") {
			p.directive(strings.TrimPrefix(line, "@"))
		}
	}
	if p.inData {
		p.warn("file ended inside a set's data (no \"&\")")
	}
	return p.proj, sc.Err()
}

// ParseFile reads a Grace project from a file.
func ParseFile(name string) (*Project, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}
