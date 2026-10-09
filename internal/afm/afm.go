// Package afm gives plot-go the metrics of the 14 standard PDF fonts:
// each character's advance width and ink box, from Adobe's Core 14 AFM
// files (adobe-core14/, distributed with Adobe's MustRead.html as its
// terms require; see NOTICE).
package afm

import (
	"bufio"
	"embed"
	"fmt"
	"strconv"
	"strings"
	"sync"
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
	Name   string
	glyphs map[byte]glyph
}

var (
	mu    sync.Mutex
	cache = map[string]*Font{}
)

// Get returns the metrics of a standard font by PostScript name
// ("Helvetica", "Times-Roman", "Symbol", ...).
func Get(name string) (*Font, error) {
	mu.Lock()
	defer mu.Unlock()
	if f := cache[name]; f != nil {
		return f, nil
	}
	b, err := files.ReadFile("adobe-core14/" + name + ".afm")
	if err != nil {
		return nil, fmt.Errorf("no metrics for font %q", name)
	}
	f, err := parse(name, string(b))
	if err != nil {
		return nil, fmt.Errorf("%s.afm: %w", name, err)
	}
	cache[name] = f
	return f, nil
}

// parse reads the character metrics: lines like
// "C 48 ; WX 556 ; N zero ; B 37 -19 519 703 ;". Codes are the font's own
// encoding (Adobe Standard for text fonts), which is ASCII for the
// printable characters except the quotes.
func parse(name, src string) (*Font, error) {
	f := &Font{Name: name, glyphs: map[byte]glyph{}}
	byName := map[string]glyph{}
	sc := bufio.NewScanner(strings.NewReader(src))
	for sc.Scan() {
		line := sc.Text()
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
				code, _ = strconv.Atoi(w[1])
			case "WX":
				x, _ := strconv.ParseFloat(w[1], 64)
				g.width = x / 1000
			case "N":
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
			f.glyphs[byte(code)] = g
		}
	}
	if len(f.glyphs) == 0 {
		return nil, fmt.Errorf("no character metrics")
	}
	if name != "Symbol" && name != "ZapfDingbats" {
		// gracebat's encoding is Adobe Standard in the ASCII range except
		// that 96 is the grave accent, not quoteleft; 39 stays quoteright
		// and 45 the hyphen (docs/grace-behaviour.md).
		if g, ok := byName["grave"]; ok {
			f.glyphs['`'] = g
		}
	}
	return f, sc.Err()
}

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
