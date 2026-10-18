package fontfile

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// The disk cache of the font index: for each font file seen, its size and
// modification time and its faces' names. A run lists the font folders and
// reads the name table only of a file that is new or changed since; the
// cache is rewritten when anything changed. It lives in the user's cache
// folder (os.UserCacheDir: ~/Library/Caches on macOS, $XDG_CACHE_HOME or
// ~/.cache elsewhere), "plot-go/fonts.json". A cache that is missing,
// unreadable or of another version is rebuilt; failing to write it is not
// an error - the next run reads the names again.

// cacheVersion changes whenever what readNames returns does.
const cacheVersion = 1

type cacheEntry struct {
	Size  int64      `json:"size"`
	Mod   int64      `json:"mod"` // modification time, Unix nanoseconds
	Faces [][]string `json:"faces,omitempty"`
}

type cacheFile struct {
	Version int                   `json:"version"`
	Files   map[string]cacheEntry `json:"files"`
}

func cachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "plot-go", "fonts.json")
}

func loadCache() cacheFile {
	empty := cacheFile{Version: cacheVersion, Files: map[string]cacheEntry{}}
	p := cachePath()
	if p == "" {
		return empty
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return empty
	}
	var c cacheFile
	if json.Unmarshal(b, &c) != nil || c.Version != cacheVersion || c.Files == nil {
		return empty
	}
	return c
}

// saveCache writes the cache through a temporary file, so a run reading it
// at the same time sees the old one or the new one, whole.
func saveCache(files map[string]cacheEntry) {
	p := cachePath()
	if p == "" {
		return
	}
	b, err := json.Marshal(cacheFile{Version: cacheVersion, Files: files})
	if err != nil || os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "fonts-*.json")
	if err != nil {
		return
	}
	_, werr := tmp.Write(b)
	if cerr := tmp.Close(); werr != nil || cerr != nil || os.Rename(tmp.Name(), p) != nil {
		os.Remove(tmp.Name())
	}
}

// sameFiles tells whether a fresh listing found the files the cache has,
// each as it was.
func sameFiles(a, b map[string]cacheEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for p, e := range a {
		f, ok := b[p]
		if !ok || f.Size != e.Size || f.Mod != e.Mod {
			return false
		}
	}
	return true
}
