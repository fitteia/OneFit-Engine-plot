// Package cff subsets an OpenType font's CFF table (Adobe Technical Note
// #5176, "The Compact Font Format Specification") into a bare CFF font
// holding only the glyphs a PDF uses, so the PDF can embed it as a Type1C
// font program (FontFile3), as Ghostscript embeds the URW base-35 fonts.
//
// The subset keeps the font's name, Top DICT (but its unique IDs) and
// Private DICT; it holds .notdef and the glyphs asked for, a charset with
// their original names, a built-in encoding giving each its byte code, only
// the strings it names, and the subroutines as they are numbered, those its
// glyphs do not call emptied to a bare return.
package cff

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Table is the 'CFF ' table of an OpenType font file.
func Table(otf []byte) ([]byte, error) {
	if len(otf) < 12 {
		return nil, errors.New("not an OpenType font")
	}
	if tag := string(otf[:4]); tag != "OTTO" {
		return nil, fmt.Errorf("not a CFF-flavoured OpenType font (%q)", tag)
	}
	n := int(binary.BigEndian.Uint16(otf[4:]))
	for i := 0; i < n; i++ {
		rec := 12 + 16*i
		if rec+16 > len(otf) {
			break
		}
		if string(otf[rec:rec+4]) != "CFF " {
			continue
		}
		off := int(binary.BigEndian.Uint32(otf[rec+8:]))
		length := int(binary.BigEndian.Uint32(otf[rec+12:]))
		if off < 0 || length < 0 || off+length > len(otf) {
			return nil, errors.New("CFF table outside the file")
		}
		return otf[off : off+length], nil
	}
	return nil, errors.New("no CFF table")
}

// index is a CFF INDEX: its items, and where it ends.
type index struct {
	items [][]byte
	end   int
}

func readIndex(b []byte, at int) (index, error) {
	if at+2 > len(b) {
		return index{}, errors.New("INDEX outside the font")
	}
	count := int(binary.BigEndian.Uint16(b[at:]))
	if count == 0 {
		return index{end: at + 2}, nil
	}
	if at+3 > len(b) {
		return index{}, errors.New("INDEX outside the font")
	}
	offSize := int(b[at+2])
	if offSize < 1 || offSize > 4 {
		return index{}, fmt.Errorf("INDEX offset size %d", offSize)
	}
	offs := at + 3
	data := offs + (count+1)*offSize - 1 // offsets count from the byte before the data
	if offs+(count+1)*offSize > len(b) {
		return index{}, errors.New("INDEX outside the font")
	}
	off := func(i int) int {
		v := 0
		for k := 0; k < offSize; k++ {
			v = v<<8 | int(b[offs+i*offSize+k])
		}
		return v
	}
	ix := index{items: make([][]byte, count)}
	for i := 0; i < count; i++ {
		s, e := data+off(i), data+off(i+1)
		if s < data+1 || e < s || e > len(b) {
			return index{}, errors.New("INDEX item outside the font")
		}
		ix.items[i] = b[s:e]
	}
	ix.end = data + off(count)
	return ix, nil
}

func writeIndex(items [][]byte) []byte {
	if len(items) == 0 {
		return []byte{0, 0}
	}
	total := 1
	for _, it := range items {
		total += len(it)
	}
	offSize := 1
	for total > 1<<(8*offSize)-1 {
		offSize++
	}
	out := []byte{byte(len(items) >> 8), byte(len(items)), byte(offSize)}
	put := func(v int) {
		for k := offSize - 1; k >= 0; k-- {
			out = append(out, byte(v>>(8*k)))
		}
	}
	o := 1
	put(o)
	for _, it := range items {
		o += len(it)
		put(o)
	}
	for _, it := range items {
		out = append(out, it...)
	}
	return out
}

// entry is a DICT entry: its operator (12 xx as 1200+xx), its operands'
// bytes as written, and their integer values (reals are 0).
type entry struct {
	op       int
	raw      []byte
	operands []int
}

