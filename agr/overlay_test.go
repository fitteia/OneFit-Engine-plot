package agr_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

const twoGraphs = "@g0 on\n@with g0\n@    world 0, 0, 1, 1\n@g1 on\n@with g1\n@    world 0, 0, 1, 1\n@g2 on\n@with g2\n@    world 0, 0, 1, 1\n"

func TestOverlayLinkRoundTrip(t *testing.T) {
	p, err := agr.Parse(strings.NewReader(twoGraphs + "# plot-go: g1 overlays g0 shared x\n# plot-go: g2 overlays g0 shared y\n"))
	if err != nil || len(p.Warnings) > 0 {
		t.Fatal(err, p.Warnings)
	}
	if p.Graphs[0].Overlay != (agr.Link{}) || p.Graphs[1].Overlay != (agr.Link{Base: 0, Shared: "x"}) || p.Graphs[2].Overlay != (agr.Link{Base: 0, Shared: "y"}) {
		t.Fatalf("links %+v %+v %+v", p.Graphs[0].Overlay, p.Graphs[1].Overlay, p.Graphs[2].Overlay)
	}
	var b bytes.Buffer
	if err := agr.Write(&b, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "# plot-go: g1 overlays g0 shared x\n@g1 on\n") {
		t.Fatalf("link not written before its graph:\n%s", b.String())
	}
	q, err := agr.Parse(&b)
	if err != nil || len(q.Warnings) > 0 {
		t.Fatal(err, q.Warnings)
	}
	for i := range p.Graphs {
		if q.Graphs[i].Overlay != p.Graphs[i].Overlay {
			t.Fatalf("g%d read back %+v, want %+v", i, q.Graphs[i].Overlay, p.Graphs[i].Overlay)
		}
	}
	// unlinked, nothing is written
	p.Graphs[1].Overlay, p.Graphs[2].Overlay = agr.Link{}, agr.Link{}
	b.Reset()
	agr.Write(&b, p)
	if strings.Contains(b.String(), "overlays") {
		t.Fatal("an unlinked graph wrote a link")
	}
}

func TestOverlayLinkWarnings(t *testing.T) {
	for _, c := range []struct{ links, warn string }{
		{"# plot-go: g1 overlays g5 shared x", "does not have"},
		{"# plot-go: g7 overlays g0 shared x", "does not have"},
		{"# plot-go: g1 overlays g1 shared x", "itself"},
		{"# plot-go: g1 overlays g0 shared x\n# plot-go: g2 overlays g1 shared x", "overlay itself"},
		{"# plot-go: g1 overlays g0 shared x\n# plot-go: g1 overlays g2 shared y", "second"},
		{"# plot-go: g1 overlays g0 shared z", "invalid"},
	} {
		p, err := agr.Parse(strings.NewReader(twoGraphs + c.links + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0].Msg, c.warn) {
			t.Errorf("%q: warnings %v, want one with %q", c.links, p.Warnings, c.warn)
		}
		if p.Graphs[2].Overlay != (agr.Link{}) {
			t.Errorf("%q: g2 linked", c.links)
		}
	}
}
