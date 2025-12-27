//go:build astcenc_native && cgo

package astc_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestCompress_UseDecodeUNORM8_U8SSE_CloseToNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	const (
		blockX  = 6
		blockY  = 6
		blockZ  = 1
		width   = 48
		height  = 48
		depth   = 1
		quality = float32(60)
	)

	rnd := rand.New(rand.NewSource(1))
	src := make([]byte, width*height*depth*4)
	_, _ = rnd.Read(src)

	cfgGo, err := astc.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, quality, astc.FlagUseDecodeUNORM8)
	if err != nil {
		t.Fatalf("astc.ConfigInit: %v", err)
	}
	ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
	if err != nil {
		t.Fatalf("astc.ContextAlloc: %v", err)
	}
	defer ctxGo.Close()

	cfgN, err := native.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, quality, native.FlagUseDecodeUNORM8)
	if err != nil {
		t.Fatalf("native.ConfigInit: %v", err)
	}
	ctxN, err := native.ContextAlloc(&cfgN, 1)
	if err != nil {
		t.Fatalf("native.ContextAlloc: %v", err)
	}
	defer ctxN.Close()

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

	decGo := make([]byte, len(src))
	decN := make([]byte, len(src))
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, hdr, outGo, decGo); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto(go): %v", err)
	}
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, hdr, outN, decN); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto(native): %v", err)
	}

	errGo := float64(sumSquaredDiffU8(src, decGo))
	errN := float64(sumSquaredDiffU8(src, decN))
	if errN == 0 {
		if errGo != 0 {
			t.Fatalf("u8 sse mismatch: go=%g native=%g", errGo, errN)
		}
		return
	}

	if !(errGo <= errN*1.6+1e-6) && !(math.IsInf(errGo, 0) && math.IsInf(errN, 0)) {
		t.Fatalf("u8 sse too high: go=%g native=%g", errGo, errN)
	}
}
