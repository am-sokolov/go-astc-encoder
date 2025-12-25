package astc

import "testing"

func TestEncodeRGBA8Volume_Uses3DBlockModes(t *testing.T) {
	const (
		w  = 4
		h  = 4
		d  = 4
		bx = 4
		by = 4
		bz = 4
	)

	pix := make([]byte, w*h*d*4)
	for z := 0; z < d; z++ {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				off := ((z*h+y)*w + x) * 4
				pix[off+0] = uint8(x * 37)
				pix[off+1] = uint8(y * 53)
				pix[off+2] = uint8(z * 71)
				pix[off+3] = uint8(255 - x*11 - z*7)
			}
		}
	}

	out, err := EncodeRGBA8VolumeWithProfileAndQuality(pix, w, h, d, bx, by, bz, ProfileLDR, EncodeMedium)
	if err != nil {
		t.Fatalf("EncodeRGBA8VolumeWithProfileAndQuality: %v", err)
	}

	hdr, blocks, err := ParseFile(out)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if hdr.BlockZ != bz || hdr.SizeZ != d {
		t.Fatalf("unexpected header: block=%dx%dx%d size=%dx%dx%d", hdr.BlockX, hdr.BlockY, hdr.BlockZ, hdr.SizeX, hdr.SizeY, hdr.SizeZ)
	}
	if len(blocks) < BlockBytes {
		t.Fatalf("unexpected blocks payload size: %d", len(blocks))
	}

	// Ensure the first block isn't constant-color (which wouldn't exercise 3D block modes).
	if _, _, _, _, err := DecodeConstBlockRGBA8(blocks[:BlockBytes]); err == nil {
		t.Fatalf("unexpected const block; test input should produce non-const blocks")
	}

	blockMode := int(readBits(11, 0, blocks[:BlockBytes]))
	_, _, zw, _, _, _, ok := decodeBlockMode3D(blockMode)
	if !ok {
		t.Fatalf("expected a valid 3D block mode, got mode=%d", blockMode)
	}
	if zw <= 0 {
		t.Fatalf("unexpected zWeights=%d for 3D block mode=%d", zw, blockMode)
	}
}
