package backend

import (
	"bytes"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// Invalid page dimensions previously produced a 2x2-point PDF despite the
// renderer announcing a normal page. Every backend must agree with a
// project that states the fallback explicitly, without changing the input.
func TestInvalidPageSizesMatchExplicitFallback(t *testing.T) {
	for name, write := range map[string]func(io.Writer, *draw.Drawing, float64, float64, string) error{
		"pdf": WritePDF, "eps": WriteEPS, "svg": WriteSVG,
	} {
		t.Run(name, func(t *testing.T) {
			p, err := agr.ParseFile("../testdata/corpus/test3-1.agr")
			if err != nil {
				t.Fatal(err)
			}
			p.PageWidth, p.PageHeight = 792, 612
			d, _ := render.Render(p)
			var want bytes.Buffer
			if err := write(&want, d, 792, 612, "test"); err != nil {
				t.Fatal(err)
			}
			for _, size := range [][2]float64{{0, 0}, {-1, 600}, {773, 0}, {math.NaN(), 600}, {773, math.Inf(1)}} {
				p.PageWidth, p.PageHeight = size[0], size[1]
				d, warnings := render.Render(p)
				if !strings.Contains(strings.Join(warnings, " "), "page size") {
					t.Fatalf("%v: missing warning", size)
				}
				var got bytes.Buffer
				if err := write(&got, d, p.PageWidth, p.PageHeight, "test"); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got.Bytes(), want.Bytes()) {
					t.Errorf("%v differs from explicit fallback", size)
				}
				if p.PageHeight != size[1] || (!math.IsNaN(size[0]) && p.PageWidth != size[0]) {
					t.Fatal("project mutated")
				}
			}
		})
	}
}
