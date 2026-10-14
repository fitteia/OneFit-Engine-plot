package render

// Tick label formulas ("ticklabel formula"): an expression in $t, the
// tick's value, whose result is the label (docs/grace-behaviour.md, "Tick
// label formulas"). The language is what gracebat accepts, measured:
//
//	numbers   1, 2.5, .5, 2., 1e-3       pi        $t (any case)
//	||, or    &&, and    == != < > <= >=  (0 or 1)
//	+ -       * / %      unary - ! not   ^ (from the right, above unary -)
//	abs sqr sqrt exp ln log10 log2 floor ceil rint, sin cos tan asin acos
//	atan sinh cosh tanh asinh acosh atanh; mod(a, b)
//
// "/" or "%" by zero is an error, as a syntax error is; mod(a, 0) is NaN.
// Words are case-insensitive. Not in it: log, int, min, max, pow, e, **,
// unary +.

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type formula struct{ root fnode }

type fnode func(t float64) (float64, error)

var errFormulaZero = fmt.Errorf("division by zero")

var formulaFuncs1 = map[string]func(float64) float64{
	"abs": math.Abs, "sqr": func(v float64) float64 { return v * v }, "sqrt": math.Sqrt,
	"exp": math.Exp, "ln": math.Log, "log10": math.Log10, "log2": math.Log2,
	"floor": math.Floor, "ceil": math.Ceil, "rint": math.RoundToEven,
	"sin": math.Sin, "cos": math.Cos, "tan": math.Tan,
	"asin": math.Asin, "acos": math.Acos, "atan": math.Atan,
	"sinh": math.Sinh, "cosh": math.Cosh, "tanh": math.Tanh,
	"asinh": math.Asinh, "acosh": math.Acosh, "atanh": math.Atanh,
}

func parseFormula(s string) (*formula, error) {
	p := &fparser{src: s}
	if err := p.lex(); err != nil {
		return nil, err
	}
	n, err := p.or()
	if err != nil {
		return nil, err
	}
	if p.i < len(p.toks) {
		return nil, fmt.Errorf("unexpected %q", p.toks[p.i])
	}
	return &formula{root: n}, nil
}

func (f *formula) eval(t float64) (float64, error) { return f.root(t) }

type fparser struct {
	src  string
	toks []string
	i    int
}

func (p *fparser) lex() error {
	s := p.src
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c >= '0' && c <= '9' || c == '.':
			j := i
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
				j++
			}
			if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
				k := j + 1
				if k < len(s) && (s[k] == '+' || s[k] == '-') {
					k++
				}
				if k < len(s) && s[k] >= '0' && s[k] <= '9' {
					for k < len(s) && s[k] >= '0' && s[k] <= '9' {
						k++
					}
					j = k
				}
			}
			p.toks = append(p.toks, s[i:j])
			i = j
		case c == '$' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i + 1
			for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
				j++
			}
			p.toks = append(p.toks, strings.ToLower(s[i:j]))
			i = j
		default:
			two := ""
			if i+1 < len(s) {
				two = s[i : i+2]
			}
			switch two {
			case "||", "&&", "==", "!=", "<=", ">=":
				p.toks = append(p.toks, two)
				i += 2
				continue
			}
			if !strings.ContainsRune("+-*/%^()<>!,", rune(c)) {
				return fmt.Errorf("unexpected %q", string(c))
			}
			p.toks = append(p.toks, string(c))
			i++
		}
	}
	return nil
}

func (p *fparser) peek() string {
	if p.i < len(p.toks) {
		return p.toks[p.i]
	}
	return ""
}

func (p *fparser) binary(next func() (fnode, error), ops map[string]func(a, b float64) (float64, error)) (fnode, error) {
	l, err := next()
	if err != nil {
		return nil, err
	}
	for {
		op, ok := ops[p.peek()]
		if !ok {
			return l, nil
		}
		p.i++
		r, err := next()
		if err != nil {
			return nil, err
		}
		a := l
		l = func(t float64) (float64, error) {
			x, err := a(t)
			if err != nil {
				return 0, err
			}
			y, err := r(t)
			if err != nil {
				return 0, err
			}
			return op(x, y)
		}
	}
}

