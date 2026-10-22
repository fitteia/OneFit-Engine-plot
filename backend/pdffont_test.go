package backend

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	pdraw "github.com/fitteia/OneFit-Engine-plot/draw"
	"github.com/fitteia/OneFit-Engine-plot/internal/glyph"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// With the URW base-35 fonts installed the standard fonts a PDF uses are
// embedded, as subsets, with their Unicode text.
func TestPDFEmbedsStandardFonts(t *testing.T) {
	if _, _, ok := glyph.URWGlyph("Helvetica", 'A'); !ok {
		t.Skip("URW base-35 fonts not installed")
	}
	src := "@version 50125\n@page size 773, 600\n@g0 hidden true\n" +
		"@map font 0 to \"Times-Roman\", \"Times-Roman\"\n@map font 4 to \"Helvetica\", \"Helvetica\"\n@map font 12 to \"Symbol\", \"Symbol\"\n" +
		"@with string\n@string on\n@string 0.2, 0.5\n@string font 4\n@string char size 2\n" +
		"@string def \"T1 = 2.5 \\#{b5}s, \\#{e9}t\\#{e9} \\x\\#{61}\\f{} \\f{Times-Roman}fit\"\n"
	p, err := agr.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	d, warnings := render.Render(p)
	if len(warnings) != 0 {
		t.Fatal(warnings)
	}
	var b bytes.Buffer
	if err := WritePDF(&b, d, p.PageWidth, p.PageHeight, "fonts"); err != nil {
		t.Fatal(err)
	}
	pdf := b.String()
	checkFormStreams(t, d, p.PageWidth, p.PageHeight)
	for _, name := range []string{"Helvetica", "Symbol", "Times-Roman"} {
		if !regexp.MustCompile(`/BaseFont /[A-Z]{6}\+` + name + ` `).MatchString(pdf) {
			t.Errorf("%s not embedded as a subset", name)
		}
	}
	if n := strings.Count(pdf, "/FontFile3"); n != 3 {
		t.Errorf("%d font programs, want 3", n)
	}
	if n := strings.Count(pdf, "/Subtype /Type1C"); n != 3 {
		t.Errorf("%d Type1C programs, want 3", n)
	}
	if n := strings.Count(pdf, "/ToUnicode"); n != 3 {
		t.Errorf("%d Unicode maps, want 3", n)
	}

	file := filepath.Join(t.TempDir(), "fonts.pdf")
	if err := os.WriteFile(file, b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("pdffonts"); err == nil {
		out, err := exec.Command("pdffonts", file).CombinedOutput()
		if err != nil {
			t.Fatalf("pdffonts: %v %s", err, out)
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")[2:]
		if len(lines) != 3 {
			t.Errorf("pdffonts:\n%s", out)
		}
		for _, l := range lines {
			if !regexp.MustCompile(`Type 1C +Builtin +yes yes yes`).MatchString(l) {
				t.Errorf("not embedded with Unicode: %s", l)
			}
		}
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command("pdftotext", file, "-").CombinedOutput()
		if err != nil || !strings.Contains(string(out), "T1 = 2.5 µs, été") || !strings.Contains(string(out), "fit") {
			t.Errorf("text: %v %q", err, out)
		}
	}
}

// A form (a plot placed in a Figures page or a report) renumbers its
// objects' references, but never a stream's bytes: a compressed font
// program can hold "7 0 R" by chance.
func checkFormStreams(t *testing.T, d *pdraw.Drawing, w, h float64) {
	t.Helper()
	b, err := buildPDF(d, w, h, "", PDFOptions{})
	if err != nil {
		t.Fatal(err)
	}
	const first = 100
	f, err := WritePDFForm(d, w, h, PDFOptions{}, first)
	if err != nil {
		t.Fatal(err)
	}
	stream := func(o string) string {
		if i := strings.Index(o, "\nstream\n"); i >= 0 {
			return o[i:]
		}
		return ""
	}
	programs := 0
	for i, o := range b.objs[5:] {
		if stream(f.Objects[i+1]) != stream(o) {
			t.Errorf("object %d's stream changed in the form", i+6)
		}
		if strings.Contains(o, "/Subtype /Type1C") {
			programs++
			ref := fmt.Sprintf("/FontFile3 %d 0 R", first+1+i)
			if !strings.Contains(strings.Join(f.Objects, "\n"), ref) {
				t.Errorf("no %s in the form", ref)
			}
		}
	}
	if programs == 0 {
		t.Error("no font program in the form")
	}
}
