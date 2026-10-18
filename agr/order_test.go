package agr_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

func TestShapeOrderRoundTrip(t *testing.T) {
	for _, loc := range []string{"view", "world"} {
		for _, top := range []bool{false, true} {
			p, err := agr.Parse(strings.NewReader("@g0 on\n@with g0\n@world 0, 0, 1, 1\n"))
			if err != nil {
				t.Fatal(err)
			}
			for _, kind := range []string{"line", "ellipse", "box"} {
				s := agr.NewShape(kind, p.Defaults)
				s.On = true
				s.LocType = loc
				s.OnTop = top
				s.X1 = .2
				s.Y1 = .2
				s.X2 = .8
				s.Y2 = .8
				p.AddShape(s)
			}
			check := func(want []int) {
				t.Helper()
				if got := p.ShapeOrder(0, loc == "world", top); !reflect.DeepEqual(got, want) {
					t.Fatalf("order %v want %v", got, want)
				}
				d, _ := render.Render(p)
				var got []int
				last := ""
				for _, path := range d.Paths {
					if strings.HasPrefix(path.ID, "object.") && path.ID != last {
						got = append(got, int(path.ID[len(path.ID)-1]-'0'))
						last = path.ID
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("drawn %v want %v", got, want)
				}
			}
			check([]int{2, 1, 0}) // untouched Grace order, across kinds
			if !p.MoveShape(2, 1) {
				t.Fatal("move")
			}
			check([]int{1, 2, 0})
			if !p.MoveShape(0, -1) {
				t.Fatal("backward")
			}
			check([]int{1, 0, 2})
			if p.MoveShape(1, -1) || p.MoveShape(2, 1) || p.MoveShape(-1, 1) || p.MoveShape(0, 2) {
				t.Fatal("boundary moved")
			}
			for n := 0; n < 3; n++ {
				var b bytes.Buffer
				if err := agr.Write(&b, p); err != nil {
					t.Fatal(err)
				}
				p, err = agr.Parse(&b)
				if err != nil {
					t.Fatal(err)
				}
				check([]int{1, 0, 2})
				if p.Shapes[0].Kind != "line" || p.Shapes[0].X1 != .2 || p.Shapes[0].LocType != loc {
					t.Fatal("identity/coordinates changed")
				}
			}
			// A newly read same-kind object must not inherit ordering metadata.
			var b bytes.Buffer
			agr.Write(&b, p)
			q, err := agr.Parse(strings.NewReader(b.String() + "\n@with box\n@box on\n@box def\n"))
			if err != nil || q.Shapes[3].DrawOrder != 0 {
				t.Fatal("order inherited", err)
			}
			// Reordering cannot cross a different attachment or foreground layer.
			isolated := agr.NewShape("box", p.Defaults)
			isolated.OnTop = !top
			isolated.LocType = loc
			p.AddShape(isolated)
			if p.MoveShape(3, 1) || p.MoveShape(3, -1) {
				t.Fatal("crossed layer")
			}
			other := agr.NewShape("box", p.Defaults)
			other.OnTop = top
			if loc == "world" {
				other.LocType = "world"
				other.Graph = 1
			} else {
				other.LocType = "world"
				other.Graph = 0
			}
			p.AddShape(other)
			if p.MoveShape(4, 1) || p.MoveShape(4, -1) {
				t.Fatal("crossed attachment")
			}
			copied := p.CopyShapeFrom(p, 0)
			if copied.DrawOrder != 0 {
				t.Fatal("copy imported order")
			}
			p.DeleteShape(1)
			b.Reset()
			if err := agr.Write(&b, p); err != nil {
				t.Fatal(err)
			}
		}
	}
}
