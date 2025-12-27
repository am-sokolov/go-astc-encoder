//go:build astcenc_native && cgo

package astc_test

import (
	"math"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestEncodeRGBAF32_HDRProfiles_DecodeMatchesNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	type tc struct {
		profile astc.Profile
		name    string
	}
	tests := []tc{
		{name: "hdr-rgb-ldr-a", profile: astc.ProfileHDRRGBLDRAlpha},
		{name: "hdr", profile: astc.ProfileHDR},
	}

	const (
		w, h    = 17, 13
		blockX  = 6
		blockY  = 6
		quality = astc.EncodeFast
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pix := make([]float32, w*h*4)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					off := (y*w + x) * 4
					pix[off+0] = float32(x) * 0.25
					pix[off+1] = float32(y) * 0.5
					pix[off+2] = float32(x^y) * 0.1
					if tt.profile == astc.ProfileHDR {
						pix[off+3] = 1.0 + float32(x+y)*0.01
					} else {
						pix[off+3] = float32(x) / float32(w-1)
					}
				}
			}

			astcData, err := astc.EncodeRGBAF32WithProfileAndQuality(pix, w, h, blockX, blockY, tt.profile, quality)
			if err != nil {
				t.Fatalf("EncodeRGBAF32WithProfileAndQuality: %v", err)
			}

			goPix, goW, goH, err := astc.DecodeRGBAF32WithProfile(astcData, tt.profile)
			if err != nil {
				t.Fatalf("astc.DecodeRGBAF32WithProfile: %v", err)
			}
			nPix, nW, nH, nD, err := native.DecodeRGBAF32VolumeWithProfile(astcData, tt.profile)
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

			// Ensure we actually round-trip some HDR values in HDR profile.
			if tt.profile == astc.ProfileHDR {
				var anyGT1 bool
				for i := 0; i < len(goPix); i += 4 {
					if goPix[i+0] > 1 || goPix[i+1] > 1 || goPix[i+2] > 1 || goPix[i+3] > 1 {
						anyGT1 = true
						break
					}
				}
				if !anyGT1 {
					t.Fatalf("expected at least one decoded component > 1.0 for HDR profile")
				}
			}
		})
	}
}
