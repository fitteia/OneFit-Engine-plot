package figure

// Data files in figures: sets made from a text file of numbers - a OneFit
// data file, its "# DATA" blocks one set each, or any table of columns -
// remembering file, block and columns in the set's comment
// ("src: data/run7.dat#B2:c1,c2,c3") so Update reads them again.

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

// DataBlock is a block of a data file: its number (1-based, file order),
// its TAG when it has one, and its rows - each a row's leading numbers.
type DataBlock struct {
	N    int
	Tag  string
	Rows [][]float64
}

// Columns is how many numbers the block's widest row has.
func (b DataBlock) Columns() int {
	n := 0
	for _, r := range b.Rows {
		n = max(n, len(r))
	}
	return n
}

// isDataLine: a line starting a block, as both OneFit engines split Dados
// ("# DATA", "#DATA", whitespace either side).
func isDataLine(line string) bool {
	t := strings.TrimSpace(line)
	if !strings.HasPrefix(t, "#") {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(t[1:]), "DATA")
}

// ReadData reads a data file's blocks: one per "# DATA" line, or the whole
// file as one block when it has none. A row is a line's leading numbers,
// separated by spaces, tabs, commas or semicolons; comment lines ("#"),
// blank lines and lines that start with text are not rows. Text after a
// row's numbers (a label) is left out.
func ReadData(text string) []DataBlock {
	var blocks []DataBlock
	cur := &DataBlock{N: 1}
	started := false
	for _, line := range strings.Split(text, "\n") {
		if isDataLine(line) {
			if started {
				blocks = append(blocks, *cur)
				cur = &DataBlock{N: len(blocks) + 1}
			}
			started = true
			continue
		}
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			if k, v, ok := strings.Cut(strings.TrimSpace(t[1:]), "="); ok && strings.TrimSpace(k) == "TAG" {
				cur.Tag = strings.TrimSpace(v)
			}
			continue
		}
		var row []float64
		for _, f := range strings.FieldsFunc(t, func(r rune) bool { return unicode.IsSpace(r) || r == ',' || r == ';' }) {
			v, err := strconv.ParseFloat(f, 64)
			if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
				break
			}
			row = append(row, v)
		}
		if len(row) > 0 {
			cur.Rows = append(cur.Rows, row)
		}
	}
	return append(blocks, *cur)
}

// DataRef names columns of a data file's block: the file's path (as the
// caller wants it recorded), the block (1-based) and the columns (1-based:
// x, y, then the errors).
type DataRef struct {
	Path  string
	Block int
	Cols  []int
}

func (r DataRef) String() string {
	cols := make([]string, len(r.Cols))
	for i, c := range r.Cols {
		cols[i] = "c" + strconv.Itoa(c)
	}
	return fmt.Sprintf("%s%s#B%d:%s", srcPrefix, r.Path, r.Block, strings.Join(cols, ","))
}

// DataSourceOf is the data file a set's comment records, if any.
func DataSourceOf(s *agr.Set) (DataRef, bool) {
	c, ok := strings.CutPrefix(s.Comment, srcPrefix)
	if !ok {
		return DataRef{}, false
	}
	i := strings.LastIndex(c, "#B")
	if i <= 0 {
		return DataRef{}, false
	}
	blk, cols, found := strings.Cut(c[i+2:], ":")
	n, err := strconv.Atoi(blk)
	if !found || err != nil || n < 1 {
		return DataRef{}, false
	}
	ref := DataRef{Path: c[:i], Block: n}
	for _, f := range strings.Split(cols, ",") {
		k, err := strconv.Atoi(strings.TrimPrefix(f, "c"))
		if !strings.HasPrefix(f, "c") || err != nil || k < 1 {
			return DataRef{}, false
		}
		ref.Cols = append(ref.Cols, k)
	}
	if len(ref.Cols) < 2 || len(ref.Cols) > 4 {
		return DataRef{}, false
	}
	return ref, true
}

// dataType is the set type of a column list: x and y, then none, one (dy)
// or two (dx, dy) errors.
func dataType(cols int) string {
	switch cols {
	case 3:
		return "xydy"
	case 4:
		return "xydxdy"
	}
	return "xy"
}

// rows takes the columns of a block's rows; rows without all of them are
// left out, and counted.
func rows(b DataBlock, cols []int) (out [][]float64, skipped int) {
	for _, r := range b.Rows {
		v := make([]float64, len(cols))
		ok := true
		for i, c := range cols {
			if c > len(r) {
				ok = false
				break
			}
			v[i] = r[c-1]
		}
		if ok {
			out = append(out, v)
		} else {
			skipped++
		}
	}
	return out, skipped
}

// DataSpec is what to take from a data file: the blocks (1-based; none
// means all), the x column and the y columns - one set per y column - and,
// with a single y, its error columns (dy; or dx, dy). Columns are 1-based.
type DataSpec struct {
	Blocks []int
	X      int
	Y      []int
	Errors []int
}

