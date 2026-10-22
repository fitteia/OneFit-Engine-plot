// Package pdfmerge joins PDF files into one, page after page, as
// `pdftk A.pdf B.pdf cat output All.pdf` does, and splits one into a PDF
// per page, as `pdftk A.pdf burst` does - with the standard library only,
// so OneFit needs no PDF tool for either.
//
// It reads PDF 1.0 to 1.7 and 2.0 files: cross-reference tables or
// streams (and hybrids), compressed object streams, incremental updates,
// and files whose cross-reference data is wrong, whose objects it then
// finds by scanning. Each page is copied with what it uses (contents,
// fonts, images, annotations) and the attributes it inherits from its
// page tree; streams keep their own encoding. Document-level parts
// (outlines, forms, the structure tree) are not copied, as pdftk's cat
// also leaves outlines out. Encrypted files are refused.
package pdfmerge

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
)

// Merge writes to w one PDF holding every page of pdfs, in order.
func Merge(w io.Writer, pdfs ...[]byte) error {
	if len(pdfs) == 0 {
		return errors.New("no PDF files to merge")
	}
	docs := make([]*doc, len(pdfs))
	for i, b := range pdfs {
		d, err := parse(b)
		if err != nil {
			return fmt.Errorf("PDF %d: %w", i+1, err)
		}
		docs[i] = d
	}
	return write(w, parts(docs), nil)
}

// Split returns each page of pdf as a PDF of its own, in order.
func Split(pdf []byte) ([][]byte, error) {
	d, err := parse(pdf)
	if err != nil {
		return nil, err
	}
	pages, err := d.pages()
	if err != nil {
		return nil, err
	}
	out := make([][]byte, len(pages))
	for i := range pages {
		var b bytes.Buffer
		if err := write(&b, []part{{d, []int{i}}}, nil); err != nil {
			return nil, err
		}
		out[i] = b.Bytes()
	}
	return out, nil
}

// part is the pages of one input to write: all of them when pages is nil.
type part struct {
	d     *doc
	pages []int
}

func parts(docs []*doc) []part {
	ps := make([]part, len(docs))
	for i, d := range docs {
		ps[i] = part{d: d}
	}
	return ps
}

// MergeFiles is Merge for files: it reads every one before writing, so
// the output may replace one of them.
func MergeFiles(w io.Writer, paths ...string) error {
	if len(paths) == 0 {
		return errors.New("no PDF files to merge")
	}
	docs := make([]*doc, len(paths))
	for i, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if docs[i], err = parse(b); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return write(w, parts(docs), paths)
}

// Pages counts the pages of a PDF.
func Pages(b []byte) (int, error) {
	d, err := parse(b)
	if err != nil {
		return 0, err
	}
	pages, err := d.pages()
	return len(pages), err
}

// inherited are the page attributes a page may take from its page tree.
var inherited = []name{"Resources", "MediaBox", "CropBox", "Rotate"}

// page is one page of a doc: its object number (0 when it is written
// inline in /Kids) and its dictionary with the inherited attributes in it.
type page struct {
	num int
	d   *dict
}

