# go-astc-encoder

This is a redistributable source package containing:

- A **pure-Go** ASTC (.astc) encoder/decoder library (`astc/`).
- An optional **CGO/native** backend that calls the upstream **C++ astcenc** implementation (`astc/native/`).
- The upstream **C++ astcenc** source tree (`Source/`) for reference, parity work, and native benchmarking.
- Tests (unit + regression), sample textures (CC0), and scripts for reproducible comparisons.

The goal is to make it easy to:

- Use ASTC in Go without external dependencies (pure Go path).
- Cross-check correctness against the reference C++ encoder/decoder (CGO path and tests).
- Benchmark and compare **Go vs CGO vs native C++** on the same workloads.

## Origins and acknowledgements

This package is derived from and/or includes:

- **Arm astcenc (reference implementation)** — source in `Source/` (Apache-2.0).
  - The CGO backend vendors the required upstream `.cpp/.h` into `astc/native/internal/astcenc/upstream/` for self-contained builds.
- **Third-party C/C++ single-header libraries** used by the upstream project:
  - `Source/ThirdParty/stb_image.h`, `Source/ThirdParty/stb_image_write.h` (MIT or Public Domain)
  - `Source/ThirdParty/tinyexr.h` (BSD-3-Clause + bundled OpenEXR license + bundled miniz public-domain notice)
- **Sample textures** from **Poly Haven**, downloaded by `samples/fetch_polyhaven.sh` and stored in `samples/textures/` (CC0/Public Domain).

All third-party licenses and texture provenance are bundled under `LICENSES/` and `samples/textures/SOURCES.tsv`.

## License

- Primary license: Apache 2.0 (`LICENSE.txt`).
- Third-party notices: `LICENSES/README.md`.

## Repository layout (high level)

- `astc/` — pure-Go ASTC container + codec (encode RGBA8 LDR; decode RGBA8 and RGBAF32, incl. HDR profiles)
- `astc/native/` — optional CGO/native wrapper around the upstream C++ implementation
- `cmd/astcencgo/` — minimal CLI for encoding images to `.astc` and decoding `.astc` to PNG
- `cmd/astcbench/` — Go benchmark harness (synthetic input) for encode/decode throughput
- `bench/` — C++ benchmark harness (`bench/astcbenchcpp.cpp`) and helper scripts
- `samples/` — scripts + metadata for CC0 textures and result generation
- `results/` — generated comparison outputs (ASTC + decoded PNGs) and `results/manifest.tsv`

## Build, test, install

### Requirements

- Go **1.24+** (this package uses `go 1.24.0` in `go.mod`)
- Optional (for CGO/native backend and C++ benchmarking):
  - A C++ toolchain (e.g. clang++/g++)
  - `CGO_ENABLED=1`
  - CMake (only needed to build the C++ static library for `bench/astcbenchcpp.cpp`)

### Run tests

Pure Go:

```sh
go test ./...
```

CGO/native parity tests (requires a C++ compiler):

```sh
CGO_ENABLED=1 go test -tags astcenc_native ./...
```

### Install CLI tools

Pure-Go CLI (no CGO required):

```sh
go install ./cmd/astcencgo
go install ./cmd/astcbench
```

Native-enabled CLI (build tags + CGO):

```sh
CGO_ENABLED=1 go install -tags astcenc_native ./cmd/astcencgo
CGO_ENABLED=1 go install -tags astcenc_native ./cmd/astcbench
```

## How to run

### Encode / decode with the CLI (`astcencgo`)

Encode an image to ASTC (pure Go):

```sh
astcencgo -encode -impl go -in input.png -out out.astc -block 6x6 -profile ldr -quality medium
```

Encode using the native backend (requires `-tags astcenc_native` + `CGO_ENABLED=1` build):

```sh
astcencgo -encode -impl native -in input.png -out out.astc -block 6x6 -profile ldr -quality thorough
```

Decode ASTC to PNG:

```sh
astcencgo -decode -impl go -in out.astc -out out.png -profile ldr
astcencgo -decode -impl native -in out.astc -out out.native.png -profile ldr
```

### Sample textures and result generation

If the package does not already include textures, download CC0 textures:

```sh
./samples/fetch_polyhaven.sh
```

Generate outputs into `results/` (enc/dec matrix for both implementations; writes `results/manifest.tsv`):

```sh
./samples/run_compare.sh
```

## Using as a Go package

Module path: `github.com/arm-software/astc-encoder`

Minimal example (encode RGBA8 LDR, then decode):

```go
import "github.com/arm-software/astc-encoder/astc"

astcData, err := astc.EncodeRGBA8WithProfileAndQuality(rgbaPix, w, h, 6, 6, astc.ProfileLDR, astc.EncodeMedium)
pix, w, h, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
```

