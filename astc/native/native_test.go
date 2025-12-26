//go:build astcenc_native && cgo

package native_test

import (
	"bytes"
	"math"
	"os"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestEnabled(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}
}

func TestDecodeRGBA8_MatchesPureGo_TilesLDR(t *testing.T) {
	astcData, err := os.ReadFile("../testdata/fixtures/Tiles/ldr.astc")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	gotPix, gotW, gotH, err := native.DecodeRGBA8(astcData)
	if err != nil {
		t.Fatalf("native.DecodeRGBA8: %v", err)
	}

	wantPix, wantW, wantH, err := astc.DecodeRGBA8(astcData)
	if err != nil {
		t.Fatalf("astc.DecodeRGBA8: %v", err)
	}

	if gotW != wantW || gotH != wantH {
		t.Fatalf("dimension mismatch: got %dx%d want %dx%d", gotW, gotH, wantW, wantH)
	}
	if !bytes.Equal(gotPix, wantPix) {
		t.Fatalf("decoded pixel mismatch")
	}
}

func TestEncodeRGBA8_RoundTripConst2D(t *testing.T) {
	const (
		w = 4
		h = 4
	)
	src := make([]byte, w*h*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 10
		src[i+1] = 20
		src[i+2] = 30
		src[i+3] = 40
	}

	astcData, err := native.EncodeRGBA8WithProfileAndQuality(src, w, h, 4, 4, astc.ProfileLDR, astc.EncodeMedium)
	if err != nil {
		t.Fatalf("native.EncodeRGBA8WithProfileAndQuality: %v", err)
	}

	dst, w2, h2, err := astc.DecodeRGBA8(astcData)
	if err != nil {
		t.Fatalf("astc.DecodeRGBA8: %v", err)
	}
	if w2 != w || h2 != h {
		t.Fatalf("unexpected dimensions: got %dx%d want %dx%d", w2, h2, w, h)
	}
	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestEncodeRGBA8_RoundTripConst3D(t *testing.T) {
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

	astcData, err := native.EncodeRGBA8VolumeWithProfileAndQuality(src, w, h, d, 4, 4, 4, astc.ProfileLDR, astc.EncodeMedium)
	if err != nil {
		t.Fatalf("native.EncodeRGBA8VolumeWithProfileAndQuality: %v", err)
	}

	dst, w2, h2, d2, err := astc.DecodeRGBA8VolumeWithProfile(astcData, astc.ProfileLDR)
	if err != nil {
		t.Fatalf("astc.DecodeRGBA8VolumeWithProfile: %v", err)
	}
	if w2 != w || h2 != h || d2 != d {
		t.Fatalf("unexpected dimensions: got %dx%dx%d want %dx%dx%d", w2, h2, d2, w, h, d)
	}
	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestEncodeRGBAF32_MatchesPureGoDecode_HDR2D(t *testing.T) {
	const (
		w      = 11
		h      = 9
		blockX = 6
		blockY = 6
	)

	pix := make([]float32, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			pix[off+0] = float32(x) * 0.25
			pix[off+1] = float32(y) * 0.5
			pix[off+2] = float32(x^y) * 0.1
			pix[off+3] = 1.0 + float32(x+y)*0.01
		}
	}

	enc, err := native.NewEncoderF32(blockX, blockY, 1, astc.ProfileHDR, astc.EncodeFast, 1)
	if err != nil {
		t.Fatalf("native.NewEncoderF32: %v", err)
	}
	defer enc.Close()

	astcData, err := enc.EncodeRGBAF32(pix, w, h)
	if err != nil {
		t.Fatalf("enc.EncodeRGBAF32: %v", err)
	}

	goPix, goW, goH, err := astc.DecodeRGBAF32WithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("astc.DecodeRGBAF32WithProfile: %v", err)
	}
	nPix, nW, nH, nD, err := native.DecodeRGBAF32VolumeWithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("native.DecodeRGBAF32VolumeWithProfile: %v", err)
	}
	if nD != 1 {
		t.Fatalf("native decode returned depth=%d; want 1", nD)
	}
	if goW != nW || goH != nH || goW != w || goH != h {
		t.Fatalf("dimension mismatch: go=%dx%d native=%dx%d src=%dx%d", goW, goH, nW, nH, w, h)
	}

	for i := range goPix {
		ga := goPix[i]
		gb := nPix[i]
		if math.Float32bits(ga) != math.Float32bits(gb) {
			if math.IsNaN(float64(ga)) && math.IsNaN(float64(gb)) {
				continue
			}
			t.Fatalf("float mismatch at %d: got %08x (%g) want %08x (%g)", i, math.Float32bits(ga), ga, math.Float32bits(gb), gb)
		}
	}
}

func TestEncodeRGBAF32_MatchesPureGoDecode_HDRVolume(t *testing.T) {
	const (
		w      = 5
		h      = 3
		d      = 4
		blockX = 4
		blockY = 4
		blockZ = 3
	)

	pix := make([]float32, w*h*d*4)
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				off := ((z*h+y)*w + x) * 4
				pix[off+0] = float32(x) * 0.25
				pix[off+1] = float32(y) * 0.5
				pix[off+2] = float32(z) * 1.25
				pix[off+3] = 1.0 + float32(x+y+z)*0.01
			}
		}
	}

	enc, err := native.NewEncoderF32(blockX, blockY, blockZ, astc.ProfileHDR, astc.EncodeFast, 1)
	if err != nil {
		t.Fatalf("native.NewEncoderF32: %v", err)
	}
	defer enc.Close()

	astcData, err := enc.EncodeRGBAF32Volume(pix, w, h, d)
	if err != nil {
		t.Fatalf("enc.EncodeRGBAF32Volume: %v", err)
	}

	goPix, goW, goH, goD, err := astc.DecodeRGBAF32VolumeWithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("astc.DecodeRGBAF32VolumeWithProfile: %v", err)
	}
	nPix, nW, nH, nD, err := native.DecodeRGBAF32VolumeWithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("native.DecodeRGBAF32VolumeWithProfile: %v", err)
	}
	if goW != nW || goH != nH || goD != nD || goW != w || goH != h || goD != d {
		t.Fatalf("dimension mismatch: go=%dx%dx%d native=%dx%dx%d src=%dx%dx%d", goW, goH, goD, nW, nH, nD, w, h, d)
	}

	for i := range goPix {
		ga := goPix[i]
		gb := nPix[i]
		if math.Float32bits(ga) != math.Float32bits(gb) {
			if math.IsNaN(float64(ga)) && math.IsNaN(float64(gb)) {
				continue
			}
			t.Fatalf("float mismatch at %d: got %08x (%g) want %08x (%g)", i, math.Float32bits(ga), ga, math.Float32bits(gb), gb)
		}
	}
}
