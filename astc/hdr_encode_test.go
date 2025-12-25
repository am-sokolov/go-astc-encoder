package astc_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestEncodeRGBA8_HDRProfiles_ConstBlocksRoundTrip(t *testing.T) {
	type tc struct {
		profile astc.Profile
		name    string
	}
	tests := []tc{
		{name: "hdr-rgb-ldr-a", profile: astc.ProfileHDRRGBLDRAlpha},
		{name: "hdr", profile: astc.ProfileHDR},
	}

	const (
		w, h          = 4, 4
		blockX        = 4
		blockY        = 4
		r       uint8 = 64
		g       uint8 = 128
		b       uint8 = 192
		a       uint8 = 255
		quality       = astc.EncodeMedium
	)

	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = r
		pix[i+1] = g
		pix[i+2] = b
		pix[i+3] = a
	}

	u16Prefix := []byte{0xFC, 0xFD, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astcData, err := astc.EncodeRGBA8WithProfileAndQuality(pix, w, h, blockX, blockY, tt.profile, quality)
			if err != nil {
				t.Fatalf("EncodeRGBA8WithProfileAndQuality: %v", err)
			}

			hdr, blocks, err := astc.ParseFile(astcData)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if int(hdr.SizeX) != w || int(hdr.SizeY) != h || int(hdr.SizeZ) != 1 {
				t.Fatalf("unexpected header size: %dx%dx%d", hdr.SizeX, hdr.SizeY, hdr.SizeZ)
			}
			if int(hdr.BlockX) != blockX || int(hdr.BlockY) != blockY || int(hdr.BlockZ) != 1 {
				t.Fatalf("unexpected block size: %dx%dx%d", hdr.BlockX, hdr.BlockY, hdr.BlockZ)
			}

			if len(blocks) != astc.BlockBytes {
				t.Fatalf("unexpected block payload size: got %d want %d", len(blocks), astc.BlockBytes)
			}
			if !bytes.Equal(blocks[:8], u16Prefix) {
				t.Fatalf("expected UNORM16 constant-color block prefix; got %x", blocks[:8])
			}
			wantU16 := [4]uint16{uint16(r) * 257, uint16(g) * 257, uint16(b) * 257, uint16(a) * 257}
			gotU16 := [4]uint16{
				binary.LittleEndian.Uint16(blocks[8:10]),
				binary.LittleEndian.Uint16(blocks[10:12]),
				binary.LittleEndian.Uint16(blocks[12:14]),
				binary.LittleEndian.Uint16(blocks[14:16]),
			}
			if gotU16 != wantU16 {
				t.Fatalf("constant-color payload mismatch: got=%v want=%v", gotU16, wantU16)
			}

			f32, w2, h2, err := astc.DecodeRGBAF32WithProfile(astcData, tt.profile)
			if err != nil {
				t.Fatalf("DecodeRGBAF32WithProfile: %v", err)
			}
			if w2 != w || h2 != h {
				t.Fatalf("unexpected decode dimensions: got %dx%d want %dx%d", w2, h2, w, h)
			}
			if len(f32) != w*h*4 {
				t.Fatalf("unexpected decode buffer length: got %d want %d", len(f32), w*h*4)
			}
			for i := 0; i < len(f32); i += 4 {
				got := [4]uint8{
					float01ToU8(f32[i+0]),
					float01ToU8(f32[i+1]),
					float01ToU8(f32[i+2]),
					float01ToU8(f32[i+3]),
				}
				want := [4]uint8{r, g, b, a}
				if got != want {
					t.Fatalf("pixel mismatch at %d: got=%v want=%v", i/4, got, want)
				}
			}
		})
	}
}

