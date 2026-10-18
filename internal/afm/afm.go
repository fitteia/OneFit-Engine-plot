// Package afm gives plot-go the metrics of the 14 standard PDF fonts:
// each character's advance width and ink box, from Adobe's Core 14 AFM
// files (adobe-core14/, distributed with Adobe's MustRead.html as its
// terms require; see NOTICE). Additional installed TTF/OpenType fonts are
// measured from the same glyph data used for rendering.
package afm

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/fitteia/OneFit-Engine-plot/internal/fontfile"
)

//go:embed adobe-core14/*.afm adobe-core14/MustRead.html
var files embed.FS

// AdobeNotice is Adobe's MustRead.html, whose terms require it to go with
// the AFM files wherever they are distributed - they are embedded in every
// plot-go binary, so this is too (plot-go -notices prints it).
func AdobeNotice() string {
	b, _ := files.ReadFile("adobe-core14/MustRead.html")
	return string(b)
}

// Box is an ink box in units of the font size (1 = one em).
type Box struct {
	LLX, LLY, URX, URY float64
}

// Empty reports whether the box holds no ink.
func (b Box) Empty() bool { return b.URX <= b.LLX && b.URY <= b.LLY }

// Union is the smallest box holding both.
func (b Box) Union(o Box) Box {
	if b.Empty() {
		return o
	}
	if o.Empty() {
		return b
	}
	return Box{min(b.LLX, o.LLX), min(b.LLY, o.LLY), max(b.URX, o.URX), max(b.URY, o.URY)}
}

type glyph struct {
	width float64
	box   Box
}

// Font holds one font's metrics by character code.
type Font struct {
	Name    string
	glyphs  map[byte]glyph
	kern    map[[2]byte]float64
	missing map[byte]bool
	source  *fontfile.Face // nil for immutable embedded standard metrics
}

var (
	mu    sync.Mutex
	cache = map[string]*Font{}
)

// Get returns standard AFM metrics or an additional installed font's own
// metrics, resolved by PostScript or full name.
func Get(name string) (*Font, error) {
	mu.Lock()
	defer mu.Unlock()
	if f := cache[name]; f != nil {
		if f.source == nil {
			return f, nil
		}
		face, err := fontfile.Get(name)
		if err != nil {
			return nil, err
		}
		if face == f.source {
			return f, nil
		}
	}
	b, err := files.ReadFile("adobe-core14/" + name + ".afm")
	if err != nil {
		f, err := external(name)
		if err == nil {
			cache[name] = f
		}
		return f, err
	}
	f, err := parse(name, string(b))
	if err != nil {
		return nil, fmt.Errorf("%s.afm: %w", name, err)
	}
	cache[name] = f
	return f, nil
}

// Read reads standard AFM metrics, including named kerning pairs.
func Read(name string, r io.Reader) (*Font, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return parse(name, string(b))
}

