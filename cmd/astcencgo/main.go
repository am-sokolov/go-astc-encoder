package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"

	_ "image/jpeg"
	_ "image/png"
)

func main() {
	var (
		inPath    string
		outPath   string
		block     string
		profile   string
		quality   string
		impl      string
		encode    bool
		decode    bool
		dumpInfo  bool
		dumpBlock bool
	)
	flag.StringVar(&inPath, "in", "", "input file")
	flag.StringVar(&outPath, "out", "", "output file")
	flag.StringVar(&block, "block", "4x4", "ASTC block footprint (e.g. 4x4)")
	flag.StringVar(&profile, "profile", "ldr", "decode/encode profile: ldr|srgb|hdr|hdr-rgb-ldr-a")
	flag.StringVar(&quality, "quality", "medium", "encode quality preset: fastest|fast|medium|thorough|verythorough|exhaustive")
	flag.StringVar(&impl, "impl", "go", "implementation: go|native")
	flag.BoolVar(&encode, "encode", false, "encode input image -> .astc")
	flag.BoolVar(&decode, "decode", false, "decode input .astc -> .png")
	flag.BoolVar(&dumpInfo, "info", false, "print .astc header info and exit")
	flag.BoolVar(&dumpBlock, "dump-first-block", false, "dump the first ASTC block payload as hex and exit")
	flag.Parse()

	if inPath == "" {
		fmt.Fprintln(os.Stderr, "usage: astcencgo -in <input> [-out <output>] [-encode|-decode] [-block 4x4]")
		os.Exit(2)
	}

	inData, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if dumpInfo || dumpBlock {
		h, blocks, err := astc.ParseFile(inData)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(h.String())
		if dumpBlock {
			if len(blocks) < astc.BlockBytes {
				fmt.Fprintln(os.Stderr, "astc: missing first block")
				os.Exit(1)
			}
			fmt.Println(hex.EncodeToString(blocks[:astc.BlockBytes]))
		}
		return
	}

	if encode == decode {
		fmt.Fprintln(os.Stderr, "specify exactly one of -encode or -decode")
		os.Exit(2)
	}
	if outPath == "" {
		fmt.Fprintln(os.Stderr, "missing -out")
		os.Exit(2)
	}

	profileVal, err := parseProfile(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	qualityVal, err := parseQuality(quality)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	implVal, err := parseImpl(impl)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if implVal == implNative && !native.Enabled() {
		fmt.Fprintln(os.Stderr, "native implementation is not available in this build (build with -tags astcenc_native and CGO_ENABLED=1)")
		os.Exit(2)
	}

	if encode {
		bx, by, err := parseBlock(block)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}

		img, _, err := image.Decode(bytes.NewReader(inData))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)

		var astcData []byte
		switch implVal {
		case implGo:
			astcData, err = astc.EncodeRGBA8WithProfileAndQuality(rgba.Pix, rgba.Rect.Dx(), rgba.Rect.Dy(), bx, by, profileVal, qualityVal)
		case implNative:
			astcData, err = native.EncodeRGBA8WithProfileAndQuality(rgba.Pix, rgba.Rect.Dx(), rgba.Rect.Dy(), bx, by, profileVal, qualityVal)
		default:
			err = fmt.Errorf("unsupported -impl %q", impl)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if err := os.WriteFile(outPath, astcData, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// decode
	var img *image.RGBA
	if profileVal == astc.ProfileLDR || profileVal == astc.ProfileLDRSRGB {
		var pix []byte
		var w, h int
		switch implVal {
		case implGo:
			pix, w, h, err = astc.DecodeRGBA8WithProfile(inData, profileVal)
		case implNative:
			pix, w, h, err = native.DecodeRGBA8WithProfile(inData, profileVal)
		default:
			err = fmt.Errorf("unsupported -impl %q", impl)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		img = &image.RGBA{
			Pix:    pix,
			Stride: w * 4,
			Rect:   image.Rect(0, 0, w, h),
		}
	} else {
		var pix []float32
		var w, h, d int
		switch implVal {
		case implGo:
			pix, w, h, err = astc.DecodeRGBAF32WithProfile(inData, profileVal)
			d = 1
		case implNative:
			pix, w, h, d, err = native.DecodeRGBAF32VolumeWithProfile(inData, profileVal)
			if err == nil && d != 1 {
				err = fmt.Errorf("astcencgo: 3D images are not supported by this CLI (z=%d)", d)
			}
		default:
			err = fmt.Errorf("unsupported -impl %q", impl)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		pix8 := make([]byte, w*h*4)
		for i := 0; i < len(pix8); i++ {
			v := pix[i]
			if !(v >= 0) {
				v = 0
			} else if v > 1 {
				v = 1
			}
			pix8[i] = uint8(v*255 + 0.5)
		}
		img = &image.RGBA{
			Pix:    pix8,
			Stride: w * 4,
			Rect:   image.Rect(0, 0, w, h),
		}
	}

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer out.Close()

	if err := png.Encode(out, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseBlock(s string) (x, y int, err error) {
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid -block %q (want like 4x4)", s)
	}
	_, err = fmt.Sscanf(s, "%dx%d", &x, &y)
	if err != nil || x <= 0 || y <= 0 || x > 255 || y > 255 {
		return 0, 0, fmt.Errorf("invalid -block %q (want like 4x4)", s)
	}
	return x, y, nil
}

func parseProfile(s string) (astc.Profile, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ldr":
		return astc.ProfileLDR, nil
	case "srgb", "ldr-srgb":
		return astc.ProfileLDRSRGB, nil
	case "hdr", "hdr-rgba":
		return astc.ProfileHDR, nil
	case "hdr-rgb-ldr-a", "hdr-rgb-ldr-alpha":
		return astc.ProfileHDRRGBLDRAlpha, nil
	default:
		return 0, fmt.Errorf("invalid -profile %q (want ldr|srgb|hdr|hdr-rgb-ldr-a)", s)
	}
}

func parseQuality(s string) (astc.EncodeQuality, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fastest":
		return astc.EncodeFastest, nil
	case "fast":
		return astc.EncodeFast, nil
	case "medium":
		return astc.EncodeMedium, nil
	case "thorough":
		return astc.EncodeThorough, nil
	case "verythorough", "very-thorough":
		return astc.EncodeVeryThorough, nil
	case "exhaustive":
		return astc.EncodeExhaustive, nil
	default:
		return 0, fmt.Errorf("invalid -quality %q (want fastest|fast|medium|thorough|verythorough|exhaustive)", s)
	}
}

type implKind uint8

const (
	implGo implKind = iota
	implNative
)

func parseImpl(s string) (implKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "go", "pure", "purego", "pure-go":
		return implGo, nil
	case "native", "cgo":
		return implNative, nil
	default:
		return 0, fmt.Errorf("invalid -impl %q (want go|native)", s)
	}
}
