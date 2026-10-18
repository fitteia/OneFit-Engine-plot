package backend

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
	"github.com/go-fonts/liberation/liberationsansregular"
)

// Resolving the same name to another file must refresh metrics and outlines
// together, and a newly restricted font must not reuse an old cached face.
func TestAdditionalFontResolutionRefreshes(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	write := func(data []byte) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "font.ttf")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	first := write(liberationsansregular.TTF)
	t.Setenv("OFE_FONT_PATH", first)
	old, err := afm.Get("LiberationSans")
	if err != nil {
		t.Fatal(err)
	}
	if source := glyph.Source("LiberationSans"); source != first {
		t.Fatalf("first outline source: %s", source)
	}
	modified := append([]byte{}, liberationsansregular.TTF...)
	hmtx := refreshTestTable(t, modified, "hmtx")
	hhea := refreshTestTable(t, modified, "hhea")
	for i := 0; i < int(binary.BigEndian.Uint16(hhea[34:36])); i++ {
		at := hmtx[4*i : 4*i+2]
		binary.BigEndian.PutUint16(at, binary.BigEndian.Uint16(at)+100)
	}
	second := write(modified)
	t.Setenv("OFE_FONT_PATH", second)
	current, err := afm.Get("LiberationSans")
	if err != nil {
		t.Fatal(err)
	}
	if current.Width("ABC") <= old.Width("ABC") {
		t.Fatalf("metrics did not refresh: old=%g new=%g", old.Width("ABC"), current.Width("ABC"))
	}
	if source := glyph.Source("LiberationSans"); source != second {
		t.Fatalf("outline source did not refresh: %s", source)
	}
	if segs, err := glyph.Outline("LiberationSans", 'A'); err != nil || len(segs) == 0 {
		t.Fatalf("replacement outlines: %v", err)
	}
	binary.BigEndian.PutUint16(refreshTestTable(t, modified, "OS/2")[8:10], 2)
	t.Setenv("OFE_FONT_PATH", write(modified))
	if _, err := afm.Get("LiberationSans"); err == nil {
		t.Fatal("cached metrics bypassed replacement embedding restrictions")
	}
	if _, err := glyph.Outline("LiberationSans", 'A'); err == nil {
		t.Fatal("cached outlines bypassed replacement embedding restrictions")
	}
}

func refreshTestTable(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	for i := 0; i < int(binary.BigEndian.Uint16(data[4:6])); i++ {
		at := 12 + 16*i
		if string(data[at:at+4]) == name {
			start := binary.BigEndian.Uint32(data[at+8 : at+12])
			size := binary.BigEndian.Uint32(data[at+12 : at+16])
			return data[start : start+size]
		}
	}
	t.Fatalf("test font has no %s table", name)
	return nil
}
