//go:build astcenc_native && cgo

package astc_test

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func approxEqF32(a, b float32) bool {
	const eps = 1e-4
	return math.Abs(float64(a-b)) <= eps
}

func TestConfigInit_MatchesUpstreamNative(t *testing.T) {
	type tc struct {
		profile astc.Profile
		bx      int
		by      int
		bz      int
		quality float32
		flags   astc.Flags
	}

	cases := []tc{
		{profile: astc.ProfileLDR, bx: 4, by: 4, bz: 1, quality: 60, flags: 0},
		{profile: astc.ProfileLDR, bx: 6, by: 6, bz: 1, quality: 50, flags: 0}, // interpolated
		{profile: astc.ProfileLDRSRGB, bx: 12, by: 12, bz: 1, quality: 98, flags: 0},
		{profile: astc.ProfileHDR, bx: 6, by: 6, bz: 1, quality: 60, flags: 0},
		{profile: astc.ProfileLDR, bx: 6, by: 6, bz: 1, quality: 60, flags: astc.FlagUsePerceptual},
		{profile: astc.ProfileLDR, bx: 6, by: 6, bz: 1, quality: 60, flags: astc.FlagMapRGBM},
		{profile: astc.ProfileLDR, bx: 6, by: 6, bz: 1, quality: 60, flags: astc.FlagMapNormal},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("profile=%d block=%dx%dx%d q=%.2f flags=%08x", c.profile, c.bx, c.by, c.bz, c.quality, uint32(c.flags)), func(t *testing.T) {
			got, err := astc.ConfigInit(c.profile, c.bx, c.by, c.bz, c.quality, c.flags)
			if err != nil {
				t.Fatalf("astc.ConfigInit: %v", err)
			}

			wantN, err := native.ConfigInit(c.profile, c.bx, c.by, c.bz, c.quality, native.Flags(c.flags))
			if err != nil {
				t.Fatalf("native.ConfigInit: %v", err)
			}

			if got.Profile != wantN.Profile {
				t.Fatalf("Profile: got %v want %v", got.Profile, wantN.Profile)
			}
			if got.Flags != astc.Flags(wantN.Flags) {
				t.Fatalf("Flags: got %08x want %08x", uint32(got.Flags), uint32(wantN.Flags))
			}
			if got.BlockX != wantN.BlockX || got.BlockY != wantN.BlockY || got.BlockZ != wantN.BlockZ {
				t.Fatalf("Block dims: got %dx%dx%d want %dx%dx%d", got.BlockX, got.BlockY, got.BlockZ, wantN.BlockX, wantN.BlockY, wantN.BlockZ)
			}

			// Float fields.
			gotF := []struct {
				name string
				a    float32
				b    float32
			}{
				{"CWRWeight", got.CWRWeight, wantN.CWRWeight},
				{"CWGWeight", got.CWGWeight, wantN.CWGWeight},
				{"CWBWeight", got.CWBWeight, wantN.CWBWeight},
				{"CWAWeight", got.CWAWeight, wantN.CWAWeight},
				{"RGBMMScale", got.RGBMMScale, wantN.RGBMMScale},
				{"TuneDBLimit", got.TuneDBLimit, wantN.TuneDBLimit},
				{"TuneMSEOvershoot", got.TuneMSEOvershoot, wantN.TuneMSEOvershoot},
				{"Tune2PartitionEarlyOutLimitFactor", got.Tune2PartitionEarlyOutLimitFactor, wantN.Tune2PartitionEarlyOutLimitFactor},
				{"Tune3PartitionEarlyOutLimitFactor", got.Tune3PartitionEarlyOutLimitFactor, wantN.Tune3PartitionEarlyOutLimitFactor},
				{"Tune2PlaneEarlyOutLimitCorrelation", got.Tune2PlaneEarlyOutLimitCorrelation, wantN.Tune2PlaneEarlyOutLimitCorrelation},
				{"TuneSearchMode0Enable", got.TuneSearchMode0Enable, wantN.TuneSearchMode0Enable},
			}
			for _, f := range gotF {
				if !approxEqF32(f.a, f.b) {
					t.Fatalf("%s: got %v want %v", f.name, f.a, f.b)
				}
			}

			// Integer fields.
			gotU := []struct {
				name string
				a    uint32
				b    uint32
			}{
				{"AScaleRadius", got.AScaleRadius, wantN.AScaleRadius},
				{"TunePartitionCountLimit", got.TunePartitionCountLimit, wantN.TunePartitionCountLimit},
				{"Tune2PartitionIndexLimit", got.Tune2PartitionIndexLimit, wantN.Tune2PartitionIndexLimit},
				{"Tune3PartitionIndexLimit", got.Tune3PartitionIndexLimit, wantN.Tune3PartitionIndexLimit},
				{"Tune4PartitionIndexLimit", got.Tune4PartitionIndexLimit, wantN.Tune4PartitionIndexLimit},
				{"TuneBlockModeLimit", got.TuneBlockModeLimit, wantN.TuneBlockModeLimit},
				{"TuneRefinementLimit", got.TuneRefinementLimit, wantN.TuneRefinementLimit},
				{"TuneCandidateLimit", got.TuneCandidateLimit, wantN.TuneCandidateLimit},
				{"Tune2PartitioningCandidateLimit", got.Tune2PartitioningCandidateLimit, wantN.Tune2PartitioningCandidateLimit},
				{"Tune3PartitioningCandidateLimit", got.Tune3PartitioningCandidateLimit, wantN.Tune3PartitioningCandidateLimit},
				{"Tune4PartitioningCandidateLimit", got.Tune4PartitioningCandidateLimit, wantN.Tune4PartitioningCandidateLimit},
			}
			for _, u := range gotU {
				if u.a != u.b {
					t.Fatalf("%s: got %v want %v", u.name, u.a, u.b)
				}
			}
		})
	}
}

