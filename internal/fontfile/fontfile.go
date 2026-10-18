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
	"time"

	"golang.org/x/image/font/sfnt"
)

// Face is an immutable parsed font. Callers use their own sfnt.Buffer.
type Face struct {
	Font                                  *sfnt.Font
	Path                                  string
	UnderlinePosition, UnderlineThickness float64
}

// loc is where a name's font is: its file, and the face in a collection
// (.ttc; 0 in a single-font file).
type loc struct {
	path string
	face int
	size int64
	mod  int64
}

// fresh is how long a process trusts the index it built before listing the
// font folders again: a long-running one (the OneFit GUI) sees a font
// installed meanwhile, and a run drawing one plot lists them once.
const fresh = 30 * time.Second

var (
	mu      sync.Mutex
	indexes = map[string]*index{}
	faces   = map[loc]*Face{}
)

// filesRead counts the font files read whole to index them (tests).
var filesRead int

type index struct {
	names map[string]loc
	built time.Time
}

// Get resolves a PostScript or full font name. OFE_FONT_PATH is a path-list of
// files or directories searched before the standard system/user font folders.
// The first matching file wins. Unreadable or malformed font files are skipped;
// unresolved names return an error so the renderer can report its fallback.
func Get(name string) (*Face, error) {
	mu.Lock()
	defer mu.Unlock()
	l, ok := indexLocked()[name]
	if !ok {
		return nil, fmt.Errorf("font %q not found in OFE_FONT_PATH or system font folders", name)
	}
	if f := faces[l]; f != nil {
		return f, nil
	}
	b, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("font %q: %w", name, err)
	}
	var f *sfnt.Font
	data := b // the face's own tables: a collection's face starts further in
	if isCollection(b) {
		c, err := sfnt.ParseCollection(b)
		if err != nil {
			return nil, fmt.Errorf("font %q: %w", name, err)
		}
		if f, err = c.Font(l.face); err != nil {
			return nil, fmt.Errorf("font %q: %w", name, err)
		}
		off, ok := faceOffset(b, l.face)
		if !ok {
			return nil, fmt.Errorf("font %q: bad collection", name)
		}
		data = b[off:]
	} else if f, err = sfnt.Parse(b); err != nil {
		return nil, fmt.Errorf("font %q: %w", name, err)
	}
	if flags := embeddingFlags(data, b); flags&(0x0002|0x0100|0x0200) != 0 {
		return nil, fmt.Errorf("font %q disallows outline subset embedding (OS/2 fsType 0x%04x)", name, flags)
	}
	pos, thickness := underline(data, b, float64(f.UnitsPerEm()))
	face := &Face{Font: f, Path: l.path, UnderlinePosition: pos, UnderlineThickness: thickness}
	faces[l] = face
	return face, nil
}

// indexLocked is the name index: built by listing the font folders and
// reading only the name tables of files the disk cache (cache.go) does not
// already know as they are.
func indexLocked() map[string]loc {
	key := os.Getenv("OFE_FONT_PATH") + "\x00" + os.Getenv("OFE_URW_FONTS") + "\x00" + os.Getenv("HOME") + "\x00" + os.Getenv("WINDIR")
	if ix := indexes[key]; ix != nil && time.Since(ix.built) < fresh {
		return ix.names
	}
	old := loadCache()
	files := map[string]cacheEntry{}
	names := map[string]loc{}
	for _, root := range searchPaths() {
		// WalkDir's lexical order makes duplicate names deterministic.
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".otf", ".ttf", ".ttc":
			default:
				return nil
			}
			if _, seen := files[path]; seen {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			e, ok := old.Files[path]
			if !ok || e.Size != info.Size() || e.Mod != info.ModTime().UnixNano() {
				// unreadable or not a font: remembered with no faces, so it
				// is not read again until it changes
				e = cacheEntry{Size: info.Size(), Mod: info.ModTime().UnixNano(), Faces: readNames(path)}
				filesRead++
			}
			files[path] = e
			for i, fn := range e.Faces {
				for _, n := range fn {
					if _, taken := names[n]; !taken && n != "" {
						names[n] = loc{path: path, face: i, size: e.Size, mod: e.Mod}
					}
				}
			}
			return nil
		})
	}
	if !sameFiles(old.Files, files) {
		saveCache(files)
	}
	// Parsed faces belong to a specific file revision, just like the name
	// index. Drop replaced/removed revisions rather than retaining stale
	// outlines (or accumulating old revisions in a long-running GUI).
	for l := range faces {
		e, ok := files[l.path]
		if !ok || l.size != e.Size || l.mod != e.Mod {
			delete(faces, l)
		}
	}
	indexes[key] = &index{names: names, built: time.Now()}
	return names
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
