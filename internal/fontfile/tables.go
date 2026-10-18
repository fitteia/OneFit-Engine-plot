package fontfile

import "encoding/binary"

func table(data []byte, name string) []byte {
	if len(data) < 12 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(data[4:6]))
	for i := 0; i < n; i++ {
		at := 12 + i*16
		if at+16 > len(data) {
			break
		}
		if string(data[at:at+4]) != name {
			continue
		}
		offset := uint64(binary.BigEndian.Uint32(data[at+8 : at+12]))
		length := uint64(binary.BigEndian.Uint32(data[at+12 : at+16]))
		if offset+length > uint64(len(data)) {
			return nil
		}
		return data[offset : offset+length]
	}
	return nil
}

func embeddingFlags(data []byte) uint16 {
	b := table(data, "OS/2")
	if len(b) < 10 {
		return 0
	}
	return binary.BigEndian.Uint16(b[8:10])
}

// Underline position/thickness come from the standard OpenType 'post' table.
// Bounds checks are independent of the sfnt parser; absent values use a modest
// decoration below the baseline rather than a different font's metrics.
func underline(data []byte, upm float64) (float64, float64) {
	b := table(data, "post")
	if len(b) >= 12 {
		pos := int16(binary.BigEndian.Uint16(b[8:10]))
		thick := int16(binary.BigEndian.Uint16(b[10:12]))
		if thick > 0 {
			return float64(pos) / upm, float64(thick) / upm
		}
	}
	return -.1, .05
}
