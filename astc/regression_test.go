package astc_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestDecodeRGBA8_TilesLDR_KnownPixels(t *testing.T) {
	astcData := mustReadFile(t, "testdata/fixtures/Tiles/ldr.astc")
	pix, w, h, err := astc.DecodeRGBA8(astcData)
	if err != nil {
		t.Fatalf("DecodeRGBA8: %v", err)
	}

	if w != 8 || h != 8 {
		t.Fatalf("unexpected dimensions: %dx%d", w, h)
	}

	type sample struct {
		x, y       int
		r, g, b, a byte
	}
	samples := []sample{
		// Top-left, top-right, bottom-left, bottom-right.
		{x: 0, y: 0, r: 0, g: 0, b: 0, a: 255},
		{x: 7, y: 0, r: 255, g: 0, b: 0, a: 255},
		{x: 0, y: 7, r: 255, g: 255, b: 255, a: 255},
		{x: 7, y: 7, r: 64, g: 192, b: 0, a: 221},
	}
	for _, s := range samples {
		i := (s.y*w + s.x) * 4
		if pix[i+0] != s.r || pix[i+1] != s.g || pix[i+2] != s.b || pix[i+3] != s.a {
			t.Fatalf("pixel (%d,%d) mismatch: got (%d,%d,%d,%d) want (%d,%d,%d,%d)",
				s.x, s.y,
				pix[i+0], pix[i+1], pix[i+2], pix[i+3],
				s.r, s.g, s.b, s.a)
		}
	}
}

func TestRoundTrip_TilesLDR(t *testing.T) {
	wantASTC := mustReadFile(t, "testdata/fixtures/Tiles/ldr.astc")

	pix, w, h, err := astc.DecodeRGBA8(wantASTC)
	if err != nil {
		t.Fatalf("DecodeRGBA8: %v", err)
	}

	gotASTC, err := astc.EncodeRGBA8(pix, w, h, 4, 4)
	if err != nil {
		t.Fatalf("EncodeRGBA8: %v", err)
	}
	if !bytes.Equal(gotASTC, wantASTC) {
		t.Fatalf("round-trip mismatch for testdata/fixtures/Tiles/ldr.astc")
	}
}

func TestDecodeRGBA8_LDR_A_1x1(t *testing.T) {
	astcData := mustReadFile(t, "testdata/fixtures/LDR-A-1x1.astc")
	pix, w, h, err := astc.DecodeRGBA8(astcData)
	if err != nil {
		t.Fatalf("DecodeRGBA8: %v", err)
	}
	if w != 1 || h != 1 {
		t.Fatalf("unexpected dimensions: %dx%d", w, h)
	}
	if len(pix) != 4 {
		t.Fatalf("unexpected pix length: %d", len(pix))
	}

	// Extract expected RGBA from the constant-color block in the file.
	_, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	wantR, wantG, wantB, wantA, err := astc.DecodeConstBlockRGBA8(blocks[:16])
	if err != nil {
		t.Fatalf("DecodeConstBlockRGBA8(block): %v", err)
	}

	if pix[0] != wantR || pix[1] != wantG || pix[2] != wantB || pix[3] != wantA {
		t.Fatalf("pixel mismatch: got (%d,%d,%d,%d) want (%d,%d,%d,%d)", pix[0], pix[1], pix[2], pix[3], wantR, wantG, wantB, wantA)
	}
}

func TestDecodeRGBA8_TilesHDRConstBlocks(t *testing.T) {
	astcData := mustReadFile(t, "testdata/fixtures/Tiles/hdr.astc")
	pix, w, h, err := astc.DecodeRGBA8(astcData)
	if err != nil {
		t.Fatalf("DecodeRGBA8: %v", err)
	}
	if w != 8 || h != 8 {
		t.Fatalf("unexpected dimensions: %dx%d", w, h)
	}

	// The HDR tile image uses FLOAT16 constant-color blocks. These are invalid in LDR decode profiles
	// and should decode to magenta error color, matching the reference decoder behavior.
	if pix[0] != 0xFF || pix[1] != 0x00 || pix[2] != 0xFF || pix[3] != 0xFF {
		t.Fatalf("pixel mismatch: got (%d,%d,%d,%d) want (255,0,255,255)", pix[0], pix[1], pix[2], pix[3])
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}
	return b
}
