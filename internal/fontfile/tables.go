package fontfile

import (
	"encoding/binary"
	"os"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// table is a font's table by tag: dir is the font's table directory (the
// file, or a collection's face from its offset on), file the whole file,
// whose start the table offsets count from.
func table(dir, file []byte, name string) []byte {
	if len(dir) < 12 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(dir[4:6]))
	for i := 0; i < n; i++ {
		at := 12 + i*16
		if at+16 > len(dir) {
			break
		}
		if string(dir[at:at+4]) != name {
			continue
		}
		offset := uint64(binary.BigEndian.Uint32(dir[at+8 : at+12]))
		length := uint64(binary.BigEndian.Uint32(dir[at+12 : at+16]))
		if offset+length > uint64(len(file)) {
			return nil
		}
		return file[offset : offset+length]
	}
	return nil
}

func embeddingFlags(dir, file []byte) uint16 {
	b := table(dir, file, "OS/2")
	if len(b) < 10 {
		return 0
	}
	return binary.BigEndian.Uint16(b[8:10])
}

// Underline position/thickness come from the standard OpenType 'post' table.
// Bounds checks are independent of the sfnt parser; absent values use a modest
// decoration below the baseline rather than a different font's metrics.
func underline(dir, file []byte, upm float64) (float64, float64) {
	b := table(dir, file, "post")
	if len(b) >= 12 {
		pos := int16(binary.BigEndian.Uint16(b[8:10]))
		thick := int16(binary.BigEndian.Uint16(b[10:12]))
		if thick > 0 {
			return float64(pos) / upm, float64(thick) / upm
		}
	}
	return -.1, .05
}

func isCollection(b []byte) bool { return len(b) >= 12 && string(b[:4]) == "ttcf" }

// faceOffset is where face i of a collection starts.
func faceOffset(b []byte, i int) (int, bool) {
	if !isCollection(b) {
		return 0, i == 0
	}
	n := int(binary.BigEndian.Uint32(b[8:12]))
	at := 12 + 4*i
	if i < 0 || i >= n || at+4 > len(b) {
		return 0, false
	}
	off := int(binary.BigEndian.Uint32(b[at : at+4]))
	return off, off < len(b)
}

// readNames reads a font file's faces' PostScript and full names (in that
// order): a face per entry, several in a collection (.ttc). A face golang.org/x/image
// cannot parse, or whose outlines it cannot load (a bitmap-only font: color
// emoji), has no names - it could not be drawn. Nil when the file is not a
// font. Each file is read whole, but only when it is new or changed (the
// index's disk cache, cache.go).
func readNames(path string) [][]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var fonts []*sfnt.Font
	if isCollection(b) {
		c, err := sfnt.ParseCollection(b)
		if err != nil {
			return nil
		}
		for i := 0; i < c.NumFonts(); i++ {
			f, _ := c.Font(i)
			fonts = append(fonts, f)
		}
	} else {
		f, err := sfnt.Parse(b)
		if err != nil {
			return nil
		}
		fonts = append(fonts, f)
	}
	out := make([][]string, len(fonts))
	var buf sfnt.Buffer
	for i, f := range fonts {
		if f == nil || f.NumGlyphs() < 2 {
			continue
		}
		if _, err := f.LoadGlyph(&buf, 1, fixed.I(1000), nil); err != nil {
			continue
		}
		for _, id := range []sfnt.NameID{sfnt.NameIDPostScript, sfnt.NameIDFull} {
			n, _ := f.Name(&buf, id)
			out[i] = append(out[i], n)
		}
	}
	return out
}
