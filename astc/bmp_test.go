package astc_test

import (
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func mustReadBMP32(t *testing.T, path string) (width, height int, rgba []byte) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}

	w, h, pix, err := decodeBMP32(data)
	if err != nil {
		t.Fatalf("decodeBMP32(%q): %v", path, err)
	}
	return w, h, pix
}

func decodeBMP32(data []byte) (width, height int, rgba []byte, err error) {
	if len(data) < 54 {
		return 0, 0, nil, fmt.Errorf("bmp: file too small: %d", len(data))
	}
	if data[0] != 'B' || data[1] != 'M' {
		return 0, 0, nil, fmt.Errorf("bmp: invalid signature")
	}

	pixOff := int(binary.LittleEndian.Uint32(data[10:14]))
	if pixOff < 54 || pixOff > len(data) {
		return 0, 0, nil, fmt.Errorf("bmp: invalid pixel offset %d", pixOff)
	}

	dibSize := int(binary.LittleEndian.Uint32(data[14:18]))
	if dibSize < 40 || 14+dibSize > len(data) {
		return 0, 0, nil, fmt.Errorf("bmp: unsupported DIB header size %d", dibSize)
	}

	w := int(int32(binary.LittleEndian.Uint32(data[18:22])))
	hSigned := int32(binary.LittleEndian.Uint32(data[22:26]))
	h := int(hSigned)
	if w <= 0 || h == 0 {
		return 0, 0, nil, fmt.Errorf("bmp: invalid dimensions %dx%d", w, h)
	}

	planes := binary.LittleEndian.Uint16(data[26:28])
	if planes != 1 {
		return 0, 0, nil, fmt.Errorf("bmp: invalid planes %d", planes)
	}

	bpp := binary.LittleEndian.Uint16(data[28:30])
	if bpp != 32 {
		return 0, 0, nil, fmt.Errorf("bmp: unsupported bpp %d", bpp)
	}

	compression := binary.LittleEndian.Uint32(data[30:34])
	if compression != 0 {
		return 0, 0, nil, fmt.Errorf("bmp: unsupported compression %d", compression)
	}

	topDown := false
	if hSigned < 0 {
		topDown = true
		h = -h
	}

	rowBytes := w * 4
	need := pixOff + rowBytes*h
	if need > len(data) {
		return 0, 0, nil, fmt.Errorf("bmp: truncated pixel data: want %d bytes, got %d", need, len(data))
	}

	rgba = make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		srcY := y
		if !topDown {
			srcY = h - 1 - y
		}
		srcRow := pixOff + srcY*rowBytes
		dstRow := y * w * 4
		for x := 0; x < w; x++ {
			sp := srcRow + x*4
			dp := dstRow + x*4
			b := data[sp+0]
			g := data[sp+1]
			r := data[sp+2]
			a := data[sp+3]
			rgba[dp+0] = r
			rgba[dp+1] = g
			rgba[dp+2] = b
			rgba[dp+3] = a
		}
	}

	return w, h, rgba, nil
}
