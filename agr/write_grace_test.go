package agr_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

// TestGraceReadsWrittenFiles: Grace itself draws a written file as it
// draws the original (figures are meant to open in xmgrace too).
func TestGraceReadsWrittenFiles(t *testing.T) {
	if _, err := exec.LookPath("gracebat"); err != nil {
		t.Skip("gracebat not installed")
	}
	files, _ := filepath.Glob("../testdata/corpus/*.agr")
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".agr")
		t.Run(name, func(t *testing.T) {
			p, err := agr.ParseFile(f)
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if err := agr.Write(&b, p); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			in, out := filepath.Join(dir, "w.agr"), filepath.Join(dir, "w.eps")
			os.WriteFile(in, b.Bytes(), 0o644)
			if o, err := exec.Command("gracebat", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
				t.Fatalf("gracebat: %v %s", err, o)
			} else if strings.Contains(string(o), "rror") {
				t.Errorf("gracebat complains: %s", o)
			}
			read := func(name string) *draw.Drawing {
				r, err := os.Open(name)
				if err != nil {
					t.Fatal(err)
				}
				defer r.Close()
				d, err := eps.Parse(r)
				if err != nil {
					t.Fatal(err)
				}
				return d
			}
			for _, d := range drawcmp.Compare(read(out), read(filepath.Join("..", "testdata", "ref", name+".eps")), 10) {
				t.Error(d)
			}
		})
	}
}
