package astc_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/bits"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestDecodeRGBA8Volume_3D_ConstBlock(t *testing.T) {
	h := astc.Header{
		BlockX: 4,
		BlockY: 4,
		BlockZ: 4,
		SizeX:  4,
		SizeY:  4,
		SizeZ:  4,
	}
	hdr, err := astc.MarshalHeader(h)
	if err != nil {
		t.Fatalf("MarshalHeader: %v", err)
	}
	block := astc.EncodeConstBlockRGBA8(10, 20, 30, 40)
	astcData := append(hdr[:], block[:]...)

	pix, w, h2, d, err := astc.DecodeRGBA8VolumeWithProfile(astcData, astc.ProfileLDR)
	if err != nil {
		t.Fatalf("DecodeRGBA8VolumeWithProfile: %v", err)
	}
	if w != 4 || h2 != 4 || d != 4 {
		t.Fatalf("unexpected dimensions: %dx%dx%d", w, h2, d)
	}
	if len(pix) != w*h2*d*4 {
		t.Fatalf("unexpected pix length: %d", len(pix))
	}

	samples := [][3]int{
		{0, 0, 0},
		{3, 0, 0},
		{0, 3, 0},
		{3, 3, 0},
		{0, 0, 3},
		{3, 3, 3},
		{2, 1, 3},
	}
	for _, s := range samples {
		x, y, z := s[0], s[1], s[2]
		off := ((z*h2+y)*w + x) * 4
		if pix[off+0] != 10 || pix[off+1] != 20 || pix[off+2] != 30 || pix[off+3] != 40 {
			t.Fatalf("pixel (%d,%d,%d) mismatch: got (%d,%d,%d,%d) want (10,20,30,40)",
				x, y, z,
				pix[off+0], pix[off+1], pix[off+2], pix[off+3])
		}
	}

	// The 2D API should reject 3D images.
	if _, _, _, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR); err == nil {
		t.Fatalf("DecodeRGBA8WithProfile unexpectedly accepted a 3D image")
	}
}

func TestDecodeRGBA8VolumeFromParsedWithProfileInto_MatchesDecode(t *testing.T) {
	h := astc.Header{
		BlockX: 4,
		BlockY: 4,
		BlockZ: 4,
		SizeX:  4,
		SizeY:  4,
		SizeZ:  4,
	}
	hdr, err := astc.MarshalHeader(h)
	if err != nil {
		t.Fatalf("MarshalHeader: %v", err)
	}
	block := astc.EncodeConstBlockRGBA8(10, 20, 30, 40)
	astcData := append(hdr[:], block[:]...)

	parsedH, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	dst1 := make([]byte, 4*4*4*4)
	if _, _, _, err := astc.DecodeRGBA8VolumeWithProfileInto(astcData, astc.ProfileLDR, dst1); err != nil {
		t.Fatalf("DecodeRGBA8VolumeWithProfileInto: %v", err)
	}

	dst2 := make([]byte, len(dst1))
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, parsedH, blocks, dst2); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto: %v", err)
	}

	if !bytes.Equal(dst1, dst2) {
		t.Fatalf("decoded output mismatch")
	}
}