// parse reads the character metrics: lines like
// "C 48 ; WX 556 ; N zero ; B 37 -19 519 703 ;". Codes are the font's own
// encoding (Adobe Standard for text fonts). Latin fonts are reencoded by
// glyph name to the Latin-1 bytes used by the renderer and output backends.
func parse(name, src string) (*Font, error) {
	f := &Font{Name: name, glyphs: map[byte]glyph{}}
	byName := map[string]glyph{}
	encoding := map[byte]string{}
	pairs := map[[2]string]float64{}
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "KPX ") {
			p := strings.Fields(line)
			if len(p) == 4 {
				v, err := strconv.ParseFloat(p[3], 64)
				if err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
					pairs[[2]string{p[1], p[2]}] = v / 1000
				}
			}
			continue
		}
		if !strings.HasPrefix(line, "C ") {
			continue
		}
		code, g, gname := -1, glyph{}, ""
		for _, field := range strings.Split(line, ";") {
			w := strings.Fields(field)
			if len(w) == 0 {
				continue
			}
			switch w[0] {
			case "C":
				if len(w) != 2 {
					continue
				}
				code, _ = strconv.Atoi(w[1])
			case "WX":
				if len(w) != 2 {
					continue
				}
				x, _ := strconv.ParseFloat(w[1], 64)
				g.width = x / 1000
			case "N":
				if len(w) != 2 {
					continue
				}
				gname = w[1]
			case "B":
				if len(w) == 5 {
					var v [4]float64
					for i := range v {
						v[i], _ = strconv.ParseFloat(w[i+1], 64)
					}
					g.box = Box{v[0] / 1000, v[1] / 1000, v[2] / 1000, v[3] / 1000}
				}
			}
		}
		byName[gname] = g
		if code >= 0 && code < 256 {
			encoding[byte(code)] = gname
		}
	}
	if len(encoding) == 0 {
		return nil, fmt.Errorf("no character metrics")
	}
	if name != "Symbol" && name != "ZapfDingbats" {
		// gracebat's encoding is Adobe Standard in the ASCII range except
		// that 96 is the grave accent, not quoteleft; 39 stays quoteright
		// and 45 the hyphen (docs/grace-behaviour.md).
		if _, ok := byName["grave"]; ok {
			encoding['`'] = "grave"
		}
		// AFM C codes above ASCII use Adobe Standard, not Latin-1.
		// Degree, for example, has C -1 and must be found by name.
		for code := range encoding {
			if code >= 128 {
				delete(encoding, code)
			}
		}
		for i, glyphName := range strings.Fields(latin1Names) {
			encoding[byte(0xa0+i)] = glyphName
		}
	}
	names := map[string][]byte{}
	for code, glyphName := range encoding {
		if g, ok := byName[glyphName]; ok {
			f.glyphs[code] = g
			names[glyphName] = append(names[glyphName], code)
		}
	}
	f.kern = make(map[[2]byte]float64)
	for pair, v := range pairs {
		// Space and hyphen each have two Latin-1 codes. Preserve the
		// pair for every alias, including unencoded accented AFM glyphs.
		for _, a := range names[pair[0]] {
			for _, b := range names[pair[1]] {
				f.kern[[2]byte{a, b}] = v
			}
		}
	}
	return f, sc.Err()
}

// ISO-8859-1 0xa0..0xff, in Adobe glyph names. Symbol and ZapfDingbats
// retain their own encodings; the Latin ASCII quote exceptions are above.
const latin1Names = `
space exclamdown cent sterling currency yen brokenbar section
dieresis copyright ordfeminine guillemotleft logicalnot hyphen registered macron
degree plusminus twosuperior threesuperior acute mu paragraph periodcentered
cedilla onesuperior ordmasculine guillemotright onequarter onehalf threequarters questiondown
Agrave Aacute Acircumflex Atilde Adieresis Aring AE Ccedilla
Egrave Eacute Ecircumflex Edieresis Igrave Iacute Icircumflex Idieresis
Eth Ntilde Ograve Oacute Ocircumflex Otilde Odieresis multiply
Oslash Ugrave Uacute Ucircumflex Udieresis Yacute Thorn germandbls
agrave aacute acircumflex atilde adieresis aring ae ccedilla
egrave eacute ecircumflex edieresis igrave iacute icircumflex idieresis
eth ntilde ograve oacute ocircumflex otilde odieresis divide
oslash ugrave uacute ucircumflex udieresis yacute thorn ydieresis`

// Width is the advance width of s, in ems.
func (f *Font) Width(s string) float64 {
	w := 0.0
	for i := 0; i < len(s); i++ {
		w += f.glyphs[s[i]].width
	}
	return w
}

// Ink is the box of the ink s leaves, in ems, with the pen starting at 0.
func (f *Font) Ink(s string) Box {
	var b Box
	x := 0.0
	for i := 0; i < len(s); i++ {
		g := f.glyphs[s[i]]
		if !g.box.Empty() {
			b = b.Union(Box{x + g.box.LLX, g.box.LLY, x + g.box.URX, g.box.URY})
		}
		x += g.width
	}
	return b
}

// Kern is the AFM pair adjustment in ems (zero for an unlisted pair).
func (f *Font) Kern(a, b byte) float64 { return f.kern[[2]byte{a, b}] }
