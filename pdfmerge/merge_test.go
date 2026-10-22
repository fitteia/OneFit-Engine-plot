package pdfmerge

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/backend"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// plotPDF is a corpus plot as plot-go writes it: PDF 1.4, an xref table.
func plotPDF(t *testing.T, file string) []byte {
	t.Helper()
	p, err := agr.ParseFile(filepath.Join("..", "testdata", "corpus", file))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := render.Render(p)
	var b bytes.Buffer
	if err := backend.WritePDF(&b, d, p.PageWidth, p.PageHeight, file); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// gsPDF is a plot made the way OneFit makes fit-curves-N.pdf (plot-go's
// EPS through Ghostscript's pdfwrite): PDF 1.5, its objects in an object
// stream, an xref stream.
func gsPDF(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", "pdfmerge", "gs-objstm.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func merge(t *testing.T, pdfs ...[]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := Merge(&b, pdfs...); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// content is page i's decoded content, to compare a copied page with its
// source.
func content(t *testing.T, pdf []byte, i int) []byte {
	t.Helper()
	d, err := parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := d.pages()
	if err != nil {
		t.Fatal(err)
	}
	var out []byte
	add := func(v object) {
		if s, ok := d.resolve(v).(*stream); ok {
			b, err := decode(s)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, b...)
		}
	}
	c := d.resolve(pages[i].d.get("Contents"))
	if a, ok := c.(array); ok {
		for _, v := range a {
			add(v)
		}
	} else {
		add(pages[i].d.get("Contents"))
	}
	return out
}

func TestMergeKeepsEveryPage(t *testing.T) {
	a, b := gsPDF(t), plotPDF(t, "style-avalue-1.agr")
	out := merge(t, a, b, a)
	if n, err := Pages(out); err != nil || n != 3 {
		t.Fatalf("pages = %d, %v; want 3", n, err)
	}
	for i, src := range [][]byte{a, b, a} {
		got, want := content(t, out, i), content(t, src, 0)
		if len(want) == 0 || !bytes.Equal(got, want) {
			t.Errorf("page %d: content differs from its source (%d bytes, want %d)", i+1, len(got), len(want))
		}
	}
	if !bytes.HasPrefix(out, []byte("%PDF-1.5\n")) {
		t.Errorf("header %q: the newest input's version, 1.5, expected", out[:9])
	}
	// The merged file is plain: one xref table, every offset right.
	d, err := parse(out)
	if err != nil {
		t.Fatal(err)
	}
	for n := range d.xref {
		if _, err := d.get(n); err != nil {
			t.Fatal(err)
		}
	}
	if d.rebuilt {
		t.Error("the merged file's own xref table was wrong")
	}
}

// TestMergeReadByOtherTools checks the result with every PDF reader at
// hand (each is skipped when not installed).
func TestMergeReadByOtherTools(t *testing.T) {
	out := filepath.Join(t.TempDir(), "All.pdf")
	if err := os.WriteFile(out, merge(t, gsPDF(t), plotPDF(t, "p96-10-1.agr")), 0o666); err != nil {
		t.Fatal(err)
	}
	for _, c := range [][]string{
		{"pdfinfo", out},
		{"mutool", "info", out},
		{"gs", "-q", "-dSAFER", "-dNOPAUSE", "-dBATCH", "-sDEVICE=nullpage", out},
		{"qpdf", "--check", out},
	} {
		if _, err := exec.LookPath(c[0]); err != nil {
			continue
		}
		b, err := exec.Command(c[0], c[1:]...).CombinedOutput()
		if err != nil || bytes.Contains(bytes.ToLower(b), []byte("error")) || bytes.Contains(bytes.ToLower(b), []byte("warning")) {
			t.Errorf("%s: %v\n%s", c[0], err, b)
		}
		if c[0] == "pdfinfo" && !bytes.Contains(b, []byte("Pages:           2")) {
			t.Errorf("pdfinfo does not see 2 pages:\n%s", b)
		}
	}
}

// build writes a PDF from object bodies (object i+1 is objs[i]): with an
// xref table, or with an xref stream (PNG Up predictor, as many writers
// use) and the objects listed in packed inside an object stream.
func build(objs []string, xrefStream bool, packed map[int]bool) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n")
	n := len(objs)
	offs := make([]int, n+3)
	var stmBody, stmHead bytes.Buffer
	stmIdx := map[int]int{}
	for i, body := range objs {
		if packed[i+1] {
			stmIdx[i+1] = len(stmIdx)
			fmt.Fprintf(&stmHead, "%d %d ", i+1, stmBody.Len())
			stmBody.WriteString(body + "\n")
			continue
		}
		offs[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	stm := n + 1
	if len(packed) > 0 {
		offs[stm] = b.Len()
		data := stmHead.String() + stmBody.String()
		fmt.Fprintf(&b, "%d 0 obj\n<< /Type /ObjStm /N %d /First %d /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			stm, len(packed), stmHead.Len(), len(data), data)
	}
	if !xrefStream {
		start := b.Len()
		fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", n+1)
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "%010d 00000 n \n", offs[i])
		}
		fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", n+1, start)
		return b.Bytes()
	}
	x := n + 2
	offs[x] = b.Len()
	var rows bytes.Buffer
	prev := make([]byte, 4)
	for i := 0; i <= x; i++ {
		row := []byte{0, 0, 0, 0}
		switch {
		case i == 0:
		case packed[i]:
			row = []byte{2, 0, byte(stm), byte(stmIdx[i])}
		default:
			row = []byte{1, byte(offs[i] >> 8), byte(offs[i]), 0}
		}
		rows.WriteByte(2) // Up: each byte less the one above it
		for j := range row {
			rows.WriteByte(row[j] - prev[j])
		}
		prev = row
	}
	var z bytes.Buffer
	zw := zlib.NewWriter(&z)
	zw.Write(rows.Bytes())
	zw.Close()
	fmt.Fprintf(&b, "%d 0 obj\n<< /Type /XRef /Size %d /Root 1 0 R /W [1 2 1] /Filter /FlateDecode /DecodeParms << /Predictor 12 /Columns 4 >> /Length %d >>\nstream\n",
		x, x+1, z.Len())
	b.Write(z.Bytes())
	fmt.Fprintf(&b, "\nendstream\nendobj\nstartxref\n%d\n%%%%EOF\n", offs[x])
	return b.Bytes()
}

// treePDF has two pages under an inner page-tree node; the page size, the
// rotation and the resources are inherited, from two levels.
var treePDF = []string{
	"<< /Type /Catalog /Pages 2 0 R /Outlines 9 0 R >>",
	"<< /Type /Pages /Kids [3 0 R] /Count 2 /MediaBox [0 0 200 100] /Resources 6 0 R >>",
	"<< /Type /Pages /Parent 2 0 R /Kids [4 0 R 5 0 R] /Count 2 /Rotate 90 >>",
	"<< /Type /Page /Parent 3 0 R /Contents 7 0 R >>",
	"<< /Type /Page /Parent 3 0 R /Contents 8 0 R /MediaBox [0 0 50 50] /Annots [<< /Type /Annot /Subtype /Link /Rect [0 0 1 1] /Dest [4 0 R /Fit] >>] >>",
	"<< /Font << >> >>",
	"<< /Length 9 >>\nstream\n0 0 m S\n\nendstream",
	"<< /Length 4 >>\nstream\nq Q\nendstream",
	"<< /Type /Outlines /Count 0 >>",
}

func TestMergeInheritsFromThePageTree(t *testing.T) {
	for _, xs := range []bool{false, true} {
		packed := map[int]bool{}
		if xs {
			packed = map[int]bool{2: true, 3: true, 4: true, 6: true}
		}
		src := build(treePDF, xs, packed)
		out := merge(t, src)
		d, err := parse(out)
		if err != nil {
			t.Fatal(err)
		}
		if d.rebuilt {
			t.Fatalf("xref stream %v: the source's cross-reference data was not read", xs)
		}
		pages, err := d.pages()
		if err != nil || len(pages) != 2 {
			t.Fatalf("pages: %v %v", len(pages), err)
		}
		box := func(p page) string {
			var b bytes.Buffer
			(&copier{d: d}).value(&b, d.resolve(p.d.get("MediaBox")))
			return b.String()
		}
		if got := box(pages[0]); got != "[0 0 200 100]" {
			t.Errorf("page 1 MediaBox %s, want the tree's [0 0 200 100]", got)
		}
		if got := box(pages[1]); got != "[0 0 50 50]" {
			t.Errorf("page 2 MediaBox %s, want its own [0 0 50 50]", got)
		}
		for i, p := range pages {
			if p.d.get("Rotate") != number("90") || p.d.get("Resources") == nil {
				t.Errorf("page %d: Rotate %v, Resources %v - both inherited", i+1, p.d.get("Rotate"), p.d.get("Resources"))
			}
		}
		// The link's destination is still the first page.
		annots := d.resolve(pages[1].d.get("Annots")).(array)
		dest := d.resolve(annots[0]).(*dict).get("Dest").(array)
		if dest[0] != (ref{pages[0].num, 0}) {
			t.Errorf("link destination %v, want page 1 (object %d)", dest[0], pages[0].num)
		}
		if got := string(content(t, out, 0)); got != "0 0 m S\n\n" { // its /Length 9 counts both line ends
			t.Errorf("page 1 content %q", got)
		}
		if bytes.Contains(out, []byte("/Outlines")) {
			t.Error("outlines copied")
		}
	}
}

// TestMergeFindsObjectsWhenOffsetsAreWrong: a file edited by hand (lines
// added) has every xref offset wrong; the objects are found by scanning.
func TestMergeFindsObjectsWhenOffsetsAreWrong(t *testing.T) {
	src := plotPDF(t, "style-avalue-log.agr")
	i := bytes.Index(src, []byte("1 0 obj"))
	broken := append(append(append([]byte{}, src[:i]...), strings.Repeat("% an added line\n", 3)...), src[i:]...)
	for _, b := range [][]byte{broken, src[:len(src)-30]} { // shifted; no startxref at all
		out := merge(t, b)
		if !bytes.Equal(content(t, out, 0), content(t, src, 0)) {
			t.Error("page content lost")
		}
	}
}

func TestMergeRefuses(t *testing.T) {
	enc := build([]string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [] /Count 0 >>", "<< /Filter /Standard >>"}, false, nil)
	enc = bytes.Replace(enc, []byte("/Root 1 0 R"), []byte("/Root 1 0 R /Encrypt 3 0 R"), 1)
	for _, c := range []struct {
		name string
		in   [][]byte
		want string
	}{
		{"nothing", nil, "no PDF files"},
		{"not a PDF", [][]byte{[]byte("hello")}, "not a PDF"},
		{"encrypted", [][]byte{enc}, "encrypted"},
	} {
		err := Merge(&bytes.Buffer{}, c.in...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", c.name, err, c.want)
		}
	}
}

func TestMergeFilesMayReplaceAnInput(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.pdf"), filepath.Join(dir, "b.pdf")
	os.WriteFile(a, gsPDF(t), 0o666)
	os.WriteFile(b, plotPDF(t, "style-avalue-2.agr"), 0o666)
	var out bytes.Buffer
	if err := MergeFiles(&out, a, b); err != nil {
		t.Fatal(err)
	}
	if n, _ := Pages(out.Bytes()); n != 2 {
		t.Fatalf("pages %d", n)
	}
	if err := MergeFiles(&out, a, filepath.Join(dir, "missing.pdf")); err == nil || !strings.Contains(err.Error(), "missing.pdf") {
		t.Errorf("missing input: %v", err)
	}
}

// FuzzMerge: whatever the bytes, Merge returns (an error or a PDF), never
// panics. go test -fuzz=FuzzMerge ./pdfmerge runs it beyond the seeds.
func FuzzMerge(f *testing.F) {
	f.Add(build(treePDF, false, nil))
	f.Add(build(treePDF, true, map[int]bool{2: true, 3: true, 4: true, 6: true}))
	if b, err := os.ReadFile(filepath.Join("..", "testdata", "pdfmerge", "gs-objstm.pdf")); err == nil {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		Merge(&bytes.Buffer{}, b)
	})
}
