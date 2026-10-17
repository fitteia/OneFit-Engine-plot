package agr_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

func TestAnnotatedValuesEditsSurviveSave(t *testing.T) {
	p, err := agr.Parse(strings.NewReader(`@version 50125
@with g0
@s0 avalue on
@s0 avalue type 4
@s0 avalue prepend "old"
@s0 frobnicate 7
@target G0.S0
@type xy
1 2
3 4 ""
5 6 "peak\S2\N"
7 8
9 10 "broken
11 12 "two" "labels"
13 14 "tail" junk
&
`))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	if len(p.Warnings) != 4 {
		t.Fatalf("warnings: %v", p.Warnings)
	}
	want := []string{"", "", `peak\S2\N`, ""}
	if !reflect.DeepEqual(s.Labels, want) || len(s.Data) != 4 {
		t.Fatalf("rows %v labels %q", s.Data, s.Labels)
	}
	s.AValue = agr.AnnotatedValues{On: true, Type: 3, CharSize: 1.5, Font: 4, Color: 2, Rot: 37, Format: "exponential", Prec: 2, Prepend: "[", Append: "]", Offset: [2]float64{.1, -.2}}
	s.ID = 5
	var out bytes.Buffer
	if err := agr.Write(&out, p); err != nil {
		t.Fatal(err)
	}
	q, err := agr.Parse(&out)
	if err != nil {
		t.Fatal(err)
	}
	got := q.Graphs[0].Sets[0]
	if got.AValue != s.AValue || !reflect.DeepEqual(got.Labels, want) {
		t.Fatalf("after save: %+v %q", got.AValue, got.Labels)
	}
	if !strings.Contains(strings.Join(got.Kept, "\n"), "frobnicate 7") {
		t.Fatal("unknown directive lost")
	}
}

func TestPointLabelsResetWithData(t *testing.T) {
	p, err := agr.Parse(strings.NewReader("@with g0\n@target G0.S0\n@type xy\n1 2 \"old\"\n&\n@target G0.S0\n@type xy\n3 4\n&\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	if len(s.Labels) != 0 || len(s.Data) != 1 || s.Data[0][0] != 3 {
		t.Fatalf("stale labels: %+v", s)
	}
}
