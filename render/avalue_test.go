package render

import (
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

func TestAnnotatedValuesVisibilityAndWarnings(t *testing.T) {
	p, err := agr.ParseFile("../testdata/corpus/style-avalue-4.agr")
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	d, w := Render(p)
	if len(w) != 0 || len(d.Texts) == 0 {
		t.Fatalf("render %v %v", d.Texts, w)
	}
	for _, text := range d.Texts {
		if text.ID != "g0.s0.avalue" {
			t.Errorf("annotation ID %q", text.ID)
		}
	}
	s.Hidden = true
	d, w = Render(p)
	if len(d.Texts) != 0 || len(w) != 0 {
		t.Fatalf("hidden annotations: %v %v", d.Texts, w)
	}
	s.Hidden = false
	s.AValue.Type = 99
	d, w = Render(p)
	if len(d.Texts) != 0 || len(w) != 1 || !strings.Contains(w[0], "annotated value type 99") {
		t.Fatalf("unknown type: %v %v", d.Texts, w)
	}
	s.AValue.On = false
	_, w = Render(p)
	if len(w) != 0 {
		t.Fatalf("disabled annotation warns: %v", w)
	}
}
