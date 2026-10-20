package backend

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
)

// kappa: control point distance for a quarter circle drawn as one cubic
// Bezier curve.
const kappa = 0.5522847498

// WritePDF writes the drawing as a one-page PDF cropped to its bounding box
// (the page is the box, the drawing shifted into it), as epstopdf makes
// OneFit's PDFs from gracebat's EPS. The standard fonts are named; additional
// fonts are embedded as vector glyph subsets with a Unicode text mapping.
func WritePDF(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
	return WritePDFWith(w, d, pageW, pageH, title, PDFOptions{})
}

// PDFOptions: FullPage keeps the whole page, not cropped (gracebat's PNG
// is the whole page; plot-go's rasters are cropped unless asked).
// PixelsPerPoint, when set, rounds every line's width to whole pixels of a
// raster at that scale, one at least: gracebat's PNG lines are each a whole
// number of pixels thick (measured; docs/grace-behaviour.md, "PNG"); a
// cropped page then also starts and ends on whole pixels. EvenOdd fills
// every area by the even-odd rule, the only one gracebat's raster devices
// have (a star's middle stays empty).
type PDFOptions struct {
	FullPage       bool
	PixelsPerPoint float64
	EvenOdd        bool
}

func WritePDFWith(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string, opt PDFOptions) error {
	b, err := buildPDF(d, pageW, pageH, title, opt)
	if err != nil {
		return err
	}
	objs := b.objs
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info 5 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	_, err = w.Write(out.Bytes())
	return err
}

// builtPDF is a one-page PDF's objects, numbered from 1 by their place:
// 1 catalog, 2 pages, 3 page, 4 content, 5 info, then the page's resources
// (fonts, patterns, forms, glyphs). res is the page's resource
// dictionary, content its compressed content stream, width and height the
// page's size in points.
type builtPDF struct {
	objs          []string
	res           string
	content       []byte
	width, height float64
}

