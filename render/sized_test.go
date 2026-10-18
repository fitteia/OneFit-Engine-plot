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

func TestSizedSymbolsMatchGracebat(t *testing.T) {
	grace, err := exec.LookPath("gracebat")
	if err != nil {
		t.Skip("gracebat not installed")
	}
	for _, tc := range []struct {
		name, symbol, fill, skip, data string
		legend                         bool
	}{
		{"circles", "1", "0", "0", "1 3 0.5\n3 7 2\n5 4 1", false},
		{"filled-squares", "2", "1", "0", "1 3 0.5\n3 7 2\n5 4 1", false},
		{"skip", "1", "0", "1", "1 3 0.5\n3 7 2\n5 4 1", false},
		{"zero-negative", "1", "0", "0", "1 3 0\n3 7 -2\n5 4 1", false},
		{"legend", "1", "1", "0", "1 3 0.5\n3 7 2\n5 4 1", true},
		{"diamond", "3", "1", "0", "1 3 0.5\n3 7 2\n5 4 1", false},
		{"open-symbol", "8", "0", "0", "1 3 0.5\n3 7 2\n5 4 1", false},
		{"log-clipping", "1", "0", "0", "0.1 3 2\n1 3 0.5\n3 7 2\n11 4 1", false},
		{"character", "11", "0", "0", "1 3 0.5\n3 7 2\n5 4 1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := barProject(t, "xysize", "0", tc.symbol, tc.fill, tc.skip, tc.data)
			p, err := agr.Parse(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			s := p.Graphs[0].Sets[0]
			if tc.name == "log-clipping" {
				p.Graphs[0].World.XMin = 1
				p.Graphs[0].World.XMax = 10
				p.Graphs[0].X.Scale = "Logarithmic"
				p.Graphs[0].X.Tick.Major = 10
				p.Graphs[0].X.Tick.MinorTicks = 9
			}
			s.Symbol.Size = 2
			s.Symbol.Char = 65
			s.Legend = "sizes"
			p.Graphs[0].Legend.On = tc.legend
			var encoded bytes.Buffer
			if err := agr.Write(&encoded, p); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			in, out := filepath.Join(dir, "sized.agr"), filepath.Join(dir, "grace.eps")
			if err := os.WriteFile(in, encoded.Bytes(), 0600); err != nil {
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
			got, warnings := Render(p)
			for _, w := range warnings {
				if strings.Contains(w, "xysize") {
					t.Fatal(w)
				}
			}
			if d := drawcmp.Compare(got, want, 15); len(d) > 0 {
				t.Fatalf("differs from gracebat:\n%s", strings.Join(d, "\n"))
			}
		})
	}
}
