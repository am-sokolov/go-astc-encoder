package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "decode":
		decodeCmd(os.Args[2:])
	case "encode":
		encodeCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  astcbench decode -in <file.astc> [-impl go|native] [-profile ldr|srgb|hdr|hdr-rgb-ldr-a] [-iters N] [-out u8|f32] [-checksum fnv|none]")
	fmt.Fprintln(os.Stderr, "  astcbench encode -w W -h H [-d D] -block 4x4[ xZ] [-impl go|native] [-profile ldr|srgb|hdr|hdr-rgb-ldr-a] [-quality fastest|fast|medium|thorough|verythorough|exhaustive] [-iters N] [-out file.astc] [-checksum fnv|none]")
}

func decodeCmd(args []string) {
	fs := flag.NewFlagSet("decode", flag.ExitOnError)
	var (
		inPath      string
		impl        string
		profile     string
		iters       int
		outKind     string
		checksumOpt string
		cpuprofile  string
		memprofile  string
		memprofRate int
	)
	fs.StringVar(&inPath, "in", "", "input .astc file")
	fs.StringVar(&impl, "impl", "go", "implementation: go|native (native requires -tags astcenc_native)")
	fs.StringVar(&profile, "profile", "ldr", "profile: ldr|srgb|hdr|hdr-rgb-ldr-a")
	fs.IntVar(&iters, "iters", 200, "iterations")
	fs.StringVar(&outKind, "out", "u8", "output kind: u8|f32")
	fs.StringVar(&checksumOpt, "checksum", "fnv", "checksum: fnv|none (for benchmarking)")
	fs.StringVar(&cpuprofile, "cpuprofile", "", "optional CPU profile output path")
	fs.StringVar(&memprofile, "memprofile", "", "optional memory profile output path")
	fs.IntVar(&memprofRate, "memprofilerate", 0, "optional runtime.MemProfileRate override (0 = default)")
	_ = fs.Parse(args)

	if inPath == "" {
		fmt.Fprintln(os.Stderr, "missing -in")
		os.Exit(2)
	}
	impl = strings.ToLower(strings.TrimSpace(impl))
	prof, err := parseProfile(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if iters <= 0 {
		fmt.Fprintln(os.Stderr, "iters must be > 0")
		os.Exit(2)
	}

	data, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	hdr, blocks, err := astc.ParseFile(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	w := int(hdr.SizeX)
	h := int(hdr.SizeY)
	d := int(hdr.SizeZ)

	if memprofRate > 0 {
		runtime.MemProfileRate = memprofRate
	}

	var cpuFile *os.File
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cpuFile = f
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = cpuFile.Close()
		}()
	}

	start := time.Now()
	var checksum uint64
	doChecksum := strings.ToLower(strings.TrimSpace(checksumOpt)) != "none"

	outKind = strings.ToLower(strings.TrimSpace(outKind))
	switch outKind {
	case "u8", "rgba8":
		if prof != astc.ProfileLDR && prof != astc.ProfileLDRSRGB {
			fmt.Fprintln(os.Stderr, "u8 decode requires -profile ldr or srgb")
			os.Exit(2)
		}

		dst := make([]byte, w*h*d*4)
		switch impl {
		case "go":
			for i := 0; i < iters; i++ {
				err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(prof, hdr, blocks, dst)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				if doChecksum {
					checksum = fnv1a64(checksum, dst)
				}
			}
		case "native", "cgo":
			if !native.Enabled() {
				fmt.Fprintln(os.Stderr, "native impl requested but not enabled (build with -tags astcenc_native and CGO_ENABLED=1)")
				os.Exit(2)
			}
			dec, err := native.NewDecoder(int(hdr.BlockX), int(hdr.BlockY), int(hdr.BlockZ), prof, 0)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			defer dec.Close()
			for i := 0; i < iters; i++ {
				err := dec.DecodeRGBA8VolumeInto(w, h, d, blocks, dst)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				if doChecksum {
					checksum = fnv1a64(checksum, dst)
				}
			}
		default:
			fmt.Fprintln(os.Stderr, "invalid -impl (want go|native)")
			os.Exit(2)
		}
	case "f32":
		dst := make([]float32, w*h*d*4)
		switch impl {
		case "go":
			for i := 0; i < iters; i++ {
				err := astc.DecodeRGBAF32VolumeFromParsedWithProfileInto(prof, hdr, blocks, dst)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				if doChecksum {
					checksum = fnv1a64Float32(checksum, dst)
				}
			}
		case "native", "cgo":
			if !native.Enabled() {
				fmt.Fprintln(os.Stderr, "native impl requested but not enabled (build with -tags astcenc_native and CGO_ENABLED=1)")
				os.Exit(2)
			}
			dec, err := native.NewDecoder(int(hdr.BlockX), int(hdr.BlockY), int(hdr.BlockZ), prof, 0)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			defer dec.Close()
			for i := 0; i < iters; i++ {
				err := dec.DecodeRGBAF32VolumeInto(w, h, d, blocks, dst)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				if doChecksum {
					checksum = fnv1a64Float32(checksum, dst)
				}
			}
		default:
			fmt.Fprintln(os.Stderr, "invalid -impl (want go|native)")
			os.Exit(2)
		}
	default:
		fmt.Fprintln(os.Stderr, "invalid -out (want u8|f32)")
		os.Exit(2)
	}

	dur := time.Since(start)
	texels := float64(w*h*d) * float64(iters)
	mpixPerS := texels / dur.Seconds() / 1e6

	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.WriteHeapProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	checksumStr := fmtChecksum(checksum)
	if !doChecksum {
		checksumStr = "none"
	}

	implOut := impl
	if implOut == "cgo" {
		implOut = "native"
	}
	fmt.Printf("RESULT impl=%s mode=decode out=%s profile=%s size=%dx%dx%d iters=%d seconds=%.6f mpix/s=%.3f checksum=%s\n",
		implOut,
		outKind,
		profile,
		w, h, d,
		iters,
		dur.Seconds(),
		mpixPerS,
		checksumStr,
	)
}

