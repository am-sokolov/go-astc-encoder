# go-astc-encoder

This repository is based on the upstream Arm ASTC encoder (`astcenc`) project:
`https://github.com/ARM-software/astc-encoder`.

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

Module path: `https://github.com/am-sokolov/go-astc-encoder`

Minimal example (encode RGBA8, then decode):

```go
import "https://github.com/am-sokolov/go-astc-encoder/astc"

astcData, err := astc.EncodeRGBA8WithProfileAndQuality(rgbaPix, w, h, 6, 6, astc.ProfileLDR, astc.EncodeMedium)
pix, w, h, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
```

If you want the upstream C++ reference implementation via CGO:

```go
import "https://github.com/am-sokolov/go-astc-encoder/astc/native"
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

#### Advanced: `Config` / `Context` API (astcenc-like)

If you need **upstream-like flags, swizzles, progress callbacks, or block introspection**, use the
lower-level API modeled after `astcenc`:

- `ConfigInit(profile, blockX, blockY, blockZ, quality, flags)` → `Config` (quality is `0..100`).
- `ContextAlloc(&cfg, threadCount)` → `*Context`
- `(*Context).CompressImage(img, swizzle, outBlocks, threadIndex)` — writes **block payloads only**
  (no `.astc` header).
- `(*Context).DecompressImage(blocks, imgOut, swizzle, threadIndex)`
  - Unlike compression, decompression swizzles may use `SwzZ` (see below).
- `(*Context).GetBlockInfo(block)` — inspect mode/partitions/endpoints/weights (useful for parity
  debugging).

Useful `Config` fields:

- `Flags` — enable upstream `ASTCENC_FLG_*` behaviors (`FlagMapNormal`, `FlagUseAlphaWeight`, ...).
- `CWRWeight/CWGWeight/CWBWeight/CWAWeight` — per-channel error weights.
- `AScaleRadius` — alpha-scale RDO (for 2D blocks, blocks whose filtered alpha footprint is fully
  transparent are emitted as constant-zero blocks; matches upstream).
- `ProgressCallback func(progress float32)` — progress callback (`0..100`), throttled to ~1% or
  4096 blocks (whichever is larger), always emitting `100` at completion (matches upstream).

Errors:

- `ErrorCode`, `ErrorString(code)` — upstream-style error codes (`astcenc_get_error_string` parity).
- `ErrorCodeOf(err)` — extract an `ErrorCode` from a returned error.

Swizzles:

- Compression swizzle selectors: `Swz{R,G,B,A,0,1}` (`SwzZ` is invalid for compression, matching
  upstream).
- Decompression swizzles additionally allow `SwzZ` which reconstructs a normal-map **Z** component
  from **R and A** (matching upstream).

Normal maps (`FlagMapNormal`):

- Upstream normal-map mode stores normals as a **2-component X+Y** map. It expects an input swizzle
  like `rrrg` (or `gggr`) so the data can be encoded using **Luminance+Alpha** blocks.
- Pure-Go `FlagMapNormal` encoding uses **Luminance+Alpha endpoints** (endpoint mode `4`) and an
  **angular normal error metric** during block search; it disables partition preselection to avoid
  missing the best partition under the angular metric.

Example: encode a tangent-space normal map (X in R, Y in G) using `rrrg`, then decode to XYZ using
`SwzZ`:

```go
cfg, _ := astc.ConfigInit(astc.ProfileLDR, 6, 6, 1, 60, astc.FlagMapNormal)
ctx, _ := astc.ContextAlloc(&cfg, 1)
defer ctx.Close()

img := &astc.Image{DimX: w, DimY: h, DimZ: 1, DataType: astc.TypeU8, DataU8: rgba}
blocks := make([]byte, ((w+5)/6)*((h+5)/6)*astc.BlockBytes)

// Store X in RGB (luma), Y in A.
encSwz := astc.Swizzle{R: astc.SwzR, G: astc.SwzR, B: astc.SwzR, A: astc.SwzG} // rrrg
_ = ctx.CompressImage(img, encSwz, blocks, 0)

