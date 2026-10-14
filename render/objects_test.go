package render

import (
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

// TestArrowTypeGraceHasNot: gracebat draws the line of an arrow type it
// does not have, without heads ("Internal error in draw_arrowhead()";
// measured), and so does plot-go, with a warning.
func TestArrowTypeGraceHasNot(t *testing.T) {
	p, err := agr.Parse(strings.NewReader("@version 50125\n@page size 773, 600\n" +
		"@with line\n@    line on\n@    line 0.1, 0.3, 0.4, 0.3\n@    line arrow 3\n@    line arrow type 7\n@line def\n"))
	if err != nil {
		t.Fatal(err)
	}
	d, warnings := Render(p)
	var lines int
	for _, path := range d.Paths {
		if strings.HasPrefix(path.ID, "object.") {
			lines++
		}
	}
	if lines != 1 {
		t.Errorf("%d paths drawn, gracebat draws the line alone", lines)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "arrow type 7") {
		t.Errorf("warnings %q", warnings)
	}
}
