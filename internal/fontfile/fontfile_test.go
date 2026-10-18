package fontfile

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-fonts/liberation/liberationsansbold"
	"github.com/go-fonts/liberation/liberationsansregular"
	"golang.org/x/image/font/sfnt"
)

func TestFindAdditionalFont(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arbitrary-filename.ttf")
	if err := os.WriteFile(path, liberationsansregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	// Discovery uses the font's internal name, not its filename.
	t.Setenv("OFE_FONT_PATH", path)
	f, err := Get("LiberationSans")
	if err != nil {
		t.Fatal(err)
	}
	if f.Path != path {
		t.Fatalf("source %q, want explicit file %q", f.Path, path)
	}
	full, err := Get("Liberation Sans")
	if err != nil || full.Path != path {
		t.Fatalf("full-name lookup: %+v, %v", full, err)
	}
	if _, err := Get("NotARealFont-12345"); err == nil {
		t.Fatal("unknown font found")
	}
}

func TestEmbeddingRestrictions(t *testing.T) {
	for _, flag := range []uint16{0x0002, 0x0100, 0x0200} {
		data := append([]byte{}, liberationsansregular.TTF...)
		os2 := table(data, data, "OS/2")
		if len(os2) < 10 {
			t.Fatal("test font has no OS/2 table")
		}
		binary.BigEndian.PutUint16(os2[8:10], flag)
		path := filepath.Join(t.TempDir(), "restricted.ttf")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("OFE_FONT_PATH", path)
		if _, err := Get("LiberationSans"); err == nil || !strings.Contains(err.Error(), "disallows") {
			t.Fatalf("flag %04x: %v", flag, err)
		}
	}
}

func TestSearchOrderAndBadFiles(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for _, dir := range []string{first, second} {
		if err := os.WriteFile(filepath.Join(dir, "font.ttf"), liberationsansregular.TTF, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(first, "bad.otf"), []byte("not a font"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFE_FONT_PATH", first+string(os.PathListSeparator)+second)
	f, err := Get("LiberationSans")
	if err != nil {
		t.Fatal(err)
	}
	if f.Path != filepath.Join(first, "font.ttf") {
		t.Fatalf("wrong precedence: %q", f.Path)
	}
}

// collection makes a TrueType collection of fonts: the 'ttcf' header,
// then each font with its table offsets moved by where it now starts.
func collection(fonts ...[]byte) []byte {
	head := 12 + 4*len(fonts)
	out := make([]byte, head)
	copy(out, "ttcf\x00\x01\x00\x00")
	binary.BigEndian.PutUint32(out[8:], uint32(len(fonts)))
	for i, f := range fonts {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		start := len(out)
		binary.BigEndian.PutUint32(out[12+4*i:], uint32(start))
		g := append([]byte{}, f...)
		n := int(binary.BigEndian.Uint16(g[4:6]))
		for k := 0; k < n; k++ {
			at := 12 + 16*k + 8
			binary.BigEndian.PutUint32(g[at:], binary.BigEndian.Uint32(g[at:])+uint32(start))
		}
		out = append(out, g...)
	}
	return out
}

// TestCollectionFaces: each face of a .ttc is found by its own names and
// loads as itself.
func TestCollectionFaces(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "pair.ttc")
	if err := os.WriteFile(path, collection(liberationsansregular.TTF, liberationsansbold.TTF), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFE_FONT_PATH", path)
	for name, want := range map[string]string{"LiberationSans": "LiberationSans", "LiberationSans-Bold": "LiberationSans-Bold", "Liberation Sans Bold": "LiberationSans-Bold"} {
		f, err := Get(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var buf sfnt.Buffer
		ps, _ := f.Font.Name(&buf, sfnt.NameIDPostScript)
		if f.Path != path || ps != want {
			t.Errorf("%s: face %q from %s, want %q", name, ps, f.Path, want)
		}
		if f.UnderlineThickness <= 0 {
			t.Errorf("%s: no underline from its own post table", name)
		}
	}
}

// TestIndexCache: a font file is read once; a later run (a new process,
// here a forgotten in-memory index) finds it in the disk cache, unless it
// changed.
func TestIndexCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, "a.ttf")
	if err := os.WriteFile(path, liberationsansregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OFE_FONT_PATH", dir)
	run := func() int {
		mu.Lock()
		defer mu.Unlock()
		indexes = map[string]*index{}
		before := filesRead
		if _, ok := indexLocked()["LiberationSans"]; !ok {
			t.Fatal("LiberationSans not indexed")
		}
		return filesRead - before
	}
	if n := run(); n == 0 {
		t.Fatal("first run read no font file")
	}
	if n := run(); n != 0 {
		t.Errorf("second run read %d files, the cache has them all", n)
	}
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if n := run(); n != 1 {
		t.Errorf("after a change, %d files read, want 1", n)
	}
	if _, err := os.Stat(cachePath()); err != nil {
		t.Errorf("no cache file: %v", err)
	}
}

func TestReplacedFontRefreshes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "replace.ttf")
	t.Setenv("OFE_FONT_PATH", path)
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		// Simulate the next 30-second index refresh without sleeping.
		mu.Lock()
		for _, ix := range indexes {
			ix.built = time.Now().Add(-fresh - time.Second)
		}
		mu.Unlock()
	}
	write(liberationsansregular.TTF)
	old, err := Get("LiberationSans")
	if err != nil {
		t.Fatal(err)
	}
	write(liberationsansbold.TTF)
	current, err := Get("LiberationSans-Bold")
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	name, err := current.Font.Name(&buf, sfnt.NameIDPostScript)
	if err != nil || name != "LiberationSans-Bold" || current == old {
		t.Fatalf("replaced font: name=%q err=%v old face=%v", name, err, current == old)
	}
	if cached, err := Get("LiberationSans-Bold"); err != nil || cached != current {
		t.Fatal("unchanged face was not reused")
	}
	restricted := append([]byte{}, liberationsansbold.TTF...)
	binary.BigEndian.PutUint16(table(restricted, restricted, "OS/2")[8:10], 2)
	write(restricted)
	// Ensure a changed timestamp even on filesystems with coarse clocks.
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := Get("LiberationSans-Bold"); err == nil || !strings.Contains(err.Error(), "disallows") {
		t.Fatalf("replacement embedding restriction bypassed: %v", err)
	}
}