func encodeCmd(args []string) {
	fs := flag.NewFlagSet("encode", flag.ExitOnError)
	var (
		width       int
		height      int
		depth       int
		block       string
		impl        string
		profile     string
		quality     string
		iters       int
		outPath     string
		checksumOpt string
		cpuprofile  string
	)
	fs.IntVar(&width, "w", 256, "width")
	fs.IntVar(&height, "h", 256, "height")
	fs.IntVar(&depth, "d", 1, "depth")
	fs.StringVar(&block, "block", "4x4", "block size: NxM or NxMxK")
	fs.StringVar(&impl, "impl", "go", "implementation: go|native (native requires -tags astcenc_native)")
	fs.StringVar(&profile, "profile", "ldr", "profile: ldr|srgb|hdr|hdr-rgb-ldr-a")
	fs.StringVar(&quality, "quality", "medium", "quality: fastest|fast|medium|thorough|verythorough|exhaustive")
	fs.IntVar(&iters, "iters", 20, "iterations")
	fs.StringVar(&outPath, "out", "", "optional output .astc path (writes last iteration)")
	fs.StringVar(&checksumOpt, "checksum", "fnv", "checksum: fnv|none (for benchmarking)")
	fs.StringVar(&cpuprofile, "cpuprofile", "", "optional CPU profile output path")
	_ = fs.Parse(args)

	if width <= 0 || height <= 0 || depth <= 0 {
		fmt.Fprintln(os.Stderr, "invalid dimensions")
		os.Exit(2)
	}
	impl = strings.ToLower(strings.TrimSpace(impl))

	prof, err := parseProfile(profile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	q, err := parseQuality(quality)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	bx, by, bz, err := parseBlock3D(block)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if iters <= 0 {
		fmt.Fprintln(os.Stderr, "iters must be > 0")
		os.Exit(2)
	}

	pix := make([]byte, width*height*depth*4)
	fillPatternRGBA8(pix, width, height, depth)

	var cpuFile *os.File
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		cpuFile = f
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer func() {
			pprof.StopCPUProfile()
			_ = cpuFile.Close()
		}()
	}

	start := time.Now()
	var checksum uint64
	doChecksum := strings.ToLower(strings.TrimSpace(checksumOpt)) != "none"
	var last []byte
	var enc *native.Encoder
	if impl == "native" || impl == "cgo" {
		if !native.Enabled() {
			fmt.Fprintln(os.Stderr, "native impl requested but not enabled (build with -tags astcenc_native and CGO_ENABLED=1)")
			os.Exit(2)
		}
		enc, err = native.NewEncoder(bx, by, bz, prof, q, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer enc.Close()
	}
	for i := 0; i < iters; i++ {
		var out []byte
		switch impl {
		case "go":
			if depth == 1 && bz == 1 {
				out, err = astc.EncodeRGBA8WithProfileAndQuality(pix[:width*height*4], width, height, bx, by, prof, q)
			} else {
				out, err = astc.EncodeRGBA8VolumeWithProfileAndQuality(pix, width, height, depth, bx, by, bz, prof, q)
			}
		case "native", "cgo":
			out, err = enc.EncodeRGBA8Volume(pix, width, height, depth)
		default:
			fmt.Fprintln(os.Stderr, "invalid -impl (want go|native)")
			os.Exit(2)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if doChecksum {
			checksum = fnv1a64(checksum, out)
		}
		last = out
	}
	dur := time.Since(start)

	if outPath != "" {
		if err := os.WriteFile(outPath, last, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	texels := float64(width*height*depth) * float64(iters)
	mpixPerS := texels / dur.Seconds() / 1e6

	checksumStr := fmtChecksum(checksum)
	if !doChecksum {
		checksumStr = "none"
	}

	implOut := impl
	if implOut == "cgo" {
		implOut = "native"
	}
	fmt.Printf("RESULT impl=%s mode=encode profile=%s block=%s size=%dx%dx%d iters=%d seconds=%.6f mpix/s=%.3f checksum=%s\n",
		implOut,
		profile,
		block,
		width, height, depth,
		iters,
		dur.Seconds(),
		mpixPerS,
		checksumStr,
	)
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

func parseBlock3D(s string) (x, y, z int, err error) {
	parts := strings.Split(s, "x")
	switch len(parts) {
	case 2:
		_, err = fmt.Sscanf(s, "%dx%d", &x, &y)
		z = 1
	case 3:
		_, err = fmt.Sscanf(s, "%dx%dx%d", &x, &y, &z)
	default:
		return 0, 0, 0, fmt.Errorf("invalid -block %q (want like 4x4 or 4x4x4)", s)
	}
	if err != nil || x <= 0 || y <= 0 || z <= 0 || x > 255 || y > 255 || z > 255 {
		return 0, 0, 0, fmt.Errorf("invalid -block %q (want like 4x4 or 4x4x4)", s)
	}
	return x, y, z, nil
}

func fillPatternRGBA8(pix []byte, width, height, depth int) {
	for z := 0; z < depth; z++ {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				off := ((z*height+y)*width + x) * 4
				r := uint32(x*3 + y*5 + z*7)
				g := uint32(x*11 + y*13 + z*17)
				b := uint32(x ^ y ^ z)
				a := 255 - uint32((x*5+y*7+z*3)&0xFF)
				pix[off+0] = uint8(r)
				pix[off+1] = uint8(g)
				pix[off+2] = uint8(b)
				pix[off+3] = uint8(a)
			}
		}
	}
}

func fnv1a64(seed uint64, data []byte) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := seed
	if h == 0 {
		h = offset64
	}
	for _, b := range data {
		h ^= uint64(b)
		h *= prime64
	}
	return h
}

func fnv1a64Float32(seed uint64, data []float32) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := seed
	if h == 0 {
		h = offset64
	}
	for _, v := range data {
		u := math.Float32bits(v)
		h ^= uint64(byte(u))
		h *= prime64
		h ^= uint64(byte(u >> 8))
		h *= prime64
		h ^= uint64(byte(u >> 16))
		h *= prime64
		h ^= uint64(byte(u >> 24))
		h *= prime64
	}
	return h
}

func fmtChecksum(v uint64) string {
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[7-i] = byte(v >> uint(i*8))
	}
	return hex.EncodeToString(b[:])
}