// Reconstruct Z into B, output XYZ in RGB, and set A=1.
out := make([]byte, w*h*4)
imgOut := &astc.Image{DimX: w, DimY: h, DimZ: 1, DataType: astc.TypeU8, DataU8: out}
decSwz := astc.Swizzle{R: astc.SwzR, G: astc.SwzA, B: astc.SwzZ, A: astc.Swz1}
_ = ctx.DecompressImage(blocks, imgOut, decSwz, 0)
```

RGBM (`FlagMapRGBM`):

- Use this when your input texture is **RGBM-encoded** (HDR color stored as `RGB * M * RGBMMScale`).
- `cfg.RGBMMScale` defaults to `5.0` for `FlagMapRGBM`; set it to match your RGBM encoding scheme
  before calling `ContextAlloc`.

Decode rounding (`FlagUseDecodeUNORM8`):

- Enables encoder heuristics assuming the final decode uses `decode_unorm8` rounding (matches
  upstream). This can improve quality when the output is ultimately stored as 8-bit.
- `ProfileLDRSRGB` always assumes `decode_unorm8` for error evaluation (matches upstream).

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

#### Raw `astcenc` API (native `Config` / `Context`)

In addition to the convenience `Encoder`/`Decoder` wrappers, `astc/native` also exposes a
lower-level `astcenc`-style API which mirrors the pure-Go `Config`/`Context` surface and calls
directly into upstream `astcenc`:

- `native.ConfigInit(profile, blockX, blockY, blockZ, quality, flags)` → `native.Config`
- `native.ContextAlloc(&cfg, threadCount)` → `*native.Context`
- `(*native.Context).CompressImage(...)`, `(*native.Context).DecompressImage(...)`
- `(*native.Context).GetBlockInfo(...)`

This raw API is used by the parity tests to compare pure-Go behavior to upstream, including:

- `SwzZ` decompression behavior.
- `FlagMapNormal` encode quality (angular error) relative to upstream.

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
| 4×4 | medium   | 0.654 | 1.179 |
| 6×6 | medium   | 1.217 | 0.741 |
| 8×8 | medium   | 1.391 | 0.435 |
| 4×4 | thorough | 0.116 | 0.618 |
| 6×6 | thorough | 0.233 | 0.353 |
| 8×8 | thorough | 0.312 | 0.238 |

**Encode — `-profile hdr-rgb-ldr-a`, `-checksum none`**

| Block | Quality | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|
| 4×4 | medium   | 0.681 | 0.892 |
| 6×6 | medium   | 1.246 | 0.567 |
| 8×8 | medium   | 1.422 | 0.423 |
| 4×4 | thorough | 0.126 | 0.481 |
| 6×6 | thorough | 0.241 | 0.307 |
| 8×8 | thorough | 0.326 | 0.236 |

**Decode (RGBA8) — `-profile ldr`, `-checksum none`, `-iters 200`**

| Block | Go (pure) | Go (CGO/native) |
|---|---:|---:|
| 4×4 | 91.002  | 103.694 |
| 6×6 | 126.532 | 124.595 |
| 8×8 | 172.159 | 146.553 |

**Decode (RGBAF32; HDR) — `-profile hdr`, `-out f32`, `-checksum none`, `-iters 200`**

| Block | Go (pure) | Go (CGO/native) |
|---|---:|---:|
| 4×4 | 91.162 | 111.053 |
| 6×6 | 121.411 | 139.529 |
| 8×8 | 146.132 | 169.509 |

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

HDR RGB + LDR alpha example (float output):

```sh
./astcbenchgo_native encode -w 1024 -h 1024 -block 6x6 -profile hdr-rgb-ldr-a -quality medium -iters 1 -checksum none -out /tmp/bench_hdr_rgb_ldr_a.astc -impl native
GOMAXPROCS=1 ./astcbenchgo        decode -in /tmp/bench_hdr_rgb_ldr_a.astc -profile hdr-rgb-ldr-a -iters 200 -out f32 -checksum none
GOMAXPROCS=1 ./astcbenchgo_native decode -in /tmp/bench_hdr_rgb_ldr_a.astc -profile hdr-rgb-ldr-a -iters 200 -out f32 -checksum none -impl native
```

## Acknowledgments

- Based on Arm's ASTC Encoder (`astcenc`) reference implementation: `https://github.com/ARM-software/astc-encoder`.
