package glyph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
)

// The stand-ins' outlines sit where Adobe's metrics put the ink: a
// character's outline spans about its AFM ink box.
func TestOutlinesMatchAdobeInk(t *testing.T) {
	for _, c := range []struct {
		font string
		ch   byte
	}{{"Helvetica", '0'}, {"Helvetica", 'm'}, {"Times-Roman", 'R'}, {"Symbol", 'D'}, {"Symbol", 'm'}} {
		segs, err := Outline(c.font, c.ch)
		if err != nil || len(segs) == 0 {
			t.Fatalf("%s %q: %v, %d segments", c.font, c.ch, err, len(segs))
		}
		lx, ly, ux, uy := 1e9, 1e9, -1e9, -1e9
		for _, s := range segs {
			for _, p := range s.Pts {
				lx, ly, ux, uy = min(lx, p[0]), min(ly, p[1]), max(ux, p[0]), max(uy, p[1])
			}
		}
		m, _ := afm.Get(c.font)
		ink := m.Ink(string(c.ch))
		for _, d := range []float64{lx - ink.LLX, ly - ink.LLY, ux - ink.URX, uy - ink.URY} {
			if d > 0.06 || d < -0.06 {
				t.Errorf("%s %q: outline box %.3f %.3f %.3f %.3f, Adobe ink %+v", c.font, c.ch, lx, ly, ux, uy, ink)
				break
			}
		}
	}
}

func TestRune(t *testing.T) {
	if Rune("Symbol", 'D') != 'Δ' || Rune("Symbol", 'm') != 'μ' || Rune("Helvetica", '\'') != '’' || Rune("Helvetica", 'a') != 'a' {
		t.Error("wrong characters")
	}
}

func TestUnknownFont(t *testing.T) {
	if _, err := Outline("ZapfDingbats", 'a'); err == nil {
		t.Error("ZapfDingbats has no stand-in, but an outline came back")
	}
}

// With the URW fonts installed the outlines come from them - the glyphs
// Ghostscript puts in OneFit's PDFs; without, from Liberation.
func TestSourcePrefersInstalledURW(t *testing.T) {
	if urwPath("Helvetica") == "" {
		if Source("Helvetica") != "Liberation" {
			t.Errorf("no URW fonts, but Source = %q", Source("Helvetica"))
		}
		t.Skip("URW base-35 OpenType fonts not installed")
	}
	for _, f := range []string{"Helvetica", "Times-Roman", "Symbol"} {
		if s := Source(f); !strings.HasSuffix(s, ".otf") {
			t.Errorf("%s: source %q, want the installed URW font", f, s)
		}
	}
}

// Without the URW fonts (macOS, or a machine without Ghostscript) the
// carried Liberation fonts draw, Symbol's Greek fitted to Adobe's boxes.
func TestLiberationFallback(t *testing.T) {
	saved := urwDirs
	urwDirs = nil
	t.Setenv("OFE_URW_FONTS", "")
	mu.Lock()
	cache = map[string]*font{}
	mu.Unlock()
	defer func() {
		urwDirs = saved
		mu.Lock()
		cache = map[string]*font{}
		mu.Unlock()
	}()
	for _, f := range []string{"Helvetica", "Times-Roman", "Symbol"} {
		if s := Source(f); s != "Liberation" {
			t.Errorf("%s: source %q, want Liberation", f, s)
		}
	}
	segs, err := Outline("Symbol", 'm')
	if err != nil || len(segs) == 0 {
		t.Fatalf("mu: %v, %d segments", err, len(segs))
	}
	lx, ux := 1e9, -1e9
	for _, s := range segs {
		for _, p := range s.Pts {
			lx, ux = min(lx, p[0]), max(ux, p[0])
		}
	}
	if d := (ux - lx) - (0.567 - 0.033); d > 1e-9 || d < -1e-9 {
		t.Errorf("Liberation mu spans %.3f..%.3f, want Adobe's Symbol box 0.033..0.567", lx, ux)
	}
}

// On macOS the URW fonts are found where Homebrew's font-urw-base35 cask
// puts them: ~/Library/Fonts.
func TestFindsURWInMacFontFolders(t *testing.T) {
	home := t.TempDir()
	fonts := filepath.Join(home, "Library", "Fonts")
	if err := os.MkdirAll(fonts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fonts, "NimbusSans-Regular.otf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	saved := urwDirs
	urwDirs = macFontDirs(home)
	t.Setenv("OFE_URW_FONTS", "")
	defer func() { urwDirs = saved }()
	if got, want := urwPath("Helvetica"), filepath.Join(fonts, "NimbusSans-Regular.otf"); got != want {
		t.Errorf("Helvetica found at %q, want %q", got, want)
	}
	if got := urwPath("Times-Roman"); got != "" {
		t.Errorf("Times-Roman found at %q, but it is not there", got)
	}
}

// Homebrew's place is searched - the onefite-go formula installs the URW
// fonts in share/fonts/urw-base35 - and so is the same folder next to the
// running program's bin.
func TestSearchesHomebrewPlaces(t *testing.T) {
	for _, d := range []string{"/opt/homebrew/share/fonts/urw-base35", "/usr/local/share/fonts/urw-base35"} {
		found := false
		for _, u := range urwDirs {
			found = found || u == d
		}
		if !found {
			t.Errorf("%s is not searched: %v", d, urwDirs)
		}
	}
	exe, _ := os.Executable()
	want := filepath.Join(filepath.Dir(filepath.Dir(exe)), "share", "fonts", "urw-base35")
	if dirs := besideExecutable(); len(dirs) == 0 || dirs[0] != want {
		t.Errorf("beside the program: %v, want %s first", dirs, want)
	}
}
