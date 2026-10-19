package main

import (
	"bytes"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/raster"
	"github.com/fitteia/OneFit-Engine-plot/render"
	"golang.org/x/image/tiff"
)

// TestMain: with PLOT_GO_RUN_MAIN=1 the test binary is plot-go itself, so
// tests run the command line without building it (a nested go build needs
// the toolchain, and in CI's containers git refuses to stamp the checkout).
func TestMain(m *testing.M) {
	if os.Getenv("PLOT_GO_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// plotGo runs plot-go's command line with args, as a separate process.
func plotGo(args ...string) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "PLOT_GO_RUN_MAIN=1")
	return cmd.CombinedOutput()
}

func needConverter(t *testing.T) {
	t.Helper()
	if !raster.Available() {
		t.Skip("no raster converter installed")
	}
}

// cropSize is the drawing's crop box (its PDF's) in pixels at dpi.
func cropSize(t *testing.T, agrFile string, dpi int) (w, h float64) {
	t.Helper()
	p, err := agr.ParseFile(agrFile)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := render.Render(p)
	pw, ph := draw.PageSize(p.PageWidth, p.PageHeight)
	bb := backend.BoundingBox(d, math.Min(pw, ph))
	return (bb.URX - bb.LLX) * float64(dpi) / 72, (bb.URY - bb.LLY) * float64(dpi) / 72
}

func nearSize(gotW, gotH int, w, h float64) bool {
	return math.Abs(float64(gotW)-w) <= 1 && math.Abs(float64(gotH)-h) <= 1
}

// TestGracebatCallPNG: -hdevice PNG writes a PNG at gracebat's 72 dpi
// (not plot-go's own 300), or at the DPI a -pexec DEVICE "PNG" DPI
// command sets - cropped to the drawing, unlike gracebat's whole page.
func TestGracebatCallPNG(t *testing.T) {
	needConverter(t)
	agrFile, err := filepath.Abs("../../testdata/corpus/p96-10-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pexec []string
		dpi   int
	}{{nil, 72}, {[]string{"-pexec", `DEVICE "PNG" DPI 144`}, 144}} {
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
		if w, h := cropSize(t, agrFile, c.dpi); !nearSize(cfg.Width, cfg.Height, w, h) {
			t.Errorf("%v: %dx%d, crop box %.1fx%.1f", c.pexec, cfg.Width, cfg.Height, w, h)
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
	if w, h := cropSize(t, input, 144); !nearSize(cfg.Width, cfg.Height, w, h) {
		t.Fatalf("TIFF dimensions %dx%d, crop box %.1fx%.1f", cfg.Width, cfg.Height, w, h)
	}
}

// TestFullPageFlag: plot-go -fullpage is the whole page, gracebat's PNG
// (773x600 points: 1546x1200 at 144 dpi); without it, the crop box.
func TestFullPageFlag(t *testing.T) {
	needConverter(t)
	input, err := filepath.Abs("../../testdata/corpus/p96-10-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	cw, ch := cropSize(t, input, 144)
	for _, c := range []struct {
		flags []string
		w, h  float64
	}{{nil, cw, ch}, {[]string{"-fullpage"}, 1546, 1200}} {
		out := filepath.Join(t.TempDir(), "x.png")
		args := append(append([]string{"-q", "-dpi", "144"}, c.flags...), "-o", out, input)
		if msg, err := plotGo(args...); err != nil {
			t.Fatalf("%v: %v %s", c.flags, err, msg)
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
		if !nearSize(cfg.Width, cfg.Height, c.w, c.h) {
			t.Errorf("%v: %dx%d, want %.1fx%.1f", c.flags, cfg.Width, cfg.Height, c.w, c.h)
		}
	}
}
