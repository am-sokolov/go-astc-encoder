package astc_test

import (
	"sync"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestContext_CompressCancel_StopsEarly(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}

	var ctx *astc.Context
	var cancelOnce sync.Once
	cfg.ProgressCallback = func(p float32) {
		if p >= 1 {
			cancelOnce.Do(func() {
				_ = ctx.CompressCancel()
			})
		}
	}

	ctx, err = astc.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	defer ctx.Close()

	// Use a large enough image that the upstream-like progress callback throttling
	// will report progress before completion.
	const w, h, d = 512, 512, 1
	src := make([]byte, w*h*d*4)
	for i := 0; i < len(src); i++ {
		src[i] = byte(i * 17)
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	for i := range blocks {
		blocks[i] = 0xCD
	}

	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeU8, DataU8: src}
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}

	// Ensure some blocks were not written because cancellation happened.
	totalBlocks := len(blocks) / astc.BlockBytes
	untouched := 0
	for i := 0; i < totalBlocks; i++ {
		block := blocks[i*astc.BlockBytes : (i+1)*astc.BlockBytes]
		allSentinel := true
		for _, b := range block {
			if b != 0xCD {
				allSentinel = false
				break
			}
		}
		if allSentinel {
			untouched++
		}
	}
	if untouched == 0 {
		t.Fatalf("expected cancellation to leave some blocks untouched")
	}
	if untouched == totalBlocks {
		t.Fatalf("expected cancellation to still encode some blocks")
	}
}
