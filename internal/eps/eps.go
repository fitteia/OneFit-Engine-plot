// Package eps reads the EPS that gracebat writes into a list of drawing
// primitives - paths, arcs and texts with their exact coordinates - so
// plot-go's drawing can be compared with gracebat's number by number.
//
// It understands only the small PostScript vocabulary gracebat's output
// uses (its prolog defines m, l, s, n, c, SLW, SD, SC, CC, FFSF, EARC, ...);
// it reads gracebat's output, never Grace's source (see AGENTS.md).
package eps

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/fitteia/OneFit-Engine-plot/draw"
)

var (
	fontDef  = regexp.MustCompile(`(?s)/([A-Za-z0-9-]+) findfont.*?/(Font[0-9]+) exch definefont pop`)
	colorDef = regexp.MustCompile(`/(Color[0-9]+) \{([0-9.]+) ([0-9.]+) ([0-9.]+)\} def`)
)

type state struct {
	style  draw.Style
	font   string
	matrix [4]float64 // accumulated CC, for text
	path   []draw.Segment
	cur    *draw.Segment
	at     draw.Point
}

type value struct {
	num   float64
	isNum bool
	name  string // /Name (literal) or executable word
	str   string
	isStr bool
	arr   []float64
	isArr bool
}

// Parse reads gracebat's EPS. Anything outside the known vocabulary is an
// error: the comparison must not silently skip what it does not
// understand.
func Parse(r io.Reader) (*draw.Drawing, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	src := string(b)
	colors := map[string][3]float64{}
	for _, m := range colorDef.FindAllStringSubmatch(src, -1) {
		var c [3]float64
		for i := range c {
			c[i], _ = strconv.ParseFloat(m[i+2], 64)
		}
		colors[m[1]] = c
	}
	_, body, ok := strings.Cut(src, "%%EndSetup")
	if !ok {
		return nil, fmt.Errorf("no %%%%EndSetup: not gracebat EPS")
	}
	body, _, _ = strings.Cut(body, "%%Trailer")
	fonts := map[string]string{}
	body = fontDef.ReplaceAllStringFunc(body, func(s string) string {
		m := fontDef.FindStringSubmatch(s)
		fonts[m[2]] = m[1]
		return " "
	})

	d := &draw.Drawing{}
	st := &state{matrix: [4]float64{1, 0, 0, 1}}
	var saved []state
	var stack []value
	scale := 1.0
	pop := func() (value, error) {
		if len(stack) == 0 {
			return value{}, fmt.Errorf("stack underflow")
		}
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return v, nil
	}
	nums := func(n int) ([]float64, error) {
		if len(stack) < n {
			return nil, fmt.Errorf("stack underflow")
		}
		out := make([]float64, n)
		for i, v := range stack[len(stack)-n:] {
			if !v.isNum {
				return nil, fmt.Errorf("expected a number")
			}
			out[i] = v.num
		}
		stack = stack[:len(stack)-n]
		return out, nil
	}
	emitPath := func(fill bool) {
		if st.cur != nil {
			st.path = append(st.path, *st.cur)
			st.cur = nil
		}
		if len(st.path) == 0 {
			return
		}
		style := st.style
		style.Dash = append([]float64(nil), style.Dash...)
		d.Paths = append(d.Paths, draw.Path{Segments: st.path, Fill: fill, Style: style})
		d.Order = append(d.Order, 'p')
		st.path = nil
	}

	toks, err := tokens(body)
	if err != nil {
		return nil, err
	}
	for _, t := range toks {
		if t.isNum || t.isStr || t.isArr || strings.HasPrefix(t.name, "/") {
			stack = append(stack, t)
			continue
		}
		var e error
		switch w := t.name; w {
		case "scale":
			var a []float64
			if a, e = nums(2); e == nil {
				scale = a[0]
			}
		case "n":
			st.path, st.cur = nil, nil
		case "m":
			var a []float64
			if a, e = nums(2); e == nil {
				if st.cur != nil {
					st.path = append(st.path, *st.cur)
				}
				st.at = draw.Point{X: a[0], Y: a[1]}
				st.cur = &draw.Segment{Points: []draw.Point{st.at}}
			}
		case "l":
			var a []float64
			if a, e = nums(2); e == nil {
				if st.cur == nil {
					e = fmt.Errorf("lineto without a current point")
					break
				}
				st.at = draw.Point{X: a[0], Y: a[1]}
				st.cur.Points = append(st.cur.Points, st.at)
			}
		case "c":
			if st.cur != nil {
				st.cur.Closed = true
				st.path = append(st.path, *st.cur)
				st.cur = nil
			}
		case "EARC":
			var a []float64
			if a, e = nums(6); e == nil {
				if st.cur != nil {
					st.path = append(st.path, *st.cur)
					st.cur = nil
				}
				st.path = append(st.path, draw.Segment{Arc: &draw.Arc{X: a[0], Y: a[1], RX: a[2], RY: a[3], A1: a[4], A2: a[5]}})
			}
		case "s":
			emitPath(false)
		case "fill":
			emitPath(true)
		case "SLW":
			var a []float64
			if a, e = nums(1); e == nil {
				st.style.Width = a[0]
			}
		case "SLC", "SLJ":
			var a []float64
			if a, e = nums(1); e == nil {
				if w == "SLC" {
					st.style.Cap = int(a[0])
				} else {
					st.style.Join = int(a[0])
				}
			}
		case "SD":
			var off, arr value
			if off, e = pop(); e == nil {
				arr, e = pop()
			}
			if e == nil && (!off.isNum || !arr.isArr) {
				e = fmt.Errorf("SD expects [dashes] offset")
			}
			if e == nil {
				st.style.Dash = arr.arr
			}
		case "SCS":
			_, e = pop()
		case "SC":
			var a []float64
			if a, e = nums(3); e == nil {
				st.style.Color = [3]float64{a[0], a[1], a[2]}
			}
		case "GS":
			saved = append(saved, *st)
			saved[len(saved)-1].style.Dash = append([]float64(nil), st.style.Dash...)
		case "GR":
			if len(saved) == 0 {
				e = fmt.Errorf("GR without GS")
				break
			}
			*st = saved[len(saved)-1]
			saved = saved[:len(saved)-1]
		case "CC":
			var m value
			if m, e = pop(); e == nil {
				if !m.isArr || len(m.arr) != 6 {
					e = fmt.Errorf("CC expects a 6-number matrix")
					break
				}
				a := st.matrix
				st.matrix = [4]float64{
					m.arr[0]*a[0] + m.arr[1]*a[2], m.arr[0]*a[1] + m.arr[1]*a[3],
					m.arr[2]*a[0] + m.arr[3]*a[2], m.arr[2]*a[1] + m.arr[3]*a[3],
				}
			}
		case "FFSF":
			var f value
			if f, e = pop(); e == nil {
				name := strings.TrimPrefix(f.name, "/")
				if fonts[name] == "" {
					e = fmt.Errorf("font %s not defined", name)
					break
				}
				st.font = fonts[name]
			}
		case "show":
			var s value
			if s, e = pop(); e == nil {
				if !s.isStr {
					e = fmt.Errorf("show expects a string")
					break
				}
				d.Texts = append(d.Texts, draw.Text{At: st.at, Matrix: st.matrix, Font: st.font, Color: st.style.Color, Str: s.str})
				d.Order = append(d.Order, 't')
			}
		default:
			c, ok := colors[w]
			if !ok {
				e = fmt.Errorf("unknown operator %q", w)
				break
			}
			for _, x := range c {
				stack = append(stack, value{num: x, isNum: true})
			}
		}
		if e != nil {
			return nil, fmt.Errorf("%s: %w", t.name, e)
		}
	}
	if scale != 1 && scale != 600 {
		return nil, fmt.Errorf("unexpected page scale %g", scale)
	}
	return d, nil
}

