//go:build astcenc_native && cgo

package native_test

import (
	"bytes"
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
	astcData, err := os.ReadFile("../../Test/Data/Tiles/ldr.astc")
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
