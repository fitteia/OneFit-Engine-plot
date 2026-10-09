package render

import (
	"reflect"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

func axis(scale string, major float64, minor int) agr.Axis {
	return agr.Axis{Scale: scale, Tick: agr.Ticks{Major: major, MinorTicks: minor, Default: 6}}
}

func TestTicks(t *testing.T) {
	for _, c := range []struct {
		name           string
		ax             agr.Axis
		lo, hi         float64
		majors, minors []float64
	}{
		// OneFit-Engine test 3, block 2: 0 is just outside the window
		{"edge just above a tick", axis("Normal", 5, 1), 1e-06, 29.999,
			[]float64{5, 10, 15, 20, 25}, []float64{2.5, 7.5, 12.5, 17.5, 22.5, 27.5}},
		{"edge on a tick", axis("Normal", 0.02, 1), 0, 0.05,
			[]float64{0, 0.02, 0.04}, []float64{0.01, 0.03, 0.05}},
		{"log window on decades", axis("Logarithmic", 10, 2), 1e3, 1e5,
			[]float64{1e3, 1e4, 1e5}, []float64{2e3, 3e3, 2e4, 3e4}},
		{"log window mid-decade", axis("Logarithmic", 10, 1), 2e4, 1e6,
			[]float64{1e5, 1e6}, []float64{2e4, 2e5}},
	} {
		got := makeTicks(c.ax, c.lo, c.hi)
		if !reflect.DeepEqual(got.majors, c.majors) || !near(got.minors, c.minors) {
			t.Errorf("%s: majors %v minors %v, want %v %v", c.name, got.majors, got.minors, c.majors, c.minors)
		}
	}
}

func near(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if d := a[i] - b[i]; d > 1e-12*max(1, b[i]) || d < -1e-12*max(1, b[i]) {
			return false
		}
	}
	return true
}

func TestAutoStep(t *testing.T) {
	// measured from gracebat: the smallest 1, 2, 5 x 10^n step giving at
	// most 7 intervals
	for span, want := range map[float64]float64{60: 10, 80: 20, 140: 20, 150: 50, 330: 50, 360: 100, 700: 100, 800: 200, 0.11: 0.02, 444: 100} {
		if got := autoStep(span, 6); got != want {
			t.Errorf("autoStep(%g) = %g, want %g", span, got, want)
		}
	}
}