func TestEncodeRGBA8Volume_RoundTripConst(t *testing.T) {
	const (
		w = 4
		h = 4
		d = 4
	)
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 10
		src[i+1] = 20
		src[i+2] = 30
		src[i+3] = 40
	}

	astcData, err := astc.EncodeRGBA8Volume(src, w, h, d, 4, 4, 4)
	if err != nil {
		t.Fatalf("EncodeRGBA8Volume: %v", err)
	}

	dst, w2, h2, d2, err := astc.DecodeRGBA8VolumeWithProfile(astcData, astc.ProfileLDR)
	if err != nil {
		t.Fatalf("DecodeRGBA8VolumeWithProfile: %v", err)
	}
	if w2 != w || h2 != h || d2 != d {
		t.Fatalf("unexpected dimensions: %dx%dx%d", w2, h2, d2)
	}
	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestDecodeRGBAF32_HDR_A_1x1_ConstBlockMatchesPayload(t *testing.T) {
	astcData := mustReadFile(t, "testdata/fixtures/HDR-A-1x1.astc")

	h, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if h.SizeX != 1 || h.SizeY != 1 || h.SizeZ != 1 {
		t.Fatalf("unexpected dimensions: %dx%dx%d", h.SizeX, h.SizeY, h.SizeZ)
	}
	if len(blocks) < astc.BlockBytes {
		t.Fatalf("missing first block payload")
	}

	block := blocks[:astc.BlockBytes]
	constU16Prefix := []byte{0xFC, 0xFD, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	constF16Prefix := []byte{0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	isU16 := bytes.Equal(block[:8], constU16Prefix)
	isF16 := bytes.Equal(block[:8], constF16Prefix)
	if !isU16 && !isF16 {
		t.Fatalf("unexpected block prefix: %x", block[:8])
	}

	r := binary.LittleEndian.Uint16(block[8:10])
	g := binary.LittleEndian.Uint16(block[10:12])
	b := binary.LittleEndian.Uint16(block[12:14])
	a := binary.LittleEndian.Uint16(block[14:16])

	var want [4]float32
	if isU16 {
		want[0] = halfToFloat32(unorm16ToSF16(r))
		want[1] = halfToFloat32(unorm16ToSF16(g))
		want[2] = halfToFloat32(unorm16ToSF16(b))
		want[3] = halfToFloat32(unorm16ToSF16(a))
	} else {
		want[0] = halfToFloat32(r)
		want[1] = halfToFloat32(g)
		want[2] = halfToFloat32(b)
		want[3] = halfToFloat32(a)
	}

	pix, w, h2, err := astc.DecodeRGBAF32WithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("DecodeRGBAF32WithProfile: %v", err)
	}
	if w != 1 || h2 != 1 {
		t.Fatalf("unexpected dimensions: %dx%d", w, h2)
	}
	if len(pix) != 4 {
		t.Fatalf("unexpected pix length: %d", len(pix))
	}
	for i := 0; i < 4; i++ {
		if pix[i] != want[i] {
			t.Fatalf("pixel mismatch: got (%g,%g,%g,%g) want (%g,%g,%g,%g)",
				pix[0], pix[1], pix[2], pix[3],
				want[0], want[1], want[2], want[3])
		}
	}
}

func TestDecodeRGBAF32VolumeFromParsedWithProfileInto_MatchesDecode(t *testing.T) {
	astcData := mustReadFile(t, "testdata/fixtures/HDR-A-1x1.astc")

	parsedH, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}

	dst1 := make([]float32, 4)
	if _, _, _, err := astc.DecodeRGBAF32VolumeWithProfileInto(astcData, astc.ProfileHDR, dst1); err != nil {
		t.Fatalf("DecodeRGBAF32VolumeWithProfileInto: %v", err)
	}

	dst2 := make([]float32, len(dst1))
	if err := astc.DecodeRGBAF32VolumeFromParsedWithProfileInto(astc.ProfileHDR, parsedH, blocks, dst2); err != nil {
		t.Fatalf("DecodeRGBAF32VolumeFromParsedWithProfileInto: %v", err)
	}

	for i := range dst1 {
		if math.Float32bits(dst1[i]) != math.Float32bits(dst2[i]) {
			t.Fatalf("pixel mismatch at %d: got %v want %v", i, dst2[i], dst1[i])
		}
	}
}

// halfToFloat32 converts an IEEE 754 binary16 float to float32.
func halfToFloat32(h uint16) float32 {
	sign := uint32(h>>15) & 0x1
	exp := uint32(h>>10) & 0x1F
	mant := uint32(h) & 0x3FF

	switch exp {
	case 0:
		if mant == 0 {
			return math.Float32frombits(sign << 31)
		}
		// Subnormal -> normalized float32.
		e := int32(-14)
		for (mant & 0x400) == 0 {
			mant <<= 1
			e--
		}
		mant &= 0x3FF
		exp32 := uint32(e + 127)
		mant32 := mant << 13
		return math.Float32frombits((sign << 31) | (exp32 << 23) | mant32)
	case 0x1F:
		// Inf/NaN
		return math.Float32frombits((sign << 31) | 0x7F800000 | (mant << 13))
	default:
		// Normal number.
		exp32 := exp + (127 - 15)
		mant32 := mant << 13
		return math.Float32frombits((sign << 31) | (exp32 << 23) | mant32)
	}
}

// unorm16ToSF16 converts an unorm16 value to a float16 bit pattern.
func unorm16ToSF16(p uint16) uint16 {
	if p == 0xFFFF {
		return 0x3C00 // FP16 1.0
	}
	if p < 4 {
		return p << 8
	}

	lz := bits.LeadingZeros32(uint32(p)) - 16
	if lz < 0 {
		lz = 0
	} else if lz > 32 {
		lz = 32
	}

	p32 := uint32(p) * (1 << uint(lz+1))
	p32 &= 0xFFFF
	p32 >>= 6

	exp := uint32(14 - lz)
	p32 |= exp << 10
	return uint16(p32)
}
