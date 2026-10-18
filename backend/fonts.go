package backend

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
)

// Additional fonts are embedded as vector Type-3 glyph subsets. This works
// with both TrueType quadratic and OpenType/CFF cubic outlines, requires no
// viewer-side font installation and keeps PDF text selectable through ToUnicode.
// Core-14 output remains unchanged.
type fontGlyph struct {
	code    byte
	width   float64
	box     afm.Box
	outline []glyph.Seg
}

func usedGlyphs(d *draw.Drawing, name string) ([]fontGlyph, afm.Box, error) {
	m, err := afm.Get(name)
	if err != nil {
		return nil, afm.Box{}, err
	}
	used := map[byte]bool{}
	for _, t := range d.Texts {
		if t.Font == name {
			for i := 0; i < len(t.Str); i++ {
				used[t.Str[i]] = true
			}
		}
	}
	var codes []int
	for c := range used {
		codes = append(codes, int(c))
	}
	sort.Ints(codes)
	var chars []fontGlyph
	var box afm.Box
	if len(codes) == 0 {
		codes = []int{0}
	} // valid empty font for an empty text run
	for _, c := range codes {
		s := string([]byte{byte(c)})
		outline, err := glyph.Outline(name, byte(c))
		if err != nil {
			return nil, box, err
		}
		b := m.Ink(s)
		chars = append(chars, fontGlyph{byte(c), m.Width(s), b, outline})
		box = box.Union(b)
	}
	return chars, box, nil
}

// outlineCommands converts quadratic segments to cubic curves for PDF/PS.
func outlineCommands(segs []glyph.Seg, pdf bool) string {
	var b strings.Builder
	move, line, curve := "moveto", "lineto", "curveto"
	if pdf {
		move, line, curve = "m", "l", "c"
	}
	var at [2]float64
	for _, s := range segs {
		switch s.Op {
		case glyph.MoveTo, glyph.LineTo:
			at = s.Pts[0]
			op := line
			if s.Op == glyph.MoveTo {
				op = move
			}
			fmt.Fprintf(&b, "%s %s %s\n", num(at[0]), num(at[1]), op)
		case glyph.QuadTo:
			q, end := s.Pts[0], s.Pts[1]
			fmt.Fprintf(&b, "%s %s %s %s %s %s %s\n",
				num(at[0]+2*(q[0]-at[0])/3), num(at[1]+2*(q[1]-at[1])/3),
				num(end[0]+2*(q[0]-end[0])/3), num(end[1]+2*(q[1]-end[1])/3), num(end[0]), num(end[1]), curve)
			at = end
		case glyph.CubeTo:
			fmt.Fprintf(&b, "%s %s %s %s %s %s %s\n", num(s.Pts[0][0]), num(s.Pts[0][1]), num(s.Pts[1][0]), num(s.Pts[1][1]), num(s.Pts[2][0]), num(s.Pts[2][1]), curve)
			at = s.Pts[2]
		}
	}
	if len(segs) > 0 {
		if pdf {
			b.WriteString("f\n")
		} else {
			b.WriteString("fill\n")
		}
	}
	return b.String()
}

func fontBox(b afm.Box) string {
	return fmt.Sprintf("%s %s %s %s", num(b.LLX), num(b.LLY), num(b.URX), num(b.URY))
}

func pdfStream(s string) string {
	return fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(s), s)
}

func embedPDFFont(objs *[]string, chars []fontGlyph, box afm.Box, name string) string {
	var procs, encoding, widths strings.Builder
	first, last := int(chars[0].code), int(chars[len(chars)-1].code)
	byCode := map[int]fontGlyph{}
	for _, g := range chars {
		body := fmt.Sprintf("%s 0 %s d1\n", num(g.width), fontBox(g.box)) + outlineCommands(g.outline, true)
		*objs = append(*objs, pdfStream(body))
		fmt.Fprintf(&procs, " /g%02x %d 0 R", g.code, len(*objs))
		fmt.Fprintf(&encoding, " %d /g%02x", g.code, g.code)
		byCode[int(g.code)] = g
	}
	for c := first; c <= last; c++ {
		fmt.Fprintf(&widths, " %s", num(byCode[c].width))
	}
	var cmap strings.Builder
	cmap.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /PlotGoUnicode def\n/CMapType 2 def\n1 begincodespacerange\n<00> <FF>\nendcodespacerange\n")
	for start := 0; start < len(chars); start += 100 {
		end := min(start+100, len(chars))
		fmt.Fprintf(&cmap, "%d beginbfchar\n", end-start)
		for _, g := range chars[start:end] {
			fmt.Fprintf(&cmap, "<%02X> <%04X>\n", g.code, glyph.Rune("", g.code))
		}
		cmap.WriteString("endbfchar\n")
	}
	cmap.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	*objs = append(*objs, pdfStream(cmap.String()))
	return fmt.Sprintf("<< /Type /Font /Subtype /Type3 /Name /%s /FontBBox [%s] /FontMatrix [1 0 0 1 0 0] /CharProcs <<%s >> /Encoding << /Type /Encoding /Differences [%s] >> /FirstChar %d /LastChar %d /Widths [%s] /Resources << >> /ToUnicode %d 0 R >>", fontNameToken(name), fontBox(box), procs.String(), encoding.String(), first, last, widths.String(), len(*objs))
}

// PDF names cannot contain whitespace, delimiters or arbitrary
// font-name bytes literally. Hex escapes retain names safely in metadata.
func fontNameToken(name string) string {
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c > 32 && c < 127 && !strings.ContainsRune("()<>[]{}/%#", rune(c)) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "#%02X", c)
		}
	}
	return b.String()
}

func embedEPSFont(chars []fontGlyph, box afm.Box, id int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "/Font%d 12 dict dup begin\n/FontType 3 def\n/FontMatrix [1 0 0 1 0 0] def\n/FontBBox [%s] def\n/Encoding 256 array def\n0 1 255 {Encoding exch /.notdef put} for\n/CharProcs %d dict dup begin\n/.notdef {} def\n", id, fontBox(box), len(chars)+1)
	for _, g := range chars {
		fmt.Fprintf(&b, "/g%02x {%s 0 %s setcachedevice\n%s} bind def\n", g.code, num(g.width), fontBox(g.box), outlineCommands(g.outline, false))
	}
	b.WriteString("end def\n")
	for _, g := range chars {
		fmt.Fprintf(&b, "Encoding %d /g%02x put\n", g.code, g.code)
	}
	b.WriteString("/BuildGlyph {exch begin CharProcs exch 2 copy known not {pop /.notdef} if get exec end} bind def\n/BuildChar {1 index /Encoding get exch get 1 index /BuildGlyph get exec} bind def\nend definefont pop\n")
	return b.String()
}
