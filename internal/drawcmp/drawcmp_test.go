package drawcmp

import (
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

func drawing(segs ...draw.Segment) *draw.Drawing {
	return &draw.Drawing{Paths: []draw.Path{{Segments: segs, Style: draw.Style{Width: 0.0015}}}, Order: []byte{'p'}}
}

func arc(rx, ry, a1, a2 float64) draw.Segment {
	return draw.Segment{Arc: &draw.Arc{X: 0.5, Y: 0.5, RX: rx, RY: ry, A1: a1, A2: a2}}
}

func square(closed bool) draw.Segment {
	return draw.Segment{Points: []draw.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}, Closed: closed}
}

// Every property the comparator checks must make it fail when it alone
// differs: negative controls, so that a pass means something.
func TestCompareFindsEachDifference(t *testing.T) {
	want := drawing(arc(0.01, 0.01, 0, 360), square(true))
	if d := Compare(drawing(arc(0.01, 0.01, 0, 360), square(true)), want, 10); len(d) != 0 {
		t.Fatalf("identical drawings differ: %v", d)
	}
	for name, got := range map[string]*draw.Drawing{
		"arc rx":     drawing(arc(0.02, 0.01, 0, 360), square(true)),
		"arc ry":     drawing(arc(0.01, 0.99, 0, 360), square(true)),
		"arc start":  drawing(arc(0.01, 0.01, 90, 360), square(true)),
		"arc end":    drawing(arc(0.01, 0.01, 0, 180), square(true)),
		"not closed": drawing(arc(0.01, 0.01, 0, 360), square(false)),
	} {
		if d := Compare(got, want, 10); len(d) == 0 {
			t.Errorf("%s: no difference found", name)
		} else if !strings.Contains(strings.Join(d, " "), "gracebat") {
			t.Errorf("%s: %v", name, d)
		}
	}
}