If you want the C++ reference implementation via CGO:

```go
import "github.com/arm-software/astc-encoder/astc/native"
```

Build with `-tags astcenc_native` and `CGO_ENABLED=1`.

## API overview

### `astc` (pure Go)

- **Encode (RGBA8)**
  - `EncodeRGBA8WithProfileAndQuality(...)`
  - `EncodeRGBA8VolumeWithProfileAndQuality(...)`
- **Decode (RGBA8)**
  - `DecodeRGBA8WithProfile(...)`
  - `DecodeRGBA8VolumeWithProfile(...)`
  - `DecodeRGBA8VolumeFromParsedWithProfileInto(...)`
- **Decode (RGBAF32; supports HDR profiles)**
  - `DecodeRGBAF32WithProfile(...)`
  - `DecodeRGBAF32VolumeWithProfile(...)`
  - `DecodeRGBAF32VolumeFromParsedWithProfileInto(...)`
- **Container parsing**
  - `ParseFile(...)`, `ParseHeader(...)`, `MarshalHeader(...)`
- **Enums**
  - `Profile{LDR,LDRSRGB,HDRRGBLDRAlpha,HDR}`
  - `EncodeQuality{Fastest,Fast,Medium,Thorough,VeryThorough,Exhaustive}`

### `astc/native` (CGO → upstream C++)

- Mirrors the `astc` decode surface for RGBA8 and RGBAF32 volume decode.
- Provides `Encoder`/`Decoder` reusable contexts:
  - `native.NewEncoder(...)`, `(*Encoder).EncodeRGBA8Volume(...)`
  - `native.NewDecoder(...)`, `(*Decoder).DecodeRGBA8VolumeInto(...)`, `(*Decoder).DecodeRGBAF32VolumeInto(...)`
- Build-gated:
  - Enabled only with `-tags astcenc_native` and `CGO_ENABLED=1` (`native.Enabled()` reports availability).

## Benchmarks (Go vs CGO vs native C++)

### Methodology

- Synthetic RGBA8 input pattern (identical in Go and C++)
- Size: **1024×1024×1**
- Single-threaded: `GOMAXPROCS=1` (native backend uses the same thread count)
- Checksum disabled: `-checksum none`
- Decode benchmark uses the **C++-encoded** payload for that block size (`*_medium.astc`; HDR uses `*_hdr_medium.astc`)

### Environment (example results)

- Host: **Apple M3 Max** (arm64), macOS
- Go: **go1.24.0**
- C++ compiler: **Apple clang 17.0.0**
- C++ ISA: **NEON**

### Results (mpix/s)

**Encode — `-profile ldr`, `-checksum none`**

| Block | Quality | C++ (bench) | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|---:|
| 4×4 | medium   | 0.937 | 1.244 | 0.935 |
| 6×6 | medium   | 0.598 | 1.864 | 0.598 |
| 8×8 | medium   | 0.450 | 1.794 | 0.446 |
| 4×4 | thorough | 0.558 | 0.358 | 0.551 |
| 6×6 | thorough | 0.315 | 0.530 | 0.318 |
| 8×8 | thorough | 0.237 | 0.566 | 0.236 |

**Encode — `-profile hdr`, `-checksum none`**

| Block | Quality | C++ (bench) | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|---:|
| 4×4 | medium   | 1.198 | 1.221 | 1.195 |
| 6×6 | medium   | 0.727 | 1.841 | 0.724 |
| 8×8 | medium   | 0.419 | 1.771 | 0.415 |
| 4×4 | thorough | 0.618 | 0.352 | 0.609 |
| 6×6 | thorough | 0.358 | 0.529 | 0.354 |
| 8×8 | thorough | 0.239 | 0.564 | 0.238 |

**Decode (RGBA8) — `-profile ldr`, `-checksum none`, `-iters 200`**

| Block | C++ (bench) | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|
| 4×4 | 107.209 | 91.002  | 103.694 |
| 6×6 | 130.353 | 126.532 | 124.595 |
| 8×8 | 155.225 | 172.159 | 146.553 |

**Decode (RGBAF32; HDR) — `-profile hdr`, `-out f32`, `-checksum none`, `-iters 200`**

| Block | C++ (bench) | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|
| 4×4 | 108.679 | 75.737 | 107.477 |
| 6×6 | 136.707 | 102.772 | 136.806 |
| 8×8 | 172.455 | 120.974 | 166.936 |

**Encode preset sweep (6×6 only) — 512×512×1, `-profile ldr`, `-checksum none`**

