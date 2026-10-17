package agr_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// withoutKept is a copy of p without its kept directives.
func withoutKept(p *agr.Project) *agr.Project {
	c := *p
	c.Kept = nil
	c.Graphs = nil
	for _, g := range p.Graphs {
		gc := *g
		gc.Kept, gc.KeptHeader, gc.Sets = nil, nil, nil
		for _, s := range g.Sets {
			sc := *s
			sc.Kept = nil
			gc.Sets = append(gc.Sets, &sc)
		}
		c.Graphs = append(c.Graphs, &gc)
	}
	return &c
}

// TestWriteKeepsWhatItDoesNotModel: directives plot-go neither models nor
// draws - here a region, a stacked graph, an axis bar
// and an unknown one - are written back as read, the last of each kind
// winning as in Grace, and stay with their set when it is renumbered
// (a figure's sets are).
func TestWriteKeepsWhatItDoesNotModel(t *testing.T) {
	in := `@version 50125
@r0 on
@link r0 to g0
@g0 on
@g0 stacked true
@with g0
@    world 0, 0, 1, 1
@    xaxis  bar on
@    s0 avalue off
@    s0 avalue on
@    s0 avalue prec 3
@    s0 frobnicate 7
@target G0.S0
@type xy
0 0
1 1
&
`
	p, err := agr.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	p.Graphs[0].Sets[0].ID = 5
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"@r0 on\n", "@link r0 to g0\n", "@g0 stacked true\n", "@    xaxis  bar on\n",
		"@    s5 avalue on\n", "@    s5 avalue prec 3\n", "@    s5 frobnicate 7\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("not written: %q", want)
		}
	}
	for _, gone := range []string{"avalue off\n", "@g0 stacked false", "xaxis  bar off", "@r0 off"} {
		if strings.Contains(out, gone) {
			t.Errorf("written although replaced: %q", gone)
		}
	}
	q, err := agr.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(q.Graphs[0].Sets[0].Kept, "\n") + "\n"
	for _, want := range []string{"frobnicate 7\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("read back: set keeps %q, not %q", got, want)
		}
	}
}

// TestWriteKeepsDrawingObjects: each drawing object is written back whole,
// as gracebat writes it - two lines are not one line's settings, the second
// replacing the first.
func TestWriteKeepsDrawingObjects(t *testing.T) {
	objects := `@with line
@    line on
@    line loctype view
@    line 0.3, 0.3, 0.6, 0.6
@    line color 2
@    line arrow 2
@line def
@with box
@    box on
@    box 0.4, 0.4, 0.5, 0.5
@    box fill color 3
@box def
@with line
@    line off
@    line loctype world
@    line g0
@    line 0.1, 0.8, 0.2, 0.9
@    line color 4
@line def
`
	in := "@version 50125\n@timestamp def \"\"\n" + objects + "@g0 on\n@with g0\n@    world 0, 0, 1, 1\n"
	p, err := agr.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Objects) != 3 {
		t.Fatalf("%d objects read, not 3", len(p.Objects))
	}
	if len(p.Warnings) != 0 {
		t.Errorf("warnings: %v", p.Warnings)
	}
	// each read as well (defaults: measured from gracebat)
	want := []agr.Shape{
		{Kind: "line", On: true, LocType: "view", X1: 0.3, Y1: 0.3, X2: 0.6, Y2: 0.6, LineStyle: 1, LineWidth: 1, Color: 2, FillColor: 1, Arrow: 2, ArrowLength: 1, ArrowDL: 1, ArrowLL: 1},
		{Kind: "box", On: true, LocType: "view", X1: 0.4, Y1: 0.4, X2: 0.5, Y2: 0.5, LineStyle: 1, LineWidth: 1, Color: 1, FillColor: 3, ArrowLength: 1, ArrowDL: 1, ArrowLL: 1},
		{Kind: "line", LocType: "world", X1: 0.1, Y1: 0.8, X2: 0.2, Y2: 0.9, LineStyle: 1, LineWidth: 1, Color: 4, FillColor: 1, Arrow: 2, ArrowLength: 1, ArrowDL: 1, ArrowLL: 1},
	}
	for i, s := range p.Shapes {
		if *s != want[i] {
			t.Errorf("object %d read as %+v, not %+v", i, *s, want[i])
		}
	}
	if len(p.Graphs) != 1 || len(p.Graphs[0].Kept) != 0 || len(p.Kept) != 0 {
		t.Errorf("object lines kept elsewhere too: project %q, graph %q", p.Kept, p.Graphs[0].Kept)
	}
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "@timestamp def \"\"\n"+objects+"@g0 on\n") {
		t.Errorf("objects not written as read, after the timestamp:\n%s", b.String())
	}
}

func TestEditedDrawingObjectsSurviveRoundTrip(t *testing.T) {
	in := `@with line
@    line on
@    line loctype world
@    line g0
@    line 1, 2, 3, 4
@    line color 2
@    line future setting 42
@line def
@with box
@    box on
@    box 0.1, 0.2, 0.3, 0.4
@box def
@g0 on
@with g0
@    world 0, 0, 5, 5
`
	p, err := agr.Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	// The second block's effective settings must stay the same even if
	// Grace inherited them from the first block.
	box := *p.Shapes[1]
	p.Shapes[0].X2 = 4.5
	p.Shapes[0].Arrow = 3
	p.Shapes[0].ArrowType = 1
	p.Shapes[0].ArrowLength = 2
	p.Shapes[0].ArrowDL = 0.8
	p.Shapes[0].ArrowLL = 0.5
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "line future setting 42") {
		t.Fatal("the unknown setting was lost")
	}
	q, err := agr.Parse(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if *q.Shapes[0] != *p.Shapes[0] || *q.Shapes[1] != box {
		t.Fatalf("edited shape or later shape changed: %+v; %+v", q.Shapes[0], q.Shapes[1])
	}
	var again bytes.Buffer
	if err := agr.Write(&again, q); err != nil || again.String() != b.String() {
		t.Fatal("edited object is not stable after reopening")
	}
}

