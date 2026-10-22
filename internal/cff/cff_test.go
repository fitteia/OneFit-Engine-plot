package cff

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// urwSans is a URW base-35 OpenType font where fonts-urw-base35 installs
// it; the tests on a real font are skipped without it.
const urwSans = "/usr/share/fonts/opentype/urw-base35/NimbusSans-Regular.otf"

// font is a CFF font's INDEXes and DICTs, read back as a reader does.
type font struct {
	names, strs, gsubrs, charStrings, lsubrs index
	top, priv                                []entry
	sids                                     []int
	encoding                                 []byte
}

func parse(t *testing.T, b []byte) font {
	t.Helper()
	var f font
	var err error
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	f.names, err = readIndex(b, int(b[2]))
	must(err)
	tops, err := readIndex(b, f.names.end)
	must(err)
	f.strs, err = readIndex(b, tops.end)
	must(err)
	f.gsubrs, err = readIndex(b, f.strs.end)
	must(err)
	f.top, err = readDict(tops.items[0])
	must(err)
	charset, encoding, privSize, privOff := 0, 0, 0, 0
	for _, e := range f.top {
		switch e.op {
		case 15:
			charset = last(e.operands)
		case 16:
			encoding = last(e.operands)
		case 17:
			f.charStrings, err = readIndex(b, last(e.operands))
			must(err)
		case 18:
			privSize, privOff = e.operands[0], e.operands[1]
		}
	}
	f.sids, err = readCharset(b, charset, len(f.charStrings.items))
	must(err)
	if encoding > 1 {
		f.encoding = b[encoding : encoding+2+int(b[encoding+1])]
	}
	f.priv, err = readDict(b[privOff : privOff+privSize])
	must(err)
	for _, e := range f.priv {
		if e.op == 19 {
			f.lsubrs, err = readIndex(b, privOff+last(e.operands))
			must(err)
		}
	}
	return f
}

// glyphName is a glyph's name as its string ID: a standard string's ID,
// or the string itself.
func (f font) glyphName(g int) string {
	sid := f.sids[g]
	if sid < standardStrings {
		return string(binary.BigEndian.AppendUint16(nil, uint16(sid)))
	}
	return string(f.strs.items[sid-standardStrings])
}

func TestSubsetURWFont(t *testing.T) {
	otf, err := os.ReadFile(urwSans)
	if err != nil {
		t.Skip("URW base-35 fonts not installed")
	}
	table, err := Table(otf)
	if err != nil {
		t.Fatal(err)
	}
	orig := parse(t, table)
	// the glyphs of A, B and z by name, wherever the font has them
	want := map[byte]int{}
	for g := range orig.charStrings.items {
		switch orig.glyphName(g) {
		case string([]byte{0, 34}):
			want['A'] = g
		case string([]byte{0, 35}):
			want['B'] = g
		case string([]byte{0, 91}):
			want['z'] = g
		}
	}
	if len(want) != 3 {
		t.Fatalf("glyphs A, B, z not found: %v", want)
	}
	codes := []byte{'A', 'B', 'z'}
	sub, err := Subset(table, codes, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(sub) >= len(table)/4 {
		t.Errorf("subset is %d bytes of the font's %d", len(sub), len(table))
	}
	s := parse(t, sub)
	if !bytes.Equal(s.names.items[0], orig.names.items[0]) {
		t.Errorf("name %q, want %q", s.names.items[0], orig.names.items[0])
	}
	if len(s.charStrings.items) != 1+len(codes) {
		t.Fatalf("%d glyphs, want %d", len(s.charStrings.items), 1+len(codes))
	}
	if !bytes.Equal(s.encoding, []byte{0, 3, 'A', 'B', 'z'}) {
		t.Errorf("encoding % x", s.encoding)
	}
	for i, c := range codes {
		g := want[c]
		if !bytes.Equal(s.charStrings.items[i+1], orig.charStrings.items[g]) {
			t.Errorf("glyph of %c changed", c)
		}
		if s.glyphName(i+1) != orig.glyphName(g) {
			t.Errorf("glyph of %c named %q, want %q", c, s.glyphName(i+1), orig.glyphName(g))
		}
	}
	// the subroutines keep their numbers (the bias with them)
	if len(s.gsubrs.items) != len(orig.gsubrs.items) || len(s.lsubrs.items) != len(orig.lsubrs.items) {
		t.Errorf("subroutines %d/%d, want %d/%d", len(s.gsubrs.items), len(s.lsubrs.items), len(orig.gsubrs.items), len(orig.lsubrs.items))
	}
	// the Top DICT's strings are the font's
	strs := func(f font) map[int]string {
		m := map[int]string{}
		for _, e := range f.top {
			if sidOperators[e.op] && len(e.operands) == 1 {
				if sid := e.operands[0]; sid >= standardStrings {
					m[e.op] = string(f.strs.items[sid-standardStrings])
				}
			}
		}
		return m
	}
	o, n := strs(orig), strs(s)
	if len(o) == 0 || len(o) != len(n) {
		t.Errorf("Top DICT strings %v, want %v", n, o)
	}
	for op, v := range o {
		if n[op] != v {
			t.Errorf("Top DICT operator %d: %q, want %q", op, n[op], v)
		}
	}
	for _, e := range s.top {
		if e.op == 13 || e.op == 14 {
			t.Errorf("unique ID %d kept", e.op)
		}
	}
}

func TestSubsetRefusesMissingGlyph(t *testing.T) {
	otf, err := os.ReadFile(urwSans)
	if err != nil {
		t.Skip("URW base-35 fonts not installed")
	}
	table, err := Table(otf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Subset(table, []byte{'A'}, map[byte]int{}); err == nil {
		t.Error("a code without a glyph was subset")
	}
	if _, err := Table([]byte("OTTO\x00\x00........")); err == nil {
		t.Error("a font without a CFF table read")
	}
}

// Subroutines are found through calls, nested ones too, and through the
// operand a hint mask's bytes would hide.
func TestSubrsUsed(t *testing.T) {
	// number -107 + bias 107 = subroutine 0; 32 is -107, 33 is -106
	lsubrs := [][]byte{
		{33, 10, 11}, // calls local 1
		{11},
		{11},
	}
	gsubrs := [][]byte{{11}, {11}}
	cs := []byte{
		139, 139, 1, // hstem: one stem
		19, 0xff, // hintmask: one byte for one stem; 0xff is no operand
		32, 10, // callsubr 0
		33, 29, // callgsubr 1
		14, // endchar
	}
	usedG, usedL := map[int]bool{}, map[int]bool{}
	if err := subrsUsed(cs, gsubrs, lsubrs, usedG, usedL); err != nil {
		t.Fatal(err)
	}
	if !usedL[0] || !usedL[1] || usedL[2] || usedG[0] || !usedG[1] {
		t.Errorf("local %v, global %v", usedL, usedG)
	}
	if err := subrsUsed([]byte{139 + 50, 10, 14}, gsubrs, lsubrs, usedG, usedL); err == nil {
		t.Error("a call outside the subroutines passed")
	}
}