// pages lists the document's pages in order, and the numbers of its page
// tree's inner nodes.
func (d *doc) pages() ([]page, error) {
	root, err := d.dictOf(d.trailer.get("Root"))
	if err != nil || root == nil {
		return nil, errors.New("no document catalog")
	}
	var out []page
	seen := map[int]bool{}
	var walk func(v object, inh map[name]object, depth int) error
	walk = func(v object, inh map[name]object, depth int) error {
		if depth > 100 {
			return errors.New("page tree nested too deeply")
		}
		num := 0
		if r, ok := v.(ref); ok {
			if seen[r.num] {
				return nil // a loop in a broken page tree
			}
			seen[r.num] = true
			num = r.num
		}
		node, err := d.dictOf(v)
		if err != nil {
			return err
		}
		if node == nil {
			return nil
		}
		kids, isTree := d.resolve(node.get("Kids")).(array)
		if !isTree && node.get("Type") != name("Pages") {
			p := &dict{keys: append([]name(nil), node.keys...), vals: append([]object(nil), node.vals...)}
			for _, k := range inherited {
				if p.get(k) == nil && inh[k] != nil {
					p.set(k, inh[k])
				}
			}
			out = append(out, page{num, p})
			return nil
		}
		next := map[name]object{}
		for k, v := range inh {
			next[k] = v
		}
		for _, k := range inherited {
			if v := node.get(k); v != nil {
				next[k] = v
			}
		}
		for _, kid := range kids {
			if err := walk(kid, next, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root.get("Pages"), map[name]object{}, 0); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *doc) resolve(v object) object {
	if r, ok := v.(ref); ok {
		o, _ := d.get(r.num)
		return o
	}
	return v
}

func (d *doc) dictOf(v object) (*dict, error) {
	if r, ok := v.(ref); ok {
		o, err := d.get(r.num)
		if err != nil {
			return nil, err
		}
		v = o
	}
	switch o := v.(type) {
	case *dict:
		return o, nil
	case *stream:
		return o.d, nil
	}
	return nil, nil
}

// output numbers the merged file's objects: 1 the catalog, 2 the page
// tree, then each input's pages and what they use, as they are reached.
type output struct {
	objs [][]byte // the body of object i+1
}

func (o *output) alloc() int {
	o.objs = append(o.objs, nil)
	return len(o.objs)
}

func write(w io.Writer, docs []part, paths []string) error {
	o := &output{}
	catalog, tree := o.alloc(), o.alloc()
	version := "1.4"
	var kids []int
	for i, part := range docs {
		d := part.d
		label := fmt.Sprintf("PDF %d", i+1)
		if paths != nil {
			label = paths[i]
		}
		if d.version > version {
			version = d.version
		}
		if r, err := d.dictOf(d.trailer.get("Root")); err == nil && r != nil {
			if v, ok := r.get("Version").(name); ok && string(v) > version && len(v) == 3 {
				version = string(v)
			}
		}
		pages, err := d.pages()
		if err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		all := pages
		if part.pages != nil {
			chosen := make([]page, 0, len(part.pages))
			for _, j := range part.pages {
				if j < 0 || j >= len(pages) {
					return fmt.Errorf("%s: no page %d", label, j+1)
				}
				chosen = append(chosen, pages[j])
			}
			pages = chosen
		}
		c := &copier{d: d, o: o, newNum: map[int]int{}, skip: map[int]bool{}}
		// A page not written (Split) is not reached through a link to it.
		for _, p := range all {
			if p.num != 0 {
				c.skip[p.num] = true
			}
		}
		// References to the old page tree's nodes lead to the new one.
		for _, n := range d.treeNodes() {
			c.newNum[n] = tree
		}
		nums := make([]int, len(pages))
		for j, p := range pages {
			nums[j] = o.alloc()
			if p.num != 0 {
				c.newNum[p.num] = nums[j]
				delete(c.skip, p.num)
			}
		}
		for j, p := range pages {
			pd := &dict{keys: p.d.keys, vals: p.d.vals}
			pd.set("Parent", outRef(tree))
			body, err := c.body(pd)
			if err != nil {
				return fmt.Errorf("%s: %w", label, err)
			}
			o.objs[nums[j]-1] = body
		}
		if err := c.drain(); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		kids = append(kids, nums...)
	}
	// No page found at all is a file not understood, not an empty result:
	// a PDF of no pages is not a valid PDF.
	if len(kids) == 0 {
		return errors.New("no pages found in the PDF files")
	}
	var b bytes.Buffer
	b.WriteString("<< /Type /Pages /Kids [")
	for i, k := range kids {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%d 0 R", k)
	}
	fmt.Fprintf(&b, "] /Count %d >>", len(kids))
	o.objs[tree-1] = b.Bytes()
	o.objs[catalog-1] = []byte(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", tree))
	info := o.alloc()
	o.objs[info-1] = []byte("<< /Producer (plot-go) >>")
	return o.writeTo(w, version, catalog, info)
}

// treeNodes are the object numbers of the page tree's inner nodes.
func (d *doc) treeNodes() []int {
	root, err := d.dictOf(d.trailer.get("Root"))
	if err != nil || root == nil {
		return nil
	}
	var nodes []int
	seen := map[int]bool{}
	var walk func(v object)
	walk = func(v object) {
		r, ok := v.(ref)
		if !ok || seen[r.num] {
			return
		}
		seen[r.num] = true
		node, _ := d.dictOf(r)
		if node == nil {
			return
		}
		kids, ok := d.resolve(node.get("Kids")).(array)
		if !ok && node.get("Type") != name("Pages") {
			return
		}
		nodes = append(nodes, r.num)
		for _, k := range kids {
			walk(k)
		}
	}
	walk(root.get("Pages"))
	return nodes
}

// outRef refers to an object of the output, not of an input.
type outRef int

// copier copies one input's objects into the output, renumbering them.
type copier struct {
	d      *doc
	o      *output
	newNum map[int]int
	skip   map[int]bool // pages left out, written as null where referred to
	queue  []int        // input numbers given a new number, still to copy
}

func (c *copier) num(old int) int {
	if n, ok := c.newNum[old]; ok {
		return n
	}
	n := c.o.alloc()
	c.newNum[old] = n
	c.queue = append(c.queue, old)
	return n
}

func (c *copier) drain() error {
	for len(c.queue) > 0 {
		old := c.queue[0]
		c.queue = c.queue[1:]
		v, err := c.d.get(old)
		if err != nil {
			return err
		}
		body, err := c.body(v)
		if err != nil {
			return err
		}
		c.o.objs[c.newNum[old]-1] = body
	}
	return nil
}

// body is object v written out, its references renumbered.
func (c *copier) body(v object) ([]byte, error) {
	var b bytes.Buffer
	if s, ok := v.(*stream); ok {
		d := &dict{keys: append([]name(nil), s.d.keys...), vals: append([]object(nil), s.d.vals...)}
		d.set("Length", number(strconv.Itoa(len(s.data))))
		c.value(&b, d)
		b.WriteString("\nstream\n")
		b.Write(s.data)
		b.WriteString("\nendstream")
		return b.Bytes(), nil
	}
	c.value(&b, v)
	return b.Bytes(), nil
}

func (c *copier) value(b *bytes.Buffer, v object) {
	switch o := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(o))
	case number:
		b.WriteString(string(o))
	case name:
		b.WriteByte('/')
		b.WriteString(encodeName(o))
	case str:
		b.Write(o)
	case outRef:
		fmt.Fprintf(b, "%d 0 R", int(o))
	case ref:
		if _, ok := c.d.xref[o.num]; !ok || c.skip[o.num] {
			b.WriteString("null") // a reference to nothing is null
			return
		}
		fmt.Fprintf(b, "%d 0 R", c.num(o.num))
	case array:
		b.WriteByte('[')
		for i, e := range o {
			if i > 0 {
				b.WriteByte(' ')
			}
			c.value(b, e)
		}
		b.WriteByte(']')
	case *dict:
		b.WriteString("<<")
		for i, k := range o.keys {
			b.WriteString(" /")
			b.WriteString(encodeName(k))
			b.WriteByte(' ')
			c.value(b, o.vals[i])
		}
		b.WriteString(" >>")
	case *stream:
		// A stream is always an indirect object, never inside another.
		b.WriteString("null")
	}
}

func (o *output) writeTo(w io.Writer, version string, root, info int) error {
	bw := bufio.NewWriter(w)
	off := 0
	put := func(s string) {
		n, _ := bw.WriteString(s)
		off += n
	}
	putBytes := func(s []byte) {
		n, _ := bw.Write(s)
		off += n
	}
	put("%PDF-" + version + "\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(o.objs))
	for i, body := range o.objs {
		offsets[i] = off
		put(strconv.Itoa(i+1) + " 0 obj\n")
		if body == nil {
			body = []byte("null")
		}
		putBytes(body)
		put("\nendobj\n")
	}
	start := off
	put(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(o.objs)+1))
	for _, x := range offsets {
		put(fmt.Sprintf("%010d 00000 n \n", x))
	}
	put(fmt.Sprintf("trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(o.objs)+1, root, info, start))
	return bw.Flush()
}
