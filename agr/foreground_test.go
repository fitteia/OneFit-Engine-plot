package agr_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

func TestObjectForegroundRoundTrip(t *testing.T) {
	src := `@version 50125
@page size 600, 450
@g0 on
@with g0
@view 0.1, 0.1, 0.9, 0.9
@world 0, 0, 10, 10
@legend on
@legend char size 1
@legend loctype view
@legend 0.6, 0.7
@legend box fill pattern 1
@s0 legend "Legend row"
@with string
@string on
@string 0.3, 0.7
@string def "Page text"
@target G0.S0
@type xy
1 2
3 4
&
`
	for _, loc := range []string{"world", "view"} {
		for _, kind := range []string{"box", "ellipse", "line"} {
			t.Run(loc+kind, func(t *testing.T) {
				p, err := agr.Parse(strings.NewReader(src + fmt.Sprintf("@with %s\n@%s on\n@%s loctype %s\n@%s g0\n@%s 0.2, 0.2, 0.8, 0.8\n@%s def\n", kind, kind, kind, loc, kind, kind, kind)))
				if err != nil {
					t.Fatal(err)
				}
				normal, _ := render.Render(p)
				foundLegend := false
				for _, text := range normal.Texts {
					if text.ID == "g0.legend" {
						foundLegend = true
					}
				}
				if !foundLegend {
					t.Fatal("fixture did not draw legend")
				}
				last := func() string {
					if normal.Order[len(normal.Order)-1] == 'p' {
						return normal.Paths[len(normal.Paths)-1].ID
					}
					return normal.Texts[len(normal.Texts)-1].ID
				}()
				if last == "object.0" {
					t.Fatal("ordinary object unexpectedly on top")
				}
				p.Shapes[0].OnTop = true
				for round := 0; round < 3; round++ {
					var b bytes.Buffer
					if err := agr.Write(&b, p); err != nil {
						t.Fatal(err)
					}
					if strings.Count(b.String(), "# plot-go: next object on-top") != 1 {
						t.Fatal("foreground marker missing or duplicated")
					}
					p, err = agr.Parse(strings.NewReader(b.String()))
					if err != nil {
						t.Fatal(err)
					}
					if !p.Shapes[0].OnTop || p.Shapes[0].LocType != loc {
						t.Fatal("layer or attachment lost")
					}
					d, _ := render.Render(p)
					if d.Order[len(d.Order)-1] != 'p' || d.Paths[len(d.Paths)-1].ID != "object.0" {
						t.Fatal("foreground object not above page text and legend")
					}
				}
				// New same-kind objects must not inherit a plot-go-only setting.
				var b bytes.Buffer
				agr.Write(&b, p)
				q, err := agr.Parse(strings.NewReader(b.String() + fmt.Sprintf("\n@with %s\n@%s on\n@%s def\n", kind, kind, kind)))
				if err != nil {
					t.Fatal(err)
				}
				if q.Shapes[1].OnTop {
					t.Fatal("foreground setting inherited")
				}
				p.Shapes[0].OnTop = false
				b.Reset()
				agr.Write(&b, p)
				if strings.Contains(b.String(), "next object on-top") {
					t.Fatal("foreground marker survived disabling")
				}
			})
		}
	}
}
