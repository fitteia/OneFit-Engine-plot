// plot-go renders a Grace project file (.agr) the way gracebat does,
// without Grace: to PDF, EPS or SVG.
//
//	plot-go [-o OUT] [-format pdf|eps|svg] FILE.agr
//
// Without -o the output is FILE.pdf (or .eps, .svg) next to the input.
// Lines plot-go cannot use, and features it does not draw, are warnings on
// stderr; it still writes the file, as gracebat does.
//
// It also takes gracebat's own command line (see grace.go), so OneFit can
// call it in Grace's place:
//
//	plot-go -settype xydy DATA -nxy CURVES -param PAR.agr-par \
//	  -hdevice EPS -hardcopy -printfile OUT.eps -saveall OUT.agr
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

type writer func(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error

var writers = map[string]writer{
	"pdf": backend.WritePDF,
	"eps": backend.WriteEPS,
	"svg": backend.WriteSVG,
}

func main() {
	if isGrace(os.Args[1:]) {
		os.Exit(runGrace(os.Args[1:], os.Stdout, os.Stderr))
	}
	out := flag.String("o", "", "output file (default: the input with the format's extension)")
	format := flag.String("format", "", "pdf, eps or svg (default: from -o's extension, else pdf)")
	quiet := flag.Bool("q", false, "do not print warnings")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: plot-go [-o OUT] [-format pdf|eps|svg] [-q] FILE.agr")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	in := flag.Arg(0)
	f := strings.ToLower(*format)
	if f == "" {
		f = strings.TrimPrefix(strings.ToLower(filepath.Ext(*out)), ".")
		if writers[f] == nil {
			f = "pdf"
		}
	}
	write := writers[f]
	if write == nil {
		fail(fmt.Errorf("unknown format %q (pdf, eps or svg)", f))
	}
	if *out == "" {
		*out = strings.TrimSuffix(in, filepath.Ext(in)) + "." + f
	}

	p, err := agr.ParseFile(in)
	if err != nil {
		fail(err)
	}
	d, warnings := render.Render(p)
	if !*quiet {
		for _, w := range p.Warnings {
			fmt.Fprintf(os.Stderr, "plot-go: %s:%d: %s\n", in, w.Line, w.Msg)
		}
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "plot-go: %s: %s\n", in, w)
		}
	}
	if err := writeFile(*out, func(w io.Writer) error {
		return write(w, d, p.PageWidth, p.PageHeight, in)
	}); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "plot-go:", err)
	os.Exit(1)
}
