package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/agr"
	"github.com/fitteia/OneFit-Engine-plot/render"
)

// The gracebat-compatible command line: the arguments OneFit's C core gives
// Grace, for example
//
//	grace -settype xydy gnu0.da_ -nxy fit-curves-1 -param fit1.agr-par \
//	  -hdevice EPS -hardcopy -printfile fit-curves-1.eps -saveall fit-curves-1.agr
//
// Arguments are taken in order, as Grace takes them: data files become sets
// in loading order (-nxy: one set per column after x; otherwise one set of
// the current -settype, "&" separating sets), and a parameter file applies
// its settings to what has been loaded so far.

// graceFlags are the options that make a command line gracebat's.
var graceFlags = map[string]bool{
	"-nxy": true, "-settype": true, "-settypexydy": true, "-param": true, "-log": true,
	"-hdevice": true, "-device": true, "-hardcopy": true, "-printfile": true, "-saveall": true,
	"-version": true, "-batch": true, "-noask": true, "-nosafe": true, "-safe": true, "-free": true,
	"-xydy": true,
}

// isGrace reports whether args are a gracebat command line.
func isGrace(args []string) bool {
	for _, a := range args {
		if graceFlags[a] {
			return true
		}
	}
	return false
}

// dataset is a set loaded from a data file.
type dataset struct {
	typ  string
	rows [][]float64
}

type graceRun struct {
	settype  string
	settings strings.Builder // directives, in order, each starting with "@"
	data     []dataset
	device   string
	print    string
	saveall  string
	warn     func(string)
}

func runGrace(args []string, stdout, stderr io.Writer) int {
	g := &graceRun{settype: "xy", warn: func(s string) { fmt.Fprintln(stderr, "plot-go:", s) }}
	next := func(i *int, flag string) (string, bool) {
		if *i+1 >= len(args) {
			g.warn(flag + " needs an argument")
			return "", false
		}
		*i++
		return args[*i], true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-version":
			fmt.Fprintf(stdout, "plot-go %s: draws Grace projects without Grace; gracebat-compatible (Grace-5.1.25 file format)\n", version)
			return 0
		case "-hardcopy", "-batch", "-noask", "-nosafe", "-safe", "-free":
		case "-settype":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			g.settype = strings.ToLower(v)
		case "-settypexydy", "-xydy":
			// "-settypexydy" is a misspelling in old OneFit command lines;
			// "-xydy FILE" is xmgr's spelling of "-settype xydy FILE"
			g.settype = "xydy"
		case "-nxy":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			if err := g.loadNXY(v); err != nil {
				g.warn(err.Error())
				return 1
			}
		case "-param":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			if err := g.loadParam(v); err != nil {
				g.warn(err.Error())
				return 1
			}
		case "-log":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			switch strings.ToLower(v) {
			case "x":
				g.settings.WriteString("@    xaxes scale Logarithmic\n")
			case "y":
				g.settings.WriteString("@    yaxes scale Logarithmic\n")
			case "xy":
				g.settings.WriteString("@    xaxes scale Logarithmic\n@    yaxes scale Logarithmic\n")
			default:
				g.warn("-log " + v + ": expected x, y or xy")
			}
		case "-hdevice", "-device":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			g.device = v
		case "-printfile":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			g.print = v
		case "-saveall":
			v, ok := next(&i, a)
			if !ok {
				return 2
			}
			g.saveall = v
		default:
			if strings.HasPrefix(a, "-") && len(a) > 1 {
				g.warn("option " + a + " is not supported; ignored")
				continue
			}
			if err := g.loadFile(a); err != nil {
				g.warn(err.Error())
				return 1
			}
		}
	}
	return g.finish()
}

// loadFile loads a positional argument: a Grace project, or a data file
// read as the current -settype.
func (g *graceRun) loadFile(name string) error {
	b, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	if isProject(string(b)) {
		return g.loadParam(name)
	}
	sets, err := readData(string(b), g.settype)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	for _, rows := range sets {
		g.data = append(g.data, dataset{typ: g.settype, rows: rows})
	}
	return nil
}

