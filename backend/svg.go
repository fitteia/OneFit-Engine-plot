package backend

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"math"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

// families maps the standard fonts to CSS font families; a browser has no
// Helvetica or Times by those names everywhere.
var families = map[string]string{
	"Helvetica":             "Helvetica, Arial, 'Liberation Sans', sans-serif",
	"Helvetica-Bold":        "Helvetica, Arial, 'Liberation Sans', sans-serif",
	"Helvetica-Oblique":     "Helvetica, Arial, 'Liberation Sans', sans-serif",
	"Helvetica-BoldOblique": "Helvetica, Arial, 'Liberation Sans', sans-serif",
	"Times-Roman":           "'Times New Roman', Times, 'Liberation Serif', serif",
	"Times-Bold":            "'Times New Roman', Times, 'Liberation Serif', serif",
	"Times-Italic":          "'Times New Roman', Times, 'Liberation Serif', serif",
	"Times-BoldItalic":      "'Times New Roman', Times, 'Liberation Serif', serif",
	"Courier":               "'Courier New', Courier, 'Liberation Mono', monospace",
	"Courier-Bold":          "'Courier New', Courier, 'Liberation Mono', monospace",
	"Courier-Oblique":       "'Courier New', Courier, 'Liberation Mono', monospace",
	"Courier-BoldOblique":   "'Courier New', Courier, 'Liberation Mono', monospace",
	"Symbol":                "'Times New Roman', Times, 'Liberation Serif', serif",
}

// symbolGreek is the Symbol font's letters as the Greek characters they
// draw (Adobe's Symbol.afm names them: Alpha, Beta, Chi, ...; theta1,
// sigma1, phi1, omega1 are the variant forms).
var symbolGreek = map[byte]rune{
	'A': 'Α', 'B': 'Β', 'C': 'Χ', 'D': 'Δ', 'E': 'Ε', 'F': 'Φ', 'G': 'Γ', 'H': 'Η',
	'I': 'Ι', 'J': 'ϑ', 'K': 'Κ', 'L': 'Λ', 'M': 'Μ', 'N': 'Ν', 'O': 'Ο', 'P': 'Π',
	'Q': 'Θ', 'R': 'Ρ', 'S': 'Σ', 'T': 'Τ', 'U': 'Υ', 'V': 'ς', 'W': 'Ω', 'X': 'Ξ',
	'Y': 'Ψ', 'Z': 'Ζ',
	'a': 'α', 'b': 'β', 'c': 'χ', 'd': 'δ', 'e': 'ε', 'f': 'φ', 'g': 'γ', 'h': 'η',
	'i': 'ι', 'j': 'ϕ', 'k': 'κ', 'l': 'λ', 'm': 'μ', 'n': 'ν', 'o': 'ο', 'p': 'π',
	'q': 'θ', 'r': 'ρ', 's': 'σ', 't': 'τ', 'u': 'υ', 'v': 'ϖ', 'w': 'ω', 'x': 'ξ',
	'y': 'ψ', 'z': 'ζ',
}

// Unicode turns a string in one of the standard fonts' encodings into
// Unicode text: Symbol letters become Greek, Latin-1 bytes their
// characters.
func Unicode(font, s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if font == "Symbol" {
			if g, ok := symbolGreek[c]; ok {
				b.WriteRune(g)
				continue
			}
		}
		if c == '\'' && font != "Symbol" && font != "ZapfDingbats" {
			b.WriteRune('’') // quoteright in gracebat's encoding
			continue
		}
		b.WriteRune(rune(c))
	}
	return b.String()
}

// WriteSVG writes the drawing as SVG, cropped to the same box as the EPS
// and PDF, in points.
func WriteSVG(w io.Writer, d *draw.Drawing, pageW, pageH float64, title string) error {
	unit := math.Min(pageW, pageH)
	bb := BoundingBox(d, unit)
	wpt, hpt := bb.URX-bb.LLX, bb.URY-bb.LLY
	b := bufio.NewWriter(w)
	p := func(format string, a ...any) { fmt.Fprintf(b, format, a...) }
	// x right, y up in viewport units -> SVG points, y down
	X := func(x float64) string { return num(x*unit - bb.LLX) }
	Y := func(y float64) string { return num(bb.URY - y*unit) }

	p(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="%spt" height="%spt" viewBox="0 0 %s %s">
<title>%s</title>
`, num(wpt), num(hpt), num(wpt), num(hpt), html.EscapeString(title))
	rgb := func(c [3]float64) string {
		return fmt.Sprintf("#%02x%02x%02x", int(math.Round(c[0]*255)), int(math.Round(c[1]*255)), int(math.Round(c[2]*255)))
	}
	pi, ti := 0, 0
	for _, kind := range d.Order {
		if kind == 't' {
			t := d.Texts[ti]
			ti++
			size := math.Hypot(t.Matrix[0], t.Matrix[1])
			angle := math.Atan2(t.Matrix[1], t.Matrix[0]) * 180 / math.Pi
			tr := ""
			if math.Abs(angle) > 1e-9 {
				tr = fmt.Sprintf(` transform="rotate(%s %s %s)"`, num(-angle), X(t.At.X), Y(t.At.Y))
			}
			p(`<text x="%s" y="%s" font-family="%s" font-size="%s" fill="%s" xml:space="preserve"%s>%s</text>`+"\n",
				X(t.At.X), Y(t.At.Y), html.EscapeString(family(t.Font)), num(size*unit), rgb(t.Color), tr+style(t.Font),
				html.EscapeString(Unicode(t.Font, t.Str)))
			continue
		}
		path := d.Paths[pi]
		pi++
		st := path.Style
		attrs := ""
		if path.Fill {
			attrs = fmt.Sprintf(`fill="%s" stroke="none"`, rgb(st.Color))
		} else {
			attrs = fmt.Sprintf(`fill="none" stroke="%s" stroke-width="%s"`, rgb(st.Color), num(st.Width*unit))
			if len(st.Dash) > 0 {
				var ds []string
				for _, x := range st.Dash {
					ds = append(ds, num(x*unit))
				}
				attrs += fmt.Sprintf(` stroke-dasharray="%s"`, strings.Join(ds, " "))
			}
		}
		for _, s := range path.Segments {
			if a := s.Arc; a != nil {
				p(`<ellipse cx="%s" cy="%s" rx="%s" ry="%s" %s/>`+"\n", X(a.X), Y(a.Y), num(a.RX*unit), num(a.RY*unit), attrs)
				continue
			}
		}
		var dpath strings.Builder
		for _, s := range path.Segments {
			if s.Arc != nil {
				continue
			}
			for i, q := range s.Points {
				op := "L"
				if i == 0 {
					op = "M"
				}
				fmt.Fprintf(&dpath, "%s%s %s", op, X(q.X), Y(q.Y))
			}
			if s.Closed {
				dpath.WriteString("Z")
			}
		}
		if dpath.Len() > 0 {
			p(`<path d="%s" %s/>`+"\n", dpath.String(), attrs)
		}
	}
	p("</svg>\n")
	return b.Flush()
}

func family(font string) string {
	if f, ok := families[font]; ok {
		return f
	}
	return "sans-serif"
}

// style adds weight and slant for the bold and oblique standard fonts.
func style(font string) string {
	s := ""
	if strings.Contains(font, "Bold") {
		s += ` font-weight="bold"`
	}
	if strings.Contains(font, "Oblique") || strings.Contains(font, "Italic") {
		s += ` font-style="italic"`
	}
	return s
}
