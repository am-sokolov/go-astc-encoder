# go-astc-encoder

This is a Go-focused source package containing:

- A **pure-Go** ASTC (`.astc`) encoder/decoder library (`astc/`).
- An optional **CGO/native** backend that builds and calls the upstream **C++ astcenc** implementation (`astc/native/`).
- Go tests plus a small, versioned test corpus under `astc/testdata/`.

## License

- Apache 2.0 (`LICENSE.txt`).

## Repository layout

- `astc/` — pure-Go ASTC container + codec (encode RGBA8 and RGBAF32 for HDR profiles; decode RGBA8 and RGBAF32)
- `astc/native/` — CGO/native wrapper around upstream `astcenc` (C++ sources vendored in `astc/native/internal/astcenc/upstream/`)
- `astc/testdata/` — regression fixtures and image corpus for Go tests
- `cmd/astcencgo/` — minimal CLI for encoding images to `.astc` and decoding `.astc` to PNG
- `cmd/astcbench/` — benchmark harness (synthetic input) for encode/decode throughput

## Build and test

Pure Go:

```sh
go test ./...
```

CGO/native parity tests (requires a C++ compiler and CGO enabled):

```sh
CGO_ENABLED=1 go test -tags astcenc_native ./...
```

## CLI (`astcencgo`)

Encode an image to ASTC (pure Go):

```sh
go run ./cmd/astcencgo -encode -impl go -in input.png -out out.astc -block 6x6 -profile ldr -quality medium
```

Encode using the native backend:

```sh
CGO_ENABLED=1 go run -tags astcenc_native ./cmd/astcencgo -encode -impl native -in input.png -out out.astc -block 6x6 -profile ldr -quality thorough
```

Decode ASTC to PNG:

```sh
go run ./cmd/astcencgo -decode -impl go -in out.astc -out out.png -profile ldr
CGO_ENABLED=1 go run -tags astcenc_native ./cmd/astcencgo -decode -impl native -in out.astc -out out.native.png -profile ldr
```

## Using as a Go package

Module path: `github.com/arm-software/astc-encoder`

Minimal example (encode RGBA8, then decode):

```go
import "github.com/arm-software/astc-encoder/astc"

astcData, err := astc.EncodeRGBA8WithProfileAndQuality(rgbaPix, w, h, 6, 6, astc.ProfileLDR, astc.EncodeMedium)
pix, w, h, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
```

If you want the upstream C++ reference implementation via CGO:

```go
import "github.com/arm-software/astc-encoder/astc/native"
```

Build with `-tags astcenc_native` and `CGO_ENABLED=1`.

## API overview

ASTC files do not store a “profile” in the container; you must pass the profile you intend to use at
encode/decode time (see `Profile{LDR,LDRSRGB,HDRRGBLDRAlpha,HDR}`).

### Package `astc` (pure Go)

#### Data types and constants

- `type Header` (in `astc/container.go`) is the 16-byte `.astc` header.
  - `Header.BlockCount()` returns `(blocksX, blocksY, blocksZ, total)`.
  - `HeaderSize` is the header byte size (`16`).
- `BlockBytes` is the ASTC block payload size (`16`).
- Pixel buffer layouts:
  - RGBA8: `[]byte` length `width*height*depth*4`, in x-major, then y, then z order:
    `((z*height+y)*width + x) * 4`.
  - RGBAF32: `[]float32` length `width*height*depth*4`, same layout. For HDR profiles values may
    be outside `[0,1]` (matches reference decoder behavior).

#### Container parsing

- `ParseHeader(data []byte) (Header, error)` — validate and parse a 16-byte ASTC header.
- `MarshalHeader(h Header) ([HeaderSize]byte, error)` — encode a header (validates dimensions).
- `ParseFile(data []byte) (Header, blocks []byte, error)` — parse a full file and return a blocks
  slice (aliases `data`).

Example: inspect dimensions without decoding:

