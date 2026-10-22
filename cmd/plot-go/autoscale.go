package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// A command line with data but no data window ("world") - OneFit's
// compiled engine run without a -param file - is scaled from the data, as
// Grace autoscales it. The rule is OneFit's own (--autox/--autoy,
// OneFit-Engine's Axis.scale), so a bare plot looks like OneFit's: on a
// Normal axis a major step of 2, 5 or 10 times a power of ten giving about
// five intervals, the ends a quarter step out and on a step; on a
// Logarithmic one (-log x, y or xy) a decade step with 9 minor ticks, from
// 0.9 times the smallest value to 1.1 times the largest. The ranges hold
// the error bars too. The rest is OneFit's own project template
// (ofetemplate.go), so a bare plot has the suite's line widths and layout.

const autoIntervals = 5

// errorColumns gives, for each set type readData loads, the columns of
// its x and y errors (see agr.Columns: dxdx is right then left, dydy up
// then down).
var errorColumns = map[string][2][]int{
	"xy":         {nil, nil},
	"xydx":       {{2}, nil},
	"xydy":       {nil, {2}},
	"xydxdx":     {{2, 3}, nil},
	"xydydy":     {nil, {2, 3}},
	"xydxdy":     {{2}, {3}},
	"xydxdxdydy": {{2, 3}, {4, 5}},
}

// axisScale is an axis' data window and ticks.
type axisScale struct {
	min, max, step float64
	minor, prec    int
	format         string
}

// hasWorld reports whether the settings set the data window.
func hasWorld(settings string) bool {
	for _, line := range strings.Split(settings, "\n") {
		f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "@"))
		if len(f) > 0 && f[0] == "world" {
			return true
		}
	}
	return false
}

// logAxes reports which axes the settings make Logarithmic.
func logAxes(settings string) (x, y bool) {
	for _, line := range strings.Split(settings, "\n") {
		f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "@"))
		if len(f) == 3 && f[1] == "scale" {
			switch f[0] {
			case "xaxes":
				x = strings.EqualFold(f[2], "Logarithmic")
			case "yaxes":
				y = strings.EqualFold(f[2], "Logarithmic")
			}
		}
	}
	return x, y
}

// extent is the smallest and largest of the sets' values on one axis
// (axis 0 x, 1 y), error bars included; on a log axis only the positive
// ones count. ok is false when there is no such value.
func (g *graceRun) extent(axis int, log bool) (lo, hi float64, ok bool) {
	lo, hi = math.Inf(1), math.Inf(-1)
	add := func(v float64) {
		if math.IsNaN(v) || math.IsInf(v, 0) || (log && v <= 0) {
			return
		}
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	for _, d := range g.data {
		errs := errorColumns[d.typ][axis]
		for _, row := range d.rows {
			if len(row) <= axis {
				continue
			}
			v := row[axis]
			add(v)
			for _, c := range errs {
				if c < len(row) {
					add(v + math.Abs(row[c]))
					add(v - math.Abs(row[c]))
				}
			}
		}
	}
	return lo, hi, lo <= hi
}

// scaleAxis is OneFit's rule for one axis over [lo, hi].
func scaleAxis(lo, hi float64, log bool) axisScale {
	if log {
		return axisScale{min: 0.9 * lo, max: 1.1 * hi, step: 10, minor: 9, prec: 0, format: "power"}
	}
	if lo == hi {
		// one value: a window around it, as wide as the value (1 for 0)
		w := math.Abs(lo)
		if w == 0 {
			w = 1
		}
		lo, hi = lo-w/2, hi+w/2
	}
	raw := (hi - lo) / autoIntervals
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := 10 * mag
	for _, m := range []float64{2, 5} {
		if m*mag >= raw {
			step = m * mag
			break
		}
	}
	s := axisScale{
		min:    round12(math.Floor((lo-step/4)/step) * step),
		max:    round12(math.Ceil((hi+step/4)/step) * step),
		step:   round12(step),
		minor:  1,
		format: "decimal",
	}
	if e := math.Floor(math.Log10(step)); e < 0 {
		s.prec = int(-e)
	}
	return s
}

// round12 drops the binary noise of a product of steps (0.30000000000000004).
func round12(v float64) float64 {
	r, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 12, 64), 64)
	return r
}

// autoSettings are the settings scaling the loaded data, or "" when the
// settings give a data window or there is no data. Axes with no value to
// scale from (a log axis of no positive values) are warned about; a log
// one is drawn Normal, as the fallback appended after the settings.
func (g *graceRun) autoSettings() (before, after string) {
	settings := g.settings.String()
	if len(g.data) == 0 || hasWorld(settings) {
		return "", ""
	}
	logX, logY := logAxes(settings)
	var axes [2]axisScale
	var b, a strings.Builder
	for i, log := range []bool{logX, logY} {
		name := []string{"x", "y"}[i]
		lo, hi, ok := g.extent(i, log)
		if !ok && log {
			g.warn("no positive " + name + " values for a logarithmic " + name + " axis; Normal used")
			fmt.Fprintf(&a, "@    %saxes scale Normal\n", name)
			log = false
			lo, hi, ok = g.extent(i, false)
		}
		if !ok {
			g.warn("no " + name + " values to scale the " + name + " axis from; 0 to 1 used")
			lo, hi = 0, 1
		}
		axes[i] = scaleAxis(lo, hi, log)
	}
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	scale := func(log bool) string {
		if log {
			return "Logarithmic"
		}
		return "Normal"
	}
	r := []string{}
	for i, s := range axes {
		a := []string{"x", "y"}[i]
		r = append(r, "{"+a+"min}", num(s.min), "{"+a+"max}", num(s.max), "{"+a+"scale}", scale(s.format == "power"),
			"{"+a+"step}", num(s.step), "{"+a+"minor}", strconv.Itoa(s.minor),
			"{"+a+"format}", s.format, "{"+a+"prec}", strconv.Itoa(s.prec))
	}
	b.WriteString(ofeHead)
	b.WriteString(strings.NewReplacer(r...).Replace(ofeGraph))
	// a set with error bars is OneFit's data (symbols, its error bars
	// drawn: they are in the window), one without a curve (a line)
	for k, d := range g.data {
		symbol, line := "0", "1"
		if len(errorColumns[d.typ][0])+len(errorColumns[d.typ][1]) > 0 {
			symbol, line = "1", "0"
		}
		b.WriteString(strings.NewReplacer("{k}", strconv.Itoa(k), "{type}", d.typ, "{symbol}", symbol,
			"{line}", line, "{errorbar}", "on").Replace(ofeSet))
	}
	return paramText(b.String()), a.String()
}
