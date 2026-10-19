package figure

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// overlayFigure is test3 with n-1 more graphs from the same plot, each
// set's y scaled by 10^k so the free axes differ.
func overlayFigure(t *testing.T, n int) *agr.Project {
	t.Helper()
	fig := FromPlot(load(t, "test3-1.agr"), "a/fit-curves-1.agr")
	for k := 1; k < n; k++ {
		id, err := AddGraph(fig, load(t, "test3-1.agr"), fmt.Sprintf("b%d/fit-curves-1.agr", k))
		if err != nil {
			t.Fatal(err)
		}
		g := graph(fig, id)
		f := math.Pow(10, float64(k))
		for _, s := range g.Sets {
			for _, row := range s.Data {
				for c := 1; c < len(row); c++ {
					row[c] *= f
				}
			}
		}
		g.World.YMin, g.World.YMax = g.World.YMin*f, g.World.YMax*f
		g.Y.Tick.Major *= f
	}
	return fig
}

func TestOverlayLinksAndPlaces(t *testing.T) {
	fig := overlayFigure(t, 3)
	if err := Layout(fig, "stacked"); err != nil {
		t.Fatal(err)
	}
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	b, o1, o2 := fig.Graphs[0], fig.Graphs[1], fig.Graphs[2]
	if o1.Overlay != (agr.Link{Base: 0, Shared: "x"}) || o2.Overlay != (agr.Link{Base: 0, Shared: "x"}) {
		t.Fatalf("links %+v %+v", o1.Overlay, o2.Overlay)
	}
	for _, o := range []*agr.Graph{o1, o2} {
		if o.View != b.View {
			t.Errorf("g%d at %+v, not over the base %+v", o.ID, o.View, b.View)
		}
		if o.X.On || o.Frame.LineStyle != 0 || o.Y.Tick.Place != "opposite" || o.Y.TickLabel.Place != "opposite" || !o.Y.Bar.On {
			t.Errorf("g%d: shared copy on %v, frame %d, free axis %s/%s bar %v", o.ID, o.X.On, o.Frame.LineStyle, o.Y.Tick.Place, o.Y.TickLabel.Place, o.Y.Bar.On)
		}
	}
	if b.Y.Tick.Place != "normal" {
		t.Errorf("base y ticks %s, want normal", b.Y.Tick.Place)
	}
	// the second free axis just beyond all the first draws
	d, _ := render.Render(fig)
	if r := reach(d, o1, "y", 0); o1.Y.Offset[1] != 0 || r < 0.08 || math.Abs(o2.Y.Offset[1]-r-AxisGap) > 1e-3 {
		t.Errorf("free axis offsets %v %v, first reaches %v", o1.Y.Offset, o2.Y.Offset, r)
	}
	// linked both ways
	o2.World.XMin, o2.World.XMax, o2.X.Scale, o2.X.Tick.Major = 0.5, 50, "Logarithmic", 10
	SyncLinks(fig, o2.ID, "x")
	for _, g := range fig.Graphs {
		if g.World.XMin != 0.5 || g.World.XMax != 50 || g.X.Scale != "Logarithmic" || g.X.Tick.Major != 10 {
			t.Errorf("g%d x not synced: %v..%v %s %v", g.ID, g.World.XMin, g.World.XMax, g.X.Scale, g.X.Tick.Major)
		}
	}
	// adding data to one graph grows the shared axis in all
	o1.Sets[0].Data = append(o1.Sets[0].Data, []float64{200, 0.05})
	if grown, _ := Extend(fig, o1.ID, []int{o1.Sets[0].ID}); !grown || b.World.XMax < 200 || o2.World.XMax != b.World.XMax {
		t.Errorf("extended %v: base x to %v, other overlay %v", grown, b.World.XMax, o2.World.XMax)
	}
	if o1.X.On || !b.X.On {
		t.Error("syncing switched a copy of the shared axis on or off")
	}
	b.World.XMin = 1
	SyncLinks(fig, b.ID, "x")
	if o1.World.XMin != 1 || o2.World.XMin != 1 {
		t.Error("base to overlays not synced")
	}
	// the free axes are not linked
	y := o1.World.YMax
	b.World.YMax = 123
	SyncLinks(fig, b.ID, "y")
	if o1.World.YMax != y {
		t.Error("a free axis was synced")
	}
	if err := AxisColor(fig, o1.ID, "x", 2); err != nil {
		t.Fatal(err)
	}
	if b.X.TickLabel.Color != 2 || o2.X.Bar.Color != 2 {
		t.Error("the shared axis color is not shared")
	}
	if err := AxisColor(fig, o1.ID, "y", 4); err != nil {
		t.Fatal(err)
	}
	if o1.Y.TickLabel.Color != 4 || b.Y.TickLabel.Color == 4 || o2.Y.TickLabel.Color == 4 {
		t.Error("a free axis color is not its own")
	}
	// the link is saved
	var w bytes.Buffer
	if err := agr.Write(&w, fig); err != nil {
		t.Fatal(err)
	}
	back, err := agr.Parse(&w)
	if err != nil || len(back.Warnings) > 0 {
		t.Fatal(err, back.Warnings)
	}
	if back.Graphs[2].Overlay != o2.Overlay {
		t.Errorf("read back %+v", back.Graphs[2].Overlay)
	}
}