func TestConfigInit_BadDecodeMode_HDR(t *testing.T) {
	if _, err := astc.ConfigInit(astc.ProfileHDR, 6, 6, 1, 60, astc.FlagUseDecodeUNORM8); err == nil {
		t.Fatalf("astc.ConfigInit: got nil error, want error")
	}
	if _, err := native.ConfigInit(astc.ProfileHDR, 6, 6, 1, 60, native.FlagUseDecodeUNORM8); err == nil {
		t.Fatalf("native.ConfigInit: got nil error, want error")
	}
}

func TestCompress_AlphaScaleRadius_TransparentBlock_ConstZero(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	cfgGo, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("astc.ConfigInit: %v", err)
	}
	cfgGo.AScaleRadius = 1

	cfgN, err := native.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("native.ConfigInit: %v", err)
	}
	cfgN.AScaleRadius = 1

	ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
	if err != nil {
		t.Fatalf("astc.ContextAlloc: %v", err)
	}

	ctxN, err := native.ContextAlloc(&cfgN, 1)
	if err != nil {
		t.Fatalf("native.ContextAlloc: %v", err)
	}
	defer ctxN.Close()

	const w, h, d = 4, 4, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 123
		src[i+1] = 45
		src[i+2] = 67
		src[i+3] = 0
	}

	want := astc.EncodeConstBlockRGBA8(0, 0, 0, 0)

	outGo := make([]byte, astc.BlockBytes)
	imgGo := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}
	if err := ctxGo.CompressImage(&imgGo, astc.SwizzleRGBA, outGo, 0); err != nil {
		t.Fatalf("astc.CompressImage: %v", err)
	}
	if !bytes.Equal(outGo, want[:]) {
		t.Fatalf("astc output: got %x want %x", outGo, want[:])
	}

	outN := make([]byte, astc.BlockBytes)
	imgN := native.Image{DimX: w, DimY: h, DimZ: d, DataType: native.TypeU8, DataU8: src}
	if err := ctxN.CompressImage(&imgN, native.SwizzleRGBA, outN, 0); err != nil {
		t.Fatalf("native.CompressImage: %v", err)
	}
	if !bytes.Equal(outN, want[:]) {
		t.Fatalf("native output: got %x want %x", outN, want[:])
	}
}

