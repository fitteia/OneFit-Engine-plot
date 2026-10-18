package fontfile

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-fonts/liberation/liberationsansregular"
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
		os2 := table(data, "OS/2")
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
