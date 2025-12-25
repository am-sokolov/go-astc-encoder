package astc_test

import (
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestImageCorpus_Small_EncodeDecode_PSNRSane(t *testing.T) {
	images := collectPNGCorpusImages(t, "Small")

	type cfg struct {
		blockX  int
		blockY  int
		quality astc.EncodeQuality
		minPSNR float64
	}

	// Medium is our default regression preset; thorough exercises additional search paths.
	configs := []cfg{
		{blockX: 4, blockY: 4, quality: astc.EncodeMedium, minPSNR: 15},
		{blockX: 5, blockY: 5, quality: astc.EncodeMedium, minPSNR: 15},
		{blockX: 6, blockY: 6, quality: astc.EncodeMedium, minPSNR: 15},
		{blockX: 8, blockY: 8, quality: astc.EncodeMedium, minPSNR: 15},
		{blockX: 12, blockY: 12, quality: astc.EncodeMedium, minPSNR: 15},
		{blockX: 6, blockY: 6, quality: astc.EncodeThorough, minPSNR: 15},
		{blockX: 12, blockY: 12, quality: astc.EncodeThorough, minPSNR: 15},
	}

	for _, img := range images {
		t.Run(img.id, func(t *testing.T) {
			src, w, h := decodePNGToNRGBA(t, img.path)

			for _, c := range configs {
				astcData, err := astc.EncodeRGBA8WithProfileAndQuality(src, w, h, c.blockX, c.blockY, img.profile, c.quality)
				if err != nil {
					t.Fatalf("EncodeRGBA8WithProfileAndQuality(block=%dx%d profile=%d quality=%d): %v", c.blockX, c.blockY, img.profile, c.quality, err)
				}

				dst, w2, h2, err := astc.DecodeRGBA8WithProfile(astcData, img.profile)
				if err != nil {
					t.Fatalf("DecodeRGBA8WithProfile(profile=%d): %v", img.profile, err)
				}
				if w2 != w || h2 != h {
					t.Fatalf("unexpected dimensions: got %dx%d want %dx%d", w2, h2, w, h)
				}

				gotPSNR := psnrU8(src, dst, img.channels)
				if !(gotPSNR >= c.minPSNR) {
					t.Fatalf("psnr too low: got %.3f dB want >= %.3f dB (block=%dx%d quality=%d)", gotPSNR, c.minPSNR, c.blockX, c.blockY, c.quality)
				}
			}
		})
	}
}