func readDict(b []byte) ([]entry, error) {
	var es []entry
	start := 0
	var ops []int
	for i := 0; i < len(b); {
		c := int(b[i])
		switch {
		case c <= 21:
			op := c
			i++
			if c == 12 {
				if i >= len(b) {
					return nil, errors.New("DICT ends in an escape")
				}
				op = 1200 + int(b[i])
				i++
			}
			es = append(es, entry{op: op, raw: b[start:i], operands: ops})
			ops, start = nil, i
		case c == 28:
			if i+3 > len(b) {
				return nil, errors.New("DICT operand outside")
			}
			ops = append(ops, int(int16(binary.BigEndian.Uint16(b[i+1:]))))
			i += 3
		case c == 29:
			if i+5 > len(b) {
				return nil, errors.New("DICT operand outside")
			}
			ops = append(ops, int(int32(binary.BigEndian.Uint32(b[i+1:]))))
			i += 5
		case c == 30:
			i++
			for i < len(b) && b[i]&0x0f != 0x0f && b[i]>>4 != 0x0f {
				i++
			}
			i++
			ops = append(ops, 0)
		case c >= 32 && c <= 246:
			ops = append(ops, c-139)
			i++
		case c >= 247 && c <= 250:
			if i+2 > len(b) {
				return nil, errors.New("DICT operand outside")
			}
			ops = append(ops, (c-247)*256+int(b[i+1])+108)
			i += 2
		case c >= 251 && c <= 254:
			if i+2 > len(b) {
				return nil, errors.New("DICT operand outside")
			}
			ops = append(ops, -(c-251)*256-int(b[i+1])-108)
			i += 2
		default:
			return nil, fmt.Errorf("DICT byte %d", c)
		}
	}
	return es, nil
}