```go
h, err := astc.ParseHeader(astcData)
if err != nil { /* ... */ }
fmt.Printf("block=%dx%dx%d size=%dx%dx%d\n", h.BlockX, h.BlockY, h.BlockZ, h.SizeX, h.SizeY, h.SizeZ)
```

#### Encode (RGBA8 source)

- `EncodeRGBA8(pix, width, height, blockX, blockY)` — convenience wrapper for LDR+Medium.
- `EncodeRGBA8WithProfileAndQuality(pix, width, height, blockX, blockY, profile, quality)` —
  encode a 2D RGBA8 image into a `.astc` file.
- `EncodeRGBA8Volume(pix, width, height, depth, blockX, blockY, blockZ)` — LDR+Medium, 3D.
- `EncodeRGBA8VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, profile, quality)` —
  encode a 3D RGBA8 volume.

Example: encode RGBA8 to ASTC:

```go
astcData, err := astc.EncodeRGBA8WithProfileAndQuality(rgbaPix, w, h, 6, 6, astc.ProfileLDR, astc.EncodeMedium)
if err != nil { /* ... */ }
```

Notes:
- For true HDR source encoding (values outside `[0,1]`) use the RGBAF32 encode APIs below.

#### Encode (RGBAF32 source; HDR profiles)

- `EncodeRGBAF32WithProfileAndQuality(pix, width, height, blockX, blockY, profile, quality)` —
  encode an RGBAF32 2D image into a `.astc` file.
- `EncodeRGBAF32VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, profile, quality)` —
  encode an RGBAF32 3D volume.

Notes:
- Supported profiles: `ProfileHDR` and `ProfileHDRRGBLDRAlpha`.
- For `ProfileHDRRGBLDRAlpha`, the encoder treats alpha as LDR (clamped to `[0,1]`) and encodes it
  as UNORM16 endpoints (matching the profile semantics).

#### Decode (RGBA8 output; LDR profiles only)

- `DecodeRGBA8(astcData)` — LDR, 2D convenience wrapper.
- `DecodeRGBA8WithProfile(astcData, profile)` — decode a 2D `.astc` file into RGBA8.
  - Limitations: only `ProfileLDR` and `ProfileLDRSRGB`.
- `DecodeRGBA8VolumeWithProfile(astcData, profile)` — decode into a newly allocated `[]byte`.
- `DecodeRGBA8VolumeWithProfileInto(astcData, profile, dst)` — decode into caller-provided `dst`.
- `DecodeRGBA8VolumeFromParsedWithProfileInto(profile, header, blocks, dst)` — like above, but
  skips parsing (useful for benchmarks / repeated decode).

Example: decode to RGBA8:

```go
pix, w, h, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
if err != nil { /* ... */ }
_ = pix // length is w*h*4
```

#### Decode (RGBAF32 output; supports HDR profiles)

- `DecodeRGBAF32WithProfile(astcData, profile)` — decode a 2D `.astc` file into `[]float32`.
- `DecodeRGBAF32VolumeWithProfile(astcData, profile)` — decode into a newly allocated `[]float32`.
- `DecodeRGBAF32VolumeWithProfileInto(astcData, profile, dst)` — decode into caller-provided `dst`.
- `DecodeRGBAF32VolumeFromParsedWithProfileInto(profile, header, blocks, dst)` — skip parsing.

Example: HDR decode to float32 (2D):

```go
pix, w, h, err := astc.DecodeRGBAF32WithProfile(astcData, astc.ProfileHDR)
if err != nil { /* ... */ }
_ = pix // length is w*h*4; values may be outside [0,1] for HDR blocks
```

Example: reuse parsed blocks + output buffer:

```go
h, blocks, err := astc.ParseFile(astcData)
if err != nil { /* ... */ }
dst := make([]float32, int(h.SizeX*h.SizeY*h.SizeZ)*4)
err = astc.DecodeRGBAF32VolumeFromParsedWithProfileInto(astc.ProfileHDR, h, blocks, dst)
```