func buildPDF(d *draw.Drawing, pageW, pageH float64, title string, opt PDFOptions) (builtPDF, error) {
	pageW, pageH = draw.PageSize(pageW, pageH)
	unit := math.Min(pageW, pageH)
	bb := BoundingBox(d, unit)
	if opt.FullPage {
		bb = Box{0, 0, pageW, pageH}
	} else if opt.PixelsPerPoint > 0 {
		// outward to the raster's pixels, so lines snapped to whole
		// pixels stay on them and the image is a whole number of pixels
		px := opt.PixelsPerPoint
		bb = Box{math.Floor(bb.LLX*px) / px, math.Floor(bb.LLY*px) / px,
			math.Ceil(bb.URX*px) / px, math.Ceil(bb.URY*px) / px}
	}

	fonts := map[string]int{}
	var fontOrder []string
	for _, t := range d.Texts {
		if _, ok := fonts[t.Font]; !ok {
			fonts[t.Font] = len(fontOrder) + 1
			fontOrder = append(fontOrder, t.Font)
		}
	}

	var c bytes.Buffer
	p := func(format string, a ...any) { fmt.Fprintf(&c, format, a...) }
	p("1 0 0 1 %s %s cm\n%s 0 0 %s 0 0 cm\n", num(-bb.LLX), num(-bb.LLY), num(unit), num(unit))
	var cur draw.Style
	first := true
	patterns := map[int]int{}
	var patternOrder []int
	// each ellipse shape (radii and how it is painted) is written once, as
	// a form XObject, and drawn where it goes: a set of 200000 circle
	// symbols is 40 bytes a symbol, not 220 - the content gracebat's EPS
	// gets from its own symbol procedures
	forms := map[ellipseForm]int{}
	var formOrder []ellipseForm
	pi, ti := 0, 0
	for _, kind := range d.Order {
		if kind == 't' {
			t := d.Texts[ti]
			ti++
			p("%s %s %s rg\nBT /F%d 1 Tf %s %s %s %s %s %s Tm (%s) Tj ET\n",
				num(t.Color[0]), num(t.Color[1]), num(t.Color[2]), fonts[t.Font],
				num(t.Matrix[0]), num(t.Matrix[1]), num(t.Matrix[2]), num(t.Matrix[3]), num(t.At.X), num(t.At.Y), psString(t.Str))
			continue
		}
		path := d.Paths[pi]
		pi++
		st := path.Style
		if path.Fill && st.Pattern != 0 {
			if _, ok := patterns[st.Pattern]; !ok {
				patterns[st.Pattern] = len(patternOrder)
				patternOrder = append(patternOrder, st.Pattern)
			}
			p("/CSP cs %s %s %s /P%d scn\n", num(st.Color[0]), num(st.Color[1]), num(st.Color[2]), st.Pattern)
		} else if path.Fill {
			p("%s %s %s rg\n", num(st.Color[0]), num(st.Color[1]), num(st.Color[2]))
		} else {
			if st.Pattern != 0 {
				if _, ok := patterns[st.Pattern]; !ok {
					patterns[st.Pattern] = len(patternOrder)
					patternOrder = append(patternOrder, st.Pattern)
				}
				if first || st.Color != cur.Color || st.Pattern != cur.Pattern {
					p("/CSP CS %s %s %s /P%d SCN\n", num(st.Color[0]), num(st.Color[1]), num(st.Color[2]), st.Pattern)
				}
			} else if first || st.Color != cur.Color || cur.Pattern != 0 {
				p("%s %s %s RG\n", num(st.Color[0]), num(st.Color[1]), num(st.Color[2]))
			}
			if first || st.Width != cur.Width {
				w := st.Width
				if px := unit * opt.PixelsPerPoint; px > 0 {
					w = math.Max(1, math.Round(w*px)) / px
				}
				p("%s w\n", num(w))
			}
			if first || !sameDash(st.Dash, cur.Dash) {
				p("[")
				for i, x := range st.Dash {
					if i > 0 {
						p(" ")
					}
					p("%s", num(x))
				}
				p("] 0 d\n")
			}
			if first || st.Cap != cur.Cap {
				p("%d J\n", st.Cap)
			}
			if first || st.Join != cur.Join {
				p("%d j\n", st.Join)
			}
			cur, first = st, false
		}
		paint := "S"
		switch {
		case path.Fill && (path.EvenOdd || opt.EvenOdd):
			paint = "f*"
		case path.Fill:
			paint = "f"
		}
		// a pattern inside a form would tile from the form's origin
		if len(path.Segments) == 1 && path.Segments[0].Arc != nil && st.Pattern == 0 {
			a := path.Segments[0].Arc
			k := ellipseForm{rx: num(a.RX), ry: num(a.RY), paint: paint}
			n, ok := forms[k]
			if !ok {
				n = len(formOrder)
				forms[k] = n
				formOrder = append(formOrder, k)
			}
			p("q 1 0 0 1 %s %s cm /E%d Do Q\n", num(a.X), num(a.Y), n)
			continue
		}
		for _, s := range path.Segments {
			if a := s.Arc; a != nil {
				ellipse(p, a)
				continue
			}
			for i, q := range s.Points {
				op := "l"
				if i == 0 {
					op = "m"
				}
				p("%s %s %s\n", num(q.X), num(q.Y), op)
			}
			if s.Closed {
				p("h\n")
			}
		}
		if paint == "f*" {
			p("f*\n")
		} else if paint == "f" {
			p("f\n")
		} else {
			p("S\n")
		}
	}

	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(c.Bytes())
	zw.Close()

	// objects: 1 catalog, 2 pages, 3 page, 4 content, 5 info, 6.. fonts
	var objs []string
	objs = append(objs, "<< /Type /Catalog /Pages 2 0 R >>")
	objs = append(objs, "<< /Type /Pages /Kids [3 0 R] /Count 1 >>")
	var res strings.Builder
	res.WriteString("<< /Font <<")
	for i := range fontOrder {
		fmt.Fprintf(&res, " /F%d %d 0 R", i+1, 6+i)
	}
	res.WriteString(" >>")
	if len(patternOrder) > 0 {
		// uncolored tiling patterns, as gracebat's EPS makes them: the
		// pattern's bits as an image mask, its tiles PatternUnit of the
		// viewport on the page, starting at the page's origin
		res.WriteString(" /ColorSpace << /CSP [/Pattern /DeviceRGB] >> /Pattern <<")
		for i, n := range patternOrder {
			fmt.Fprintf(&res, " /P%d %d 0 R", n, 6+len(fontOrder)+i)
		}
		res.WriteString(" >>")
	}
	if len(formOrder) > 0 {
		// after the fonts' and the patterns' objects
		res.WriteString(" /XObject <<")
		for i := range formOrder {
			fmt.Fprintf(&res, " /E%d %d 0 R", i, 6+len(fontOrder)+len(patternOrder)+i)
		}
		res.WriteString(" >>")
	}
	res.WriteString(" >>")
	objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources %s /Contents 4 0 R >>",
		num(bb.URX-bb.LLX), num(bb.URY-bb.LLY), res.String()))
	objs = append(objs, fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", z.Len(), z.String()))
	objs = append(objs, fmt.Sprintf("<< /Producer (plot-go) /Title (%s) >>", psString(title)))
	for _, f := range fontOrder {
		if !afm.Standard(f) {
			objs = append(objs, "") // filled after pattern objects, so their IDs stay stable
			continue
		}
		// WinAnsi has the hyphen at 45 and grave at 96 already; 39 is
		// quoteright in gracebat's encoding
		enc := " /Encoding << /Type /Encoding /BaseEncoding /WinAnsiEncoding /Differences [39 /quoteright] >>"
		if f == "Symbol" || f == "ZapfDingbats" {
			enc = ""
		}
		objs = append(objs, fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s%s >>", f, enc))
	}
	for _, n := range patternOrder {
		// the mask is turned on both axes, as gracebat's
		// [-1 0 0 -1 16 16] imagemask; bits set are painted
		body := fmt.Sprintf("q -16 0 0 16 16 0 cm\nBI /W 16 /H 16 /IM true /BPC 1 /D [1 0] /F /AHx ID\n%s>\nEI\nQ\n", draw.PatternHex(n))
		s := unit * draw.PatternUnit
		objs = append(objs, fmt.Sprintf("<< /Type /Pattern /PatternType 1 /PaintType 2 /TilingType 1 /BBox [0 0 16 16] /XStep 16 /YStep 16 /Resources << >> /Matrix [%s 0 0 %s %s %s] /Length %d >>\nstream\n%sendstream",
			num(s), num(s), num(-bb.LLX), num(-bb.LLY), len(body), body))
	}
	for _, k := range formOrder {
		var b bytes.Buffer
		ellipse(func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }, &draw.Arc{RX: parseNum(k.rx), RY: parseNum(k.ry)})
		b.WriteString(k.paint + "\n")
		// the box only clips: a whole page around the shape is room for
		// any stroke
		objs = append(objs, fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [-2 -2 2 2] /Length %d >>\nstream\n%sendstream", b.Len(), b.String()))
	}
	for i, f := range fontOrder {
		if afm.Standard(f) {
			continue
		}
		chars, box, err := usedGlyphs(d, f)
		if err != nil {
			return builtPDF{}, fmt.Errorf("embed font %q: %w", f, err)
		}
		if len(chars) == 0 {
			continue
		}
		fontObj := embedPDFFont(&objs, chars, box, f)
		objs[5+i] = fontObj
	}
	return builtPDF{objs: objs, res: res.String(), content: z.Bytes(), width: bb.URX - bb.LLX, height: bb.URY - bb.LLY}, nil
}

// PDFForm is a drawing as a PDF form XObject, for another PDF (a document
// of many pages, such as OneFit's fit report) to place where it wants, as
// vectors: Objects[0] is the form, the rest its resources (fonts,
// patterns, glyphs), numbered First, First+1, ... in that order. The
// form's own space is Width x Height points with its origin at the bottom
// left, the drawing cropped as WritePDF crops it (or the whole page with
// FullPage).
type PDFForm struct {
	First         int
	Objects       []string
	Width, Height float64
}

var pdfRef = regexp.MustCompile(`(\d+) 0 R\b`)

// WritePDFForm makes the drawing a form whose objects are numbered from
// first. It is WritePDFWith's page, so a plot looks the same placed in a
// document as in its own PDF.
func WritePDFForm(d *draw.Drawing, pageW, pageH float64, opt PDFOptions, first int) (PDFForm, error) {
	b, err := buildPDF(d, pageW, pageH, "", opt)
	if err != nil {
		return PDFForm{}, err
	}
	// the page's resources are objects 6...: they become first+1...
	renumber := func(s string) string {
		return pdfRef.ReplaceAllStringFunc(s, func(m string) string {
			n, _ := strconv.Atoi(pdfRef.FindStringSubmatch(m)[1])
			return fmt.Sprintf("%d 0 R", first+1+n-6)
		})
	}
	f := PDFForm{First: first, Width: b.width, Height: b.height}
	f.Objects = append(f.Objects, fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 %s %s] /Resources %s /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream",
		num(b.width), num(b.height), renumber(b.res), len(b.content), b.content))
	for _, o := range b.objs[5:] {
		f.Objects = append(f.Objects, renumber(o))
	}
	return f, nil
}

