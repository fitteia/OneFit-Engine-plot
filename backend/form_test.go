package backend

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// formDocument is a one-page PDF whose page is the form, placed at its
// origin: what a document placing a plot does.
func formDocument(f PDFForm) []byte {
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources << /XObject << /Plot %d 0 R >> >> /Contents 4 0 R >>", num(f.Width), num(f.Height), f.First),
		pdfStream("/Plot Do\n"),
	}
	objs = append(objs, f.Objects...)
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes()
}

func rasterizePDF(t *testing.T, pdf []byte) image.Image {
	t.Helper()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in.pdf"), filepath.Join(dir, "out.png")
	if err := os.WriteFile(in, pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := exec.Command("gs", "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=png16m", "-r144",
		"-dTextAlphaBits=4", "-dGraphicsAlphaBits=4", "-sOutputFile="+out, in).CombinedOutput(); err != nil {
		t.Fatalf("gs: %v\n%s", err, b)
	}
	return readPNG(t, out)
}

// A plot placed as a form draws as its own PDF does: OneFit's symbols
// (ellipse forms), fill patterns, the Symbol font and an embedded font.
func TestPDFFormDrawsAsItsPDF(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("gs not installed")
	}
	cases := map[string]string{}
	for _, name := range []string{"p96-10-1.agr", "style-patterns-a.agr", "style-avalue-symbol.agr"} {
		b, err := os.ReadFile(filepath.Join("..", "testdata", "corpus", name))
		if err != nil {
			t.Fatal(err)
		}
		cases[name] = string(b)
	}
	if _, err := afm.Get("URWGothic-Book"); err == nil {
		cases["embedded font"] = "@version 50125\n@page size 773,600\n@g0 hidden true\n@map font 20 to \"URWGothic-Book\", \"URWGothic-Book\"\n@with string\n@string on\n@string loctype view\n@string 0.3, 0.5\n@string font 20\n@string char size 2\n@string def \"AVATAR \\#{e9}\"\n"
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := agr.Parse(strings.NewReader(src))
			if err != nil {
				t.Fatal(err)
			}
			d, _ := render.Render(p)
			var own bytes.Buffer
			if err := WritePDF(&own, d, p.PageWidth, p.PageHeight, ""); err != nil {
				t.Fatal(err)
			}
			f, err := WritePDFForm(d, p.PageWidth, p.PageHeight, PDFOptions{}, 5)
			if err != nil {
				t.Fatal(err)
			}
			if f.First != 5 || len(f.Objects) == 0 || !strings.HasPrefix(f.Objects[0], "<< /Type /XObject /Subtype /Form") {
				t.Fatalf("form: first %d, %d objects", f.First, len(f.Objects))
			}
			a, b := rasterizePDF(t, own.Bytes()), rasterizePDF(t, formDocument(f))
			if a.Bounds() != b.Bounds() {
				t.Fatalf("size %v, placed %v", a.Bounds(), b.Bounds())
			}
			diff := 0
			for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
				for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
					if a.At(x, y) != b.At(x, y) {
						diff++
					}
				}
			}
			if diff > 0 {
				t.Errorf("%d pixels differ", diff)
			}
		})
	}
}