func TestCompress_AlphaScaleRadius_Radius2_SeesAlphaAtDistance2(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	cfgGo, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("astc.ConfigInit: %v", err)
	}
	cfgGo.AScaleRadius = 2

	cfgN, err := native.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("native.ConfigInit: %v", err)
	}
	cfgN.AScaleRadius = 2

	ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
	if err != nil {
		t.Fatalf("astc.ContextAlloc: %v", err)
	}

	ctxN, err := native.ContextAlloc(&cfgN, 1)
	if err != nil {
		t.Fatalf("native.ContextAlloc: %v", err)
	}
	defer ctxN.Close()

	const w, h, d = 8, 4, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 123
		src[i+1] = 45
		src[i+2] = 67
		src[i+3] = 0
	}

	// Place a single opaque texel at distance 2 from the x=0 block. This should
	// prevent alpha-scale RDO from substituting the block with a constant-zero
	// block when AScaleRadius=2.
	src[(0*w+5)*4+3] = 255

	wantFirst := astc.EncodeConstBlockRGBA8(123, 45, 67, 0)

	outGo := make([]byte, 2*astc.BlockBytes)
	imgGo := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}
	if err := ctxGo.CompressImage(&imgGo, astc.SwizzleRGBA, outGo, 0); err != nil {
		t.Fatalf("astc.CompressImage: %v", err)
	}
	if !bytes.Equal(outGo[:astc.BlockBytes], wantFirst[:]) {
		t.Fatalf("astc block[0]: got %x want %x", outGo[:astc.BlockBytes], wantFirst[:])
	}

	outN := make([]byte, 2*astc.BlockBytes)
	imgN := native.Image{DimX: w, DimY: h, DimZ: d, DataType: native.TypeU8, DataU8: src}
	if err := ctxN.CompressImage(&imgN, native.SwizzleRGBA, outN, 0); err != nil {
		t.Fatalf("native.CompressImage: %v", err)
	}
	if !bytes.Equal(outN[:astc.BlockBytes], wantFirst[:]) {
		t.Fatalf("native block[0]: got %x want %x", outN[:astc.BlockBytes], wantFirst[:])
	}
}

func TestDecompress_SwizzleZ_MatchesNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	cfgGo, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("astc.ConfigInit: %v", err)
	}
	ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
	if err != nil {
		t.Fatalf("astc.ContextAlloc: %v", err)
	}

	cfgN, err := native.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("native.ConfigInit: %v", err)
	}
	ctxN, err := native.ContextAlloc(&cfgN, 1)
	if err != nil {
		t.Fatalf("native.ContextAlloc: %v", err)
	}
	defer ctxN.Close()

	block := astc.EncodeConstBlockRGBA8(64, 0, 0, 192)

	swzGo := astc.Swizzle{R: astc.SwzR, G: astc.SwzA, B: astc.SwzZ, A: astc.Swz1}
	swzN := native.Swizzle{R: native.SwzR, G: native.SwzA, B: native.SwzZ, A: native.Swz1}

	got := make([]byte, 4*4*4)
	want := make([]byte, 4*4*4)

	outGo := astc.Image{DimX: 4, DimY: 4, DimZ: 1, DataType: astc.TypeU8, DataU8: got}
	if err := ctxGo.DecompressImage(block[:], &outGo, swzGo, 0); err != nil {
		t.Fatalf("astc.DecompressImage: %v", err)
	}

	outN := native.Image{DimX: 4, DimY: 4, DimZ: 1, DataType: native.TypeU8, DataU8: want}
	if err := ctxN.DecompressImage(block[:], &outN, swzN, 0); err != nil {
		t.Fatalf("native.DecompressImage: %v", err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("swizzle-z mismatch")
	}
}
