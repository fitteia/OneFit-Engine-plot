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
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// version is set by scripts/build-release.sh (-ldflags -X main.version=...).
var version = "dev"

// The repository's LICENSE and NOTICE, kept in step by
// TestNoticesMatchTheRepository, travel inside the binary with Adobe's
// notice for the font metrics it embeds: plot-go -notices prints them.
//
//go:embed NOTICE
var notice string

//go:embed LICENSE
var license string

func printNotices() {
	fmt.Print(notice, "\n", "---- Adobe Core 14 AFM files (embedded font metrics): MustRead.html ----\n\n", afm.AdobeNotice(),
		"\n\n---- Liberation fonts (embedded): SIL Open Font License 1.1 ----\n\n", glyph.FontLicense,
		"\n---- github.com/go-fonts/liberation (embedded): BSD license ----\n\n", glyph.PackageLicense,
		"\n---- LICENSE ----\n\n", license)
}

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
	showVersion := flag.Bool("version", false, "print the version and exit")
	showNotices := flag.Bool("notices", false, "print the license and third-party notices and exit")
	outlineText := flag.Bool("outline-text", false, "SVG: draw text as glyph outlines (the same in every browser) instead of <text>")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: plot-go [-o OUT] [-format pdf|eps|svg] [-q] FILE.agr")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showNotices {
		printNotices()
		return
	}
	if *showVersion {
		fmt.Println("plot-go", version)
		return
	}
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
	if f == "svg" && *outlineText {
		write = func(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
			return backend.WriteSVGWith(w, d, pageW, pageH, title, backend.SVGOptions{TextAsPaths: true})
		}
	}
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
