package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShape(t *testing.T) {
	for in, want := range map[string]string{
		`@    s12 line color 3`:              "s# line color N",
		`@    world 1e-05, -1.1, 3, 1.1`:     "world N, N, N, N",
		`@    xaxis  label "\xt\f{} (s)"`:    "xaxis label STR",
		`@with g0`:                           "with g#",
		`@target G0.S1`:                      "target G#.S#",
		`@    xaxes scale Logarithmic`:       "xaxes scale Logarithmic",
		`@    s0 symbol size 0.500000`:       "s# symbol size N",
		`@    legend 0.85, 0.8`:              "legend N, N",
		`@map color 1 to (0, 0, 0), "black"`: "map color N to (N, N, N), STR",
	} {
		if got := shape(in); got != want {
			t.Errorf("shape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAddCountsDirectivesEscapesAndSkipsDuplicates(t *testing.T) {
	dir := t.TempDir()
	agr := "# Grace project file\n@version 50125\n@    s0 line color 1\n@    s1 line color 2\n" +
		"@    xaxis  label \"T\\S1\\N (s\\S-1\\N)\"\n@target G0.S0\n@type xy\n1 2\n3 4\n&\n"
	for _, name := range []string{"a.agr", "b.agr", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(agr), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inv := newInventory()
	if err := inv.walk(dir); err != nil {
		t.Fatal(err)
	}
	if inv.files != 1 || inv.duplicates != 1 {
		t.Errorf("files = %d, duplicates = %d; want 1 and 1 (c.txt is not a project)", inv.files, inv.duplicates)
	}
	if got := inv.directives["s# line color N"]; got == nil || got.lines != 2 {
		t.Errorf("s# line color N counted %v, want 2 lines", got)
	}
	if inv.dataRows != 3 {
		t.Errorf("dataRows = %d, want 3", inv.dataRows)
	}
	for _, e := range []string{`\S`, `\N`} {
		if inv.escapes[e] == nil || inv.escapes[e].lines != 2 {
			t.Errorf("escape %s = %v, want 2 uses", e, inv.escapes[e])
		}
	}
}

func TestValues(t *testing.T) {
	for in, want := range map[string]string{
		`@    s12 line color 3`:              "3",
		`@    world 1e-05, -1.1, 3, 1.1`:     "1e-05 -1.1 3 1.1",
		`@    xaxes scale Logarithmic`:       "",
		`@map color 1 to (0, 0, 0), "black"`: `"black" 1 0 0 0`,
		`@    s0 legend  "\xt\f{} (s)"`:      `"\xt\f{} (s)"`,
	} {
		if got := values(in); got != want {
			t.Errorf("values(%q) = %q, want %q", in, got, want)
		}
	}
}