// tokens splits PostScript into numbers, names, strings and number arrays.
func tokens(s string) ([]value, error) {
	var out []value
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\n' || c == '\r' || c == '\t':
			i++
		case c == '%':
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '(':
			var b strings.Builder
			depth := 1
			i++
			for i < len(s) && depth > 0 {
				ch := s[i]
				switch {
				case ch == '\\' && i+1 < len(s):
					i++
					n := s[i]
					if n >= '0' && n <= '7' {
						j := i
						for j < len(s) && j < i+3 && s[j] >= '0' && s[j] <= '7' {
							j++
						}
						v, _ := strconv.ParseUint(s[i:j], 8, 8)
						b.WriteByte(byte(v))
						i = j
						continue
					}
					switch n {
					case 'n':
						b.WriteByte('\n')
					case 't':
						b.WriteByte('\t')
					default:
						b.WriteByte(n)
					}
				case ch == '(':
					depth++
					b.WriteByte(ch)
				case ch == ')':
					depth--
					if depth > 0 {
						b.WriteByte(ch)
					}
				default:
					b.WriteByte(ch)
				}
				i++
			}
			out = append(out, value{str: b.String(), isStr: true})
		case c == '[':
			j := strings.IndexByte(s[i:], ']')
			if j < 0 {
				return nil, fmt.Errorf("unterminated array")
			}
			inner := s[i+1 : i+j]
			v := value{isArr: true}
			for _, f := range strings.Fields(inner) {
				x, err := strconv.ParseFloat(f, 64)
				if err != nil {
					// a non-numeric array: [/DeviceRGB] for SCS
					v.arr = nil
					v.name = inner
					break
				}
				v.arr = append(v.arr, x)
			}
			out = append(out, v)
			i += j + 1
		default:
			j := i
			for j < len(s) && !strings.ContainsRune(" \n\r\t()[]%", rune(s[j])) {
				j++
			}
			w := s[i:j]
			if x, err := strconv.ParseFloat(w, 64); err == nil {
				out = append(out, value{num: x, isNum: true})
			} else {
				out = append(out, value{name: w})
			}
			i = j
		}
	}
	return out, nil
}
