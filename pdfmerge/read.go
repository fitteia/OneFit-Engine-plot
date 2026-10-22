package pdfmerge

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// A PDF object as read: nil (null), bool, number (its text, kept to
// write back unchanged), name (without the slash, as written), str (the
// whole literal or hex string as written), array, *dict, ref or *stream.
type (
	object any
	number string
	name   string
	str    []byte
	array  []object
	ref    struct{ num, gen int }
)

// dict keeps its keys in order, so a copied dictionary reads as the
// original.
type dict struct {
	keys []name
	vals []object
}

func (d *dict) get(k name) object {
	for i, key := range d.keys {
		if key == k {
			return d.vals[i]
		}
	}
	return nil
}

func (d *dict) set(k name, v object) {
	for i, key := range d.keys {
		if key == k {
			d.vals[i] = v
			return
		}
	}
	d.keys = append(d.keys, k)
	d.vals = append(d.vals, v)
}

func (d *dict) del(k name) {
	for i, key := range d.keys {
		if key == k {
			d.keys = append(d.keys[:i:i], d.keys[i+1:]...)
			d.vals = append(d.vals[:i:i], d.vals[i+1:]...)
			return
		}
	}
}

// stream is a stream object: its dictionary and its data still encoded.
type stream struct {
	d    *dict
	data []byte
}

func isSpace(c byte) bool {
	return c == 0 || c == '\t' || c == '\n' || c == '\f' || c == '\r' || c == ' '
}

