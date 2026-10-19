package raster

import (
	"bytes"
	"context"
	"encoding/binary"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"golang.org/x/image/tiff"
)

func TestRasterValidation(t *testing.T) {
	for _, opt := range []Options{{"jpeg", 300}, {"png", 0}, {"tiff", -1}, {"png", MaxDPI + 1}, {"tiff", 2400}} {
		if err := Validate(792, 612, opt); err == nil {
			t.Fatalf("accepted %+v", opt)
		}
	}
	for _, format := range []string{"png", "tiff", "tif"} {
		if err := Validate(792, 612, Options{format, 600}); err != nil {
			t.Fatal(err)
		}
	}
}
func TestWindowsGhostscriptDiscovery(t *testing.T) {
	if got := ghostscriptNames("windows"); !reflect.DeepEqual(got, []string{"gswin64c", "gswin32c", "gs"}) {
		t.Fatal(got)
	}
	root := filepath.Join(t.TempDir(), "Program Files")
	for _, version := range []string{"gs9.56.1", "gs10.9.0", "gs10.10.0"} {
		bin := filepath.Join(root, "gs", version, "bin")
		if err := os.MkdirAll(bin, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(bin, "gswin64c.exe"), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths := windowsGhostscriptPaths([]string{root, root, ""})
	if len(paths) != 3 || !strings.Contains(paths[0], "gs10.10.0") || !strings.Contains(paths[1], "gs10.9.0") {
		t.Fatal(paths)
	}
}
func TestRasterMissingConverterAndCancellation(t *testing.T) {
	var b bytes.Buffer
	err := writeWith(context.Background(), &b, &draw.Drawing{}, 72, 72, "", Options{"png", 72}, nil)
	if err == nil || !strings.Contains(err.Error(), "none was found") || b.Len() != 0 {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Write(ctx, &b, &draw.Drawing{}, 72, 72, "", Options{"tiff", 72}); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestGhostscriptRaster(t *testing.T) {
	var gs []converter
	for _, c := range converters() {
		if c.args != nil && (c.name == "gs" || strings.HasPrefix(c.name, "gswin")) {
			gs = append(gs, c)
		}
	}
	if len(gs) == 0 {
		if os.Getenv("PLOT_GO_REQUIRE_GS") == "1" {
			t.Fatal("Ghostscript required but not discovered")
		}
		t.Skip("Ghostscript not installed")
	}
	for _, c := range gs {
		testFormats(t, c)
	}
}
func TestRasterConverterFormats(t *testing.T) {
	all := converters()
	if len(all) == 0 {
		t.Skip("no converter installed")
	}
	for _, c := range all {
		testFormats(t, c)
	}
}
func testFormats(t *testing.T, c converter) {
	t.Helper()
	for _, format := range []string{"png", "tiff"} {
		for _, dpi := range []int{72, 144} {
			t.Run(c.name+"/"+format+"/"+strconv.Itoa(dpi), func(t *testing.T) {
				var out bytes.Buffer
				if err := writeWith(context.Background(), &out, &draw.Drawing{}, 72, 36, "test", Options{format, dpi}, []converter{c}); err != nil {
					t.Fatal(err)
				}
				var w, h int
				if format == "png" {
					cfg, err := png.DecodeConfig(bytes.NewReader(out.Bytes()))
					if err != nil {
						t.Fatal(err)
					}
					w, h = cfg.Width, cfg.Height
					if got := pngDPI(t, out.Bytes()); math.Abs(got-float64(dpi)) > 0.1 {
						t.Fatalf("PNG resolution %v, want %d", got, dpi)
					}
				} else {
					cfg, err := tiff.DecodeConfig(bytes.NewReader(out.Bytes()))
					if err != nil {
						t.Fatal(err)
					}
					w, h = cfg.Width, cfg.Height
					// TIFF must preserve physical resolution, not merely pixel dimensions.
					if got := tiffDPI(t, out.Bytes()); math.Abs(got-float64(dpi)) > 0.01 {
						t.Fatalf("TIFF resolution %v, want %d", got, dpi)
					}
				}
				if w != dpi || h != dpi/2 {
					t.Fatalf("size %dx%d, want %dx%d", w, h, dpi, dpi/2)
				}
			})
		}
	}
}
func tiffDPI(t *testing.T, b []byte) float64 {
	t.Helper()
	var order binary.ByteOrder = binary.LittleEndian
	if string(b[:2]) == "MM" {
		order = binary.BigEndian
	}
	off := int(order.Uint32(b[4:8]))
	n := int(order.Uint16(b[off : off+2]))
	for i := 0; i < n; i++ {
		ent := b[off+2+i*12 : off+2+(i+1)*12]
		if order.Uint16(ent[:2]) == 282 {
			p := int(order.Uint32(ent[8:12]))
			return float64(order.Uint32(b[p:p+4])) / float64(order.Uint32(b[p+4:p+8]))
		}
	}
	t.Fatal("missing TIFF XResolution")
	return 0
}

func pngDPI(t *testing.T, b []byte) float64 {
	t.Helper()
	for p := 8; p+12 <= len(b); {
		n := int(binary.BigEndian.Uint32(b[p : p+4]))
		if p+12+n > len(b) {
			break
		}
		if string(b[p+4:p+8]) == "pHYs" && n == 9 && b[p+16] == 1 {
			return float64(binary.BigEndian.Uint32(b[p+8:p+12])) * 0.0254
		}
		p += 12 + n
	}
	t.Fatal("missing PNG resolution metadata")
	return 0
}
