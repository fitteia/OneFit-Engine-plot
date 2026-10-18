package render

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

func barProject(t *testing.T, typ, lineType, symbolType, fillPattern, symbolSkip, data string) string {
	t.Helper()
	b, err := os.ReadFile("../testdata/corpus/style-objects-order.agr")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	replace := func(old, new string) {
		t.Helper()
		if !strings.Contains(s, old) {
			t.Fatalf("bar fixture lacks %q", old)
		}
		s = strings.Replace(s, old, new, 1)
	}
	replace("@    s0 type xy", "@    s0 type "+typ)
	replace("@    s0 line type 1", "@    s0 line type "+lineType)
	replace("@    s0 symbol 0", "@    s0 symbol "+symbolType)
	replace("@    s0 symbol fill pattern 0", "@    s0 symbol fill pattern "+fillPattern)
	replace("@    s0 symbol skip 0", "@    s0 symbol skip "+symbolSkip)
	replace("@target G0.S0\n@type xy\n               0                0\n              10               10\n&",
		"@target G0.S0\n@type "+typ+"\n"+data+"\n&")
	p, err := agr.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	p.Shapes, p.Objects, p.Strings = nil, nil, nil
	if len(p.Graphs) == 0 || len(p.Graphs[0].Sets) == 0 {
		t.Fatal("bar fixture has no graph/set")
	}
	p.Graphs = p.Graphs[:1]
	p.Graphs[0].Sets = p.Graphs[0].Sets[:1]
	p.Graphs[0].Sets[0].ErrorBar.On = false
	p.Graphs[0].Legend.On = false
	var out bytes.Buffer
	if err := agr.Write(&out, p); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestBarFamilyMatchesGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	tests := []struct {
		name, typ, lineType, symbolType, fillPattern, symbolSkip, data string
	}{
		{"bar-filled", "bar", "1", "0", "1", "0", "1 3\n3 7\n5 4"},
		{"bar-symbol", "bar", "0", "1", "1", "0", "1 3\n3 7\n5 4"},
		{"bar-legend", "bar", "1", "0", "1", "0", "1 3\n3 7\n5 4"},
		{"bardy-errors", "bardy", "1", "0", "1", "0", "1 3 0.5\n3 7 1\n5 4 0.75"},
		{"bar-line", "bar", "1", "0", "0", "0", "1 3\n3 7\n5 4"},
		{"bardy-line", "bardy", "1", "0", "0", "0", "1 3 0.5\n3 7 1\n5 4 0.75"},
		{"bardydy-line", "bardydy", "1", "0", "0", "0", "1 3 0.25 0.75\n3 7 1 2\n5 4 0.5 1.5"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := barProject(t, tc.typ, tc.lineType, tc.symbolType, tc.fillPattern, tc.symbolSkip, tc.data)
			if strings.Contains(tc.name, "legend") || strings.Contains(tc.name, "errors") {
				project, err := agr.Parse(strings.NewReader(input))
				if err != nil {
					t.Fatal(err)
				}
				project.Graphs[0].Legend.On = strings.Contains(tc.name, "legend")
				project.Graphs[0].Sets[0].Legend = "bars"
				project.Graphs[0].Sets[0].ErrorBar.On = strings.Contains(tc.name, "errors")
				var encoded bytes.Buffer
				if err := agr.Write(&encoded, project); err != nil {
					t.Fatal(err)
				}
				input = encoded.String()
			}
			dir := t.TempDir()
			in, out := filepath.Join(dir, "bars.agr"), filepath.Join(dir, "grace.eps")
			if err := os.WriteFile(in, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			if log, err := exec.Command(grace, "-nosafe", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
				t.Fatalf("gracebat: %v: %s", err, log)
			}
			f, err := os.Open(out)
			if err != nil {
				t.Fatal(err)
			}
			want, err := eps.Parse(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			p, err := agr.Parse(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			got, warnings := Render(p)
			for _, warning := range warnings {
				if strings.Contains(strings.ToLower(warning), "bar") {
					t.Fatalf("unexpected bar warning: %v", warnings)
				}
			}
			if differences := drawcmp.Compare(got, want, 20); len(differences) > 0 {
				t.Fatalf("plot-go differs from gracebat:\n%s", strings.Join(differences, "\n"))
			}
		})
	}
}

func TestBarSetRectanglesAndSkip(t *testing.T) {
	input := barProject(t, "bar", "0", "1", "0", "1", "1 3\n3 7\n5 4")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	d, warnings := Render(p)
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), "bar") {
			t.Fatalf("unexpected bar warning: %v", warnings)
		}
	}
	var bars []int
	for i, path := range d.Paths {
		if path.ID == "g0.s0.bars" {
			bars = append(bars, i)
		}
	}
	if len(bars) != 2 {
		t.Fatalf("got %d bars with symbol skip 1, want 2", len(bars))
	}
	for _, i := range bars {
		seg := d.Paths[i].Segments[0]
		if !seg.Closed || len(seg.Points) != 5 || seg.Points[0] != seg.Points[4] {
			t.Errorf("bar path is not a four-sided rectangle: %+v", seg)
		}
	}
}

func TestBarSymbolFillLegendAndErrors(t *testing.T) {
	input := barProject(t, "bardydy", "0", "1", "1", "0", "1 3 0.25 0.75\n3 7 1 2")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	s.Legend = "measurements"
	s.ErrorBar.On = true
	p.Graphs[0].Legend.On = true
	d, warnings := Render(p)
	for _, warning := range warnings {
		if strings.Contains(strings.ToLower(warning), "bar") {
			t.Fatalf("unexpected bar warning: %v", warnings)
		}
	}
	seen := map[string]bool{}
	for _, path := range d.Paths {
		seen[path.ID] = true
	}
	for _, id := range []string{"g0.s0.bars.fill", "g0.s0.bars", "g0.s0.errorbars", "g0.legend.bars.fill", "g0.legend.bars"} {
		if !seen[id] {
			t.Errorf("missing rendered bar path %q", id)
		}
	}
}

func TestBarFillOnlyLegend(t *testing.T) {
	input := barProject(t, "bar", "0", "1", "1", "0", "1 3\n3 7")
	p, err := agr.Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	s := p.Graphs[0].Sets[0]
	s.Symbol.LineStyle = 0
	s.Legend = "filled bars"
	p.Graphs[0].Legend.On = true
	d, _ := Render(p)
	seen := false
	for _, path := range d.Paths {
		if path.ID == "g0.legend.bars" {
			t.Fatal("fill-only legend has an outline")
		}
		if path.ID == "g0.legend.bars.fill" {
			seen = true
		}
	}
	if !seen {
		t.Fatal("fill-only legend sample is missing")
	}
}
