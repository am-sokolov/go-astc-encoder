package astc_test

import (
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestEncodeRGBA8_ImprovesOverConstBlocks(t *testing.T) {
	const (
		w      = 8
		h      = 8
		blockX = 4
		blockY = 4
	)

	src := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			src[off+0] = uint8(x * 32)
			src[off+1] = uint8(y * 32)
			src[off+2] = uint8((x + y) * 16)
			src[off+3] = uint8(255 - x*8)
		}
	}

	astcNew, err := astc.EncodeRGBA8WithProfileAndQuality(src, w, h, blockX, blockY, astc.ProfileLDR, astc.EncodeMedium)
	if err != nil {
		t.Fatalf("EncodeRGBA8WithProfileAndQuality: %v", err)
	}
	decNew, w2, h2, err := astc.DecodeRGBA8(astcNew)
	if err != nil {
		t.Fatalf("DecodeRGBA8(new): %v", err)
	}
	if w2 != w || h2 != h {
		t.Fatalf("DecodeRGBA8(new) dimensions: got %dx%d want %dx%d", w2, h2, w, h)
	}

	astcConst, err := encodeRGBA8ConstBlocks(src, w, h, blockX, blockY)
	if err != nil {
		t.Fatalf("encodeRGBA8ConstBlocks: %v", err)
	}
	decConst, w3, h3, err := astc.DecodeRGBA8(astcConst)
	if err != nil {
		t.Fatalf("DecodeRGBA8(const): %v", err)
	}
	if w3 != w || h3 != h {
		t.Fatalf("DecodeRGBA8(const) dimensions: got %dx%d want %dx%d", w3, h3, w, h)
	}

	errNew := sumSquaredDiffU8(src, decNew)
	errConst := sumSquaredDiffU8(src, decConst)
	if errNew >= errConst {
		t.Fatalf("expected real encoder to improve error: got new=%d const=%d", errNew, errConst)
	}
}

func sumSquaredDiffU8(a, b []byte) uint64 {
	if len(a) != len(b) {
		panic("sumSquaredDiffU8: length mismatch")
	}
	var sum uint64
	for i := 0; i < len(a); i++ {
		d := int(a[i]) - int(b[i])
		sum += uint64(d * d)
	}
	return sum
}

func encodeRGBA8ConstBlocks(pix []byte, width, height, blockX, blockY int) ([]byte, error) {
	h := astc.Header{
		BlockX: uint8(blockX),
		BlockY: uint8(blockY),
		BlockZ: 1,
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  1,
	}
	headerBytes, err := astc.MarshalHeader(h)
	if err != nil {
		return nil, err
	}
	blocksX, blocksY, _, total, err := h.BlockCount()
	if err != nil {
		return nil, err
	}

	out := make([]byte, 0, astc.HeaderSize+total*astc.BlockBytes)
	out = append(out, headerBytes[:]...)

	for by := 0; by < blocksY; by++ {
		for bx := 0; bx < blocksX; bx++ {
			r, g, b, a := avgRGBA8(pix, width, height, bx*blockX, by*blockY, blockX, blockY)
			block := astc.EncodeConstBlockRGBA8(r, g, b, a)
			out = append(out, block[:]...)
		}
	}

	return out, nil
}

func avgRGBA8(pix []byte, width, height, x0, y0, blockX, blockY int) (r, g, b, a uint8) {
	var sumR, sumG, sumB, sumA uint32
	count := 0
	for y := 0; y < blockY; y++ {
		yy := y0 + y
		if yy >= height {
			break
		}
		row := yy * width * 4
		for x := 0; x < blockX; x++ {
			xx := x0 + x
			if xx >= width {
				break
			}
			off := row + xx*4
			sumR += uint32(pix[off+0])
			sumG += uint32(pix[off+1])
			sumB += uint32(pix[off+2])
			sumA += uint32(pix[off+3])
			count++
		}
	}
	if count == 0 {
		return 0, 0, 0, 0
	}
	half := uint32(count / 2)
	return uint8((sumR + half) / uint32(count)),
		uint8((sumG + half) / uint32(count)),
		uint8((sumB + half) / uint32(count)),
		uint8((sumA + half) / uint32(count))
}