func TestOverlayRefusals(t *testing.T) {
	fig := overlayFigure(t, 3)
	if Overlay(fig, 0, 0, "x") == nil || Overlay(fig, 0, 1, "z") == nil || Overlay(fig, 0, 9, "x") == nil {
		t.Error("a bad overlay was made")
	}
	if err := Overlay(fig, 0, 1, "x"); err != nil {
		t.Fatal(err)
	}
	if Overlay(fig, 1, 2, "x") == nil {
		t.Error("overlaid on an overlay")
	}
	if Overlay(fig, 2, 0, "x") == nil {
		t.Error("a base was overlaid")
	}
}

func TestOverlayShiftAndUnlink(t *testing.T) {
	fig := overlayFigure(t, 2)
	if err := OverlayAll(fig, "y"); err != nil {
		t.Fatal(err)
	}
	b, o := fig.Graphs[0], fig.Graphs[1]
	if o.X.Tick.Place != "opposite" || o.Y.On {
		t.Fatal("shared y: the free x axis is not on top")
	}
	label := b.Y.Label.Text
	if label == "" {
		t.Fatal("test3 has a y label")
	}
	if err := OverlayShift(fig, o.ID, 0.03); err != nil {
		t.Fatal(err)
	}
	if !o.Y.On || math.Abs(Shift(fig, o.ID)-0.03) > 1e-12 || o.View.YMin != b.View.YMin || math.Abs(b.View.XMin-o.View.XMin-0.03) > 1e-12 {
		t.Errorf("shifted to %+v from %+v, copy on %v", o.View, b.View, o.Y.On)
	}
	// the outer copy, the shifted one, has the numbers and the label
	if b.Y.TickLabel.On || b.Y.Label.Text != "" || !o.Y.TickLabel.On || o.Y.Label.Text != label {
		t.Errorf("shifted: base numbers %v label %q, overlay %v %q", b.Y.TickLabel.On, b.Y.Label.Text, o.Y.TickLabel.On, o.Y.Label.Text)
	}
	OverlayShift(fig, o.ID, 0)
	if !b.Y.TickLabel.On || b.Y.Label.Text != label || o.Y.On || o.Y.Label.Text != "" {
		t.Errorf("back over: base numbers %v label %q, overlay on %v %q", b.Y.TickLabel.On, b.Y.Label.Text, o.Y.On, o.Y.Label.Text)
	}
	OverlayShift(fig, o.ID, 0.03)
	if OverlayShift(fig, o.ID, -1) == nil || OverlayShift(fig, b.ID, 0.01) == nil {
		t.Error("a bad shift was made")
	}
	// a layout keeps the group together, the shift too
	if err := Layout(fig, "single"); err != nil {
		t.Fatal(err)
	}
	if math.Abs(Shift(fig, o.ID)-0.03) > 1e-12 || o.View.YMax != b.View.YMax {
		t.Errorf("after layout %+v base %+v", o.View, b.View)
	}
	if Arrangement(fig) != "single" {
		t.Errorf("arrangement %s", Arrangement(fig))
	}
	Unlink(fig, b.ID)
	if o.Overlay.Shared != "" || o.Frame.LineStyle != 1 || !o.Y.On || !o.Y.TickLabel.On {
		t.Errorf("unlinked %+v frame %d", o.Overlay, o.Frame.LineStyle)
	}
	if b.Y.Label.Text != label || o.Y.Label.Text != label {
		t.Errorf("labels after unlink: base %q, unlinked %q", b.Y.Label.Text, o.Y.Label.Text)
	}
	if o.X.TickLabel.Place != "normal" || o.X.Offset != [2]float64{} || o.X.Bar.On {
		t.Errorf("the unlinked graph's other axis is not back on its side: %s %v", o.X.TickLabel.Place, o.X.Offset)
	}
}

