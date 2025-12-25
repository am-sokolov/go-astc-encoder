# Benchmarks

This folder contains a small harness to compare:

- **C++** `astcenc` (e.g. `-mavx2` build)
- **Go** port (optionally built with `GOEXPERIMENT=simd`)

## Quick start (x86_64)

Run:

`bench/compare.sh`

This will:

1. Build a C++ AVX2 static library via CMake.
2. Build the C++ benchmark binary `build-bench-cpp-avx2/astcbenchcpp`.
3. Build the Go benchmark binary `build-bench-go/astcbenchgo` (uses `gotip` if available).
4. Run encode (synthetic input) and decode (same `.astc` payload) and print throughput.

You can override parameters:

`W=1024 H=1024 BLOCK=6x6 PROFILE=ldr QUALITY=medium ITERS_DECODE=50 bench/compare.sh`

Optional:
- Set `CHECKSUM=none` to benchmark codec throughput without the per-iteration checksum.

Notes:
- `bench/compare.sh` runs the C++ library single-threaded; set `GO_PROCS=1` (default) for an apples-to-apples single-core comparison of Go vs C++.

## Manual runs

Go:

`GOEXPERIMENT=simd GOMAXPROCS=1 gotip run ./cmd/astcbench decode -in <file.astc> -profile ldr -iters 200 -out u8`

C++:

1. Build `libastcenc-<isa>-static.a` using CMake (e.g. `-DASTCENC_ISA_AVX2=ON`).
2. Compile `bench/astcbenchcpp.cpp` and link against the static library.
