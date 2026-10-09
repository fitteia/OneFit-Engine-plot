// agr-inventory lists which parts of the Grace project format (.agr and
// .agr-par files) a set of files actually uses: every directive, reduced to
// its shape (graph/set numbers become #, numbers N, strings STR), with how
// many lines and files use it, and every text escape found inside strings.
// Its report decides what plot-go has to implement.
//
//	agr-inventory [-o report.md] DIR|FILE...
package main

import (
	"bufio"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	quoted  = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	indexed = regexp.MustCompile(`(?i)\b([gsr])[0-9]+\b`)
	number  = regexp.MustCompile(`[-+]?(?:[0-9]+\.?[0-9]*|\.[0-9]+)(?:[eE][-+]?[0-9]+)?\b`)
	spaces  = regexp.MustCompile(`\s+`)
	escape  = regexp.MustCompile(`\\(?:[A-Za-z]\{[^}]*\}|#\{[^}]*\}|[A-Za-z0-9+\-.])`)
	dataRow = regexp.MustCompile(`^[-+0-9.eE\s,]+$`)
)

// maxValues: up to this many distinct values are listed one by one.
const maxValues = 12

// tally counts uses of one key: lines, the files they are in, and one
// original line as an example.
type tally struct {
	lines   int
	files   map[string]bool
	example string
	values  map[string]int // the line's numbers and strings, as written
}

type inventory struct {
	directives map[string]*tally
	escapes    map[string]*tally
	files      int
	dataRows   int
	seen       map[[32]byte]bool
	duplicates int
}

func newInventory() *inventory {
	return &inventory{
		directives: map[string]*tally{},
		escapes:    map[string]*tally{},
		seen:       map[[32]byte]bool{},
	}
}

func count(m map[string]*tally, key, file, example string) {
	t := m[key]
	if t == nil {
		t = &tally{files: map[string]bool{}, example: example, values: map[string]int{}}
		m[key] = t
	}
	t.lines++
	t.files[file] = true
	t.values[values(example)]++
}

// values is what shape replaces in a line: its strings and numbers, in order.
func values(line string) string {
	var vs []string
	vs = append(vs, quoted.FindAllString(line, -1)...)
	rest := indexed.ReplaceAllString(quoted.ReplaceAllString(line, ""), "")
	vs = append(vs, number.FindAllString(rest, -1)...)
	return strings.Join(vs, " ")
}

// valueSummary lists the distinct values a directive takes, most used first,
// or just how many there are when that list would be long.
func valueSummary(t *tally) string {
	if len(t.values) > maxValues {
		return fmt.Sprintf("%d different", len(t.values))
	}
	vs := make([]string, 0, len(t.values))
	for v := range t.values {
		vs = append(vs, v)
	}
	sort.Slice(vs, func(i, j int) bool {
		if t.values[vs[i]] != t.values[vs[j]] {
			return t.values[vs[i]] > t.values[vs[j]]
		}
		return vs[i] < vs[j]
	})
	parts := make([]string, len(vs))
	for i, v := range vs {
		label := v
		if label == "" {
			label = "-"
		}
		parts[i] = fmt.Sprintf("%s (%d)", label, t.values[v])
	}
	return strings.Join(parts, ", ")
}

// shape reduces a directive to the form that matters for an implementation:
// "@    s12 line color 3" -> "s# line color N".
func shape(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "@"))
	line = quoted.ReplaceAllString(line, "STR")
	line = indexed.ReplaceAllStringFunc(line, func(m string) string { return m[:1] + "#" })
	line = number.ReplaceAllString(line, "N")
	return spaces.ReplaceAllString(line, " ")
}

func (inv *inventory) add(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	if inv.seen[sum] {
		inv.duplicates++
		return nil
	}
	inv.seen[sum] = true
	inv.files++
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "", strings.HasPrefix(line, "#"):
			continue
		case line == "&" || dataRow.MatchString(line):
			inv.dataRows++
			continue
		}
		count(inv.directives, shape(line), path, line)
		for _, s := range quoted.FindAllString(line, -1) {
			for _, e := range escape.FindAllString(s, -1) {
				count(inv.escapes, e, path, s)
			}
		}
	}
	return sc.Err()
}

func isProject(path string) bool {
	return strings.HasSuffix(path, ".agr") || strings.HasSuffix(path, ".agr-par")
}

func (inv *inventory) walk(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && isProject(path) {
			return inv.add(path)
		}
		return nil
	})
}

func sorted(m map[string]*tally) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", `\|`)
	if len(s) > 70 {
		s = s[:67] + "..."
	}
	return "`" + s + "`"
}

func (inv *inventory) report(w io.Writer) {
	fmt.Fprintf(w, "# Grace format inventory\n\n")
	fmt.Fprintf(w, "%d distinct files (%d duplicates skipped), %d data rows skipped.\n\n", inv.files, inv.duplicates, inv.dataRows)
	fmt.Fprintf(w, "## Directives (%d shapes)\n\n", len(inv.directives))
	fmt.Fprintf(w, "| directive | lines | files | values (lines) |\n|---|---:|---:|---|\n")
	for _, k := range sorted(inv.directives) {
		t := inv.directives[k]
		fmt.Fprintf(w, "| %s | %d | %d | %s |\n", cell(k), t.lines, len(t.files), strings.ReplaceAll(valueSummary(t), "|", `\|`))
	}
	fmt.Fprintf(w, "\n## Text escapes in strings (%d)\n\n", len(inv.escapes))
	fmt.Fprintf(w, "| escape | uses | files | example string |\n|---|---:|---:|---|\n")
	for _, k := range sorted(inv.escapes) {
		t := inv.escapes[k]
		fmt.Fprintf(w, "| %s | %d | %d | %s |\n", cell(k), t.lines, len(t.files), cell(t.example))
	}
}

func main() {
	out := flag.String("o", "", "write the report to this file (default: stdout)")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: agr-inventory [-o report.md] DIR|FILE...")
		os.Exit(2)
	}
	inv := newInventory()
	for _, root := range flag.Args() {
		if err := inv.walk(root); err != nil {
			fmt.Fprintln(os.Stderr, "agr-inventory:", err)
			os.Exit(1)
		}
	}
	w := io.Writer(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "agr-inventory:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	inv.report(w)
}
