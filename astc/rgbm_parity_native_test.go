//go:build astcenc_native && cgo

package astc_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestCompress_MapRGBM_HDRMSE_CloseToNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	const (
		blockX  = 6
		blockY  = 6
		blockZ  = 1
		width   = 24
		height  = 24
		depth   = 1
		quality = float32(60)
	)

	cfgGo, err := astc.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, quality, astc.FlagMapRGBM)
	if err != nil {
		t.Fatalf("astc.ConfigInit: %v", err)
	}
	ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
	if err != nil {
		t.Fatalf("astc.ContextAlloc: %v", err)
	}
	defer ctxGo.Close()

	cfgN, err := native.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, quality, native.FlagMapRGBM)
	if err != nil {
		t.Fatalf("native.ConfigInit: %v", err)
	}
	ctxN, err := native.ContextAlloc(&cfgN, 1)
	if err != nil {
		t.Fatalf("native.ContextAlloc: %v", err)
	}
	defer ctxN.Close()

	rnd := rand.New(rand.NewSource(1))
	src := make([]byte, width*height*depth*4)
	ref := make([][3]float64, width*height*depth)
	for i := range ref {
		// Random HDR values in [0, rgbmScale) so RGBM doesn't saturate.
		hdr := [3]float64{
			rnd.Float64() * float64(cfgGo.RGBMMScale) * 0.95,
			rnd.Float64() * float64(cfgGo.RGBMMScale) * 0.95,
			rnd.Float64() * float64(cfgGo.RGBMMScale) * 0.95,
		}

		r8, g8, b8, m8 := encodeRGBM8(hdr, float64(cfgGo.RGBMMScale))
		src[i*4+0] = r8
		src[i*4+1] = g8
		src[i*4+2] = b8
		src[i*4+3] = m8
		ref[i] = hdr
	}

	imgGo := &astc.Image{DimX: width, DimY: height, DimZ: depth, DataType: astc.TypeU8, DataU8: src}
	imgN := &native.Image{DimX: width, DimY: height, DimZ: depth, DataType: native.TypeU8, DataU8: src}

	blocksX := (width + blockX - 1) / blockX
	blocksY := (height + blockY - 1) / blockY
	outLen := blocksX * blocksY * astc.BlockBytes
	outGo := make([]byte, outLen)
	outN := make([]byte, outLen)

	swzGo := astc.SwizzleRGBA
	swzNative := native.Swizzle{R: native.SwzR, G: native.SwzG, B: native.SwzB, A: native.SwzA}

	if err := ctxGo.CompressImage(imgGo, swzGo, outGo, 0); err != nil {
		t.Fatalf("astc compress: %v", err)
	}
	if err := ctxN.CompressImage(imgN, swzNative, outN, 0); err != nil {
		t.Fatalf("native compress: %v", err)
	}

	hdr := astc.Header{
		BlockX: uint8(blockX),
		BlockY: uint8(blockY),
		BlockZ: uint8(blockZ),
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  uint32(depth),
	}

	decGo := make([]byte, width*height*depth*4)
	decN := make([]byte, width*height*depth*4)
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, hdr, outGo, decGo); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto(go): %v", err)
	}
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, hdr, outN, decN); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto(native): %v", err)
	}

	meanGo := meanRGBMMSE(ref, decGo, float64(cfgGo.RGBMMScale))
	meanN := meanRGBMMSE(ref, decN, float64(cfgGo.RGBMMScale))

	if meanN == 0 {
		if meanGo != 0 {
			t.Fatalf("rgbm mse mismatch: go=%g native=%g", meanGo, meanN)
		}
		return
	}

	if meanGo > meanN*1.6+1e-12 {
		t.Fatalf("rgbm mse too high: go=%g native=%g", meanGo, meanN)
	}
}

func encodeRGBM8(hdr [3]float64, scale float64) (r, g, b, m byte) {
	maxRGB := hdr[0]
	if hdr[1] > maxRGB {
		maxRGB = hdr[1]
	}
	if hdr[2] > maxRGB {
		maxRGB = hdr[2]
	}

	var mByte int
	if maxRGB > 0 && scale > 0 {
		mByte = int(math.Ceil(maxRGB / scale * 255.0))
	}
	if mByte < 1 {
		mByte = 1
	} else if mByte > 255 {
		mByte = 255
	}

	mNorm := float64(mByte) / 255.0
	denom := mNorm * scale
	if denom <= 0 {
		denom = 1
	}

	rf := hdr[0] / denom
	gf := hdr[1] / denom
	bf := hdr[2] / denom

	return roundToU8(rf * 255.0),
		roundToU8(gf * 255.0),
		roundToU8(bf * 255.0),
		byte(mByte)
}

func roundToU8(v float64) byte {
	if !(v >= 0) {
		v = 0
	}
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return byte(math.Round(v))
}

func meanRGBMMSE(ref [][3]float64, decodedRGBA8 []byte, scale float64) float64 {
	if len(decodedRGBA8) != len(ref)*4 {
		panic("meanRGBMMSE: length mismatch")
	}
	var sum float64
	for i, hdr := range ref {
		off := i * 4
		mNorm := float64(decodedRGBA8[off+3]) / 255.0
		decR := float64(decodedRGBA8[off+0]) / 255.0 * mNorm * scale
		decG := float64(decodedRGBA8[off+1]) / 255.0 * mNorm * scale
		decB := float64(decodedRGBA8[off+2]) / 255.0 * mNorm * scale

		dr := hdr[0] - decR
		dg := hdr[1] - decG
		db := hdr[2] - decB
		sum += dr*dr + dg*dg + db*db
	}

	return sum / float64(len(ref)*3)
}
