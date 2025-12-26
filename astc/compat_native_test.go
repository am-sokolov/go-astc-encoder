//go:build astcenc_native && cgo

package astc_test

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestDecodeRGBA8_MatchesNative_ForRepoFixtures(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	type fixture struct {
		path    string
		profile astc.Profile
	}
	fixtures := []fixture{
		{path: "testdata/fixtures/Tiles/ldr.astc", profile: astc.ProfileLDR},
		{path: "testdata/fixtures/LDR-A-1x1.astc", profile: astc.ProfileLDR},
		{path: "testdata/fixtures/LDRS-A-1x1.astc", profile: astc.ProfileLDRSRGB},
		// This file contains HDR constant blocks; LDR profile should decode to magenta error pixels.
		{path: "testdata/fixtures/Tiles/hdr.astc", profile: astc.ProfileLDR},
	}

	for _, tc := range fixtures {
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			h, blocks, err := astc.ParseFile(data)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}

			w := int(h.SizeX)
			hh := int(h.SizeY)
			d := int(h.SizeZ)

			got := make([]byte, w*hh*d*4)
			want := make([]byte, w*hh*d*4)

			if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(tc.profile, h, blocks, got); err != nil {
				t.Fatalf("astc.DecodeRGBA8VolumeFromParsedWithProfileInto: %v", err)
			}
			if err := native.DecodeRGBA8VolumeFromParsedWithProfileInto(tc.profile, h, blocks, want); err != nil {
				t.Fatalf("native.DecodeRGBA8VolumeFromParsedWithProfileInto: %v", err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("decoded pixel mismatch")
			}
		})
	}
}

func TestDecodeRGBAF32_MatchesNative_ForRepoFixtures(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	type fixture struct {
		path    string
		profile astc.Profile
	}
	fixtures := []fixture{
		{path: "testdata/fixtures/HDR-A-1x1.astc", profile: astc.ProfileHDRRGBLDRAlpha},
		{path: "testdata/fixtures/HDR-A-1x1.astc", profile: astc.ProfileHDR},
		{path: "testdata/fixtures/Tiles/hdr.astc", profile: astc.ProfileHDR},
	}

	for _, tc := range fixtures {
		t.Run(fmt.Sprintf("%s/%d", tc.path, tc.profile), func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}

			h, blocks, err := astc.ParseFile(data)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}

			w := int(h.SizeX)
			hh := int(h.SizeY)
			d := int(h.SizeZ)

			got := make([]float32, w*hh*d*4)
			want := make([]float32, w*hh*d*4)

			if err := astc.DecodeRGBAF32VolumeFromParsedWithProfileInto(tc.profile, h, blocks, got); err != nil {
				t.Fatalf("astc.DecodeRGBAF32VolumeFromParsedWithProfileInto: %v", err)
			}
			if err := native.DecodeRGBAF32VolumeFromParsedWithProfileInto(tc.profile, h, blocks, want); err != nil {
				t.Fatalf("native.DecodeRGBAF32VolumeFromParsedWithProfileInto: %v", err)
			}

			for i := range got {
				ga := got[i]
				gb := want[i]
				if math.Float32bits(ga) != math.Float32bits(gb) {
					// Treat NaNs as equal; different NaN payloads are not semantically meaningful.
					if math.IsNaN(float64(ga)) && math.IsNaN(float64(gb)) {
						continue
					}
					t.Fatalf("float mismatch at %d: got %08x (%g) want %08x (%g)", i, math.Float32bits(ga), ga, math.Float32bits(gb), gb)
				}
			}
		})
	}
}

