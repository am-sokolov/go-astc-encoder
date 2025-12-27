package astc_test

import (
	"bytes"
	"sync"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func blocksLenBytes(width, height, depth, blockX, blockY, blockZ int) int {
	blocksX := (width + blockX - 1) / blockX
	blocksY := (height + blockY - 1) / blockY
	blocksZ := (depth + blockZ - 1) / blockZ
	return blocksX * blocksY * blocksZ * astc.BlockBytes
}

func TestContext_CompressDecompress_RGBA8_Constant(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := astc.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}

	const w, h, d = 8, 8, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 10
		src[i+1] = 20
		src[i+2] = 30
		src[i+3] = 40
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}
	if err := ctx.CompressReset(); err != nil {
		t.Fatalf("CompressReset: %v", err)
	}

	dst := make([]byte, len(src))
	out := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: dst}
	if err := ctx.DecompressImage(blocks, &out, astc.SwizzleRGBA, 0); err != nil {
		t.Fatalf("DecompressImage: %v", err)
	}
	if err := ctx.DecompressReset(); err != nil {
		t.Fatalf("DecompressReset: %v", err)
	}

	if !bytes.Equal(dst, src) {
		t.Fatalf("round-trip mismatch")
	}

	var first [astc.BlockBytes]byte
	copy(first[:], blocks[:astc.BlockBytes])
	info, err := ctx.GetBlockInfo(first)
	if err != nil {
		t.Fatalf("GetBlockInfo: %v", err)
	}
	if !info.IsConstantBlock || info.IsErrorBlock {
		t.Fatalf("unexpected block kind: constant=%v error=%v", info.IsConstantBlock, info.IsErrorBlock)
	}
}

func TestContext_CompressDecompress_Swizzle(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := astc.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}

	const w, h, d = 4, 4, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = 1
		src[i+1] = 2
		src[i+2] = 3
		src[i+3] = 4
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}

	// Swap R<->G in both directions (swap is self-inverse).
	swz := astc.Swizzle{R: astc.SwzG, G: astc.SwzR, B: astc.SwzB, A: astc.SwzA}

	if err := ctx.CompressImage(&img, swz, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}
	_ = ctx.CompressReset()

	dst := make([]byte, len(src))
	out := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: dst}
	if err := ctx.DecompressImage(blocks, &out, swz, 0); err != nil {
		t.Fatalf("DecompressImage: %v", err)
	}

	if !bytes.Equal(dst, src) {
		t.Fatalf("swizzle round-trip mismatch")
	}
}

func TestContext_MultiThread_ResetRequired(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileLDR, 6, 6, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := astc.ContextAlloc(&cfg, 4)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}

	const w, h, d = 32, 32, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i++ {
		src[i] = byte(i * 17)
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}

	var wg sync.WaitGroup
	wg.Add(4)
	for i := 0; i < 4; i++ {
		threadIndex := i
		go func() {
			defer wg.Done()
			if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, threadIndex); err != nil {
				t.Errorf("CompressImage(thread=%d): %v", threadIndex, err)
			}
		}()
	}
	wg.Wait()

	// Multi-threaded contexts require a reset between images.
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err == nil {
		t.Fatalf("CompressImage without reset: got nil error, want error")
	}
	if err := ctx.CompressReset(); err != nil {
		t.Fatalf("CompressReset: %v", err)
	}
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage after reset: %v", err)
	}
}