// AddData adds sets made from a data file's text (recorded as path) to
// graph g of the figure, as new sets numbered after the graph's last:
// points (circles, no line), each set's legend the block's TAG, else the
// file's name, with the y column when there are several. It returns the
// new sets' numbers and what it left out.
func AddData(fig *agr.Project, g int, path, text string, spec DataSpec) ([]int, []string, error) {
	dst := graph(fig, g)
	if dst == nil {
		return nil, nil, fmt.Errorf("the figure has no graph %d", g)
	}
	if spec.X < 1 || len(spec.Y) == 0 {
		return nil, nil, fmt.Errorf("choose an x column and at least one y column")
	}
	if len(spec.Errors) > 2 {
		return nil, nil, fmt.Errorf("at most two error columns (dx, dy)")
	}
	if len(spec.Errors) > 0 && len(spec.Y) > 1 {
		return nil, nil, fmt.Errorf("error columns go with a single y column")
	}
	blocks := ReadData(text)
	chosen := spec.Blocks
	if len(chosen) == 0 {
		for _, b := range blocks {
			chosen = append(chosen, b.N)
		}
	}
	next := 0
	for _, s := range dst.Sets {
		next = max(next, s.ID+1)
	}
	var added []int
	var notes []string
	for _, n := range chosen {
		if n < 1 || n > len(blocks) {
			return added, notes, fmt.Errorf("%s has no block %d", path, n)
		}
		b := blocks[n-1]
		for _, y := range spec.Y {
			// x, y, then dy - or dx, dy: Grace's xydy and xydxdy columns
			cols := append([]int{spec.X, y}, spec.Errors...)
			data, skipped := rows(b, cols)
			if len(data) == 0 {
				notes = append(notes, fmt.Sprintf("block %d has no rows with columns %v; no set", n, cols))
				continue
			}
			if skipped > 0 {
				notes = append(notes, fmt.Sprintf("block %d: %d row(s) without all of columns %v left out", n, skipped, cols))
			}
			s := agr.NewSet(next, fig.Defaults, len(fig.Colors))
			s.Type = dataType(len(cols))
			s.Line.Type = 0
			s.Symbol.Type = 1
			s.Data = data
			s.Comment = DataRef{Path: path, Block: n, Cols: cols}.String()
			s.Legend = b.Tag
			if s.Legend == "" {
				s.Legend = filepath.Base(path)
			}
			if len(spec.Y) > 1 {
				s.Legend += " c" + strconv.Itoa(y)
			}
			dst.Sets = append(dst.Sets, s)
			added = append(added, next)
			next++
		}
	}
	return added, notes, nil
}

// updateData reads a data set's columns again from its file's text.
func updateData(s *agr.Set, ref DataRef, text string) error {
	blocks := ReadData(text)
	if ref.Block > len(blocks) {
		return fmt.Errorf("%s has no block %d", ref.Path, ref.Block)
	}
	data, _ := rows(blocks[ref.Block-1], ref.Cols)
	if len(data) == 0 {
		return fmt.Errorf("%s block %d has no rows with columns %v", ref.Path, ref.Block, ref.Cols)
	}
	s.Type = dataType(len(ref.Cols))
	s.Data = data
	s.Labels = nil
	return nil
}

// AddDataGraph adds a new graph holding sets from a data file (AddData):
// styled as the figure's first graph - its axes, frame and legend - with
// no sets and no title, and its window fitted to the new data (Extend from
// the first point). It returns the graph's number with AddData's results.
// Layout places it.
func AddDataGraph(fig *agr.Project, path, text string, spec DataSpec) (int, []int, []string, error) {
	if len(fig.Graphs) == 0 {
		return 0, nil, nil, fmt.Errorf("the figure has no graph to take a style from")
	}
	from := fig.Graphs[0]
	id := 0
	for _, g := range fig.Graphs {
		id = max(id, g.ID+1)
	}
	g := *from
	g.ID = id
	g.Sets = nil
	g.Kept = append([]string(nil), from.Kept...)
	g.KeptHeader = append([]string(nil), from.KeptHeader...)
	for _, a := range []*agr.Axis{&g.X, &g.Y, &g.AltX, &g.AltY} {
		a.Tick.Spec = append([]agr.SpecTick(nil), a.Tick.Spec...)
	}
	g.Title.Text, g.Subtitle.Text = "", ""
	fig.Graphs = append(fig.Graphs, &g)
	added, notes, err := AddData(fig, id, path, text, spec)
	if err != nil || len(added) == 0 {
		fig.Graphs = fig.Graphs[:len(fig.Graphs)-1]
		if err == nil {
			err = fmt.Errorf("%s gives no data with these columns", path)
		}
		return 0, nil, notes, err
	}
	// the window from the first point the axes can show, then grown to
	// all of the new data
	for _, s := range g.Sets {
		for _, r := range s.Data {
			if (g.X.Scale == "Logarithmic" && r[0] <= 0) || (g.Y.Scale == "Logarithmic" && r[1] <= 0) {
				continue
			}
			g.World = agr.Rect{XMin: r[0], XMax: r[0], YMin: r[1], YMax: r[1]}
			if g.X.Invert {
				g.World.XMin, g.World.XMax = g.World.XMax, g.World.XMin
			}
			_, err := Extend(fig, id, added)
			return id, added, notes, err
		}
	}
	return id, added, notes, nil
}
