package filesystem

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// parseRecycleI parses a Windows Recycle Bin $I* metadata blob (Vista+).
func parseRecycleI(data []byte) (orig string, size int64, err error) {
	if len(data) < 24 {
		return "", 0, fmt.Errorf("recycle $I too short")
	}
	ver := binary.LittleEndian.Uint64(data[0:8])
	size = int64(binary.LittleEndian.Uint64(data[8:16]))
	var u16 []byte
	switch ver {
	case 1:
		u16 = data[24:]
	case 2:
		if len(data) < 28 {
			return "", 0, fmt.Errorf("recycle $I v2 truncated")
		}
		nchars := int(binary.LittleEndian.Uint32(data[24:28]))
		start := 28
		end := start + nchars*2
		if nchars <= 0 || end > len(data) {
			end = len(data)
		}
		u16 = data[start:end]
	default:
		return "", 0, fmt.Errorf("recycle $I version %d", ver)
	}
	orig = utf16LEToString(u16)
	orig = strings.TrimRight(orig, "\x00")
	if orig == "" {
		return "", 0, fmt.Errorf("recycle $I empty path")
	}
	return orig, size, nil
}

func utf16LEToString(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	runes := make([]rune, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := uint16(b[i]) | uint16(b[i+1])<<8
		if c == 0 {
			break
		}
		runes = append(runes, rune(c))
	}
	return string(runes)
}
