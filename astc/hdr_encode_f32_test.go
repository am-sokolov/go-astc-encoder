package astc_test

import (
	"bytes"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestEncodeRGBAF32_HDRProfiles_ConstBlocksRoundTrip(t *testing.T) {
	type tc struct {
		profile astc.Profile
		name    string
		r, g, b float32
		a       float32
	}
	tests := []tc{
		{name: "hdr-rgb-ldr-a", profile: astc.ProfileHDRRGBLDRAlpha, r: 2.0, g: 0.5, b: 10.0, a: 0.75},
		{name: "hdr", profile: astc.ProfileHDR, r: 2.0, g: 0.5, b: 10.0, a: 1.5},
	}

	const (
		w, h    = 4, 4
		blockX  = 4
		blockY  = 4
		quality = astc.EncodeMedium
	)

	constF16Prefix := []byte{0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pix := make([]float32, w*h*4)
			for i := 0; i < len(pix); i += 4 {
				pix[i+0] = tt.r
				pix[i+1] = tt.g
				pix[i+2] = tt.b
				pix[i+3] = tt.a
			}

			astcData, err := astc.EncodeRGBAF32WithProfileAndQuality(pix, w, h, blockX, blockY, tt.profile, quality)
			if err != nil {
				t.Fatalf("EncodeRGBAF32WithProfileAndQuality: %v", err)
			}

			hdr, blocks, err := astc.ParseFile(astcData)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if int(hdr.SizeX) != w || int(hdr.SizeY) != h || int(hdr.SizeZ) != 1 {
				t.Fatalf("unexpected header size: %dx%dx%d", hdr.SizeX, hdr.SizeY, hdr.SizeZ)
			}
			if int(hdr.BlockX) != blockX || int(hdr.BlockY) != blockY || int(hdr.BlockZ) != 1 {
				t.Fatalf("unexpected block size: %dx%dx%d", hdr.BlockX, hdr.BlockY, hdr.BlockZ)
			}
			if len(blocks) != astc.BlockBytes {
				t.Fatalf("unexpected block payload size: got %d want %d", len(blocks), astc.BlockBytes)
			}
			if !bytes.Equal(blocks[:8], constF16Prefix) {
				t.Fatalf("expected FP16 constant-color block prefix; got %x", blocks[:8])
			}

			want := decodeConstBlockRGBAF32(t, blocks[:astc.BlockBytes])
			got, w2, h2, err := astc.DecodeRGBAF32WithProfile(astcData, tt.profile)
			if err != nil {
				t.Fatalf("DecodeRGBAF32WithProfile: %v", err)
			}
			if w2 != w || h2 != h {
				t.Fatalf("unexpected decode dimensions: got %dx%d want %dx%d", w2, h2, w, h)
			}
			if len(got) != w*h*4 {
				t.Fatalf("unexpected decode buffer length: got %d want %d", len(got), w*h*4)
			}
			for i := 0; i < len(got); i += 4 {
				for c := 0; c < 4; c++ {
					if got[i+c] != want[c] {
						t.Fatalf("pixel mismatch at %d ch=%d: got=%v want=%v", i/4, c, got[i+c], want[c])
					}
				}
			}
		})
	}
}

func TestEncodeRGBAF32_HDR_EncodeDecode_Sane(t *testing.T) {
	const (
		w, h    = 11, 9
		blockX  = 6
		blockY  = 6
		quality = astc.EncodeFastest
	)

	pix := make([]float32, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			pix[off+0] = float32(x) * 0.25
			pix[off+1] = float32(y) * 0.5
			pix[off+2] = float32(x^y) * 0.1
			pix[off+3] = 1.0
		}
	}

	astcData, err := astc.EncodeRGBAF32WithProfileAndQuality(pix, w, h, blockX, blockY, astc.ProfileHDR, quality)
	if err != nil {
		t.Fatalf("EncodeRGBAF32WithProfileAndQuality: %v", err)
	}
	got, w2, h2, err := astc.DecodeRGBAF32WithProfile(astcData, astc.ProfileHDR)
	if err != nil {
		t.Fatalf("DecodeRGBAF32WithProfile: %v", err)
	}
	if w2 != w || h2 != h {
		t.Fatalf("unexpected decode dimensions: got %dx%d want %dx%d", w2, h2, w, h)
	}
	if len(got) != w*h*4 {
		t.Fatalf("unexpected decode buffer length: got %d want %d", len(got), w*h*4)
	}
}