func TestAddAndDeleteShapesKeepInheritedSettings(t *testing.T) {
	p, err := agr.Parse(strings.NewReader(`@with box
@    box on
@    box 0.1, 0.2, 0.3, 0.4
@    box color 5
@box def
@with box
@    box on
@    box 0.5, 0.6, 0.7, 0.8
@box def
`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Shapes[1].Color != 5 || !p.DeleteShape(0) || p.DeleteShape(9) {
		t.Fatal("deleting the first box failed")
	}
	for _, s := range []*agr.Shape{
		{Kind: "line", On: true, LocType: "view", Graph: 0, X1: 0.1, Y1: 0.2, X2: 0.7, Y2: 0.8, LineStyle: 1, LineWidth: 2, Color: 3, FillColor: 1, Arrow: 2, ArrowType: 1, ArrowLength: 2, ArrowDL: 0.8, ArrowLL: 0.5},
		{Kind: "ellipse", On: true, LocType: "view", Graph: 0, X1: 0.2, Y1: 0.3, X2: 0.8, Y2: 0.9, LineStyle: 1, LineWidth: 1, Color: 4, FillColor: 6, FillPattern: 3, ArrowLength: 1, ArrowDL: 1, ArrowLL: 1},
	} {
		p.AddShape(s)
	}
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	q, err := agr.Parse(strings.NewReader(b.String()))
	if err != nil || len(q.Shapes) != len(p.Shapes) {
		t.Fatalf("reopen: %v, shapes %d", err, len(q.Shapes))
	}
	for i, s := range p.Shapes {
		want := *s
		// Grace's unspecified graph (-1) and explicit g0 have the same
		// effect on page objects; the edited first block writes g0.
		if want.LocType == "view" && want.Graph == -1 {
			want.Graph = 0
		}
		if *q.Shapes[i] != want {
			t.Errorf("shape %d after reopen: %+v, want %+v", i, q.Shapes[i], s)
		}
	}
}

func TestEditedShapeDoesNotChangeLaterInheritedShape(t *testing.T) {
	p, err := agr.Parse(strings.NewReader(`@with line
@    line on
@    line 0, 0, 1, 1
@    line color 2
@line def
@with line
@    line on
@    line 0, 1, 1, 0
@line def
`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Shapes[1].Color != 2 {
		t.Fatal("second line should inherit the first's color")
	}
	p.Shapes[0].Color = 3
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	q, err := agr.Parse(strings.NewReader(b.String()))
	if err != nil || q.Shapes[0].Color != 3 || q.Shapes[1].Color != 2 {
		t.Fatalf("changed an inherited style: %v; first %+v, second %+v", err, q.Shapes[0], q.Shapes[1])
	}
}

// TestWriteRoundTrip: every corpus file, written and read back, is the same
// Project and draws the same - the figure editor saves through Write.
func TestWriteRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("../testdata/corpus/*.agr")
	if len(files) == 0 {
		t.Fatal("no corpus files")
	}
	for _, f := range files {
		p, err := agr.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := agr.Write(&b, p); err != nil {
			t.Fatal(err)
		}
		q, err := agr.Parse(strings.NewReader(b.String()))
		if err != nil {
			t.Fatal(err)
		}
		if len(q.Warnings) != 0 {
			t.Errorf("%s: written file warns: %+v", filepath.Base(f), q.Warnings[0])
		}
		p.Warnings, q.Warnings = nil, nil
		// what a file left out is written as Grace's default and kept when
		// read back: compare without the kept directives, then check that
		// writing again changes nothing
		if !reflect.DeepEqual(withoutKept(p), withoutKept(q)) {
			t.Errorf("%s: read back differently", filepath.Base(f))
			continue
		}
		var b2 bytes.Buffer
		if err := agr.Write(&b2, q); err != nil {
			t.Fatal(err)
		}
		if b2.String() != b.String() {
			t.Errorf("%s: written again differently", filepath.Base(f))
		}
		dp, _ := render.Render(p)
		dq, _ := render.Render(q)
		if !reflect.DeepEqual(dp, dq) {
			t.Errorf("%s: draws differently after a round trip", filepath.Base(f))
		}
	}
}

// TestWriteKeepsAHiddenAlternateAxis: an alternate axis switched off with
// settings of its own is written whole, and comes back with them; one at
// Grace's defaults stays the single "off" line.
func TestWriteKeepsAHiddenAlternateAxis(t *testing.T) {
	p, err := agr.ParseFile("../testdata/corpus/style-axes-alt.agr")
	if err != nil {
		t.Fatal(err)
	}
	g := p.Graphs[0]
	g.AltX.On = false
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "@    altxaxis  off\n@    altxaxis  type zero false") {
		t.Error("a hidden alternate axis with settings is not written whole")
	}
	q, err := agr.Parse(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if q.Graphs[0].AltX.On || q.Graphs[0].AltX.TickLabel.Formula != g.AltX.TickLabel.Formula || q.Graphs[0].AltX.Label.Text != g.AltX.Label.Text {
		t.Errorf("read back: on %v, formula %q, label %q", q.Graphs[0].AltX.On, q.Graphs[0].AltX.TickLabel.Formula, q.Graphs[0].AltX.Label.Text)
	}
	g.AltX = agr.DefaultAxis()
	b.Reset()
	agr.Write(&b, p)
	if strings.Count(b.String(), "altxaxis") != 1 {
		t.Error("an alternate axis at Grace's defaults is more than one line")
	}
}
