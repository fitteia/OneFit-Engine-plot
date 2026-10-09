package agr_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// TestWriteRoundTrip: every corpus file, written and read back, is the same
// Project and draws the same - the figure editor saves through Write.
func TestWriteRoundTrip(t *testing.T) {
	files, _ := filepath.Glob("../testdata/corpus/*.agr")
	if len(files) == 0 {
		t.Fatal("no corpus files")
	}
	for _, f := range files {
		p, err := agr.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := agr.Write(&b, p); err != nil {
			t.Fatal(err)
		}
		q, err := agr.Parse(strings.NewReader(b.String()))
		if err != nil {
			t.Fatal(err)
		}
		if len(q.Warnings) != 0 {
			t.Errorf("%s: written file warns: %+v", filepath.Base(f), q.Warnings[0])
		}
		p.Warnings, q.Warnings = nil, nil
		if !reflect.DeepEqual(p, q) {
			t.Errorf("%s: read back differently", filepath.Base(f))
			continue
		}
		dp, _ := render.Render(p)
		dq, _ := render.Render(q)
		if !reflect.DeepEqual(dp, dq) {
			t.Errorf("%s: draws differently after a round trip", filepath.Base(f))
		}
	}
}
