//go:build astcenc_native && cgo

package astc_test

import (
	"bytes"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestImageCorpus_Small_GoEncode_DecodeMatchesNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	images := collectPNGCorpusImages(t, "Small")

	const (
		blockX  = 6
		blockY  = 6
		quality = astc.EncodeMedium
	)

	for _, img := range images {
		t.Run(img.id, func(t *testing.T) {
			src, w, h := decodePNGToNRGBA(t, img.path)

			astcData, err := astc.EncodeRGBA8WithProfileAndQuality(src, w, h, blockX, blockY, img.profile, quality)
			if err != nil {
				t.Fatalf("EncodeRGBA8WithProfileAndQuality: %v", err)
			}

			goPix, goW, goH, err := astc.DecodeRGBA8WithProfile(astcData, img.profile)
			if err != nil {
				t.Fatalf("astc.DecodeRGBA8WithProfile: %v", err)
			}
			nPix, nW, nH, err := native.DecodeRGBA8WithProfile(astcData, img.profile)
			if err != nil {
				t.Fatalf("native.DecodeRGBA8WithProfile: %v", err)
			}

			if goW != nW || goH != nH || goW != w || goH != h {
				t.Fatalf("dimension mismatch: go=%dx%d native=%dx%d src=%dx%d", goW, goH, nW, nH, w, h)
			}
			if !bytes.Equal(goPix, nPix) {
				t.Fatalf("decoded pixel mismatch (go vs native)")
			}
		})
	}
}
