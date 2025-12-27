//go:build astcenc_native && cgo

package native_test

import (
	"sync/atomic"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func blocksLenBytes(width, height, depth, blockX, blockY, blockZ int) int {
	blocksX := (width + blockX - 1) / blockX
	blocksY := (height + blockY - 1) / blockY
	blocksZ := (depth + blockZ - 1) / blockZ
	return blocksX * blocksY * blocksZ * astc.BlockBytes
}

func TestRawConfigInit_ContextAlloc(t *testing.T) {
	cfg, err := native.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	if cfg.BlockX != 4 || cfg.BlockY != 4 || cfg.BlockZ != 1 {
		t.Fatalf("unexpected block dims: %dx%dx%d", cfg.BlockX, cfg.BlockY, cfg.BlockZ)
	}

	ctx, err := native.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	if err := ctx.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRawCompress_Decompress_RGBA8_MatchesHighLevel(t *testing.T) {
	const (
		w      = 8
		h      = 8
		d      = 1
		blockX = 4
		blockY = 4
		blockZ = 1
	)

	// Constant color: should compress into a constant-color block and round-trip exactly.
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 10
		src[i+1] = 20
		src[i+2] = 30
		src[i+3] = 40
	}

	cfg, err := native.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := native.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	t.Cleanup(func() { _ = ctx.Close() })

	blocks := make([]byte, blocksLenBytes(w, h, d, blockX, blockY, blockZ))
	img := native.Image{
		DimX:     w,
		DimY:     h,
		DimZ:     d,
		DataType: native.TypeU8,
		DataU8:   src,
	}
	if err := ctx.CompressImage(&img, native.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}
	if err := ctx.CompressReset(); err != nil {
		t.Fatalf("CompressReset: %v", err)
	}

	dst := make([]byte, len(src))
	imgOut := native.Image{
		DimX:     w,
		DimY:     h,
		DimZ:     d,
		DataType: native.TypeU8,
		DataU8:   dst,
	}
	if err := ctx.DecompressImage(blocks, &imgOut, native.SwizzleRGBA, 0); err != nil {
		t.Fatalf("DecompressImage: %v", err)
	}
	if err := ctx.DecompressReset(); err != nil {
		t.Fatalf("DecompressReset: %v", err)
	}

	if string(dst) != string(src) {
		t.Fatalf("round-trip mismatch")
	}

	enc, err := native.NewEncoder(blockX, blockY, blockZ, astc.ProfileLDR, astc.EncodeMedium, 1)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	t.Cleanup(func() { _ = enc.Close() })

	astcData, err := enc.EncodeRGBA8Volume(src, w, h, d)
	if err != nil {
		t.Fatalf("EncodeRGBA8Volume: %v", err)
	}
	_, blocks2, err := astc.ParseFile(astcData)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if string(blocks2) != string(blocks) {
		t.Fatalf("encoded blocks mismatch between raw and high-level encoders")
	}

	var firstBlock [astc.BlockBytes]byte
	copy(firstBlock[:], blocks[:astc.BlockBytes])

	info, err := ctx.GetBlockInfo(firstBlock)
	if err != nil {
		t.Fatalf("GetBlockInfo: %v", err)
	}
	if !info.IsConstantBlock || info.IsErrorBlock {
		t.Fatalf("unexpected block kind: constant=%v error=%v", info.IsConstantBlock, info.IsErrorBlock)
	}
}

func TestRawCompress_Decompress_Swizzle(t *testing.T) {
	const (
		w      = 4
		h      = 4
		d      = 1
		blockX = 4
		blockY = 4
		blockZ = 1
	)

	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 1
		src[i+1] = 2
		src[i+2] = 3
		src[i+3] = 4
	}

	cfg, err := native.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := native.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	t.Cleanup(func() { _ = ctx.Close() })

	blocks := make([]byte, blocksLenBytes(w, h, d, blockX, blockY, blockZ))
	img := native.Image{DimX: w, DimY: h, DimZ: d, DataType: native.TypeU8, DataU8: src}

	// Swap R<->G in both directions (swap is self-inverse).
	swz := native.Swizzle{R: native.SwzG, G: native.SwzR, B: native.SwzB, A: native.SwzA}

	if err := ctx.CompressImage(&img, swz, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}
	if err := ctx.CompressReset(); err != nil {
		t.Fatalf("CompressReset: %v", err)
	}

	dst := make([]byte, len(src))
	imgOut := native.Image{DimX: w, DimY: h, DimZ: d, DataType: native.TypeU8, DataU8: dst}
	if err := ctx.DecompressImage(blocks, &imgOut, swz, 0); err != nil {
		t.Fatalf("DecompressImage: %v", err)
	}

	if string(dst) != string(src) {
		t.Fatalf("swizzle round-trip mismatch")
	}
}

func TestRawCompress_ProgressCallback(t *testing.T) {
	const (
		w      = 64
		h      = 64
		d      = 1
		blockX = 6
		blockY = 6
		blockZ = 1
	)

	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i++ {
		src[i] = byte(i * 31)
	}

	var called atomic.Int32
	cfg, err := native.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	cfg.ProgressCallback = func(progress float32) {
		_ = progress
		called.Store(1)
	}

	ctx, err := native.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	t.Cleanup(func() { _ = ctx.Close() })

	blocks := make([]byte, blocksLenBytes(w, h, d, blockX, blockY, blockZ))
	img := native.Image{DimX: w, DimY: h, DimZ: d, DataType: native.TypeU8, DataU8: src}

	if err := ctx.CompressImage(&img, native.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}

	if called.Load() == 0 {
		t.Fatalf("progress callback was not invoked")
	}
}
