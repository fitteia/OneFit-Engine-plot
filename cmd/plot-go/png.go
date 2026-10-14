package main

// PNG: the whole page, as gracebat's PNG device makes it - the page's
// size in points times dpi/72 pixels (773x600 points at gracebat's
// default 72 dpi: 773x600 pixels; at 144 dpi 1546x1200) - rasterized by an
// installed converter from plot-go's own full-page PDF: pdftoppm
// (poppler-utils), else Ghostscript, else ImageMagick. As gracebat's raster: every fill
// even-odd, lines as thick (see pngConverters). Antialiased, unlike
// gracebat's; a converter places a line to the pixel its own way, so a
// line may be a pixel off gracebat's.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// graceDPI is gracebat's PNG resolution, what its command line (-hdevice
// PNG) gets here too; plot-go's own -format png defaults to defaultDPI,
// enough for a journal figure (pass -dpi 600 for line art where 600 is
// asked).
const (
	graceDPI   = 72
	defaultDPI = 300
)

// pngConverters, in the order tried; each turns in.pdf into out.png.
// wholePixels: lines are given whole-pixel widths, gracebat's; pdftoppm
// then draws them as gracebat does. Ghostscript inks every pixel a line
// touches, so it draws the widths as they are, as it draws gracebat's EPS
// (testdata/ref: both within antialiasing of gracebat's PNG).
var pngConverters = []struct {
	name        string
	wholePixels bool
	args        func(dpi int, in, out string) []string
}{
	{"pdftoppm", true, func(dpi int, in, out string) []string {
		// -singlefile writes out.png exactly (no page number)
		return []string{"-png", "-singlefile", "-r", strconv.Itoa(dpi), in, out[:len(out)-len(".png")]}
	}},
	{"gs", false, func(dpi int, in, out string) []string {
		return []string{"-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m",
			"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-r" + strconv.Itoa(dpi),
			"-sOutputFile=" + out, in}
	}},
	// ImageMagick (7's magick, 6's convert) reads the PDF through
	// Ghostscript, as gs does; its policy may forbid PDF (Debian's does)
	{"magick", false, imageMagick},
	{"convert", false, imageMagick},
}

func imageMagick(dpi int, in, out string) []string {
	return []string{"-density", strconv.Itoa(dpi), in, "-background", "white", "-alpha", "remove", out}
}

// pngWriter is a writer for PNG at dpi.
func pngWriter(dpi int) writer {
	return func(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
		return writePNG(w, d, pageW, pageH, title, dpi)
	}
}

func writePNG(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string, dpi int) error {
	if dpi <= 0 {
		return fmt.Errorf("PNG: dpi %d is not positive", dpi)
	}
	dir, err := os.MkdirTemp("", "plot-go-png-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	in, out := filepath.Join(dir, "page.pdf"), filepath.Join(dir, "page.png")
	var tried []string
	for _, c := range pngConverters {
		path, err := exec.LookPath(c.name)
		if err != nil {
			continue
		}
		opt := backend.PDFOptions{FullPage: true, EvenOdd: true}
		if c.wholePixels {
			opt.PixelsPerPoint = float64(dpi) / 72
		}
		var pdf bytes.Buffer
		if err := backend.WritePDFWith(&pdf, d, pageW, pageH, title, opt); err != nil {
			return err
		}
		if err := os.WriteFile(in, pdf.Bytes(), 0o600); err != nil {
			return err
		}
		cmd := exec.Command(path, c.args(dpi, in, out)...)
		msg, err := cmd.CombinedOutput()
		if err == nil {
			png, err := os.ReadFile(out)
			if err != nil {
				return err
			}
			_, err = w.Write(png)
			return err
		}
		tried = append(tried, fmt.Sprintf("%s: %v %s", c.name, err, bytes.TrimSpace(msg)))
	}
	if tried == nil {
		return fmt.Errorf("PNG needs pdftoppm (poppler-utils), gs (Ghostscript) or ImageMagick; none is installed")
	}
	return fmt.Errorf("PNG conversion failed: %v", tried)
}
