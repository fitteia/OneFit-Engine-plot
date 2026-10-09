// grace-probe runs gracebat on a Grace project and prints what it draws,
// one primitive per line, in viewport units - the black-box measurements
// plot-go is built from (see AGENTS.md: gracebat's output, never Grace's
// source).
//
//	grace-probe FILE.agr       run gracebat, print its drawing
//	grace-probe FILE.eps       print the drawing of an EPS gracebat wrote
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: grace-probe FILE.agr|FILE.eps")
		os.Exit(2)
	}
	in := os.Args[1]
	epsFile := in
	if !strings.HasSuffix(in, ".eps") {
		dir, err := os.MkdirTemp("", "grace-probe")
		if err != nil {
			fail(err)
		}
		defer os.RemoveAll(dir)
		epsFile = filepath.Join(dir, "out.eps")
		out, err := exec.Command("gracebat", "-hdevice", "EPS", "-printfile", epsFile, in).CombinedOutput()
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "gracebat: %s", out)
		}
		if err != nil {
			fail(err)
		}
	}
	f, err := os.Open(epsFile)
	if err != nil {
		fail(err)
	}
	d, err := eps.Parse(f)
	f.Close()
	if err != nil {
		fail(err)
	}
	print(d)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "grace-probe:", err)
	os.Exit(1)
}

func print(d *draw.Drawing) {
	pi, ti := 0, 0
	for _, kind := range d.Order {
		if kind == 't' {
			t := d.Texts[ti]
			ti++
			fmt.Printf("text %-12q %-16s at %.4f %.4f  matrix %.4f %.4f %.4f %.4f  rgb %.2f %.2f %.2f\n",
				t.Str, t.Font, t.At.X, t.At.Y, t.Matrix[0], t.Matrix[1], t.Matrix[2], t.Matrix[3], t.Color[0], t.Color[1], t.Color[2])
			continue
		}
		p := d.Paths[pi]
		pi++
		op := "stroke"
		if p.Fill {
			op = "fill"
		}
		fmt.Printf("%s w %.4f dash %v rgb %.2f %.2f %.2f:", op, p.Style.Width, p.Style.Dash, p.Style.Color[0], p.Style.Color[1], p.Style.Color[2])
		for _, s := range p.Segments {
			if s.Arc != nil {
				a := s.Arc
				fmt.Printf("  arc %.4f %.4f r %.4f %.4f", a.X, a.Y, a.RX, a.RY)
				continue
			}
			fmt.Print(" ")
			for i, pt := range s.Points {
				if i == 4 && len(s.Points) > 6 {
					fmt.Printf(" ...(%d points)", len(s.Points))
					break
				}
				fmt.Printf(" %.4f,%.4f", pt.X, pt.Y)
			}
			if s.Closed {
				fmt.Print(" close")
			}
		}
		fmt.Println()
	}
}
