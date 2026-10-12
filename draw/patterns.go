package draw

import "fmt"

// Grace's fill patterns as gracebat's EPS defines them (/Pattern0 ...
// /Pattern31): 16 rows of 16 bits, two bytes a row. gracebat paints them
// with "16 16 true [-1 0 0 -1 16 16] imagemask", so both axes are turned:
// row r, bit b (0 the high bit of the row's first byte) is the cell from
// (15-b, 15-r) to (16-b, 16-r) of a 16 by 16 tile. A pattern unit is
// PatternUnit of the viewport and the tiles start at the page's origin
// (docs/grace-behaviour.md).
var patternBits = [32]string{
	"0000000000000000000000000000000000000000000000000000000000000000", // 0
	"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", // 1
	"eeeeffffbbbbffffeeeeffffbbbbffffeeeeffffbbbbffffeeeeffffbbbbffff", // 2
	"eeeebbbbeeeebbbbeeeebbbbeeeebbbbeeeebbbbeeeebbbbeeeebbbbeeeebbbb", // 3
	"5555aaaa5555aaaa5555aaaa5555aaaa5555aaaa5555aaaa5555aaaa5555aaaa", // 4
	"1111444411114444111144441111444411114444111144441111444411114444", // 5
	"1111000044440000111100004444000011110000444400001111000044440000", // 6
	"1010000000000000010100000000000010100000000000000101000000000000", // 7
	"0000000000000000000000000000000000000000000000000000000000000000", // 8
	"1e1e0f0f8787c3c3e1e1f0f078783c3c1e1e0f0f8787c3c3e1e1f0f078783c3c", // 9
	"7878f0f0e1e1c3c387870f0f1e1e3c3c7878f0f0e1e1c3c387870f0f1e1e3c3c", // 10
	"3333333333333333333333333333333333333333333333333333333333333333", // 11
	"ffffffff00000000ffffffff00000000ffffffff00000000ffffffff00000000", // 12
	"8181424224241818181824244242818181814242242418181818242442428181", // 13
	"8080404020201010080804040202010180804040202010100808040402020101", // 14
	"0101020204040808101020204040808001010202040408081010202040408080", // 15
	"2222222222222222222222222222222222222222222222222222222222222222", // 16
	"0000ffff000000000000ffff000000000000ffff000000000000ffff00000000", // 17
	"2222ffff222222222222ffff222222222222ffff222222222222ffff22222222", // 18
	"ffffffff33333333ffffffff33333333ffffffff33333333ffffffff33333333", // 19
	"0f0f0f0f0f0f0f0ff0f0f0f0f0f0f0f00f0f0f0f0f0f0f0ff0f0f0f0f0f0f0f0", // 20
	"ff00ff00ff00ff00ff00ff00ff00ff0000ff00ff00ff00ff00ff00ff00ff00ff", // 21
	"8001800180018001800180018001ffffffff8001800180018001800180018001", // 22
	"c003c003c003c003c003c003ffffffffffffffffc003c003c003c003c003c003", // 23
	"040404040404ffff404040404040ffff040404040404ffff404040404040ffff", // 24
	"180018001800180018001800ffffffff001800180018001800180018ffffffff", // 25
	"1111b8b87c7c3a3a1111a3a3c7c78b8b1111b8b87c7c3a3a1111a3a3c7c78b8b", // 26
	"101010102828c7c70101010182827c7c101010102828c7c70101010182827c7c", // 27
	"1c1c121211112121c1c12121111112121c1c121211112121c1c1212111111212", // 28
	"3e3e414180808080e3e31414080808083e3e414180808080e3e3141408080808", // 29
	"4848888884848383848488884848383848488888848483838484888848483838", // 30
	"03030404080808080c0c12122121c0c003030404080808080c0c12122121c0c0", // 31
}

// PatternUnit is one cell of a fill pattern, in viewport units, as gracebat's
// EPS writes it ([0.0017 0 0 0.0017 0 0] makepattern).
const PatternUnit = 0.0017

// PatternCells lists the cells pattern n paints in a 16 by 16 tile, each the
// lower left corner of a unit square; nil for a pattern that is not one of
// Grace's 0 to 31.
func PatternCells(n int) [][2]int {
	if n < 0 || n >= len(patternBits) {
		return nil
	}
	h := patternBits[n]
	var cells [][2]int
	for r := 0; r < 16; r++ {
		for b := 0; b < 16; b++ {
			byteHex := h[(r*2+b/8)*2 : (r*2+b/8)*2+2]
			var v int
			fmt.Sscanf(byteHex, "%02x", &v)
			if v&(0x80>>(b%8)) != 0 {
				cells = append(cells, [2]int{15 - b, 15 - r})
			}
		}
	}
	return cells
}

// PatternHex is pattern n's bits as gracebat's EPS writes them.
func PatternHex(n int) string {
	if n < 0 || n >= len(patternBits) {
		return ""
	}
	return patternBits[n]
}
