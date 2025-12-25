package astc_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func decodeConstBlockRGBAF32(t *testing.T, block []byte) [4]float32 {
	t.Helper()
	if len(block) < astc.BlockBytes {
		t.Fatalf("block too small: %d", len(block))
	}

	constU16Prefix := []byte{0xFC, 0xFD, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	constF16Prefix := []byte{0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	isU16 := bytes.Equal(block[:8], constU16Prefix)
	isF16 := bytes.Equal(block[:8], constF16Prefix)
	if !isU16 && !isF16 {
		t.Fatalf("block is not a constant-color block (prefix=%x)", block[:8])
	}

	r := binary.LittleEndian.Uint16(block[8:10])
	g := binary.LittleEndian.Uint16(block[10:12])
	b := binary.LittleEndian.Uint16(block[12:14])
	a := binary.LittleEndian.Uint16(block[14:16])

	var out [4]float32
	if isU16 {
		out[0] = halfToFloat32(unorm16ToSF16(r))
		out[1] = halfToFloat32(unorm16ToSF16(g))
		out[2] = halfToFloat32(unorm16ToSF16(b))
		out[3] = halfToFloat32(unorm16ToSF16(a))
	} else {
		out[0] = halfToFloat32(r)
		out[1] = halfToFloat32(g)
		out[2] = halfToFloat32(b)
		out[3] = halfToFloat32(a)
	}
	return out
}

func TestDecodeRGBAF32_TilesHDR_ConstBlocksMatchPayload(t *testing.T) {
	astcData, err := os.ReadFile("../Test/Data/Tiles/hdr.astc")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	h, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if h.BlockX != 4 || h.BlockY != 4 || h.BlockZ != 1 || h.SizeX != 8 || h.SizeY != 8 || h.SizeZ != 1 {
		t.Fatalf("unexpected header: %s", h.String())
	}

	// The tile is 8x8 with 4x4 blocks => 2x2 blocks.
	blocksX, blocksY, _, total, err := h.BlockCount()
	if err != nil {
		t.Fatalf("BlockCount: %v", err)
	}
	if blocksX != 2 || blocksY != 2 || total != 4 {
		t.Fatalf("unexpected block count: %dx%d total=%d", blocksX, blocksY, total)
	}
	if len(blocks) < total*astc.BlockBytes {
		t.Fatalf("truncated blocks payload: got %d want >= %d", len(blocks), total*astc.BlockBytes)
	}

	wantTL := decodeConstBlockRGBAF32(t, blocks[0*astc.BlockBytes:(0+1)*astc.BlockBytes])
	wantTR := decodeConstBlockRGBAF32(t, blocks[1*astc.BlockBytes:(1+1)*astc.BlockBytes])
	wantBL := decodeConstBlockRGBAF32(t, blocks[2*astc.BlockBytes:(2+1)*astc.BlockBytes])
	wantBR := decodeConstBlockRGBAF32(t, blocks[3*astc.BlockBytes:(3+1)*astc.BlockBytes])

	got, w, hh, err := astc.DecodeRGBAF32WithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("DecodeRGBAF32WithProfile: %v", err)
	}
	if w != 8 || hh != 8 {
		t.Fatalf("unexpected dimensions: %dx%d", w, hh)
	}
	if len(got) != w*hh*4 {
		t.Fatalf("unexpected pix length: %d", len(got))
	}

	type sample struct {
		x, y int
		want [4]float32
	}
	samples := []sample{
		{x: 0, y: 0, want: wantTL},
		{x: 7, y: 0, want: wantTR},
		{x: 0, y: 7, want: wantBL},
		{x: 7, y: 7, want: wantBR},
	}
	for _, s := range samples {
		off := (s.y*w + s.x) * 4
		for c := 0; c < 4; c++ {
			gotV := got[off+c]
			wantV := s.want[c]
			if math.Float32bits(gotV) != math.Float32bits(wantV) {
				t.Fatalf("pixel (%d,%d) mismatch ch=%d: got %08x (%g) want %08x (%g)",
					s.x, s.y, c,
					math.Float32bits(gotV), gotV,
					math.Float32bits(wantV), wantV)
			}
		}
	}
}
