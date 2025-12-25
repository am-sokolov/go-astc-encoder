#!/usr/bin/env bash
set -euo pipefail

# Compare C++ astcenc (SIMD: AVX2 on x86_64, NEON on arm64) and Go on the same synthetic workload.
#
# Usage (x86_64):
#   bench/compare.sh
#
# Env overrides:
#   W=256 H=256 D=1 BLOCK=4x4 PROFILE=ldr QUALITY=medium ITERS_DECODE=200 ITERS_ENCODE=20

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

W="${W:-256}"
H="${H:-256}"
D="${D:-1}"
BLOCK="${BLOCK:-4x4}"
PROFILE="${PROFILE:-ldr}"
QUALITY="${QUALITY:-medium}"
ITERS_DECODE="${ITERS_DECODE:-200}"
ITERS_ENCODE="${ITERS_ENCODE:-20}"
GO_PROCS="${GO_PROCS:-1}"
CHECKSUM="${CHECKSUM:-fnv}"

ARCH="$(uname -m)"
OS="$(uname -s)"

ISA=""
CPP_LIB=""

if [[ "$ARCH" == "x86_64" ]]; then
  ISA="avx2"
  BUILD_CPP="${BUILD_CPP:-build-bench-cpp-avx2}"
  CPP_LIB="$BUILD_CPP/Source/libastcenc-avx2-static.a"

  echo "Building C++ ($ISA) in $BUILD_CPP ..."
  cmake -S . -B "$BUILD_CPP" \
    -DCMAKE_BUILD_TYPE=Release \
    -DASTCENC_CLI=OFF \
    -DASTCENC_SHAREDLIB=OFF \
    -DASTCENC_UNITTEST=OFF \
    -DASTCENC_ISA_AVX2=ON \
    -DASTCENC_ISA_SSE41=OFF \
    -DASTCENC_ISA_SSE2=OFF \
    -DASTCENC_ISA_NEON=OFF \
    -DASTCENC_ISA_NONE=OFF \
    -DASTCENC_ISA_NATIVE=OFF \
    ${OS:+-DASTCENC_UNIVERSAL_BUILD=OFF} >/dev/null
elif [[ "$ARCH" == "arm64" || "$ARCH" == "aarch64" ]]; then
  ISA="neon"
  BUILD_CPP="${BUILD_CPP:-build-bench-cpp-neon}"
  CPP_LIB="$BUILD_CPP/Source/libastcenc-neon-static.a"

  echo "Building C++ ($ISA) in $BUILD_CPP ..."
  cmake -S . -B "$BUILD_CPP" \
    -DCMAKE_BUILD_TYPE=Release \
    -DASTCENC_CLI=OFF \
    -DASTCENC_SHAREDLIB=OFF \
    -DASTCENC_UNITTEST=OFF \
    -DASTCENC_ISA_AVX2=OFF \
    -DASTCENC_ISA_SSE41=OFF \
    -DASTCENC_ISA_SSE2=OFF \
    -DASTCENC_ISA_NEON=ON \
    -DASTCENC_ISA_NONE=OFF \
    -DASTCENC_ISA_NATIVE=OFF \
    ${OS:+-DASTCENC_UNIVERSAL_BUILD=OFF} >/dev/null
else
  echo "bench/compare.sh: unsupported arch $ARCH (supported: x86_64, arm64/aarch64)" >&2
  exit 2
fi

BUILD_GO="${BUILD_GO:-build-bench-go}"

cmake --build "$BUILD_CPP" -j >/dev/null

CPP_BENCH="$BUILD_CPP/astcbenchcpp"
if [[ ! -f "$CPP_BENCH" ]]; then
  echo "Compiling $CPP_BENCH ..."
  c++ -O3 -std=c++14 -I Source bench/astcbenchcpp.cpp "$CPP_LIB" -o "$CPP_BENCH" -pthread
fi

echo "Building Go (GOEXPERIMENT=simd) in $BUILD_GO ..."
mkdir -p "$BUILD_GO"
GO_BIN="${GO_BIN:-gotip}"
if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  GO_BIN="go"
fi
if GOEXPERIMENT=simd "$GO_BIN" build -o "$BUILD_GO/astcbenchgo" ./cmd/astcbench 2>/dev/null; then
  :
else
  echo "warning: $GO_BIN does not support GOEXPERIMENT=simd; building without it" >&2
  "$GO_BIN" build -o "$BUILD_GO/astcbenchgo" ./cmd/astcbench
fi

TMP_ASTC="$BUILD_CPP/bench_input.astc"

echo
echo "== Encode (synthetic input) =="
"$CPP_BENCH" encode -w "$W" -h "$H" -d "$D" -block "$BLOCK" -profile "$PROFILE" -quality "$QUALITY" -iters "$ITERS_ENCODE" -checksum "$CHECKSUM" -out "$TMP_ASTC"
GOMAXPROCS="$GO_PROCS" "$BUILD_GO/astcbenchgo" encode -w "$W" -h "$H" -d "$D" -block "$BLOCK" -profile "$PROFILE" -quality "$QUALITY" -iters "$ITERS_ENCODE" -checksum "$CHECKSUM"

echo
echo "== Decode (same ASTC payload) =="
CPP_LINE="$("$CPP_BENCH" decode -in "$TMP_ASTC" -profile "$PROFILE" -iters "$ITERS_DECODE" -out u8 -checksum "$CHECKSUM")"
GO_LINE="$(GOMAXPROCS="$GO_PROCS" "$BUILD_GO/astcbenchgo" decode -in "$TMP_ASTC" -profile "$PROFILE" -iters "$ITERS_DECODE" -out u8 -checksum "$CHECKSUM")"
echo "$CPP_LINE"
echo "$GO_LINE"

CPP_MPIX="$(echo "$CPP_LINE" | sed -n 's/.* mpix\/s=\([0-9.]*\).*/\1/p')"
GO_MPIX="$(echo "$GO_LINE" | sed -n 's/.* mpix\/s=\([0-9.]*\).*/\1/p')"

if [[ -n "$CPP_MPIX" && -n "$GO_MPIX" ]]; then
  RATIO="$(python3 - <<PY
cpp=float("$CPP_MPIX")
go=float("$GO_MPIX")
print(cpp/go if go else 0.0)
PY
)"
  echo
  echo "Decode speed ratio (cpp/go): $RATIO"
fi
