package render

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/drawcmp"
	"github.com/fitteia/OneFit-Engine-plot/internal/eps"
)

// The style-text corpus files are captured from Grace and compared
// primitive by primitive and as raster images by the existing suites.
func TestTextEscapeCorpusHasNoWarnings(t *testing.T) {
	for _, name := range []string{"controls", "decorations", "decoration-rotation", "transforms", "flow", "kerning"} {
		p, err := agr.ParseFile("../testdata/corpus/style-text-" + name + ".agr")
		if err != nil {
			t.Fatal(err)
		}
		d, warnings := Render(p)
		if len(warnings) != 0 || len(d.Texts) == 0 {
			t.Fatalf("%s: %v, %d texts", name, warnings, len(d.Texts))
		}
		for _, path := range d.Paths[1:] {
			if !strings.HasPrefix(path.ID, "string.") {
				t.Errorf("decoration lost editable text ID: %q", path.ID)
			}
		}
	}
}

func TestMalformedTextControlsWarnWithoutPrintingArguments(t *testing.T) {
	for _, text := range []string{`A\z{bad}B`, `A\z{NaN}B`, `A\z{Inf}B`, `A\#{oops}B`, `A\t{1 2 3}B`} {
		r := &renderer{proj: &agr.Project{}}
		ts := r.layout(text, 0, 0.028)
		var got strings.Builder
		for _, rn := range ts.runs {
			got.WriteString(rn.str)
			if math.IsNaN(rn.size) || math.IsInf(rn.size, 0) {
				t.Fatalf("%q non-finite size", text)
			}
		}
		if got.String() != "AB" || len(r.warnings) != 1 {
			t.Errorf("%q: text %q, warnings %v", text, got.String(), r.warnings)
		}
	}
	r := &renderer{proj: &agr.Project{}}
	ts := r.layout(`A\z{unfinished`, 0, 0.028)
	if len(r.warnings) != 1 || len(ts.runs) != 1 || ts.runs[0].str != "A" {
		t.Fatalf("unterminated control: %v %v", ts, r.warnings)
	}
}

func TestTextStyleDoesNotLeakBetweenLabels(t *testing.T) {
	r := &renderer{proj: &agr.Project{}}
	r.layout(`\u\o\z{2}\V{1}AB`, 0, 0.028)
	ts := r.layout("CD", 0, 0.028)
	if len(ts.runs) != 1 || ts.runs[0].under || ts.runs[0].over || ts.runs[0].size != 0.028 || ts.runs[0].y != 0 {
		t.Fatalf("label inherits previous style: %+v", ts.runs)
	}
}

func TestOverlineClearsGlyphsAndUnderlineStaysBelow(t *testing.T) {
	for _, font := range []string{"Helvetica", "Helvetica-Bold", "Times-Roman", "Courier"} {
		t.Run(font, func(t *testing.T) {
			r := &renderer{proj: &agr.Project{Fonts: map[int]agr.Font{0: {Name: font}}}}
			for _, text := range []string{"CO2", "hello", "gyp"} {
				ts := r.layout(`\o`+text+`\O`, 0, 0.028)
				if len(ts.runs) != 1 || !ts.runs[0].over {
					t.Fatalf("%q did not produce one overlined run: %+v", text, ts.runs)
				}
				run := ts.runs[0]
				glyphTop := r.metrics(run.font).Ink(run.str).URY*run.size + run.y
				if ts.ink.URY < glyphTop+0.03*run.size {
					t.Errorf("%q overline is not clear of glyphs: ink top %.5f, glyph top %.5f", text, ts.ink.URY, glyphTop)
				}
			}
			ts := r.layout(`\uH2O\U`, 0, 0.028)
			glyphBottom := r.metrics(font).Ink("H2O").LLY * 0.028
			if ts.ink.LLY >= glyphBottom {
				t.Errorf("underline is not below glyphs: ink bottom %.5f, glyph bottom %.5f", ts.ink.LLY, glyphBottom)
			}
		})
	}
}

// The typesetting grid is in physical pixels, not a fixed viewport step.
func TestDecorationsOnDifferentPagesMatchGrace(t *testing.T) {
	if _, err := exec.LookPath("gracebat"); err != nil {
		t.Skip("gracebat not installed")
	}
	for _, page := range [][2]int{{773, 600}, {600, 400}, {900, 700}} {
		t.Run(fmt.Sprint(page), func(t *testing.T) {
			src := fmt.Sprintf("@version 50125\n@page size %d, %d\n@map font 0 to \"Helvetica\", \"Helvetica\"\n@page background fill off\n@g0 hidden true\n@with string\n@string on\n@string loctype view\n@string 0.2, 0.5\n@string font 0\n@string rot 30\n@string def \"A\\uBC\\U D\\oEF\\O\"\n", page[0], page[1])
			dir := t.TempDir()
			in, out := filepath.Join(dir, "in.agr"), filepath.Join(dir, "out.eps")
			if err := os.WriteFile(in, []byte(src), 0600); err != nil {
				t.Fatal(err)
			}
			if log, err := exec.Command("gracebat", "-hdevice", "EPS", "-printfile", out, in).CombinedOutput(); err != nil {
				t.Fatalf("Grace: %v %s", err, log)
			}
			p, err := agr.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatal(err)
			}
			got, w := Render(p)
			if len(w) != 0 {
				t.Fatal(w)
			}
			f, err := os.Open(out)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			want, err := eps.Parse(f)
			if err != nil {
				t.Fatal(err)
			}
			alignTextDecorationPaths(got, want)
			if diff := drawcmp.Compare(got, want, 10); len(diff) > 0 {
				t.Fatal(diff)
			}
		})
	}
}