// ellipseForm is an ellipse's shape as a form draws it: its radii (as
// written, so equal shapes share a form) and its painting operator.
type ellipseForm struct{ rx, ry, paint string }

func parseNum(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// ellipse draws a full ellipse as four Bezier quarters (OneFit's symbols
// are full circles; partial arcs are drawn full).
func ellipse(p func(string, ...any), a *draw.Arc) {
	x, y, rx, ry := a.X, a.Y, a.RX, a.RY
	kx, ky := kappa*rx, kappa*ry
	p("%s %s m\n", num(x+rx), num(y))
	p("%s %s %s %s %s %s c\n", num(x+rx), num(y+ky), num(x+kx), num(y+ry), num(x), num(y+ry))
	p("%s %s %s %s %s %s c\n", num(x-kx), num(y+ry), num(x-rx), num(y+ky), num(x-rx), num(y))
	p("%s %s %s %s %s %s c\n", num(x-rx), num(y-ky), num(x-kx), num(y-ry), num(x), num(y-ry))
	p("%s %s %s %s %s %s c\n", num(x+kx), num(y-ry), num(x+rx), num(y-ky), num(x+rx), num(y))
	p("h\n")
}

// num writes a number compactly: up to 6 decimals, no trailing zeros.
func num(v float64) string {
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}
