//go:build astcenc_native && cgo

package astc_test

import (
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestCompress_HDR_DualPlaneComponent_MatchesNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	const (
		blockX  = 6
		blockY  = 6
		blockZ  = 1
		quality = float32(80)
	)

	tests := []struct {
		name    string
		profile astc.Profile
	}{
		{name: "hdr", profile: astc.ProfileHDR},
		{name: "hdr-rgb-ldr-a", profile: astc.ProfileHDRRGBLDRAlpha},
	}

	pix := make([]float32, blockX*blockY*4)
	for y := 0; y < blockY; y++ {
		for x := 0; x < blockX; x++ {
			off := (y*blockX + x) * 4
			r := float32(x) / float32(blockX-1) * 4.0
			g := float32(y) / float32(blockY-1) * 4.0

			// A synthetic pattern where R and B vary together but G varies independently.
			// This should strongly prefer using the G component as the dual-plane channel.
			pix[off+0] = r
			pix[off+1] = g
			pix[off+2] = r
			pix[off+3] = 1.0
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgGo, err := astc.ConfigInit(tt.profile, blockX, blockY, blockZ, quality, 0)
			if err != nil {
				t.Fatalf("astc.ConfigInit: %v", err)
			}
			cfgGo.TunePartitionCountLimit = 1
			cfgGo.TuneBlockModeLimit = 100
			cfgGo.Tune2PlaneEarlyOutLimitCorrelation = 1.0

			ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
			if err != nil {
				t.Fatalf("astc.ContextAlloc: %v", err)
			}
			defer ctxGo.Close()

			cfgN, err := native.ConfigInit(tt.profile, blockX, blockY, blockZ, quality, 0)
			if err != nil {
				t.Fatalf("native.ConfigInit: %v", err)
			}
			cfgN.TunePartitionCountLimit = 1
			cfgN.TuneBlockModeLimit = 100
			cfgN.Tune2PlaneEarlyOutLimitCorrelation = 1.0

			ctxN, err := native.ContextAlloc(&cfgN, 1)
			if err != nil {
				t.Fatalf("native.ContextAlloc: %v", err)
			}
			defer ctxN.Close()

			imgGo := &astc.Image{DimX: blockX, DimY: blockY, DimZ: 1, DataType: astc.TypeF32, DataF32: pix}
			imgN := &native.Image{DimX: blockX, DimY: blockY, DimZ: 1, DataType: native.TypeF32, DataF32: pix}

			outGo := make([]byte, astc.BlockBytes)
			outN := make([]byte, astc.BlockBytes)

			if err := ctxGo.CompressImage(imgGo, astc.SwizzleRGBA, outGo, 0); err != nil {
				t.Fatalf("astc.CompressImage: %v", err)
			}
			if err := ctxN.CompressImage(imgN, native.SwizzleRGBA, outN, 0); err != nil {
				t.Fatalf("native.CompressImage: %v", err)
			}

			var blk [astc.BlockBytes]byte

			copy(blk[:], outGo)
			infoGo, err := ctxGo.GetBlockInfo(blk)
			if err != nil {
				t.Fatalf("astc.GetBlockInfo(go): %v", err)
			}

			copy(blk[:], outN)
			infoN, err := ctxN.GetBlockInfo(blk)
			if err != nil {
				t.Fatalf("astc.GetBlockInfo(native): %v", err)
			}

			if !infoGo.IsDualPlaneBlock || !infoN.IsDualPlaneBlock {
				t.Fatalf("expected dual-plane blocks: go=%v native=%v", infoGo.IsDualPlaneBlock, infoN.IsDualPlaneBlock)
			}
			if infoGo.DualPlaneComponent != infoN.DualPlaneComponent {
				t.Fatalf("dual-plane component mismatch: go=%d native=%d", infoGo.DualPlaneComponent, infoN.DualPlaneComponent)
			}
			if infoN.DualPlaneComponent == 3 {
				t.Fatalf("unexpected alpha dual-plane component for this pattern: got %d", infoN.DualPlaneComponent)
			}
		})
	}
}