func truth(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (p *fparser) or() (fnode, error) {
	f := func(a, b float64) (float64, error) { return truth(a != 0 || b != 0), nil }
	return p.binary(p.and, map[string]func(a, b float64) (float64, error){"||": f, "or": f})
}

func (p *fparser) and() (fnode, error) {
	f := func(a, b float64) (float64, error) { return truth(a != 0 && b != 0), nil }
	return p.binary(p.cmp, map[string]func(a, b float64) (float64, error){"&&": f, "and": f})
}

func (p *fparser) cmp() (fnode, error) {
	return p.binary(p.add, map[string]func(a, b float64) (float64, error){
		"==": func(a, b float64) (float64, error) { return truth(a == b), nil },
		"!=": func(a, b float64) (float64, error) { return truth(a != b), nil },
		"<":  func(a, b float64) (float64, error) { return truth(a < b), nil },
		">":  func(a, b float64) (float64, error) { return truth(a > b), nil },
		"<=": func(a, b float64) (float64, error) { return truth(a <= b), nil },
		">=": func(a, b float64) (float64, error) { return truth(a >= b), nil },
	})
}

func (p *fparser) add() (fnode, error) {
	return p.binary(p.mul, map[string]func(a, b float64) (float64, error){
		"+": func(a, b float64) (float64, error) { return a + b, nil },
		"-": func(a, b float64) (float64, error) { return a - b, nil },
	})
}

func (p *fparser) mul() (fnode, error) {
	return p.binary(p.unary, map[string]func(a, b float64) (float64, error){
		"*": func(a, b float64) (float64, error) { return a * b, nil },
		"/": func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, errFormulaZero
			}
			return a / b, nil
		},
		"%": func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, errFormulaZero
			}
			return math.Mod(a, b), nil
		},
	})
}

func (p *fparser) unary() (fnode, error) {
	switch p.peek() {
	case "-", "!", "not":
		op := p.toks[p.i]
		p.i++
		x, err := p.unary()
		if err != nil {
			return nil, err
		}
		return func(t float64) (float64, error) {
			v, err := x(t)
			if err != nil {
				return 0, err
			}
			if op == "-" {
				return -v, nil
			}
			return truth(v == 0), nil
		}, nil
	}
	return p.power()
}

// power: "^" binds tighter than unary minus (-$t^2 is -(t^2)) and groups
// from the right (2^$t^2 is 2^(t^2)); its exponent may be negated.
func (p *fparser) power() (fnode, error) {
	b, err := p.primary()
	if err != nil {
		return nil, err
	}
	if p.peek() != "^" {
		return b, nil
	}
	p.i++
	e, err := p.unary()
	if err != nil {
		return nil, err
	}
	return func(t float64) (float64, error) {
		x, err := b(t)
		if err != nil {
			return 0, err
		}
		y, err := e(t)
		if err != nil {
			return 0, err
		}
		return math.Pow(x, y), nil
	}, nil
}

func (p *fparser) primary() (fnode, error) {
	tok := p.peek()
	if tok == "" {
		return nil, fmt.Errorf("unexpected end")
	}
	p.i++
	switch {
	case tok == "(":
		n, err := p.or()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing )")
		}
		p.i++
		return n, nil
	case tok == "$t":
		return func(t float64) (float64, error) { return t, nil }, nil
	case tok == "pi":
		return func(float64) (float64, error) { return math.Pi, nil }, nil
	case tok[0] >= '0' && tok[0] <= '9' || tok[0] == '.':
		v, err := strconv.ParseFloat(tok, 64)
		if err != nil {
			return nil, fmt.Errorf("bad number %q", tok)
		}
		return func(float64) (float64, error) { return v, nil }, nil
	}
	f1, one := formulaFuncs1[tok]
	if !one && tok != "mod" {
		return nil, fmt.Errorf("unknown %q", tok)
	}
	if p.peek() != "(" {
		return nil, fmt.Errorf("%s needs (", tok)
	}
	p.i++
	var args []fnode
	for {
		a, err := p.or()
		if err != nil {
			return nil, err
		}
		args = append(args, a)
		if p.peek() == "," {
			p.i++
			continue
		}
		break
	}
	if p.peek() != ")" {
		return nil, fmt.Errorf("missing )")
	}
	p.i++
	if one {
		if len(args) != 1 {
			return nil, fmt.Errorf("%s takes one argument", tok)
		}
		a := args[0]
		return func(t float64) (float64, error) {
			v, err := a(t)
			if err != nil {
				return 0, err
			}
			return f1(v), nil
		}, nil
	}
	if len(args) != 2 {
		return nil, fmt.Errorf("mod takes two arguments")
	}
	a, b := args[0], args[1]
	return func(t float64) (float64, error) {
		x, err := a(t)
		if err != nil {
			return 0, err
		}
		y, err := b(t)
		if err != nil {
			return 0, err
		}
		if y == 0 {
			return math.NaN(), nil // measured: mod(a, 0) is "nan", not an error
		}
		return math.Mod(x, y), nil
	}, nil
}
