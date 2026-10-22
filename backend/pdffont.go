package backend

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/cff"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
)

// A standard font is embedded in the PDF when its URW base-35 font is
// installed: the glyphs the drawing uses, subset from the URW OpenType
// file's CFF outlines into a Type1C font program - what Ghostscript embeds
// when epstopdf makes OneFit's PDF from the EPS, so the PDF needs no font
// on the reader's machine and passes publishers' "all fonts embedded"
// checks. Without the URW font the standard font is named, as before.

// cffTables caches each URW file's CFF table.
var (
	cffMu     sync.Mutex
	cffTables = map[string][]byte{}
)

func cffTable(path string) ([]byte, error) {
	cffMu.Lock()
	defer cffMu.Unlock()
	if t, ok := cffTables[path]; ok {
		return t, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	t, err := cff.Table(b)
	if err != nil {
		return nil, err
	}
	cffTables[path] = t
	return t, nil
}

// usedCodes are the byte codes the drawing's texts set in font name.
func usedCodes(d *draw.Drawing, name string) []byte {
	used := map[byte]bool{}
	for _, t := range d.Texts {
		if t.Font == name {
			for i := 0; i < len(t.Str); i++ {
				used[t.Str[i]] = true
			}
		}
	}
	codes := make([]byte, 0, len(used))
	for c := range used {
		codes = append(codes, c)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	return codes
}

// subsetTag is the six capital letters naming a subset (PDF 32000-1,
// 9.6.4): from the font and the codes it holds, so equal subsets share a
// name.
func subsetTag(name string, codes []byte) string {
	h := uint32(2166136261)
	for _, b := range append([]byte(name), codes...) {
		h = (h ^ uint32(b)) * 16777619
	}
	var t [6]byte
	for i := range t {
		t[i] = 'A' + byte(h%26)
		h /= 26
	}
	return string(t[:])
}

// embedStandardPDFFont adds standard font name's subset (font program,
// descriptor, Unicode map) to objs and returns its font dictionary; ok is
// false when it cannot be embedded (no URW font, or a glyph it lacks), and
// the font is named instead.
func embedStandardPDFFont(objs *[]string, d *draw.Drawing, name string) (string, bool) {
	codes := usedCodes(d, name)
	if len(codes) == 0 {
		return "", false
	}
	m, err := afm.Get(name)
	if err != nil {
		return "", false
	}
	glyphs := map[byte]int{}
	file := ""
	var drawn []byte
	for _, c := range codes {
		f, gid, ok := glyph.URWGlyph(name, c)
		if !ok {
			if c == ' ' || m.Width(string(c)) == 0 {
				continue // a space or nothing: no glyph to draw
			}
			return "", false
		}
		file = f
		glyphs[c] = gid
		drawn = append(drawn, c)
	}
	if file == "" {
		return "", false
	}
	table, err := cffTable(file)
	if err != nil {
		return "", false
	}
	program, err := cff.Subset(table, drawn, glyphs)
	if err != nil {
		return "", false
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(program)
	zw.Close()
	*objs = append(*objs, fmt.Sprintf("<< /Subtype /Type1C /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", z.Len(), z.String()))
	programObj := len(*objs)

	h := m.Header
	base := subsetTag(name, codes) + "+" + name
	ascent, descent, capHeight := h.Ascender, h.Descender, h.CapHeight
	if ascent == 0 && descent == 0 {
		ascent, descent = h.FontBBox[3], h.FontBBox[1]
	}
	if capHeight == 0 {
		capHeight = ascent
	}
	stemV := h.StdVW
	if stemV == 0 {
		stemV = 80
	}
	flags := 4 // symbolic: the font's own encoding gives each code its glyph
	if h.FixedPitch {
		flags |= 1
	}
	if h.ItalicAngle != 0 {
		flags |= 64
	}
	*objs = append(*objs, fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags %d /FontBBox [%s %s %s %s] /ItalicAngle %s /Ascent %s /Descent %s /CapHeight %s /StemV %s /FontFile3 %d 0 R >>",
		base, flags, num(h.FontBBox[0]), num(h.FontBBox[1]), num(h.FontBBox[2]), num(h.FontBBox[3]),
		num(h.ItalicAngle), num(ascent), num(descent), num(capHeight), num(stemV), programObj))
	descriptorObj := len(*objs)

	*objs = append(*objs, pdfStream(toUnicode(name, codes)))
	unicodeObj := len(*objs)

	first, last := int(codes[0]), int(codes[len(codes)-1])
	var widths strings.Builder
	for c := first; c <= last; c++ {
		w := 0.0
		if bytes.IndexByte(codes, byte(c)) >= 0 {
			w = 1000 * m.Width(string([]byte{byte(c)}))
		}
		fmt.Fprintf(&widths, " %s", num(w))
	}
	return fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /FirstChar %d /LastChar %d /Widths [%s] /FontDescriptor %d 0 R /ToUnicode %d 0 R >>",
		base, first, last, widths.String(), descriptorObj, unicodeObj), true
}

// toUnicode is the CMap giving the Unicode characters codes set in font
// name, so a reader can select and copy the text.
func toUnicode(name string, codes []byte) string {
	var cmap strings.Builder
	cmap.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /PlotGoUnicode def\n/CMapType 2 def\n1 begincodespacerange\n<00> <FF>\nendcodespacerange\n")
	for start := 0; start < len(codes); start += 100 {
		end := min(start+100, len(codes))
		fmt.Fprintf(&cmap, "%d beginbfchar\n", end-start)
		for _, c := range codes[start:end] {
			fmt.Fprintf(&cmap, "<%02X> <%04X>\n", c, glyph.Rune(name, c))
		}
		cmap.WriteString("endbfchar\n")
	}
	cmap.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return cmap.String()
}