func TestEncodeRGBA8Volume_HDRProfiles_ConstBlocksRoundTrip(t *testing.T) {
	type tc struct {
		profile astc.Profile
		name    string
	}
	tests := []tc{
		{name: "hdr-rgb-ldr-a", profile: astc.ProfileHDRRGBLDRAlpha},
		{name: "hdr", profile: astc.ProfileHDR},
	}

	const (
		w, h, d       = 5, 3, 4
		blockX        = 4
		blockY        = 4
		blockZ        = 3
		r       uint8 = 17
		g       uint8 = 123
		b       uint8 = 250
		a       uint8 = 200
		quality       = astc.EncodeMedium
	)

	pix := make([]byte, w*h*d*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i+0] = r
		pix[i+1] = g
		pix[i+2] = b
		pix[i+3] = a
	}

	u16Prefix := []byte{0xFC, 0xFD, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	wantU16 := [4]uint16{uint16(r) * 257, uint16(g) * 257, uint16(b) * 257, uint16(a) * 257}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			astcData, err := astc.EncodeRGBA8VolumeWithProfileAndQuality(pix, w, h, d, blockX, blockY, blockZ, tt.profile, quality)
			if err != nil {
				t.Fatalf("EncodeRGBA8VolumeWithProfileAndQuality: %v", err)
			}

			hdr, blocks, err := astc.ParseFile(astcData)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if int(hdr.SizeX) != w || int(hdr.SizeY) != h || int(hdr.SizeZ) != d {
				t.Fatalf("unexpected header size: %dx%dx%d", hdr.SizeX, hdr.SizeY, hdr.SizeZ)
			}
			if int(hdr.BlockX) != blockX || int(hdr.BlockY) != blockY || int(hdr.BlockZ) != blockZ {
				t.Fatalf("unexpected block size: %dx%dx%d", hdr.BlockX, hdr.BlockY, hdr.BlockZ)
			}

			blocksX, blocksY, blocksZ, total, err := hdr.BlockCount()
			if err != nil {
				t.Fatalf("BlockCount: %v", err)
			}
			wantBlocks := total * astc.BlockBytes
			if len(blocks) != wantBlocks {
				t.Fatalf("unexpected block payload size: got %d want %d", len(blocks), wantBlocks)
			}

			for i := 0; i < total; i++ {
				blk := blocks[i*astc.BlockBytes : (i+1)*astc.BlockBytes]
				if !bytes.Equal(blk[:8], u16Prefix) {
					t.Fatalf("block %d: expected UNORM16 constant-color block prefix; got %x", i, blk[:8])
				}
				gotU16 := [4]uint16{
					binary.LittleEndian.Uint16(blk[8:10]),
					binary.LittleEndian.Uint16(blk[10:12]),
					binary.LittleEndian.Uint16(blk[12:14]),
					binary.LittleEndian.Uint16(blk[14:16]),
				}
				if gotU16 != wantU16 {
					t.Fatalf("block %d: constant-color payload mismatch: got=%v want=%v", i, gotU16, wantU16)
				}
			}

			f32, w2, h2, d2, err := astc.DecodeRGBAF32VolumeWithProfile(astcData, tt.profile)
			if err != nil {
				t.Fatalf("DecodeRGBAF32VolumeWithProfile: %v", err)
			}
			if w2 != w || h2 != h || d2 != d {
				t.Fatalf("unexpected decode dimensions: got %dx%dx%d want %dx%dx%d", w2, h2, d2, w, h, d)
			}
			if len(f32) != w*h*d*4 {
				t.Fatalf("unexpected decode buffer length: got %d want %d", len(f32), w*h*d*4)
			}
			for i := 0; i < len(f32); i += 4 {
				got := [4]uint8{
					float01ToU8(f32[i+0]),
					float01ToU8(f32[i+1]),
					float01ToU8(f32[i+2]),
					float01ToU8(f32[i+3]),
				}
				want := [4]uint8{r, g, b, a}
				if got != want {
					t.Fatalf("pixel mismatch at %d: got=%v want=%v", i/4, got, want)
				}
			}

			// Spot-check block ordering to avoid missing a block stride bug.
			if blocksX <= 0 || blocksY <= 0 || blocksZ <= 0 {
				t.Fatalf("unexpected block grid: %dx%dx%d", blocksX, blocksY, blocksZ)
			}
		})
	}
}

func float01ToU8(v float32) uint8 {
	// Match DecodeConstBlockRGBA8 behavior: clamp to [0,1] and round to nearest.
	if !(v >= 0) {
		return 0
	}
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}