#### Constant-color block helpers (advanced)

- `EncodeConstBlockRGBA8(r,g,b,a)` / `EncodeConstBlockUNorm16(...)` — construct a single 16-byte
  constant-color block payload.
- `DecodeConstBlockRGBA8(block)` — decode constant blocks to RGBA8 (UNORM16 and FP16 constant
  blocks; FP16 values clamp to `[0,1]` when converting to 8-bit).

### Package `astc/native` (CGO → upstream C++)

Build-gated: enable with `-tags astcenc_native` and `CGO_ENABLED=1` (`native.Enabled()` reports
availability).

This package mirrors the `astc` surface for encoding RGBA8 and decoding RGBA8/RGBAF32, but routes
through upstream `astcenc`.

#### Convenience functions

- `native.EncodeRGBA8WithProfileAndQuality(...)` / `native.EncodeRGBA8VolumeWithProfileAndQuality(...)`
- `native.DecodeRGBA8WithProfile(...)` / `native.DecodeRGBA8VolumeWithProfile(...)`
- `native.DecodeRGBAF32VolumeWithProfile(...)` (treat 2D as `depth=1`)
- `native.Decode*FromParsedWithProfileInto(...)` variants (skip parsing; reuse buffers)

#### Reusable contexts (recommended for repeated work)

- `native.NewEncoder(blockX, blockY, blockZ, profile, quality, threadCount)` → `*native.Encoder`
  - `(*Encoder).EncodeRGBA8(...)` / `(*Encoder).EncodeRGBA8Volume(...)`
  - `(*Encoder).Close()`
- `native.NewEncoderF32(blockX, blockY, blockZ, profile, quality, threadCount)` → `*native.EncoderF32`
  - `(*EncoderF32).EncodeRGBAF32(...)` / `(*EncoderF32).EncodeRGBAF32Volume(...)`
  - `(*EncoderF32).Close()`
- `native.NewEncoderF16(...)` → `*native.EncoderF16` (half-float input, `[]uint16` IEEE 754 binary16 bits)
- `native.NewDecoder(blockX, blockY, blockZ, profile, threadCount)` → `*native.Decoder`
  - `(*Decoder).DecodeRGBA8VolumeInto(...)`
  - `(*Decoder).DecodeRGBAF32VolumeInto(...)`
  - `(*Decoder).Close()`

Example: select native when available, otherwise fall back to pure Go:

```go
if native.Enabled() {
	enc, _ := native.NewEncoder(6, 6, 1, astc.ProfileLDR, astc.EncodeMedium, 0)
	defer enc.Close()
	astcData, _ = enc.EncodeRGBA8(rgbaPix, w, h)
} else {
	astcData, _ = astc.EncodeRGBA8WithProfileAndQuality(rgbaPix, w, h, 6, 6, astc.ProfileLDR, astc.EncodeMedium)
}
```

## Benchmarks (Go vs CGO/native)

### Methodology

- Synthetic input pattern (identical in pure Go and CGO/native)
  - LDR/SRGB encode: RGBA8
  - HDR encode: RGBAF32 (values include > 1.0)
- Size: **1024×1024×1**
- Single-threaded: `GOMAXPROCS=1` (native backend uses the same thread count)
- Checksum disabled: `-checksum none`
- Decode benchmark uses a payload encoded by the **CGO/native** backend for that block size.

### Environment (example results)

- Host: **Apple M3 Max** (arm64), macOS
- Go: **go1.24.0**
- C++ compiler: **Apple clang 17.0.0**
- C++ ISA: **NEON**

### Results (mpix/s)

**Encode — `-profile ldr`, `-checksum none`**

