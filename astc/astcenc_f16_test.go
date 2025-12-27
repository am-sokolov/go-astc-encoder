package astc_test

import (
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestContext_CompressDecompress_RGBAF16_LDR_Constant(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := astc.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	defer ctx.Close()

	const (
		w, h, d = 4, 4, 1

		f16Zero = 0x0000
		f16One  = 0x3C00
	)

	src := make([]uint16, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = f16One
		src[i+1] = f16Zero
		src[i+2] = f16One
		src[i+3] = f16One
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeF16, DataF16: src}
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}

	dst := make([]uint16, len(src))
	out := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeF16, DataF16: dst}
	if err := ctx.DecompressImage(blocks, &out, astc.SwizzleRGBA, 0); err != nil {
		t.Fatalf("DecompressImage: %v", err)
	}

	for i := range dst {
		if dst[i] != src[i] {
			t.Fatalf("decoded f16 mismatch at %d: got=%04x want=%04x", i, dst[i], src[i])
		}
	}
}

func TestContext_CompressDecompress_RGBAF16_HDR_Constant(t *testing.T) {
	cfg, err := astc.ConfigInit(astc.ProfileHDR, 4, 4, 1, 60, 0)
	if err != nil {
		t.Fatalf("ConfigInit: %v", err)
	}
	ctx, err := astc.ContextAlloc(&cfg, 1)
	if err != nil {
		t.Fatalf("ContextAlloc: %v", err)
	}
	defer ctx.Close()

	const (
		w, h, d = 4, 4, 1

		f16Half = 0x3800 // 0.5
		f16One  = 0x3C00 // 1.0
		f16Two  = 0x4000 // 2.0
	)

	src := make([]uint16, w*h*d*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0] = f16Two
		src[i+1] = f16One
		src[i+2] = f16Half
		src[i+3] = f16One
	}

	blocks := make([]byte, blocksLenBytes(w, h, d, int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)))
	img := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeF16, DataF16: src}
	if err := ctx.CompressImage(&img, astc.SwizzleRGBA, blocks, 0); err != nil {
		t.Fatalf("CompressImage: %v", err)
	}

	dst := make([]uint16, len(src))
	out := astc.Image{DimX: w, DimY: h, DimZ: d, DataType: astc.TypeF16, DataF16: dst}
	if err := ctx.DecompressImage(blocks, &out, astc.SwizzleRGBA, 0); err != nil {
		t.Fatalf("DecompressImage: %v", err)
	}

	for i := range dst {
		if dst[i] != src[i] {
			t.Fatalf("decoded f16 mismatch at %d: got=%04x want=%04x", i, dst[i], src[i])
		}
	}
}
