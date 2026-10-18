package afm

import (
	"github.com/fitteia/OneFit-Engine-plot/internal/fontfile"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Standard reports whether name is one of the embedded Core-14 AFM fonts.
func Standard(name string) bool {
	_, err := files.ReadFile("adobe-core14/" + name + ".afm")
	return err == nil
}

// external measures the same sfnt glyphs that the output backends draw.
func external(name string) (*Font, error) {
	face, err := fontfile.Get(name)
	if err != nil {
		return nil, err
	}
	f := face.Font
	var buf sfnt.Buffer
	upm := float64(f.UnitsPerEm())
	ppem := fixed.Int26_6(upm * 64)
	u := upm * 64
	m := &Font{Name: name, glyphs: map[byte]glyph{}, kern: map[[2]byte]float64{}, missing: map[byte]bool{}, source: face}
	var ids [256]sfnt.GlyphIndex
	for code := 0; code < 256; code++ {
		c := byte(code)
		id, err := f.GlyphIndex(&buf, fontfile.Rune(c))
		if err != nil {
			return nil, err
		}
		ids[code] = id
		m.missing[c] = id == 0
		box, advance, err := f.GlyphBounds(&buf, id, ppem, xfont.HintingNone)
		if err != nil {
			return nil, err
		}
		m.glyphs[c] = glyph{width: float64(advance) / u, box: Box{
			LLX: float64(box.Min.X) / u, LLY: -float64(box.Max.Y) / u,
			URX: float64(box.Max.X) / u, URY: -float64(box.Min.Y) / u,
		}}
	}
	for a, ai := range ids {
		if ai == 0 {
			continue
		}
		for b, bi := range ids {
			if bi == 0 {
				continue
			}
			if k, err := f.Kern(&buf, ai, bi, ppem, xfont.HintingNone); err == nil && k != 0 {
				m.kern[[2]byte{byte(a), byte(b)}] = float64(k) / u
			}
		}
	}
	return m, nil
}

// Missing reports an unavailable character in an additional font. Its visible
// .notdef glyph and advance are used, and the renderer reports a warning.
func (f *Font) Missing(c byte) bool { return f.missing[c] }