func isProject(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "# Grace project file") || strings.HasPrefix(s, "@")
}

// readData reads whitespace-separated rows; "&" ends a set. Comments (#)
// and blank lines are skipped, rows with too few numbers warned about.
func readData(s, typ string) ([][][]float64, error) {
	want := map[string]int{"xy": 2, "xydy": 3}[typ]
	if want == 0 {
		return nil, fmt.Errorf("set type %q is not supported (xy, xydy)", typ)
	}
	var sets [][][]float64
	var cur [][]float64
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@"):
			continue
		case line == "&":
			if len(cur) > 0 {
				sets = append(sets, cur)
			}
			cur = nil
			continue
		}
		row, err := numbers(line)
		if err != nil {
			return nil, err
		}
		if len(row) < want {
			return nil, fmt.Errorf("row %q has %d numbers, a %s set needs %d", line, len(row), typ, want)
		}
		cur = append(cur, row[:want])
	}
	if len(cur) > 0 {
		sets = append(sets, cur)
	}
	return sets, sc.Err()
}

func numbers(line string) ([]float64, error) {
	f := strings.Fields(line)
	row := make([]float64, len(f))
	for i, s := range f {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("not a number: %q", s)
		}
		row[i] = v
	}
	return row, nil
}

// loadNXY loads a file of x y1 y2 ... rows as one xy set per y column.
func (g *graceRun) loadNXY(name string) error {
	b, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	var cols [][][]float64
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "@") || line == "&" {
			continue
		}
		row, err := numbers(line)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if len(row) < 2 {
			continue
		}
		if cols == nil {
			cols = make([][][]float64, len(row)-1)
		}
		for k := 1; k < len(row) && k-1 < len(cols); k++ {
			cols[k-1] = append(cols[k-1], []float64{row[0], row[k]})
		}
	}
	for _, rows := range cols {
		g.data = append(g.data, dataset{typ: "xy", rows: rows})
	}
	return sc.Err()
}

// loadParam adds a parameter (or project) file's directives; .agr-par files
// write them without the leading "@".
func (g *graceRun) loadParam(name string) error {
	b, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if !strings.HasPrefix(t, "@") && !isDataLine(t) {
			line = "@" + line
		}
		g.settings.WriteString(line + "\n")
	}
	return sc.Err()
}

// isDataLine: inside a project file's data blocks, rows and "&" stay as
// they are.
func isDataLine(t string) bool {
	if t == "&" {
		return true
	}
	_, err := numbers(t)
	return err == nil
}

// dataText writes the loaded sets as a project's data blocks, numbers in
// Grace's default "%16.8g" format.
func (g *graceRun) dataText(types map[int]string) string {
	var b strings.Builder
	for i, d := range g.data {
		typ := d.typ
		if t := types[i]; t != "" {
			typ = t
		}
		fmt.Fprintf(&b, "@target G0.S%d\n@type %s\n", i, typ)
		for _, row := range d.rows {
			for k, v := range row {
				if k > 0 {
					b.WriteByte(' ')
				}
				fmt.Fprintf(&b, "%16.8g", v)
			}
			b.WriteByte('\n')
		}
		b.WriteString("&\n")
	}
	return b.String()
}

