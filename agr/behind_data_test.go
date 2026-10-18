package agr_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

func TestBehindDataLayerKeepsCurvesAndTextAboveObject(t *testing.T) {
	src := `@page size 600, 450
@g0 on
@with g0
@view 0.1, 0.1, 0.9, 0.9
@world 0, 0, 10, 10
@target G0.S0
@type xy
1 1
9 9
&
@with string
@string on
@string loctype world
@string 5, 5
@string def "label"
@with ellipse
@ellipse on
@ellipse loctype world
@ellipse g0
@ellipse 2, 2, 8, 8
@ellipse fill color 1
@ellipse fill pattern 1
@ellipse def
`
	p, err := agr.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Shapes) != 1 {
		t.Fatalf("shapes=%d", len(p.Shapes))
	}
	if !p.MoveShapeAcrossLayers(0, -1) || !p.Shapes[0].BehindData {
		t.Fatal("send backward did not cross behind data")
	}
	if !p.MoveShapeAcrossLayers(0, 1) || p.Shapes[0].BehindData || p.Shapes[0].OnTop {
		t.Fatal("bring forward did not return to normal layer")
	}
	if !p.MoveShapeAcrossLayers(0, 1) || !p.Shapes[0].OnTop {
		t.Fatal("bring forward did not cross above text")
	}
	if !p.MoveShapeAcrossLayers(0, -1) || p.Shapes[0].OnTop || p.Shapes[0].BehindData {
		t.Fatal("send backward did not return below text")
	}
	if !p.MoveShapeAcrossLayers(0, -1) || !p.Shapes[0].BehindData {
		t.Fatal("send backward did not return behind data")
	}
	if p.MoveShapeAcrossLayers(0, -1) {
		t.Fatal("moved past back layer")
	}
	p.Shapes[0].BehindData = true
	renderIDs := func() []string {
		d, _ := render.Render(p)
		ids := make([]string, 0, len(d.Order))
		pi, ti := 0, 0
		for _, kind := range d.Order {
			if kind == 'p' {
				ids = append(ids, d.Paths[pi].ID)
				pi++
			} else {
				ids = append(ids, d.Texts[ti].ID)
				ti++
			}
		}
		return ids
	}
	ids := renderIDs()
	index := func(id string) int {
		for i, v := range ids {
			if v == id {
				return i
			}
		}
		return -1
	}
	setIndex := -1
	for i, id := range ids {
		if strings.HasPrefix(id, "g0.s0.") {
			setIndex = i
			break
		}
	}

	if index("object.0") < 0 || setIndex < 0 || index("object.0") > setIndex {
		t.Fatalf("shape not behind data: %v", ids)
	}
	if index("string.0") < index("object.0") {
		t.Fatalf("text not above shape: %v", ids)
	}
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "# plot-go: next object behind-data") {
		t.Fatal("missing behind-data marker")
	}
	q, err := agr.Parse(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !q.Shapes[0].BehindData || q.Shapes[0].OnTop {
		t.Fatal("layer was not round-tripped")
	}
}
