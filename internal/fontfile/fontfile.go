// Package fontfile finds additional TrueType/OpenType fonts by PostScript name.
// Files are read from the machine running plot-go, never bundled with it.
package fontfile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font/sfnt"
)

// Face is an immutable parsed font. Callers use their own sfnt.Buffer.
type Face struct {
	Font                                  *sfnt.Font
	Path                                  string
	UnderlinePosition, UnderlineThickness float64
}

var mu sync.Mutex
var indexes = map[string]map[string]string{}
var faces = map[string]*Face{}

// Get resolves a PostScript or full font name. OFE_FONT_PATH is a path-list of
// files or directories searched before the standard system/user font folders.
// The first matching file wins. Unreadable or malformed font files are skipped;
// unresolved names return an error so the renderer can report its fallback.
func Get(name string) (*Face, error) {
	mu.Lock()
	defer mu.Unlock()
	index := indexLocked()
	path := index[name]
	if path == "" {
		return nil, fmt.Errorf("font %q not found in OFE_FONT_PATH or system font folders", name)
	}
	if f := faces[path]; f != nil {
		return f, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", name, err)
	}
	f, err := sfnt.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", name, err)
	}
	if flags := embeddingFlags(b); flags&(0x0002|0x0100|0x0200) != 0 {
		return nil, fmt.Errorf("font %q disallows outline subset embedding (OS/2 fsType 0x%04x)", name, flags)
	}
	pos, thickness := underline(b, float64(f.UnitsPerEm()))
	face := &Face{Font: f, Path: path, UnderlinePosition: pos, UnderlineThickness: thickness}
	faces[path] = face
	return face, nil
}

func indexLocked() map[string]string {
	key := os.Getenv("OFE_FONT_PATH") + "\x00" + os.Getenv("OFE_URW_FONTS") + "\x00" + os.Getenv("HOME") + "\x00" + os.Getenv("WINDIR")
	index := indexes[key]
	if index == nil {
		index = map[string]string{}
		for _, root := range searchPaths() {
			// WalkDir's lexical order makes duplicate names deterministic.
			_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext != ".otf" && ext != ".ttf" {
					return nil
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return nil
				}
				f, err := sfnt.Parse(b)
				if err != nil {
					return nil
				}
				var buf sfnt.Buffer
				for _, id := range []sfnt.NameID{sfnt.NameIDPostScript, sfnt.NameIDFull} {
					if n, err := f.Name(&buf, id); err == nil && n != "" && index[n] == "" {
						index[n] = path
					}
				}
				return nil
			})
		}
		indexes[key] = index
	}
	return index
}

// Names returns sorted resolvable names (PostScript names and full-name aliases).
func Names() []string {
	mu.Lock()
	defer mu.Unlock()
	var names []string
	for name := range indexLocked() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func searchPaths() []string {
	paths := filepath.SplitList(os.Getenv("OFE_FONT_PATH"))
	if p := os.Getenv("OFE_URW_FONTS"); p != "" {
		paths = append(paths, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts"), filepath.Join(home, "Library/Fonts"))
	}
	paths = append(paths, "/usr/local/share/fonts", "/usr/share/fonts", "/Library/Fonts", "/System/Library/Fonts", "/opt/homebrew/share/fonts")
	if win := os.Getenv("WINDIR"); win != "" {
		paths = append(paths, filepath.Join(win, "Fonts"))
	}
	return paths
}

// Rune applies Grace's Latin text encoding, including the curly apostrophe.
func Rune(c byte) rune {
	if c == '\'' {
		return '’'
	}
	return rune(c)
}
