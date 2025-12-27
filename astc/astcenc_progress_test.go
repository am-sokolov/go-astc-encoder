package astc_test

import (
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestContext_ProgressCallback_ThrottledAndHits100(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}

	var calls []float32
	cfg.ProgressCallback = func(p float32) {
		calls = append(calls, p)
	}

	ctx, err := astc.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	defer ctx.Close()

	// 128x128 @ 4x4 => 32x32 blocks => 1024 blocks, so upstream-like throttling
	// should only report the forced 100% completion callback.
	const w, h, d = 128, 128, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i++ {
		src[i] = byte(i * 17)
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}

	if len(calls) != 1 || calls[0] != 100.0 {
		t.Fatalf("progress callback calls: got %v want [100]", calls)
	}
}
