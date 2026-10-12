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
// draws - here annotated values, a region, a stacked graph, an axis bar
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
	for _, gone := range []string{"avalue off", "@g0 stacked false", "xaxis  bar off", "@r0 off"} {
		if strings.Contains(out, gone) {
			t.Errorf("written although replaced: %q", gone)
		}
	}
	q, err := agr.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(q.Graphs[0].Sets[0].Kept, "\n") + "\n"
	for _, want := range []string{"avalue on\n", "avalue prec 3\n", "frobnicate 7\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("read back: set keeps %q, not %q", got, want)
		}
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