Iteration counts used: `fastest=50`, `fast=20`, `medium=10`, `thorough=3`, `verythorough=2`, `exhaustive=1`.

| Quality | C++ (bench) | Go (pure) | Go (CGO/native) |
|---|---:|---:|---:|
| fastest      | 2.124 | 100.095 | 2.113 |
| fast         | 1.060 | 9.767   | 1.056 |
| medium       | 0.598 | 1.824   | 0.590 |
| thorough     | 0.316 | 0.526   | 0.319 |
| verythorough | 0.160 | 0.180   | 0.159 |
| exhaustive   | 0.122 | 0.140   | 0.123 |

### Notes on interpretation

- **Encode “quality presets” are not directly comparable** between the pure-Go encoder and the upstream `astcenc` encoder. The CGO/native path matches the C++ encoder and is the right baseline for quality/perf parity.
- Pure-Go encode accepts HDR profiles for RGBA8 input, but does not currently implement true HDR source encoding (float input / HDR endpoint modes).
- Pure-Go RGBAF32 decode uses precomputed UNORM16/LNS→float32 tables for performance.
- Decode outputs are bit-exact between pure Go and C++ for the exercised test corpus (`go test -tags astcenc_native ./astc`).

### Reproducing the benchmarks

Go bench binaries:

```sh
go build -o ./astcbenchgo ./cmd/astcbench
CGO_ENABLED=1 go build -tags astcenc_native -o ./astcbenchgo_native ./cmd/astcbench
```

C++ bench binary (arm64 NEON example; adjust ISA on x86_64):

```sh
cmake -S . -B build-bench-cpp-neon \
  -DCMAKE_BUILD_TYPE=Release \
  -DASTCENC_CLI=OFF -DASTCENC_SHAREDLIB=OFF -DASTCENC_UNITTEST=OFF \
  -DASTCENC_ISA_NEON=ON -DASTCENC_ISA_NONE=OFF -DASTCENC_ISA_NATIVE=OFF
cmake --build build-bench-cpp-neon -j
c++ -O3 -std=c++14 -I Source bench/astcbenchcpp.cpp build-bench-cpp-neon/Source/libastcenc-neon-static.a -o build-bench-cpp-neon/astcbenchcpp -pthread
```

Quick end-to-end comparison helper (builds C++ bench + Go bench and runs one config):

```sh
W=1024 H=1024 BLOCK=6x6 PROFILE=ldr QUALITY=medium ITERS_ENCODE=5 ITERS_DECODE=200 CHECKSUM=none GO_PROCS=1 bench/compare.sh
```

Run one case (example: encode + decode, 6×6 medium):

```sh
./build-bench-cpp-neon/astcbenchcpp encode -w 1024 -h 1024 -block 6x6 -profile ldr -quality medium -iters 5 -checksum none -out /tmp/bench_6x6_medium.astc
GOMAXPROCS=1 ./astcbenchgo        encode -w 1024 -h 1024 -block 6x6 -profile ldr -quality medium -iters 5 -checksum none
GOMAXPROCS=1 ./astcbenchgo_native encode -w 1024 -h 1024 -block 6x6 -profile ldr -quality medium -iters 5 -checksum none -impl native

./build-bench-cpp-neon/astcbenchcpp decode -in /tmp/bench_6x6_medium.astc -profile ldr -iters 200 -out u8 -checksum none
GOMAXPROCS=1 ./astcbenchgo        decode -in /tmp/bench_6x6_medium.astc -profile ldr -iters 200 -out u8 -checksum none
GOMAXPROCS=1 ./astcbenchgo_native decode -in /tmp/bench_6x6_medium.astc -profile ldr -iters 200 -out u8 -checksum none -impl native
```

Run one HDR case (example: decode, 6×6 medium, float output):

```sh
./build-bench-cpp-neon/astcbenchcpp encode -w 1024 -h 1024 -block 6x6 -profile hdr -quality medium -iters 1 -checksum none -out /tmp/bench_hdr_6x6_medium.astc

./build-bench-cpp-neon/astcbenchcpp decode -in /tmp/bench_hdr_6x6_medium.astc -profile hdr -iters 200 -out f32 -checksum none
GOMAXPROCS=1 ./astcbenchgo        decode -in /tmp/bench_hdr_6x6_medium.astc -profile hdr -iters 200 -out f32 -checksum none
GOMAXPROCS=1 ./astcbenchgo_native decode -in /tmp/bench_hdr_6x6_medium.astc -profile hdr -iters 200 -out f32 -checksum none -impl native
```


Open a terminal, change to the appropriate directory for your system, and run
the astcenc encoder program, like this on Linux or macOS:

    ./astcenc

... or like this on Windows:

    astcenc