| Block | Quality | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|
| 4×4 | medium   | 1.244 | 0.935 |
| 6×6 | medium   | 1.864 | 0.598 |
| 8×8 | medium   | 1.794 | 0.446 |
| 4×4 | thorough | 0.358 | 0.551 |
| 6×6 | thorough | 0.530 | 0.318 |
| 8×8 | thorough | 0.566 | 0.236 |

**Encode — `-profile hdr`, `-checksum none`**

| Block | Quality | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|
| 4×4 | medium   | 2.948 | 1.203 |
| 6×6 | medium   | 4.149 | 0.744 |
| 8×8 | medium   | 5.084 | 0.428 |
| 4×4 | thorough | 1.613 | 0.614 |
| 6×6 | thorough | 1.979 | 0.357 |
| 8×8 | thorough | 2.297 | 0.239 |

**Decode (RGBA8) — `-profile ldr`, `-checksum none`, `-iters 200`**

| Block | Go (pure) | Go (CGO/native) |
|---|---:|---:|
| 4×4 | 91.002  | 103.694 |
| 6×6 | 126.532 | 124.595 |
| 8×8 | 172.159 | 146.553 |

**Decode (RGBAF32; HDR) — `-profile hdr`, `-out f32`, `-checksum none`, `-iters 200`**

| Block | Go (pure) | Go (CGO/native) |
|---|---:|---:|
| 4×4 | 86.540 | 107.314 |
| 6×6 | 115.395 | 137.785 |
| 8×8 | 134.567 | 168.292 |

**Encode preset sweep (6×6 only) — 512×512×1, `-profile ldr`, `-checksum none`**

Iteration counts used: `fastest=50`, `fast=20`, `medium=10`, `thorough=3`, `verythorough=2`, `exhaustive=1`.

| Quality | Go (pure) | Go (CGO/native) |
|---|---:|---:|
| fastest      | 100.095 | 2.113 |
| fast         | 9.767   | 1.056 |
| medium       | 1.824   | 0.590 |
| thorough     | 0.526   | 0.319 |
| verythorough | 0.180   | 0.159 |
| exhaustive   | 0.140   | 0.123 |

### Notes on interpretation

- **Encode “quality presets” are not directly comparable** between the pure-Go encoder and the upstream `astcenc` encoder. The CGO/native path matches the C++ encoder and is the right baseline for quality/perf parity.
- Pure-Go RGBAF32 decode uses precomputed UNORM16/LNS→float32 tables for performance.
- Decode outputs are bit-exact between pure Go and C++ for the exercised test corpus (`go test -tags astcenc_native ./astc`).

### Reproducing the benchmarks

Build the bench binaries:

```sh
go build -o ./astcbenchgo ./cmd/astcbench
CGO_ENABLED=1 go build -tags astcenc_native -o ./astcbenchgo_native ./cmd/astcbench
```

Generate a payload using the native backend (example: 6×6 LDR):

```sh
./astcbenchgo_native encode -w 1024 -h 1024 -block 6x6 -profile ldr -quality medium -iters 1 -checksum none -out /tmp/bench.astc -impl native
```

Run decode benchmarks against the same payload:

```sh
GOMAXPROCS=1 ./astcbenchgo        decode -in /tmp/bench.astc -profile ldr -iters 200 -out u8  -checksum none
GOMAXPROCS=1 ./astcbenchgo_native decode -in /tmp/bench.astc -profile ldr -iters 200 -out u8  -checksum none -impl native
```

HDR example (float output):

```sh
./astcbenchgo_native encode -w 1024 -h 1024 -block 6x6 -profile hdr -quality medium -iters 1 -checksum none -out /tmp/bench_hdr.astc -impl native
GOMAXPROCS=1 ./astcbenchgo        decode -in /tmp/bench_hdr.astc -profile hdr -iters 200 -out f32 -checksum none
GOMAXPROCS=1 ./astcbenchgo_native decode -in /tmp/bench_hdr.astc -profile hdr -iters 200 -out f32 -checksum none -impl native
```
