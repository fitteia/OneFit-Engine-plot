package main

import (
	"bytes"
	"github.com/fitteia/OneFit-Engine-plot/raster"
	"golang.org/x/image/tiff"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func needConverter(t *testing.T) {
	t.Helper()
	if !raster.Available() {
		t.Skip("no raster converter installed")
	}
}

// TestGracebatCallPNG: -hdevice PNG writes a PNG at gracebat's 72 dpi
// (not plot-go's own 300), or at the DPI a -pexec DEVICE "PNG" DPI
// command sets.
func TestGracebatCallPNG(t *testing.T) {
	needConverter(t)
	agrFile, err := filepath.Abs("../../testdata/corpus/p96-10-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pexec []string
		w, h  int
	}{{nil, 773, 600}, {[]string{"-pexec", `DEVICE "PNG" DPI 144`}, 1546, 1200}} {
		out := filepath.Join(t.TempDir(), "x.png")
		args := append(append([]string{"-hdevice", "PNG"}, c.pexec...), "-hardcopy", "-printfile", out, agrFile)
		var stdout, stderr bytes.Buffer
		if code := runGrace(args, &stdout, &stderr); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr.String())
		}
		f, err := os.Open(out)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != c.w || cfg.Height != c.h {
			t.Errorf("%v: %dx%d, gracebat %dx%d", c.pexec, cfg.Width, cfg.Height, c.w, c.h)
		}
	}
}

func TestGracebatCallTIFF(t *testing.T) {
	needConverter(t)
	input, err := filepath.Abs("../../testdata/corpus/p96-10-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "figure.tiff")
	var stdout, stderr bytes.Buffer
	args := []string{"-hdevice", "TIFF", "-pexec", `DEVICE "TIFF" DPI 144`, "-hardcopy", "-printfile", out, input}
	if code := runGrace(args, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := tiff.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 1546 || cfg.Height != 1200 {
		t.Fatalf("TIFF dimensions %dx%d", cfg.Width, cfg.Height)
	}
}