func TestRemoveOverlayBase(t *testing.T) {
	fig := overlayFigure(t, 3)
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	view := fig.Graphs[0].View
	if err := RemoveGraph(fig, 0); err != nil {
		t.Fatal(err)
	}
	nb, o := fig.Graphs[0], fig.Graphs[1]
	if nb.Overlay.Shared != "" || nb.Frame.LineStyle != 1 || !nb.X.On || nb.Y.TickLabel.Place != "normal" || nb.Y.Offset != [2]float64{} {
		t.Errorf("new base %+v frame %d x on %v y %s %v", nb.Overlay, nb.Frame.LineStyle, nb.X.On, nb.Y.TickLabel.Place, nb.Y.Offset)
	}
	if o.Overlay != (agr.Link{Base: nb.ID, Shared: "x"}) || o.Y.Offset[1] != 0 || o.View != nb.View {
		t.Errorf("other overlay %+v offset %v view %+v", o.Overlay, o.Y.Offset, o.View)
	}
	if nb.View == view {
		// "single" places the one panel left as OneFit's plot; the test
		// only needs it placed
		t.Log("same place")
	}
	if err := RemoveGraph(fig, o.ID); err != nil {
		t.Fatal(err)
	}
	if len(fig.Graphs) != 1 {
		t.Fatal(len(fig.Graphs))
	}
}

// TestOverlayMatchesGracebat draws overlays as Overlay makes them -
// two y axes, three (one moved out), a shifted one, and a shared y - with
// plot-go and with gracebat, and compares.
func TestOverlayMatchesGracebat(t *testing.T) {
	if _, err := exec.LookPath("gracebat"); err != nil {
		t.Skip("gracebat not installed")
	}
	for _, c := range []struct {
		name   string
		n      int
		shared string
		shift  float64
		legend bool
	}{
		{"two", 2, "x", 0, false}, {"three", 3, "x", 0, false}, {"shifted", 2, "x", 0.03, false}, {"shared-y", 3, "y", 0.02, false},
		{"legends", 3, "x", 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			fig := overlayFigure(t, c.n)
			for _, g := range fig.Graphs {
				if c.legend {
					g.Legend.On, g.Legend.LocType, g.Legend.X, g.Legend.Y = true, "view", 0.3, 0.8
					for _, s := range g.Sets {
						s.Legend = fmt.Sprintf("graph %d set %d", g.ID, s.ID)
					}
				}
			}
			if err := OverlayAll(fig, c.shared); err != nil {
				t.Fatal(err)
			}
			if c.shift > 0 {
				if err := OverlayShift(fig, 1, c.shift); err != nil {
					t.Fatal(err)
				}
			}
			if err := Layout(fig, "single"); err != nil {
				t.Fatal(err)
			}
			for k, g := range fig.Graphs[1:] {
				AxisColor(fig, g.ID, free(c.shared), []int{2, 4}[k])
			}
			var b bytes.Buffer
			if err := agr.Write(&b, fig); err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			in, out := filepath.Join(dir, "o.agr"), filepath.Join(dir, "o.eps")
			os.WriteFile(in, b.Bytes(), 0o644)
			if os.Getenv("PLOT_GO_KEEP") != "" {
				os.WriteFile(filepath.Join(os.Getenv("PLOT_GO_KEEP"), c.name+".agr"), b.Bytes(), 0o644)
			}
			if o, err := exec.Command("gracebat", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
				t.Fatalf("gracebat: %v %s", err, o)
			}
			r, err := os.Open(out)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			want, err := eps.Parse(r)
			if err != nil {
				t.Fatal(err)
			}
			got, warnings := render.Render(fig)
			for _, w := range warnings {
				t.Errorf("warning: %s", w)
			}
			for _, d := range drawcmp.Compare(got, want, 30) {
				t.Error(d)
			}
		})
	}
}