func TestDecodeRGBA8_MatchesNative_ForRandomNativeEncodes(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	type blockSize struct {
		bx, by, bz int
	}
	blockSizes := []blockSize{
		{4, 4, 1},
		{5, 4, 1},
		{6, 6, 1},
		{8, 5, 1},
		{10, 6, 1},
		{12, 12, 1},
		{4, 4, 4},
		{6, 5, 5},
	}

	qualities := []astc.EncodeQuality{
		astc.EncodeFastest,
		astc.EncodeMedium,
		astc.EncodeThorough,
	}

	profiles := []astc.Profile{
		astc.ProfileLDR,
		astc.ProfileLDRSRGB,
	}

	rnd := rand.New(rand.NewSource(1))

	// Keep the test runtime bounded; thorough is expensive, so only use it on tiny images.
	maxCasesPerConfig := 4
	if v := os.Getenv("ASTC_COMPAT_CASES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxCasesPerConfig = n
		}
	}

	for _, bs := range blockSizes {
		for _, prof := range profiles {
			for _, q := range qualities {
				if q == astc.EncodeThorough && (bs.bx*bs.by*bs.bz > 64) {
					continue
				}

				t.Run(fmt.Sprintf("b=%dx%dx%d prof=%d q=%d", bs.bx, bs.by, bs.bz, prof, q), func(t *testing.T) {
					enc, err := native.NewEncoder(bs.bx, bs.by, bs.bz, prof, q, 1)
					if err != nil {
						t.Fatalf("native.NewEncoder: %v", err)
					}
					defer enc.Close()

					for i := 0; i < maxCasesPerConfig; i++ {
						w := 1 + rnd.Intn(bs.bx*3+1)
						hh := 1 + rnd.Intn(bs.by*3+1)
						d := 1
						if bs.bz > 1 {
							d = 1 + rnd.Intn(bs.bz*3+1)
						}

						pix := make([]byte, w*hh*d*4)
						_, _ = rnd.Read(pix)

						astcData, err := enc.EncodeRGBA8Volume(pix, w, hh, d)
						if err != nil {
							t.Fatalf("native encode: %v", err)
						}

						hdr, blocks, err := astc.ParseFile(astcData)
						if err != nil {
							t.Fatalf("ParseFile: %v", err)
						}

						dstGo := make([]byte, w*hh*d*4)
						dstNative := make([]byte, w*hh*d*4)

						if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(prof, hdr, blocks, dstGo); err != nil {
							t.Fatalf("go decode: %v", err)
						}
						if err := native.DecodeRGBA8VolumeFromParsedWithProfileInto(prof, hdr, blocks, dstNative); err != nil {
							t.Fatalf("native decode: %v", err)
						}

						if !bytes.Equal(dstGo, dstNative) {
							t.Fatalf("decoded pixel mismatch (case %d, size=%dx%dx%d)", i, w, hh, d)
						}
					}
				})
			}
		}
	}
}

func TestDecodeRGBAF32_MatchesNative_ForRandomNativeEncodes(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	type blockSize struct {
		bx, by, bz int
	}
	blockSizes := []blockSize{
		{4, 4, 1},
		{5, 4, 1},
		{6, 6, 1},
		{8, 5, 1},
		{10, 6, 1},
		{12, 12, 1},
		{4, 4, 4},
		{6, 5, 5},
	}

	qualities := []astc.EncodeQuality{
		astc.EncodeFastest,
		astc.EncodeMedium,
	}

	profiles := []astc.Profile{
		astc.ProfileHDRRGBLDRAlpha,
		astc.ProfileHDR,
	}

	rnd := rand.New(rand.NewSource(2))

	// Keep the test runtime bounded; HDR medium is more expensive than LDR.
	maxCasesPerConfig := 3
	if v := os.Getenv("ASTC_COMPAT_CASES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxCasesPerConfig = n
		}
	}

	for _, bs := range blockSizes {
		for _, prof := range profiles {
			for _, q := range qualities {
				// Avoid worst-case encode runtimes while still exercising a reasonable variety of blocks.
				if q == astc.EncodeMedium && (bs.bx*bs.by*bs.bz > 80) {
					continue
				}

				t.Run(fmt.Sprintf("b=%dx%dx%d prof=%d q=%d", bs.bx, bs.by, bs.bz, prof, q), func(t *testing.T) {
					enc, err := native.NewEncoder(bs.bx, bs.by, bs.bz, prof, q, 1)
					if err != nil {
						t.Fatalf("native.NewEncoder: %v", err)
					}
					defer enc.Close()

					for i := 0; i < maxCasesPerConfig; i++ {
						w := 1 + rnd.Intn(bs.bx*3+1)
						hh := 1 + rnd.Intn(bs.by*3+1)
						d := 1
						if bs.bz > 1 {
							d = 1 + rnd.Intn(bs.bz*3+1)
						}

						pix := make([]byte, w*hh*d*4)
						_, _ = rnd.Read(pix)

						astcData, err := enc.EncodeRGBA8Volume(pix, w, hh, d)
						if err != nil {
							t.Fatalf("native encode: %v", err)
						}

						hdr, blocks, err := astc.ParseFile(astcData)
						if err != nil {
							t.Fatalf("ParseFile: %v", err)
						}

						dstGo := make([]float32, w*hh*d*4)
						dstNative := make([]float32, w*hh*d*4)

						if err := astc.DecodeRGBAF32VolumeFromParsedWithProfileInto(prof, hdr, blocks, dstGo); err != nil {
							t.Fatalf("go decode: %v", err)
						}
						if err := native.DecodeRGBAF32VolumeFromParsedWithProfileInto(prof, hdr, blocks, dstNative); err != nil {
							t.Fatalf("native decode: %v", err)
						}

						for j := range dstGo {
							ga := dstGo[j]
							gb := dstNative[j]
							if math.Float32bits(ga) != math.Float32bits(gb) {
								// Treat NaNs as equal; different NaN payloads are not semantically meaningful.
								if math.IsNaN(float64(ga)) && math.IsNaN(float64(gb)) {
									continue
								}
								t.Fatalf("float mismatch at %d: got %08x (%g) want %08x (%g) (case %d, size=%dx%dx%d)",
									j, math.Float32bits(ga), ga, math.Float32bits(gb), gb,
									i, w, hh, d)
							}
						}
					}
				})
			}
		}
	}
}