func isDelim(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

func isRegular(c byte) bool { return !isSpace(c) && !isDelim(c) }

// parser reads objects from b starting at pos; doc resolves an indirect
// /Length while a stream is read (nil when there is none to resolve).
type parser struct {
	b   []byte
	pos int
	doc *doc
}

// at is a parser reading b from pos. An offset taken from the file may
// point anywhere: one outside b reads as the end of the file (no object
// there), so it is an error or a reason to rebuild, never a crash.
func at(b []byte, pos int, d *doc) *parser {
	if pos < 0 || pos > len(b) {
		pos = len(b)
	}
	return &parser{b: b, pos: pos, doc: d}
}

func (p *parser) skip() {
	for p.pos < len(p.b) {
		c := p.b[p.pos]
		switch {
		case isSpace(c):
			p.pos++
		case c == '%':
			for p.pos < len(p.b) && p.b[p.pos] != '\n' && p.b[p.pos] != '\r' {
				p.pos++
			}
		default:
			return
		}
	}
}

// word reads the run of regular characters at pos (a number or keyword).
func (p *parser) word() string {
	start := p.pos
	for p.pos < len(p.b) && isRegular(p.b[p.pos]) {
		p.pos++
	}
	return string(p.b[start:p.pos])
}

// keyword reports whether the next token is kw, consuming it if so.
func (p *parser) keyword(kw string) bool {
	p.skip()
	end := p.pos + len(kw)
	if end > len(p.b) || string(p.b[p.pos:end]) != kw || (end < len(p.b) && isRegular(p.b[end])) {
		return false
	}
	p.pos = end
	return true
}

func isInt(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

var errEOF = errors.New("unexpected end of file")

// object reads one direct object (or a reference "N G R").
func (p *parser) object() (object, error) {
	return p.objectDepth(0)
}

func (p *parser) objectDepth(depth int) (object, error) {
	if depth > 200 {
		return nil, errors.New("objects nested too deeply")
	}
	p.skip()
	if p.pos >= len(p.b) {
		return nil, errEOF
	}
	switch c := p.b[p.pos]; {
	case c == '/':
		p.pos++
		return decodeName(p.word()), nil
	case c == '(':
		return p.literal()
	case c == '<' && p.pos+1 < len(p.b) && p.b[p.pos+1] == '<':
		p.pos += 2
		d := &dict{}
		for {
			p.skip()
			if p.pos+1 < len(p.b) && p.b[p.pos] == '>' && p.b[p.pos+1] == '>' {
				p.pos += 2
				return d, nil
			}
			if p.pos >= len(p.b) {
				return nil, errEOF
			}
			k, err := p.objectDepth(depth + 1)
			if err != nil {
				return nil, err
			}
			key, ok := k.(name)
			if !ok {
				return nil, fmt.Errorf("dictionary key is not a name at offset %d", p.pos)
			}
			v, err := p.objectDepth(depth + 1)
			if err != nil {
				return nil, err
			}
			d.set(key, v)
		}
	case c == '<':
		end := bytes.IndexByte(p.b[p.pos:], '>')
		if end < 0 {
			return nil, errEOF
		}
		s := str(p.b[p.pos : p.pos+end+1])
		p.pos += end + 1
		return s, nil
	case c == '[':
		p.pos++
		a := array{}
		for {
			p.skip()
			if p.pos >= len(p.b) {
				return nil, errEOF
			}
			if p.b[p.pos] == ']' {
				p.pos++
				return a, nil
			}
			v, err := p.objectDepth(depth + 1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
	case isRegular(c):
		w := p.word()
		switch w {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		}
		if w[0] != '+' && w[0] != '-' && w[0] != '.' && (w[0] < '0' || w[0] > '9') {
			return nil, fmt.Errorf("unexpected %q at offset %d", w, p.pos-len(w))
		}
		if isInt(w) {
			// N G R: look ahead without consuming unless it is one.
			save := p.pos
			p.skip()
			g := p.word()
			if isInt(g) && p.keyword("R") {
				n, _ := strconv.Atoi(w)
				gen, _ := strconv.Atoi(g)
				return ref{n, gen}, nil
			}
			p.pos = save
		}
		return number(w), nil
	default:
		return nil, fmt.Errorf("unexpected %q at offset %d", c, p.pos)
	}
}

// decodeName undoes a name's #xx escapes: /Pa#67es is the name /Pages.
// A # not followed by two hex digits stays as it is.
func decodeName(raw string) name {
	if !strings.Contains(raw, "#") {
		return name(raw)
	}
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '#' && i+2 < len(raw) {
			if v, err := strconv.ParseUint(raw[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(raw[i])
	}
	return name(b.String())
}

// encodeName writes a name's bytes back, escaping as #xx those a name
// cannot hold as they are (white space, delimiters, #, and bytes outside
// printable ASCII).
func encodeName(n name) string {
	var b strings.Builder
	for i := 0; i < len(n); i++ {
		c := n[i]
		if c < '!' || c > '~' || c == '#' || isDelim(c) {
			fmt.Fprintf(&b, "#%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// literal reads a (string), whose parentheses nest and whose backslash
// escapes the next byte.
func (p *parser) literal() (object, error) {
	start, depth := p.pos, 0
	for p.pos < len(p.b) {
		c := p.b[p.pos]
		p.pos++
		switch c {
		case '\\':
			p.pos++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return str(p.b[start:p.pos]), nil
			}
		}
	}
	return nil, errEOF
}

// indirect reads "N G obj ... endobj" at pos: the object, or a stream
// when its dictionary is followed by one.
func (p *parser) indirect() (int, object, error) {
	p.skip()
	n := p.word()
	p.skip()
	g := p.word()
	if !isInt(n) || !isInt(g) || !p.keyword("obj") {
		return 0, nil, fmt.Errorf("no object at offset %d", p.pos)
	}
	num, _ := strconv.Atoi(n)
	v, err := p.object()
	if err != nil {
		return 0, nil, err
	}
	d, ok := v.(*dict)
	if !ok || !p.keyword("stream") {
		return num, v, nil
	}
	// The data starts after the end of the "stream" line.
	if p.pos < len(p.b) && p.b[p.pos] == '\r' {
		p.pos++
	}
	if p.pos < len(p.b) && p.b[p.pos] == '\n' {
		p.pos++
	}
	start := p.pos
	length := -1
	if l, ok := p.resolve(d.get("Length")).(number); ok {
		if n, err := strconv.Atoi(string(l)); err == nil && n >= 0 {
			length = n
		}
	}
	if length >= 0 && start+length <= len(p.b) {
		q := at(p.b, start+length, nil)
		if q.keyword("endstream") {
			p.pos = q.pos
			return num, &stream{d, p.b[start : start+length]}, nil
		}
	}
	// A wrong /Length: the data runs to "endstream", less its end of line.
	end := bytes.Index(p.b[start:], []byte("endstream"))
	if end < 0 {
		return 0, nil, fmt.Errorf("stream %d without endstream", num)
	}
	data := p.b[start : start+end]
	data = bytes.TrimSuffix(data, []byte("\n"))
	data = bytes.TrimSuffix(data, []byte("\r"))
	p.pos = start + end + len("endstream")
	return num, &stream{d, data}, nil
}

func (p *parser) resolve(v object) object {
	if r, ok := v.(ref); ok && p.doc != nil {
		o, _ := p.doc.get(r.num)
		return o
	}
	return v
}

// xrefEntry places object num: at an offset in the file (inStm false),
// or as the idx-th object of object stream stm.
type xrefEntry struct {
	inStm bool
	off   int
	stm   int
	idx   int
}

// doc is one input PDF: its bytes (from %PDF on), where each object is,
// and its trailer.
type doc struct {
	b       []byte
	version string // "1.5" from the header
	xref    map[int]xrefEntry
	trailer *dict
	cache   map[int]object
	busy    map[int]bool     // objects being read, against loops
	stms    map[int][]object // object streams already unpacked
	stmNums map[int][]int    // ... and their objects' numbers
	rebuilt bool
}

var headerRE = regexp.MustCompile(`%PDF-(\d\.\d)`)

func parse(b []byte) (*doc, error) {
	head := b
	if len(head) > 1024 {
		head = head[:1024]
	}
	m := headerRE.FindSubmatchIndex(head)
	if m == nil {
		return nil, errors.New("not a PDF file (no %PDF- header)")
	}
	d := &doc{b: b[m[0]:], version: string(head[m[2]:m[3]]), cache: map[int]object{},
		busy: map[int]bool{}, stms: map[int][]object{}, stmNums: map[int][]int{}}
	if err := d.readXref(); err != nil || d.trailer == nil || d.trailer.get("Root") == nil {
		if err := d.rebuild(); err != nil {
			return nil, err
		}
	}
	if d.trailer.get("Encrypt") != nil {
		return nil, errors.New("encrypted PDF files are not supported")
	}
	return d, nil
}

// readXref follows startxref and every /Prev section. The newest
// section comes first, so an object's first entry is the one that counts.
func (d *doc) readXref() error {
	tail := d.b
	if len(tail) > 2048 {
		tail = tail[len(tail)-2048:]
	}
	i := bytes.LastIndex(tail, []byte("startxref"))
	if i < 0 {
		return errors.New("no startxref")
	}
	p := at(tail, i+len("startxref"), nil)
	p.skip()
	off, err := strconv.Atoi(p.word())
	if err != nil {
		return errors.New("bad startxref")
	}
	d.xref = map[int]xrefEntry{}
	seen := map[int]bool{}
	for {
		if off <= 0 || off >= len(d.b) || seen[off] {
			if len(seen) == 0 {
				return errors.New("startxref out of range")
			}
			return nil
		}
		seen[off] = true
		trailer, err := d.readSection(off)
		if err != nil {
			return err
		}
		if d.trailer == nil {
			d.trailer = trailer
		}
		prev, ok := trailer.get("Prev").(number)
		if !ok {
			return nil
		}
		if off, err = strconv.Atoi(string(prev)); err != nil {
			return nil
		}
	}
}

// readSection reads the xref table or xref stream at off and returns its
// trailer dictionary.
func (d *doc) readSection(off int) (*dict, error) {
	p := at(d.b, off, nil)
	if !p.keyword("xref") {
		_, v, err := p.indirect()
		if err != nil {
			return nil, err
		}
		s, ok := v.(*stream)
		if !ok || s.d.get("Type") != name("XRef") {
			return nil, fmt.Errorf("no xref at offset %d", off)
		}
		return s.d, d.readXrefStream(s)
	}
	for {
		p.skip()
		if p.keyword("trailer") {
			break
		}
		first, count := p.word(), ""
		p.skip()
		count = p.word()
		f, err1 := strconv.Atoi(first)
		c, err2 := strconv.Atoi(count)
		if err1 != nil || err2 != nil || c < 0 {
			return nil, fmt.Errorf("bad xref table at offset %d", p.pos)
		}
		for i := 0; i < c; i++ {
			p.skip()
			o := p.word()
			p.skip()
			p.word() // generation
			p.skip()
			kind := p.word()
			n, err := strconv.Atoi(o)
			if err != nil || (kind != "n" && kind != "f") {
				return nil, fmt.Errorf("bad xref entry at offset %d", p.pos)
			}
			if _, ok := d.xref[f+i]; !ok && kind == "n" {
				d.xref[f+i] = xrefEntry{off: n}
			}
		}
	}
	v, err := p.object()
	if err != nil {
		return nil, err
	}
	trailer, ok := v.(*dict)
	if !ok {
		return nil, errors.New("bad trailer")
	}
	// A hybrid file lists its compressed objects in an xref stream too.
	if x, ok := trailer.get("XRefStm").(number); ok {
		if n, err := strconv.Atoi(string(x)); err == nil {
			q := at(d.b, n, nil)
			if _, v, err := q.indirect(); err == nil {
				if s, ok := v.(*stream); ok {
					if err := d.readXrefStream(s); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return trailer, nil
}

func intOf(v object) (int, bool) {
	n, ok := v.(number)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(string(n))
	return i, err == nil
}

func (d *doc) readXrefStream(s *stream) error {
	data, err := decode(s)
	if err != nil {
		return fmt.Errorf("xref stream: %w", err)
	}
	w, ok := s.d.get("W").(array)
	if !ok || len(w) != 3 {
		return errors.New("xref stream without /W")
	}
	var widths [3]int
	row := 0
	for i := range widths {
		if widths[i], ok = intOf(w[i]); !ok || widths[i] < 0 || widths[i] > 8 {
			return errors.New("bad xref stream /W")
		}
		row += widths[i]
	}
	if row == 0 {
		return errors.New("bad xref stream /W")
	}
	size, _ := intOf(s.d.get("Size"))
	index := []int{0, size}
	if a, ok := s.d.get("Index").(array); ok {
		index = index[:0]
		for _, v := range a {
			n, ok := intOf(v)
			if !ok {
				return errors.New("bad xref stream /Index")
			}
			index = append(index, n)
		}
	}
	field := func(b []byte) int {
		n := 0
		for _, c := range b {
			n = n<<8 | int(c)
		}
		return n
	}
	pos := 0
	for i := 0; i+1 < len(index); i += 2 {
		for num := index[i]; num < index[i]+index[i+1]; num++ {
			if pos+row > len(data) {
				return nil
			}
			kind := 1 // a missing type field means 1
			if widths[0] > 0 {
				kind = field(data[pos : pos+widths[0]])
			}
			f2 := field(data[pos+widths[0] : pos+widths[0]+widths[1]])
			f3 := field(data[pos+widths[0]+widths[1] : pos+row])
			pos += row
			if _, ok := d.xref[num]; ok {
				continue
			}
			switch kind {
			case 1:
				d.xref[num] = xrefEntry{off: f2}
			case 2:
				d.xref[num] = xrefEntry{inStm: true, stm: f2, idx: f3}
			}
		}
	}
	return nil
}

// objRE finds "N G obj" at the start of a line, for rebuild.
var objRE = regexp.MustCompile(`(?m)(?:^|[\r\n\s])(\d+)\s+(\d+)\s+obj\b`)

// rebuild finds every object by scanning the file, for a file whose
// cross-reference data is missing or wrong (offsets shifted by an edit,
// a truncated end). A later definition of a number wins, as an update
// appended to a file would.
func (d *doc) rebuild() error {
	d.rebuilt = true
	d.xref = map[int]xrefEntry{}
	d.cache = map[int]object{}
	d.stms = map[int][]object{}
	d.stmNums = map[int][]int{}
	for _, m := range objRE.FindAllSubmatchIndex(d.b, -1) {
		n, _ := strconv.Atoi(string(d.b[m[2]:m[3]]))
		d.xref[n] = xrefEntry{off: m[2]}
	}
	// Objects inside object streams, unless written directly too.
	direct := map[int]bool{}
	for n := range d.xref {
		direct[n] = true
	}
	for n := range direct {
		p := at(d.b, d.xref[n].off, nil)
		_, v, err := p.indirect()
		s, ok := v.(*stream)
		if err != nil || !ok || s.d.get("Type") != name("ObjStm") {
			continue
		}
		nums, _, err := d.unpack(n, s)
		if err != nil {
			continue
		}
		for i, m := range nums {
			if !direct[m] {
				d.xref[m] = xrefEntry{inStm: true, stm: n, idx: i}
			}
		}
	}
	// The trailer: the last "trailer" dictionary, else the last xref
	// stream's, else one made from the catalog.
	trailer := &dict{}
	if i := bytes.LastIndex(d.b, []byte("trailer")); i >= 0 {
		p := at(d.b, i+len("trailer"), nil)
		if v, err := p.object(); err == nil {
			if t, ok := v.(*dict); ok {
				trailer = t
			}
		}
	}
	if trailer.get("Root") == nil {
		for n := range d.xref {
			v, err := d.get(n)
			if err != nil {
				continue
			}
			var dd *dict
			switch o := v.(type) {
			case *dict:
				dd = o
			case *stream:
				dd = o.d
			}
			if dd == nil {
				continue
			}
			if dd.get("Type") == name("XRef") && dd.get("Root") != nil {
				trailer = dd
				break
			}
			if dd.get("Type") == name("Catalog") {
				trailer.set("Root", ref{n, 0})
			}
		}
	}
	if trailer.get("Root") == nil {
		return errors.New("no document catalog found")
	}
	d.trailer = trailer
	return nil
}

// get returns object num, or nil (null) when the file has no such object.
func (d *doc) get(num int) (object, error) {
	if v, ok := d.cache[num]; ok {
		return v, nil
	}
	e, ok := d.xref[num]
	if !ok {
		return nil, nil
	}
	if d.busy[num] {
		return nil, fmt.Errorf("object %d refers to itself", num)
	}
	d.busy[num] = true
	defer delete(d.busy, num)
	var v object
	var err error
	if e.inStm {
		v, err = d.fromStream(num, e)
	} else {
		p := at(d.b, e.off, d)
		var n int
		n, v, err = p.indirect()
		if err == nil && n != num {
			err = fmt.Errorf("object %d: offset %d holds object %d", num, e.off, n)
		}
	}
	if err != nil {
		if !d.rebuilt {
			// The cross-reference data was wrong: find the objects again.
			if rerr := d.rebuild(); rerr == nil {
				delete(d.busy, num)
				return d.get(num)
			}
		}
		return nil, err
	}
	d.cache[num] = v
	return v, nil
}

func (d *doc) fromStream(num int, e xrefEntry) (object, error) {
	objs, ok := d.stms[e.stm]
	if !ok {
		v, err := d.get(e.stm)
		if err != nil {
			return nil, err
		}
		s, ok := v.(*stream)
		if !ok {
			return nil, fmt.Errorf("object %d: object stream %d missing", num, e.stm)
		}
		nums, all, err := d.unpack(e.stm, s)
		if err != nil {
			return nil, err
		}
		d.stms[e.stm], d.stmNums[e.stm] = all, nums
		objs = all
	}
	nums := d.stmNums[e.stm]
	if e.idx < len(nums) && nums[e.idx] == num {
		return objs[e.idx], nil
	}
	for i, n := range nums { // a wrong index: look the number up
		if n == num {
			return objs[i], nil
		}
	}
	return nil, fmt.Errorf("object %d not in object stream %d", num, e.stm)
}

// unpack reads the objects of object stream stm: their numbers and values.
func (d *doc) unpack(stm int, s *stream) ([]int, []object, error) {
	data, err := decode(s)
	if err != nil {
		return nil, nil, fmt.Errorf("object stream %d: %w", stm, err)
	}
	n, ok1 := intOf(s.d.get("N"))
	first, ok2 := intOf(s.d.get("First"))
	if !ok1 || !ok2 || n < 0 || first < 0 || first > len(data) {
		return nil, nil, fmt.Errorf("object stream %d: bad /N or /First", stm)
	}
	p := at(data[:first], 0, nil)
	nums := make([]int, 0, n)
	offs := make([]int, 0, n)
	for i := 0; i < n; i++ {
		p.skip()
		a, err1 := strconv.Atoi(p.word())
		p.skip()
		b, err2 := strconv.Atoi(p.word())
		if err1 != nil || err2 != nil {
			return nil, nil, fmt.Errorf("object stream %d: bad header", stm)
		}
		nums, offs = append(nums, a), append(offs, b)
	}
	objs := make([]object, n)
	for i, off := range offs {
		q := at(data, first+off, d)
		if first+off > len(data) {
			return nil, nil, fmt.Errorf("object stream %d: bad offset", stm)
		}
		if objs[i], err = q.object(); err != nil {
			return nil, nil, fmt.Errorf("object stream %d: %w", stm, err)
		}
	}
	return nums, objs, nil
}

// decode undoes a stream's /FlateDecode (with or without a PNG predictor),
// the only filter cross-reference and object streams use in practice.
func decode(s *stream) ([]byte, error) {
	filter := s.d.get("Filter")
	if a, ok := filter.(array); ok {
		if len(a) > 1 {
			return nil, errors.New("more than one filter")
		}
		filter = nil
		if len(a) == 1 {
			filter = a[0]
		}
	}
	data := s.data
	switch filter {
	case nil:
	case name("FlateDecode"), name("Fl"):
		r, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		out, err := io.ReadAll(r)
		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && len(out) == 0 {
			return nil, err
		}
		data = out
	default:
		return nil, fmt.Errorf("unsupported filter %v", filter)
	}
	parms := s.d.get("DecodeParms")
	if a, ok := parms.(array); ok && len(a) == 1 {
		parms = a[0]
	}
	pd, ok := parms.(*dict)
	if !ok {
		return data, nil
	}
	pred, _ := intOf(pd.get("Predictor"))
	if pred < 10 {
		if pred > 1 {
			return nil, fmt.Errorf("unsupported predictor %d", pred)
		}
		return data, nil
	}
	columns, ok := intOf(pd.get("Columns"))
	if !ok {
		columns = 1
	}
	colors, ok := intOf(pd.get("Colors"))
	if !ok {
		colors = 1
	}
	bpc, ok := intOf(pd.get("BitsPerComponent"))
	if !ok {
		bpc = 8
	}
	return unpredictPNG(data, columns, colors, bpc)
}

// unpredictPNG undoes PNG row filters: each row is a filter-type byte
// followed by the row's bytes.
func unpredictPNG(data []byte, columns, colors, bpc int) ([]byte, error) {
	bpp := (colors*bpc + 7) / 8
	rowLen := (columns*colors*bpc + 7) / 8
	if rowLen <= 0 || bpp <= 0 {
		return nil, errors.New("bad predictor parameters")
	}
	prev := make([]byte, rowLen)
	var out []byte
	for i := 0; i+1+rowLen <= len(data); i += 1 + rowLen {
		kind, row := data[i], append([]byte(nil), data[i+1:i+1+rowLen]...)
		for j := range row {
			var left, upLeft byte
			if j >= bpp {
				left, upLeft = row[j-bpp], prev[j-bpp]
			}
			up := prev[j]
			switch kind {
			case 0:
			case 1:
				row[j] += left
			case 2:
				row[j] += up
			case 3:
				row[j] += byte((int(left) + int(up)) / 2)
			case 4:
				row[j] += paeth(left, up, upLeft)
			default:
				return nil, fmt.Errorf("bad PNG predictor %d", kind)
			}
		}
		out = append(out, row...)
		prev = row
	}
	return out, nil
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := abs(p-int(a)), abs(p-int(b)), abs(p-int(c))
	switch {
	case pa <= pb && pa <= pc:
		return a
	case pb <= pc:
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