// TestOverlayFitsThePage: the free axes of an overlay, on the right or on
// top, stay on the page in every layout.
func TestOverlayFitsThePage(t *testing.T) {
	for _, shared := range []string{"x", "y"} {
		for _, layout := range Layouts {
			fig := overlayFigure(t, 3)
			if err := OverlayAll(fig, shared); err != nil {
				t.Fatal(err)
			}
			if err := Layout(fig, layout); err != nil {
				t.Fatal(err)
			}
			d, _ := render.Render(fig)
			const unit = 1000
			box := backend.BoundingBox(d, unit)
			if box.URX/unit > d.Width || box.URY/unit > d.Height {
				t.Errorf("shared %s, %s: drawn to %.3f, %.3f on a %.3f x %.3f page", shared, layout, box.URX/unit, box.URY/unit, d.Width, d.Height)
			}
		}
	}
}

// TestOverlayKeepsLabels: a shared axis shows its label once, but each
// graph gets it back when it leaves the group, however often it is
// overlaid again.
func TestOverlayKeepsLabels(t *testing.T) {
	fig := overlayFigure(t, 3)
	want := fig.Graphs[0].X.Label.Text
	if want == "" {
		t.Fatal("test3 has an x label")
	}
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	if err := OverlayShift(fig, 1, 0.03); err != nil {
		t.Fatal(err)
	}
	Unlink(fig, 1)
	if err := OverlayAll(fig, "y"); err != nil {
		t.Fatal(err)
	}
	for _, g := range fig.Graphs {
		if g.X.Label.Text != want {
			t.Errorf("g%d x label %q, want %q", g.ID, g.X.Label.Text, want)
		}
	}
}

// TestOverlayLegendsApart: overlaid, each legend goes below the one
// before, left edges aligned; legends already apart stay.
func TestOverlayLegendsApart(t *testing.T) {
	fig := overlayFigure(t, 3)
	for _, g := range fig.Graphs {
		g.Legend.On, g.Legend.LocType, g.Legend.X, g.Legend.Y = true, "view", 0.3, 0.8
		for _, s := range g.Sets {
			s.Legend = fmt.Sprintf("graph %d set %d", g.ID, s.ID)
		}
	}
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	d, _ := render.Render(fig)
	var boxes []backend.Box
	for _, g := range fig.Graphs {
		b, ok := part(d, fmt.Sprintf("g%d.legend", g.ID))
		if !ok {
			t.Fatalf("g%d draws no legend", g.ID)
		}
		boxes = append(boxes, b)
	}
	for i := 1; i < len(boxes); i++ {
		a, b := boxes[i-1], boxes[i]
		if math.Abs(b.LLX-a.LLX) > 2e-4 || math.Abs(a.LLY-LegendGap-b.URY) > 2e-4 {
			t.Errorf("legend %d at %+v, legend %d above at %+v", i, b, i-1, a)
		}
	}
	// moved apart by hand, a legend stays when overlaid again
	g2 := fig.Graphs[2]
	g2.Legend.X, g2.Legend.Y = 0.3, 0.3
	Unlink(fig, g2.ID)
	if err := Overlay(fig, 0, g2.ID, "x"); err != nil {
		t.Fatal(err)
	}
	if g2.Legend.X != 0.3 || g2.Legend.Y != 0.3 {
		t.Errorf("a legend apart was moved to %v, %v", g2.Legend.X, g2.Legend.Y)
	}
}

// TestCandidates: graphs at one place sharing an axis's range, without a
// link (as an overlay saved by Grace would be), are offered; Link adds only
// the link.
func TestCandidates(t *testing.T) {
	fig := overlayFigure(t, 3)
	if err := Layout(fig, "grid"); err != nil {
		t.Fatal(err)
	}
	if c := Candidates(fig); len(c) != 0 {
		t.Fatalf("separate panels offered: %+v", c)
	}
	g0, g1, g2 := fig.Graphs[0], fig.Graphs[1], fig.Graphs[2]
	g1.View, g2.View = g0.View, g0.View
	g1.Y.TickLabel.Place = "opposite"
	g2.World.XMax *= 2 // its y range is g0's x 100 too: nothing shared
	got := Candidates(fig)
	if len(got) != 1 || got[0] != (Candidate{Over: 1, Base: 0, Shared: "x"}) {
		t.Fatalf("candidates %+v", got)
	}
	// both axes the same: the one whose other axis is drawn opposite
	g2.World = g0.World
	g2.X.TickLabel.Place = "opposite"
	if got = Candidates(fig); len(got) != 2 || got[1] != (Candidate{Over: 2, Base: 0, Shared: "y"}) {
		t.Fatalf("candidates %+v", got)
	}
	frame := g1.Frame
	g1.X.Tick.Major = 0.05
	if err := Link(fig, 0, 1, "x"); err != nil {
		t.Fatal(err)
	}
	if g1.Overlay != (agr.Link{Base: 0, Shared: "x"}) || g1.Frame != frame || g1.X.Tick.Major != g0.X.Tick.Major || g1.Y.TickLabel.Place != "opposite" {
		t.Errorf("linked %+v frame %+v tick %v", g1.Overlay, g1.Frame, g1.X.Tick.Major)
	}
	if got = Candidates(fig); len(got) != 1 || got[0].Over != 2 {
		t.Errorf("after linking g1: %+v", got)
	}
	if Link(fig, 1, 2, "x") == nil || Link(fig, 0, 1, "x") == nil || Link(fig, 0, 2, "z") == nil {
		t.Error("a bad link was made")
	}
}

