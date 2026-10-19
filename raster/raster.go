// Package raster exports whole-page PNG and losslessly compressed TIFF through
// an installed PDF converter. Vector backends do not need these programs.
package raster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/draw"
)

const DefaultDPI = 300
const MaxDPI = 2400
const MaxPixels = 100_000_000

// Options controls the raster format and resolution. DPI must be explicit;
// callers wanting the default supply DefaultDPI. Both formats keep a white
// page background and record the chosen resolution in the output metadata.
type Options struct {
	Format string
	DPI    int
}

// Validate rejects unsupported formats and impractically large raster pages
// before starting a converter. Dimensions follow the full PDF page, not its ink.
func Validate(pageW, pageH float64, opt Options) error {
	if opt.Format != "png" && opt.Format != "tiff" && opt.Format != "tif" {
		return fmt.Errorf("raster format must be png or tiff")
	}
	if opt.DPI < 1 || opt.DPI > MaxDPI {
		return fmt.Errorf("DPI must be an integer from 1 to %d", MaxDPI)
	}
	w, h := draw.PageSize(pageW, pageH)
	pixels := math.Ceil(w*float64(opt.DPI)/72) * math.Ceil(h*float64(opt.DPI)/72)
	if math.IsInf(pixels, 0) || pixels > MaxPixels {
		return fmt.Errorf("raster page exceeds %d pixels; choose a lower DPI", MaxPixels)
	}
	return nil
}

type converter struct {
	name, path  string
	wholePixels bool
	args        func(Options, string, string) []string
}

func popplerArgs(opt Options, in, out string) []string {
	args := []string{"-singlefile", "-r", strconv.Itoa(opt.DPI)}
	if opt.Format == "png" {
		args = append(args, "-png")
	} else {
		args = append(args, "-tiff", "-tiffcompression", "lzw")
	}
	return append(args, in, strings.TrimSuffix(out, filepath.Ext(out)))
}
func ghostscriptArgs(opt Options, in, out string) []string {
	device := "png16m"
	if opt.Format != "png" {
		device = "tiff24nc"
	}
	args := []string{"-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-dFirstPage=1", "-dLastPage=1", "-sDEVICE=" + device, "-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-r" + strconv.Itoa(opt.DPI)}
	if opt.Format != "png" {
		args = append(args, "-sCompression=lzw")
	}
	return append(args, "-sOutputFile="+out, in)
}
func imageMagickArgs(opt Options, in, out string) []string {
	args := []string{"-units", "PixelsPerInch", "-density", strconv.Itoa(opt.DPI), in + "[0]", "-background", "white", "-alpha", "remove", "-alpha", "off"}
	if opt.Format != "png" {
		args = append(args, "-compress", "Zip")
	}
	return append(args, out)
}

func ghostscriptNames(goos string) []string {
	if goos == "windows" {
		return []string{"gswin64c", "gswin32c", "gs"}
	}
	return []string{"gs"}
}

// Windows' normal Ghostscript installer need not add its bin folder to PATH.
// Search only the explicit Program Files roots, newest installed version first.
func windowsGhostscriptPaths(roots []string) []string {
	var paths, dirs []string
	for _, root := range roots {
		if root == "" {
			continue
		}
		found, _ := filepath.Glob(filepath.Join(root, "gs", "gs*"))
		dirs = append(dirs, found...)
	}
	slices.SortFunc(dirs, func(a, b string) int { return compareGSVersion(b, a) })
	for _, dir := range dirs {
		for _, name := range []string{"gswin64c.exe", "gswin32c.exe"} {
			p := filepath.Join(dir, "bin", name)
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() && !slices.Contains(paths, p) {
				paths = append(paths, p)
			}
		}
	}
	return paths
}
func compareGSVersion(a, b string) int {
	av := strings.Split(strings.TrimPrefix(filepath.Base(a), "gs"), ".")
	bv := strings.Split(strings.TrimPrefix(filepath.Base(b), "gs"), ".")
	for i := 0; i < max(len(av), len(bv)); i++ {
		x, y := 0, 0
		if i < len(av) {
			x, _ = strconv.Atoi(av[i])
		}
		if i < len(bv) {
			y, _ = strconv.Atoi(bv[i])
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return strings.Compare(a, b)
}
func converters() []converter {
	var out []converter
	add := func(name string, whole bool, args func(Options, string, string) []string) {
		if path, err := exec.LookPath(name); err == nil {
			out = append(out, converter{name, path, whole, args})
		}
	}
	add("pdftoppm", true, popplerArgs)
	for _, name := range ghostscriptNames(runtime.GOOS) {
		add(name, false, ghostscriptArgs)
	}
	if runtime.GOOS == "windows" {
		for _, path := range windowsGhostscriptPaths([]string{os.Getenv("ProgramW6432"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}) {
			out = append(out, converter{filepath.Base(path), path, false, ghostscriptArgs})
		}
	}
	add("magick", false, imageMagickArgs)
	// Windows' built-in convert.exe is a filesystem utility, not ImageMagick.
	if runtime.GOOS != "windows" {
		add("convert", false, imageMagickArgs)
	}
	return out
}

// Available reports whether at least one converter can be found. A converter's
// PDF policy or runtime dependencies can still make an export fail.
func Available() bool { return len(converters()) > 0 }

// Write converts a drawing without changing its source. It respects cancellation
// and bounds converter runtime to two minutes. Poppler is preferred, followed by
// Ghostscript and ImageMagick. Windows console executables and default install
// folders are supported without invoking a shell.
func Write(ctx context.Context, w io.Writer, d *draw.Drawing, pageW, pageH float64, title string, opt Options) error {
	if err := Validate(pageW, pageH, opt); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return writeWith(ctx, w, d, pageW, pageH, title, opt, converters())
}
func writeWith(ctx context.Context, w io.Writer, d *draw.Drawing, pageW, pageH float64, title string, opt Options, all []converter) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "plot-go-raster-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	format := opt.Format
	if format == "tif" {
		format = "tiff"
	}
	ext := format
	if ext == "tiff" {
		ext = "tif"
	} // pdftoppm uses .tif, regardless of the requested download suffix.
	in, out := filepath.Join(dir, "page.pdf"), filepath.Join(dir, "page."+ext)
	var tried []string
	for _, c := range all {
		pdfOpt := backend.PDFOptions{FullPage: true, EvenOdd: true}
		if c.wholePixels {
			pdfOpt.PixelsPerPoint = float64(opt.DPI) / 72
		}
		var pdf bytes.Buffer
		if err := backend.WritePDFWith(&pdf, d, pageW, pageH, title, pdfOpt); err != nil {
			return err
		}
		if err := os.WriteFile(in, pdf.Bytes(), 0600); err != nil {
			return err
		}
		os.Remove(out) // A failed earlier converter must not supply stale output.
		cmd := exec.CommandContext(ctx, c.path, c.args(opt, in, out)...)
		msg, err := cmd.CombinedOutput()
		if err == nil {
			f, openErr := os.Open(out)
			if openErr == nil {
				_, copyErr := io.Copy(w, f)
				closeErr := f.Close()
				if copyErr != nil {
					return copyErr
				}
				return closeErr
			}
			err = openErr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		tried = append(tried, fmt.Sprintf("%s: %v %s", c.name, err, bytes.TrimSpace(msg)))
	}
	if len(all) == 0 {
		return fmt.Errorf("PNG/TIFF export needs Poppler (pdftoppm), Ghostscript (gs or Windows gswin64c/gswin32c), or ImageMagick; none was found")
	}
	return fmt.Errorf("%s conversion failed: %s", strings.ToUpper(format), strings.Join(tried, "; "))
}
