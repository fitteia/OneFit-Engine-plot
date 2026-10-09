package render

import (
	"math"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

// maxTicks: gracebat gives up on the file's tick spacing above about this
// many major ticks, or minor ticks across the major intervals the window
// touches, and picks its own (docs/grace-behaviour.md).
const maxTicks = 255

// ticks are an axis's tick values in world units.
type ticks struct {
	majors, minors []float64
	auto           bool // the file's spacing was replaced (too many ticks)
}

// edgeTol is how close to the window's edge a tick may fall and still count
// as inside it, relative to the step.
const edgeTol = 1e-6

func makeTicks(ax agr.Axis, lo, hi float64) ticks {
	if lo > hi {
		lo, hi = hi, lo
	}
	major, minor := ax.Tick.Major, ax.Tick.MinorTicks
	if ax.Scale == "Logarithmic" {
		if major <= 1 || lo <= 0 {
			return ticks{}
		}
		return logTicks(lo, hi, major, minor)
	}
	if !(major > 0) {
		return ticks{}
	}
	if tooMany(lo, hi, major, minor) {
		major = autoStep(hi-lo, ax.Tick.Default)
		minor = min(minor, 1)
		t := linTicks(lo, hi, major, minor)
		t.auto = true
		return t
	}
	return linTicks(lo, hi, major, minor)
}

func tooMany(lo, hi, major float64, minor int) bool {
	first, last := math.Floor(lo/major), math.Ceil(hi/major)
	intervals := last - first
	majors := math.Floor(hi/major+edgeTol) - math.Ceil(lo/major-edgeTol) + 1
	return majors > maxTicks || intervals*float64(minor) > maxTicks
}

// autoStep is the smallest of 1, 2, 5 x 10^n giving at most def+1
// intervals over span (def is the axis's "tick default", 6 in OneFit's
// files).
func autoStep(span float64, def int) float64 {
	if def <= 0 {
		def = 6
	}
	limit := float64(def + 1)
	p := math.Pow(10, math.Floor(math.Log10(span/limit)))
	for {
		for _, m := range []float64{1, 2, 5} {
			if span/(m*p) <= limit*(1+edgeTol) {
				return m * p
			}
		}
		p *= 10
	}
}

func linTicks(lo, hi, major float64, minor int) ticks {
	var t ticks
	first := math.Floor(lo/major) - 1
	last := math.Ceil(hi/major) + 1
	tol := major * edgeTol
	for k := first; k <= last; k++ {
		v := k * major
		if v >= lo-tol && v <= hi+tol {
			t.majors = append(t.majors, clean(v, major))
		}
		// minors only from intervals reaching into the window, not
		// merely touching its edge
		if v+major <= lo+tol || v >= hi-tol {
			continue
		}
		for i := 1; i <= minor; i++ {
			m := v + float64(i)*major/float64(minor+1)
			if m >= lo-tol && m <= hi+tol {
				t.minors = append(t.minors, m)
			}
		}
	}
	return t
}

// clean turns a value that should be a multiple of step but carries
// rounding noise (0.06000000000000001, -1e-18) into the exact multiple's
// nearest float.
func clean(v, step float64) float64 {
	n := math.Round(v / step)
	if math.Abs(v/step-n) < 1e-9 {
		v = n * step
		if n == 0 {
			v = 0
		}
	}
	return v
}

// logTicks: majors at powers of the major factor, minors at 2x, 3x, ...
// (m+1)x each major (docs/grace-behaviour.md).
func logTicks(lo, hi, factor float64, minor int) ticks {
	var t ticks
	lf := math.Log(factor)
	first := math.Floor(math.Log(lo)/lf) - 1
	last := math.Ceil(math.Log(hi)/lf) + 1
	if last-first > 4*maxTicks {
		return t
	}
	in := func(v float64) bool { return v >= lo*(1-edgeTol) && v <= hi*(1+edgeTol) }
	for k := first; k <= last; k++ {
		v := math.Pow(factor, k)
		if r := math.Round(v); math.Abs(v-r) < 1e-9*v {
			v = r
		}
		if in(v) {
			t.majors = append(t.majors, v)
		}
		// minors only from decades reaching into the window, not merely
		// touching its edge (a window starting at 1e4 gets none from
		// 1e3..1e4; one starting at 2e4 gets 1e4's from 2e4 on)
		if v*factor <= lo*(1+edgeTol) || v >= hi*(1-edgeTol) {
			continue
		}
		for i := 1; i <= minor; i++ {
			if m := v * float64(1+i); in(m) {
				t.minors = append(t.minors, m)
			}
		}
	}
	return t
}