// TestOverlayLabelEdits: the shared axis's label edited on any copy - the
// one drawing it or one that does not - is the label; cleared where it is
// drawn, it is cleared.
func TestOverlayLabelEdits(t *testing.T) {
	edit := func(fig *agr.Project, g *agr.Graph, text string) {
		g.X.Label.Text = text
		SyncLinks(fig, g.ID, "x")
		ArrangeOverlays(fig)
	}
	shown := func(fig *agr.Project) (string, int) {
		text, n := "", 0
		for _, g := range fig.Graphs {
			if g.X.Label.Text != "" {
				text, n = g.X.Label.Text, n+1
			}
		}
		return text, n
	}
	for _, shift := range []float64{0, 0.03} {
		fig := overlayFigure(t, 3)
		if err := OverlayAll(fig, "x"); err != nil {
			t.Fatal(err)
		}
		if err := OverlayShift(fig, 1, shift); err != nil {
			t.Fatal(err)
		}
		carrier := fig.Graphs[0]
		if shift > 0 {
			carrier = fig.Graphs[1]
		}
		for _, g := range fig.Graphs {
			edit(fig, g, fmt.Sprintf("from g%d", g.ID))
			if text, n := shown(fig); n != 1 || text != fmt.Sprintf("from g%d", g.ID) || carrier.X.Label.Text != text {
				t.Errorf("shift %v, edited on g%d: %q on %d copies, carrier has %q", shift, g.ID, text, n, carrier.X.Label.Text)
			}
		}
		// other properties edited on a copy without the label keep it
		fig.Graphs[2].World.XMax *= 2
		SyncLinks(fig, 2, "x")
		ArrangeOverlays(fig)
		if text, _ := shown(fig); text != "from g2" {
			t.Errorf("shift %v: a range edit changed the label to %q", shift, text)
		}
		edit(fig, carrier, "")
		if text, n := shown(fig); n != 0 {
			t.Errorf("shift %v: cleared, %q is left", shift, text)
		}
	}
}

// TestOverlayMovesTitles: an overlaid graph's title and subtitle go to
// the base, after its own; the base draws them once.
func TestOverlayMovesTitles(t *testing.T) {
	fig := overlayFigure(t, 4)
	g := fig.Graphs
	g[0].Title.Text, g[0].Subtitle.Text = "zone12", ""
	g[1].Title.Text, g[1].Subtitle.Text = "zone13", "300 K"
	g[2].Title.Text, g[2].Subtitle.Text = "zone12", ""
	g[3].Title.Text, g[3].Subtitle.Text = "", "310 K"
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	if g[0].Title.Text != "zone12 / zone13" || g[0].Subtitle.Text != "300 K / 310 K" {
		t.Errorf("base title %q, subtitle %q", g[0].Title.Text, g[0].Subtitle.Text)
	}
	for _, o := range g[1:] {
		if o.Title.Text != "" || o.Subtitle.Text != "" {
			t.Errorf("g%d still draws %q / %q", o.ID, o.Title.Text, o.Subtitle.Text)
		}
	}
}

