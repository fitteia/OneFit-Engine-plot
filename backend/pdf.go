package backend

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// kappa: control point distance for a quarter circle drawn as one cubic
// Bezier curve.
const kappa = 0.5522847498

// WritePDF writes the drawing as a one-page PDF cropped to its bounding box
// (the page is the box, the drawing shifted into it), as epstopdf makes
// OneFit's PDFs from gracebat's EPS. The standard fonts are named, not
// embedded.
func WritePDF(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
	unit := math.Min(pageW, pageH)
	bb := BoundingBox(d, unit)

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
		if path.Fill {
			p("%s %s %s rg\n", num(st.Color[0]), num(st.Color[1]), num(st.Color[2]))
		} else {
			if first || st.Color != cur.Color {
				p("%s %s %s RG\n", num(st.Color[0]), num(st.Color[1]), num(st.Color[2]))
			}
			if first || st.Width != cur.Width {
				p("%s w\n", num(st.Width))
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
		if path.Fill {
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
	res.WriteString(" >> >>")
	objs = append(objs, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources %s /Contents 4 0 R >>",
		num(bb.URX-bb.LLX), num(bb.URY-bb.LLY), res.String()))
	objs = append(objs, fmt.Sprintf("<< /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", z.Len(), z.String()))
	objs = append(objs, fmt.Sprintf("<< /Producer (plot-go) /Title (%s) >>", psString(title)))
	for _, f := range fontOrder {
		// WinAnsi has the hyphen at 45 and grave at 96 already; 39 is
		// quoteright in gracebat's encoding
		enc := " /Encoding << /Type /Encoding /BaseEncoding /WinAnsiEncoding /Differences [39 /quoteright] >>"
		if f == "Symbol" || f == "ZapfDingbats" {
			enc = ""
		}
		objs = append(objs, fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s%s >>", f, enc))
	}

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
	_, err := w.Write(out.Bytes())
	return err
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
