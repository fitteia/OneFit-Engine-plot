package main

import (
	"context"
	"io"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/raster"
)

// Grace's PNG device defaults to 72 DPI. The native CLI uses 300 unless
// -dpi is supplied. Both CLI and Figures share the same converter pipeline;
// both crop to the drawing, as the PDF, EPS and SVG are, unless -fullpage.
const (
	graceDPI   = 72
	defaultDPI = raster.DefaultDPI
)

func rasterWriter(format string, dpi int, fullPage bool) writer {
	return func(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
		return raster.Write(context.Background(), w, d, pageW, pageH, title, raster.Options{Format: format, DPI: dpi, FullPage: fullPage})
	}
}
func pngWriter(dpi int) writer { return rasterWriter("png", dpi, false) }
func writePNG(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string, dpi int) error {
	return pngWriter(dpi)(w, d, pageW, pageH, title)
}
