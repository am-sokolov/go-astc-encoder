package astc_test

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

type corpusImage struct {
	path     string
	id       string
	profile  astc.Profile
	channels int // 3 (RGB) or 4 (RGBA) for PSNR calculations.
}

func collectPNGCorpusImages(t *testing.T, imageSet string) []corpusImage {
	t.Helper()

	root := filepath.Join("..", "Test", "Images", imageSet)
	if _, err := os.Stat(root); err != nil {
		t.Skipf("image corpus not found (%s): %v", root, err)
	}

	var images []corpusImage
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".png" {
			return nil
		}

		base := filepath.Base(path)
		stem := strings.TrimSuffix(base, filepath.Ext(base))
		parts := strings.Split(stem, "-")
		if len(parts) < 3 {
			// Not a structured test image (e.g. results CSVs), skip.
			return nil
		}

		profileStr := parts[0]
		formatStr := parts[1]

		var prof astc.Profile
		switch profileStr {
		case "ldr":
			prof = astc.ProfileLDR
		case "ldrs":
			prof = astc.ProfileLDRSRGB
		default:
			// This helper only loads PNG images (LDR); skip other corpus formats.
			return nil
		}

		ch := 3
		if formatStr == "rgba" {
			ch = 4
		}

		rel, _ := filepath.Rel(root, path)
		images = append(images, corpusImage{
			path:     path,
			id:       filepath.ToSlash(filepath.Join(imageSet, rel)),
			profile:  prof,
			channels: ch,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk image corpus: %v", err)
	}

	sort.Slice(images, func(i, j int) bool { return images[i].id < images[j].id })
	if len(images) == 0 {
		t.Skipf("no PNG images found under %s", root)
	}
	return images
}

func decodePNGToNRGBA(t *testing.T, path string) (pix []byte, width, height int) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", path, err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode(%q): %v", path, err)
	}

	b := img.Bounds()
	width, height = b.Dx(), b.Dy()

	// Use NRGBA to avoid premultiplication during conversion.
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst.Pix, width, height
}

func psnrU8(a, b []byte, channels int) float64 {
	if len(a) != len(b) || channels <= 0 {
		return math.NaN()
	}

	var sse uint64
	samples := 0
	for i := 0; i < len(a); i += 4 {
		for c := 0; c < channels; c++ {
			d := int(a[i+c]) - int(b[i+c])
			sse += uint64(d * d)
			samples++
		}
	}
	if samples == 0 {
		return math.NaN()
	}
	if sse == 0 {
		// Lossless round-trip.
		return 999.99
	}
	mse := float64(sse) / float64(samples)
	return 10 * math.Log10((255.0*255.0)/mse)
}