func (g *graceRun) finish() int {
	// the data first and the settings after, so a parameter file's settings
	// (set types included) apply to the loaded sets, as in Grace
	text := g.dataText(nil) + g.settings.String()
	p, err := agr.Parse(strings.NewReader(text))
	if err != nil {
		g.warn(err.Error())
		return 1
	}
	for _, w := range p.Warnings {
		g.warn(w.Msg + ": " + strings.TrimSpace(w.Text))
	}
	d, warnings := render.Render(p)
	for _, w := range warnings {
		g.warn(w)
	}
	status := 0
	if g.print != "" {
		format := strings.ToLower(g.device)
		switch format {
		case "", "eps", "postscript", "ps", "1":
			format = "eps"
		}
		if format == "eps" && strings.EqualFold(filepath.Ext(g.print), ".pdf") {
			format = "pdf"
		}
		write := writers[format]
		if write == nil {
			g.warn("device " + g.device + " is not supported (EPS, PostScript, PDF, SVG)")
			return 1
		}
		if err := writeFile(g.print, func(w io.Writer) error {
			return write(w, d, p.PageWidth, p.PageHeight, g.print)
		}); err != nil {
			g.warn(err.Error())
			status = 1
		}
	}
	if g.saveall != "" {
		// the sets' final types: what the parameter file made them
		types := map[int]string{}
		for _, gr := range p.Graphs {
			if gr.ID != 0 {
				continue
			}
			for _, s := range gr.Sets {
				types[s.ID] = s.Type
			}
		}
		project := "# Grace project file\n#\n" + canonical(g.settings.String(), p) + g.dataText(types)
		if err := writeFile(g.saveall, func(w io.Writer) error {
			_, err := io.WriteString(w, project)
			return err
		}); err != nil {
			g.warn(err.Error())
			status = 1
		}
	}
	return status
}

// writeFile writes through a temporary file, so a reader never sees half a
// file.
func writeFile(name string, write func(io.Writer) error) error {
	tmp := name + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := write(f); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, name)
}

// canonical rewrites the per-edge forms a parameter file may use
// ("world xmin 1e-05", "view ymax 0.85", ...) as the one-line forms gracebat
// saves ("world 1e-05, 0, 0.11, 0.11"), with the values the project ended
// up with: readers of saved projects (the OneFit GUI's) know only those.
// Tick label formats given by number are saved by name, and a precision
// without a value with the one used.
func canonical(settings string, p *agr.Project) string {
	var g *agr.Graph
	for _, gr := range p.Graphs {
		if gr.ID == 0 {
			g = gr
		}
	}
	if g == nil {
		return settings
	}
	num := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
	rect := func(r agr.Rect) string {
		return num(r.XMin) + ", " + num(r.YMin) + ", " + num(r.XMax) + ", " + num(r.YMax)
	}
	var b strings.Builder
	done := map[string]bool{}
	for _, line := range strings.SplitAfter(settings, "\n") {
		f := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "@"))
		// a tick label format given by number is saved by name: gracebat
		// saves an unknown one as "unknown", which it then rejects on
		// reading, so its own project no longer draws what it printed
		// a precision without a value is saved with the one used
		if len(f) == 3 && (f[0] == "xaxis" || f[0] == "yaxis") && f[1] == "ticklabel" && f[2] == "prec" {
			ax := g.X
			if f[0] == "yaxis" {
				ax = g.Y
			}
			b.WriteString("@    " + f[0] + "  ticklabel prec " + strconv.Itoa(ax.TickLabel.Prec) + "\n")
			continue
		}
		if len(f) == 4 && (f[0] == "xaxis" || f[0] == "yaxis") && f[1] == "ticklabel" && f[2] == "format" {
			if _, err := strconv.Atoi(f[3]); err == nil {
				ax := g.X
				if f[0] == "yaxis" {
					ax = g.Y
				}
				b.WriteString("@    " + f[0] + "  ticklabel format " + ax.TickLabel.Format + "\n")
				continue
			}
		}
		if len(f) == 3 && (f[0] == "world" || f[0] == "view") {
			switch f[1] {
			case "xmin", "xmax", "ymin", "ymax":
				if !done[f[0]] {
					r := g.World
					if f[0] == "view" {
						r = g.View
					}
					b.WriteString("@    " + f[0] + " " + rect(r) + "\n")
					done[f[0]] = true
				}
				continue
			}
		}
		b.WriteString(line)
	}
	return b.String()
}
