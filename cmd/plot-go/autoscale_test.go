package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScaleAxis: OneFit's --autox/--autoy rule (OneFit-Engine's
// Axis.scale): a 2/5/10 step for about five intervals, the ends a quarter
// step out and on a step; decades from 0.9 min to 1.1 max on a log axis.
func TestScaleAxis(t *testing.T) {
	for _, c := range []struct {
		lo, hi float64
		log    bool
		want   axisScale
	}{
		{1, 4, false, axisScale{min: 0, max: 5, step: 1, minor: 1, format: "decimal"}},
		{1.9, 16, false, axisScale{min: 0, max: 20, step: 5, minor: 1, format: "decimal"}},
		{0.013, 0.087, false, axisScale{min: 0, max: 0.1, step: 0.02, minor: 1, prec: 2, format: "decimal"}},
		{-3.2, 7.1, false, axisScale{min: -5, max: 10, step: 5, minor: 1, format: "decimal"}},
		{0.3, 0.3, false, axisScale{min: 0.1, max: 0.5, step: 0.1, minor: 1, prec: 1, format: "decimal"}},
		{0, 0, false, axisScale{min: -0.6, max: 0.6, step: 0.2, minor: 1, prec: 1, format: "decimal"}},
		{0.0015, 1, true, axisScale{min: 0.00135, max: 1.1, step: 10, minor: 9, format: "power"}},
	} {
		if got := scaleAxis(c.lo, c.hi, c.log); got != c.want {
			t.Errorf("scaleAxis(%g, %g, %v) = %+v, want %+v", c.lo, c.hi, c.log, got, c.want)
		}
	}
}

// bare runs plot-go on data files without a -param file and returns the
// saved project and the warnings.
func bare(t *testing.T, args []string, files map[string]string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, s := range files {
		os.WriteFile(filepath.Join(dir, name), []byte(s), 0o644)
	}
	for i, a := range args {
		if _, ok := files[a]; ok {
			args[i] = filepath.Join(dir, a)
		}
	}
	agr := filepath.Join(dir, "out.agr")
	var stdout, stderr bytes.Buffer
	args = append(args, "-hdevice", "EPS", "-hardcopy", "-printfile", filepath.Join(dir, "out.eps"), "-saveall", agr)
	if code := runGrace(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	b, err := os.ReadFile(agr)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), stderr.String()
}

// TestBareDataIsScaled: data without a data window is drawn in one from
// the data, error bars included, with OneFit's page and view.
func TestBareDataIsScaled(t *testing.T) {
	project, warnings := bare(t, []string{"-nxy", "c.dat", "-settype", "xydy", "g.dat"}, map[string]string{
		"c.dat": "1 2\n2 4\n3 9\n4 16\n",
		"g.dat": "1 2.1 0.2\n2 3.9 0.3\n3 9.2 0.4\n4 15.5 2\n",
	})
	if warnings != "" {
		t.Errorf("warnings: %s", warnings)
	}
	// y: 1.9 (2.1 - 0.2) to 17.5 (15.5 + 2, the error bar)
	for _, want := range []string{
		"@page size 773, 600", "@    world 0, 0, 5, 20", "@    view 0.183247, 0.149994, 1.038397, 0.849964",
		"@    xaxis  tick major 1", "@    yaxis  tick major 5", "@g0 on",
		// OneFit's style: curves are 2.0-wide lines, data symbols with
		// their error bars, the frame 2.0 wide
		"@    s0 line type 1", "@    s0 symbol 0", "@    s0 line linewidth 2.0",
		"@    s1 line type 0", "@    s1 symbol 1", "@    s1 errorbar on", "@    frame linewidth 2.0",
	} {
		if !strings.Contains(project, want+"\n") {
			t.Errorf("project lacks %q", want)
		}
	}
}

// TestBareLogData: -log axes scale by decades from the positive values;
// a log axis without any is drawn Normal, with a warning.
func TestBareLogData(t *testing.T) {
	project, _ := bare(t, []string{"-log", "xy", "-settype", "xydy", "l.dat"}, map[string]string{
		"l.dat": "1 0.002 0.0005\n10 0.03 0.004\n100 0.2 0.02\n1000 0.9 0.1\n",
	})
	for _, want := range []string{"@    world 0.9, 0.00135, 1100, 1.1", "@    xaxes scale Logarithmic", "@    xaxis  ticklabel format power", "@    yaxis  tick minor ticks 9"} {
		if !strings.Contains(project, want+"\n") {
			t.Errorf("project lacks %q", want)
		}
	}
	project, warnings := bare(t, []string{"-log", "y", "n.dat"}, map[string]string{"n.dat": "1 -2\n2 -1\n"})
	if !strings.Contains(warnings, "no positive y values") {
		t.Errorf("warnings: %q", warnings)
	}
	if !strings.Contains(project, "@    yaxes scale Normal\n") || !strings.Contains(project, "@    world 0.8, -2.2, 2.2, -0.8\n") {
		t.Errorf("project:\n%s", project)
	}
}

// TestDataWindowGivenIsKept: a parameter file's data window is used as it
// is - the recorded OneFit calls draw as gracebat did (TestGracebatCall*).
func TestDataWindowGivenIsKept(t *testing.T) {
	project, _ := bare(t, []string{"-nxy", "c.dat", "-param", "p.par"}, map[string]string{
		"c.dat": "1 2\n2 4\n",
		"p.par": "with g0\n    world xmin -10\n    world xmax 10\n    world ymin -1\n    world ymax 1\n",
	})
	if !strings.Contains(project, "world -10, -1, 10, 1\n") || strings.Contains(project, "@page size 773, 600") {
		t.Errorf("project:\n%s", project)
	}
}