// TestOverlayArrangeAfterResize: when what a free axis draws grows, the
// free axes beyond it move out, each just beyond the one before as drawn
// now - one ArrangeOverlays, with three overlays on one side, so the third
// is placed by a second whose offset changed in the same call.
func TestOverlayArrangeAfterResize(t *testing.T) {
	fig := overlayFigure(t, 4)
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	fig.Graphs[1].Y.TickLabel.CharSize *= 3
	ArrangeOverlays(fig)
	d, _ := render.Render(fig)
	for k := 2; k < 4; k++ {
		prev, o := fig.Graphs[k-1], fig.Graphs[k]
		want := prev.Y.Offset[1] + reach(d, prev, "y", prev.Y.Offset[1]) + AxisGap
		if math.Abs(o.Y.Offset[1]-want) > 1e-3 {
			t.Errorf("g%d free axis at %.4f, want %.4f (just beyond g%d)", o.ID, o.Y.Offset[1], want, prev.ID)
		}
	}
}

// TestOverlayShareLabel: a label edited on any copy of a shared axis is
// the group's - typed or cleared, on the copy that draws it or not.
func TestOverlayShareLabel(t *testing.T) {
	fig := overlayFigure(t, 3)
	if err := OverlayAll(fig, "x"); err != nil {
		t.Fatal(err)
	}
	set := func(g *agr.Graph, text string) {
		g.X.Label.Text = text
		ShareLabel(fig, g.ID, "x")
		ArrangeOverlays(fig)
	}
	set(fig.Graphs[0], "time (s)")
	for _, g := range fig.Graphs[1:] {
		set(fig.Graphs[0], "time (s)")
		set(g, "")
		for _, h := range fig.Graphs {
			if h.X.Label.Text != "" {
				t.Errorf("cleared on g%d, g%d still has %q", g.ID, h.ID, h.X.Label.Text)
			}
		}
	}
	set(fig.Graphs[2], "t")
	if fig.Graphs[0].X.Label.Text != "t" {
		t.Errorf("typed on g2: the base draws %q", fig.Graphs[0].X.Label.Text)
	}
}

// TestCandidatesInGraceFiles: an overlay in Grace's format without a link
// (written by hand; testdata/overlay/README.md) is found as read, as
// gracebat saved it, and as the installed gracebat saves it now; linking
// it leaves the drawing as it is.
func TestCandidatesInGraceFiles(t *testing.T) {
	files := []string{"../testdata/overlay/grace-overlay.agr", "../testdata/overlay/grace-overlay-saved.agr"}
	if _, err := exec.LookPath("gracebat"); err == nil {
		dir := t.TempDir()
		saved := filepath.Join(dir, "saved.agr")
		if o, err := exec.Command("gracebat", "-nosafe", "-hdevice", "EPS", "-printfile", filepath.Join(dir, "out.eps"), files[0], "-saveall", saved).CombinedOutput(); err != nil {
			t.Fatalf("gracebat: %v %s", err, o)
		}
		files = append(files, saved)
	}
	for _, f := range files {
		p, err := agr.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Graphs) != 2 {
			t.Fatalf("%s: %d graphs", f, len(p.Graphs))
		}
		if got := Candidates(p); len(got) != 1 || got[0] != (Candidate{Over: 1, Base: 0, Shared: "x"}) {
			t.Errorf("%s: candidates %+v", f, got)
		}
		before, _ := render.Render(p)
		if err := Link(p, 0, 1, "x"); err != nil {
			t.Fatal(err)
		}
		after, _ := render.Render(p)
		for _, d := range drawcmp.Compare(after, before, 5) {
			t.Errorf("%s: linking changed the drawing: %s", f, d)
		}
		if len(Candidates(p)) != 0 {
			t.Errorf("%s: a candidate is left", f)
		}
		// both copies with a label, as a file may have them: cleared where
		// it is drawn, the label is cleared; typed on the other, it is it
		p.Graphs[1].X.Label.Text = "x too"
		p.Graphs[0].X.Label.Text = ""
		SyncLinks(p, 0, "x")
		ArrangeOverlays(p)
		if p.Graphs[0].X.Label.Text != "" || p.Graphs[1].X.Label.Text != "" {
			t.Errorf("%s: cleared on the base, the labels are %q and %q", f, p.Graphs[0].X.Label.Text, p.Graphs[1].X.Label.Text)
		}
		p.Graphs[1].X.Label.Text = "typed"
		SyncLinks(p, 1, "x")
		ArrangeOverlays(p)
		if p.Graphs[0].X.Label.Text != "typed" || p.Graphs[1].X.Label.Text != "" {
			t.Errorf("%s: typed on the overlay, the labels are %q and %q", f, p.Graphs[0].X.Label.Text, p.Graphs[1].X.Label.Text)
		}
	}
}