Invoking `astcenc -help` gives an extensive help message, including usage
instructions and details of all available command line options. A summary of
the main encoder options are shown below.

## Compressing an image

Compress an image using the `-cl` \ `-cs` \ `-ch` \ `-cH` modes. For example:

    astcenc -cl example.png example.astc 6x6 -medium

This compresses `example.png` using the LDR color profile and a 6x6 block
footprint (3.56 bits/pixel). The `-medium` quality preset gives a reasonable
image quality for a relatively fast compression speed, so is a good starting
point for compression. The output is stored to a linear color space compressed
image, `example.astc`.

The modes available are:

* `-cl` : use the linear LDR color profile.
* `-cs` : use the sRGB LDR color profile.
* `-ch` : use the HDR color profile, tuned for HDR RGB and LDR A.
* `-cH` : use the HDR color profile, tuned for HDR RGBA.

If you intend to use the resulting image with the decode mode extensions to
limit the decompressed precision to UNORM8, it is recommended that you also
specify the `-decode_unorm8` flag. This will ensure that the compressor uses
the correct rounding rules when choosing encodings.

## Decompressing an image

Decompress an image using the `-dl` \ `-ds` \ `-dh` \ `-dH` modes. For example:

    astcenc -dh example.astc example.tga

This decompresses `example.astc` using the full HDR feature profile, storing
the decompressed output to `example.tga`.

The modes available mirror the options used for compression, but use a `d`
prefix. Note that for decompression there is no difference between the two HDR
modes, they are both provided simply to maintain symmetry across operations.

## Measuring image quality

Review the compression quality using the `-tl` \ `-ts` \ `-th` \ `-tH` modes.
For example:

    astcenc -tl example.png example.tga 5x5 -thorough

This is equivalent to using using the LDR color profile and a 5x5 block size
to compress the image, using the `-thorough` quality preset, and then
immediately decompressing the image and saving the result. This can be used
to enable a visual inspection of the compressed image quality. In addition
this mode also prints out some image quality metrics to the console.

The modes available mirror the options used for compression, but use a `t`
prefix.

## Experimenting

Efficient real-time graphics benefits from minimizing compressed texture size,
as it reduces memory footprint, reduces memory bandwidth, saves energy, and can
improve texture cache efficiency. However, like any lossy compression format
there will come a point where the compressed image quality is unacceptable
because there are simply not enough bits to represent the output with the
precision needed. We recommend experimenting with the block footprint to find
the optimum balance between size and quality, as the finely adjustable
compression ratio is one of major strengths of the ASTC format.

The compression speed can be controlled from `-fastest`, through `-fast`,
`-medium` and `-thorough`, up to `-exhaustive`. In general, the more time the
encoder has to spend looking for good encodings the better the results, but it
does result in increasingly small improvements for the amount of time required.

There are many other command line options for tuning the encoder parameters
which can be used to fine tune the compression algorithm. See the command line
help message for more details.

# Documentation

The [ASTC Format Overview](./Docs/FormatOverview.md) page provides a high level
introduction to the ASTC texture format, how it encodes data, and why it is
both flexible and efficient.

The [Effective ASTC Encoding](./Docs/Encoding.md) page looks at some of the
guidelines that should be followed when compressing data using `astcenc`.
It covers:

* How to efficiently encode data with fewer than 4 channels.
* How to efficiently encode normal maps, sRGB data, and HDR data.
* Coding equivalents to other compression formats.

The [ASTC Developer Guide][5] document (external link) provides a more detailed
guide for developers using the `astcenc` compressor.

The [.astc File Format](./Docs/FileFormat.md) page provides a light-weight
specification for the `.astc` file format and how to read or write it.

The [Building ASTC Encoder](./Docs/Building.md) page provides instructions on
how to build `astcenc` from the sources in this repository.

The [Testing ASTC Encoder](./Docs/Testing.md) page provides instructions on
how to test any modifications to the source code in this repository.

# Support

If you have issues with the `astcenc` encoder, or questions about the ASTC
texture format itself, please raise them in the GitHub issue tracker.

If you have any questions about Arm GPUs, application development for Arm GPUs,
or general mobile graphics development or technology please submit them on the
[Arm Community graphics forums][4].

- - -

_Copyright © 2013-2025, Arm Limited and contributors. All rights reserved._

[1]: ./Docs/FormatOverview.md
[2]: https://www.khronos.org/registry/DataFormat/specs/1.4/dataformat.1.4.html#ASTC
[3]: https://github.com/ARM-software/astc-encoder/releases
[4]: https://community.arm.com/support-forums/f/graphics-gaming-and-vr-forum/
[5]: https://developer.arm.com/documentation/102162/latest/?lang=en
