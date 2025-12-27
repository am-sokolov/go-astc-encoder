//go:build astcenc_native && cgo

package astc_test

import (
	"math"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestEncodeRGBAF32_HDR_MSE_CloseToNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	type tc struct {
		name    string
		profile astc.Profile
	}
	tests := []tc{
		{name: "hdr", profile: astc.ProfileHDR},
		{name: "hdr-rgb-ldr-a", profile: astc.ProfileHDRRGBLDRAlpha},
	}

	const (
		blockX  = 6
		blockY  = 6
		blockZ  = 1
		width   = 24
		height  = 24
		depth   = 1
		quality = astc.EncodeMedium
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pix := make([]float32, width*height*depth*4)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					off := (y*width + x) * 4

					// Deterministic HDR-ish pattern that exercises values > 1.
					pix[off+0] = float32(x) * 0.25
					pix[off+1] = float32(y) * 0.5
					pix[off+2] = float32((x^y)&7) * 1.1

					if tt.profile == astc.ProfileHDR {
						pix[off+3] = 1.0 + float32(x+y)*0.02
					} else {
						pix[off+3] = float32(x) / float32(width-1)
					}
				}
			}

			goData, err := astc.EncodeRGBAF32WithProfileAndQuality(pix, width, height, blockX, blockY, tt.profile, quality)
			if err != nil {
				t.Fatalf("astc.EncodeRGBAF32WithProfileAndQuality: %v", err)
			}

			encN, err := native.NewEncoderF32(blockX, blockY, blockZ, tt.profile, quality, 1)
			if err != nil {
				t.Fatalf("native.NewEncoderF32: %v", err)
			}
			defer encN.Close()
			nData, err := encN.EncodeRGBAF32Volume(pix, width, height, depth)
			if err != nil {
				t.Fatalf("native.EncodeRGBAF32Volume: %v", err)
			}

			goDec, w, h, err := astc.DecodeRGBAF32WithProfile(goData, tt.profile)
			if err != nil {
				t.Fatalf("astc.DecodeRGBAF32WithProfile(go): %v", err)
			}
			nDec, nw, nh, err := astc.DecodeRGBAF32WithProfile(nData, tt.profile)
			if err != nil {
				t.Fatalf("astc.DecodeRGBAF32WithProfile(native): %v", err)
			}
			if w != nw || h != nh || w != width || h != height {
				t.Fatalf("dimension mismatch: go=%dx%d native=%dx%d src=%dx%d", w, h, nw, nh, width, height)
			}
			if len(goDec) != len(pix) || len(nDec) != len(pix) {
				t.Fatalf("decode length mismatch: go=%d native=%d src=%d", len(goDec), len(nDec), len(pix))
			}

			codeMSEGo := meanHDRCodeMSE(tt.profile, pix, goDec)
			codeMSEN := meanHDRCodeMSE(tt.profile, pix, nDec)

			if codeMSEN == 0 {
				if codeMSEGo != 0 {
					t.Fatalf("code mse mismatch: go=%g native=%g", codeMSEGo, codeMSEN)
				}
				return
			}

			// The pure-Go encoder isn't bit-exact, but should remain in the same ballpark as native.
			// Use a loose tolerance until we close the remaining search-space gaps.
			if codeMSEGo > codeMSEN*2.5+1e-12 {
				t.Fatalf("code mse too high: go=%g native=%g", codeMSEGo, codeMSEN)
			}
		})
	}
}

func meanHDRCodeMSE(profile astc.Profile, ref, dec []float32) float64 {
	if len(ref) != len(dec) {
		panic("meanHDRCodeMSE: length mismatch")
	}
	var sum float64
	for i := 0; i < len(ref); i += 4 {
		for c := 0; c < 4; c++ {
			var a, b uint16
			if c == 3 && profile == astc.ProfileHDRRGBLDRAlpha {
				a = floatToUNorm16(ref[i+c])
				b = floatToUNorm16(dec[i+c])
			} else {
				a = floatToLNS(ref[i+c])
				b = floatToLNS(dec[i+c])
			}

			d := float64(int32(a) - int32(b))
			if math.IsNaN(d) || math.IsInf(d, 0) {
				continue
			}
			sum += d * d
		}
	}
	return sum / float64(len(ref))
}

func floatToUNorm16(v float32) uint16 {
	if !(v >= 0) {
		v = 0
	}
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 0xFFFF
	}
	return uint16(v*65535.0 + 0.5)
}

func floatToLNS(v float32) uint16 {
	// Copy of astc.hdrTexelToLNS(), for black-box parity testing.
	if !(v > (1.0 / 67108864.0)) { // Underflow/NaN/negative
		return 0
	}
	if v >= 65536.0 {
		return 0xFFFF
	}

	a := v
	mant, exp := math.Frexp(float64(a))

	// If input is smaller than 2^-14, multiply by 2^25 and don't bias.
	if exp < -13 {
		a = float32(a * 33554432.0)
		exp = 0
	} else {
		a = float32((mant - 0.5) * 4096.0)
		exp = exp + 14
	}

	if a < 384.0 {
		a = a * (4.0 / 3.0)
	} else if a <= 1408.0 {
		a = a + 128.0
	} else {
		a = (a + 512.0) * (4.0 / 5.0)
	}

	a = a + float32(exp)*2048.0 + 1.0
	if a <= 0 {
		return 0
	}
	if a >= 65535.0 {
		return 0xFFFF
	}
	return uint16(a + 0.5)
}