// int32Operand is v in the fixed five-byte form, so offsets can be written
// before they are known.
func int32Operand(v int) []byte {
	return []byte{29, byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

// Subset is the CFF font cff (a 'CFF ' table) with .notdef and, for each
// byte code in codes, the glyph glyphs[code] (an index into the font's
// CharStrings) - the font's built-in encoding giving each its code. codes
// are in increasing order.
func Subset(cff []byte, codes []byte, glyphs map[byte]int) ([]byte, error) {
	if len(cff) < 4 {
		return nil, errors.New("CFF too short")
	}
	names, err := readIndex(cff, int(cff[2]))
	if err != nil {
		return nil, fmt.Errorf("Name INDEX: %w", err)
	}
	if len(names.items) != 1 {
		return nil, fmt.Errorf("%d fonts in the CFF, want 1", len(names.items))
	}
	tops, err := readIndex(cff, names.end)
	if err != nil {
		return nil, fmt.Errorf("Top DICT INDEX: %w", err)
	}
	strs, err := readIndex(cff, tops.end)
	if err != nil {
		return nil, fmt.Errorf("String INDEX: %w", err)
	}
	gsubrs, err := readIndex(cff, strs.end)
	if err != nil {
		return nil, fmt.Errorf("Global Subr INDEX: %w", err)
	}
	top, err := readDict(tops.items[0])
	if err != nil {
		return nil, fmt.Errorf("Top DICT: %w", err)
	}
	charsetOff, charStringsOff, privSize, privOff := 0, -1, 0, -1
	var kept []entry
	for _, e := range top {
		switch e.op {
		case 1230: // ROS
			return nil, errors.New("CID-keyed fonts are not subset")
		case 15:
			charsetOff = last(e.operands)
		case 17:
			charStringsOff = last(e.operands)
		case 18:
			if len(e.operands) == 2 {
				privSize, privOff = e.operands[0], e.operands[1]
			}
		case 13, 14, 16: // UniqueID, XUID (a subset is another font), Encoding
		default:
			kept = append(kept, e)
		}
	}
	if charStringsOff < 0 || privOff < 0 {
		return nil, errors.New("no CharStrings or Private DICT")
	}
	charStrings, err := readIndex(cff, charStringsOff)
	if err != nil {
		return nil, fmt.Errorf("CharStrings: %w", err)
	}
	sids, err := readCharset(cff, charsetOff, len(charStrings.items))
	if err != nil {
		return nil, fmt.Errorf("charset: %w", err)
	}
	if privOff+privSize > len(cff) || privSize < 0 {
		return nil, errors.New("Private DICT outside the font")
	}
	priv, err := readDict(cff[privOff : privOff+privSize])
	if err != nil {
		return nil, fmt.Errorf("Private DICT: %w", err)
	}
	// the Private DICT, its Subrs offset rewritten to follow it
	var privKept []byte
	var lsubrs index
	for _, e := range priv {
		if e.op == 19 && len(e.operands) == 1 {
			if lsubrs, err = readIndex(cff, privOff+last(e.operands)); err != nil {
				return nil, fmt.Errorf("local Subrs: %w", err)
			}
			continue
		}
		privKept = append(privKept, e.raw...)
	}

	keep := []int{0}
	charset := []byte{0}
	encoding := []byte{0, byte(len(codes))}
	for _, c := range codes {
		g, ok := glyphs[c]
		if !ok || g <= 0 || g >= len(charStrings.items) {
			return nil, fmt.Errorf("no glyph %d for code %d", g, c)
		}
		keep = append(keep, g)
		encoding = append(encoding, c)
	}

	// only the subroutines the kept glyphs call; the others become a bare
	// return, so the subroutines keep their numbers
	usedG, usedL := map[int]bool{}, map[int]bool{}
	var glyphCS [][]byte
	for _, g := range keep {
		if err := subrsUsed(charStrings.items[g], gsubrs.items, lsubrs.items, usedG, usedL); err != nil {
			return nil, fmt.Errorf("glyph %d: %w", g, err)
		}
		glyphCS = append(glyphCS, charStrings.items[g])
	}
	gsubrIx := writeIndex(onlyUsed(gsubrs.items, usedG))
	lsubrIx := []byte(nil)
	if len(lsubrs.items) > 0 {
		lsubrIx = writeIndex(onlyUsed(lsubrs.items, usedL))
	}

	// only the strings the subset names: its glyphs' and the Top DICT's
	var strItems [][]byte
	newSID := map[int]int{}
	sid := func(old int) (int, error) {
		if old < standardStrings {
			return old, nil
		}
		if n, ok := newSID[old]; ok {
			return n, nil
		}
		if old-standardStrings >= len(strs.items) {
			return 0, fmt.Errorf("string %d outside the String INDEX", old)
		}
		strItems = append(strItems, strs.items[old-standardStrings])
		newSID[old] = standardStrings + len(strItems) - 1
		return newSID[old], nil
	}
	for _, g := range keep[1:] {
		n, err := sid(sids[g])
		if err != nil {
			return nil, err
		}
		charset = append(charset, byte(n>>8), byte(n))
	}
	var keptTop []byte
	for _, e := range kept {
		if sidOperators[e.op] && len(e.operands) == 1 {
			n, err := sid(e.operands[0])
			if err != nil {
				return nil, err
			}
			keptTop = append(keptTop, int32Operand(n)...)
			keptTop = append(keptTop, opBytes(e.op)...)
			continue
		}
		keptTop = append(keptTop, e.raw...)
	}

	newPriv := append([]byte{}, privKept...)
	if lsubrIx != nil {
		at := len(newPriv) + 5 + 1
		newPriv = append(append(newPriv, int32Operand(at)...), 19)
	}

	// header, Name INDEX, Top DICT INDEX, String INDEX, Global Subr INDEX,
	// charset, encoding, CharStrings INDEX, Private DICT and Subrs
	header := []byte{1, 0, 4, 4}
	nameIx := writeIndex(names.items)
	csIx := writeIndex(glyphCS)
	topLen := len(keptTop) + 3*(5+1) + 5 + 5 + 1 // charset, Encoding, CharStrings: an offset and the operator; Private: size, offset, operator
	topIxLen := len(writeIndex([][]byte{make([]byte, topLen)}))
	strIx := writeIndex(strItems)
	at := len(header) + len(nameIx) + topIxLen + len(strIx) + len(gsubrIx)
	charsetAt := at
	encodingAt := charsetAt + len(charset)
	csAt := encodingAt + len(encoding)
	privAt := csAt + len(csIx)

	newTop := append([]byte{}, keptTop...)
	newTop = append(append(newTop, int32Operand(charsetAt)...), 15)
	newTop = append(append(newTop, int32Operand(encodingAt)...), 16)
	newTop = append(append(newTop, int32Operand(csAt)...), 17)
	newTop = append(append(append(newTop, int32Operand(len(newPriv))...), int32Operand(privAt)...), 18)
	if len(newTop) != topLen {
		return nil, fmt.Errorf("Top DICT is %d bytes, %d planned", len(newTop), topLen)
	}

	var out []byte
	out = append(out, header...)
	out = append(out, nameIx...)
	out = append(out, writeIndex([][]byte{newTop})...)
	out = append(out, strIx...)
	out = append(out, gsubrIx...)
	out = append(out, charset...)
	out = append(out, encoding...)
	out = append(out, csIx...)
	out = append(out, newPriv...)
	out = append(out, lsubrIx...)
	return out, nil
}

// standardStrings is how many strings CFF predefines: string IDs from it
// on are the String INDEX's.
const standardStrings = 391

// sidOperators are the Top DICT operators whose operand is a string ID:
// version, Notice, FullName, FamilyName, Weight, Copyright, PostScript,
// BaseFontName.
var sidOperators = map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true, 1200: true, 1221: true, 1222: true}

func opBytes(op int) []byte {
	if op >= 1200 {
		return []byte{12, byte(op - 1200)}
	}
	return []byte{byte(op)}
}

// onlyUsed is subrs with each one not used replaced by a bare return.
func onlyUsed(subrs [][]byte, used map[int]bool) [][]byte {
	out := make([][]byte, len(subrs))
	for i, s := range subrs {
		if used[i] {
			out[i] = s
		} else {
			out[i] = []byte{11}
		}
	}
	return out
}

// bias is what a Type 2 charstring adds to a subroutine number (5176,
// "Subrs" / Type 2 Charstring Format, 4.7).
func bias(n int) int {
	switch {
	case n < 1240:
		return 107
	case n < 33900:
		return 1131
	}
	return 32768
}

// subrsUsed marks the global and local subroutines Type 2 charstring cs
// calls, directly or through others. It follows the charstring as an
// interpreter does as far as subroutine calls and hint masks need: the
// operand stack (the number a call takes), and the stem hints counted for
// the size of hintmask's and cntrmask's masks.
func subrsUsed(cs []byte, gsubrs, lsubrs [][]byte, usedG, usedL map[int]bool) error {
	var stack []float64
	stems := 0
	depth := 0
	var run func(b []byte) (bool, error)
	run = func(b []byte) (bool, error) {
		if depth++; depth > 10 {
			return false, errors.New("subroutines nested too deep")
		}
		defer func() { depth-- }()
		for i := 0; i < len(b); {
			c := int(b[i])
			switch {
			case c == 28:
				if i+3 > len(b) {
					return false, errors.New("charstring ends in a number")
				}
				stack = append(stack, float64(int16(binary.BigEndian.Uint16(b[i+1:]))))
				i += 3
				continue
			case c >= 32 && c <= 246:
				stack = append(stack, float64(c-139))
				i++
				continue
			case c >= 247 && c <= 254:
				if i+2 > len(b) {
					return false, errors.New("charstring ends in a number")
				}
				v := (c-247)*256 + int(b[i+1]) + 108
				if c >= 251 {
					v = -(c-251)*256 - int(b[i+1]) - 108
				}
				stack = append(stack, float64(v))
				i += 2
				continue
			case c == 255:
				if i+5 > len(b) {
					return false, errors.New("charstring ends in a number")
				}
				stack = append(stack, float64(int32(binary.BigEndian.Uint32(b[i+1:])))/65536)
				i += 5
				continue
			}
			i++
			switch c {
			case 1, 3, 18, 23: // hstem, vstem, hstemhm, vstemhm
				stems += len(stack) / 2
				stack = stack[:0]
			case 19, 20: // hintmask, cntrmask: a vstem's operands may precede them
				stems += len(stack) / 2
				stack = stack[:0]
				i += (stems + 7) / 8
			case 10, 29: // callsubr, callgsubr
				if len(stack) == 0 {
					return false, errors.New("subroutine call without a number")
				}
				n := int(stack[len(stack)-1])
				stack = stack[:len(stack)-1]
				subrs, used := lsubrs, usedL
				if c == 29 {
					subrs, used = gsubrs, usedG
				}
				k := n + bias(len(subrs))
				if k < 0 || k >= len(subrs) {
					return false, fmt.Errorf("subroutine %d not in the font", n)
				}
				used[k] = true
				end, err := run(subrs[k])
				if err != nil || end {
					return end, err
				}
			case 11: // return
				return false, nil
			case 14: // endchar
				return true, nil
			case 12:
				i++ // the escaped operator
				stack = stack[:0]
			default:
				stack = stack[:0]
			}
		}
		return false, nil
	}
	_, err := run(cs)
	return err
}

func last(v []int) int {
	if len(v) == 0 {
		return 0
	}
	return v[len(v)-1]
}

// readCharset is each glyph's string ID (.notdef's 0).
func readCharset(b []byte, at, n int) ([]int, error) {
	sids := make([]int, n)
	if at <= 2 {
		if at != 0 {
			return nil, fmt.Errorf("predefined charset %d", at)
		}
		for i := range sids { // ISOAdobe: string ID = glyph index
			sids[i] = i
		}
		return sids, nil
	}
	if at >= len(b) {
		return nil, errors.New("charset outside the font")
	}
	format := b[at]
	p := at + 1
	need := func(k int) error {
		if p+k > len(b) {
			return errors.New("charset outside the font")
		}
		return nil
	}
	for g := 1; g < n; {
		switch format {
		case 0:
			if err := need(2); err != nil {
				return nil, err
			}
			sids[g] = int(binary.BigEndian.Uint16(b[p:]))
			p += 2
			g++
		case 1, 2:
			k := 3
			if format == 2 {
				k = 4
			}
			if err := need(k); err != nil {
				return nil, err
			}
			first := int(binary.BigEndian.Uint16(b[p:]))
			left := int(b[p+2])
			if format == 2 {
				left = int(binary.BigEndian.Uint16(b[p+2:]))
			}
			p += k
			for i := 0; i <= left && g < n; i++ {
				sids[g] = first + i
				g++
			}
		default:
			return nil, fmt.Errorf("charset format %d", format)
		}
	}
	return sids, nil
}
