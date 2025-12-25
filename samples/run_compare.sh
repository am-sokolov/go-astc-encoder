#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SAMPLES_DIR="$ROOT/samples"
TEX_DIR="$SAMPLES_DIR/textures"
OUT_DIR="${OUT_DIR:-$ROOT/results}"
BIN_DIR="${BIN_DIR:-$SAMPLES_DIR/bin}"

# Defaults target a few common ASTC bitrates.
# Override with:
#   BLOCK=6x6
#   BLOCKS='4x4 6x6 8x8'
BLOCK="${BLOCK:-}"
BLOCKS="${BLOCKS:-}"
DEFAULT_BLOCKS=("4x4" "5x5" "6x6" "8x8")

# Override with:
#   PROFILE=ldr
#   PROFILES='ldr srgb'
PROFILE="${PROFILE:-ldr}"
PROFILES="${PROFILES:-}"

GO_BIN="${GO_BIN:-go}"
GOMAXPROCS="${GOMAXPROCS:-1}"

if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  echo "samples/run_compare.sh: missing Go toolchain ($GO_BIN)" >&2
  exit 2
fi

mkdir -p "$OUT_DIR" "$BIN_DIR"

echo "Building cmd/astcencgo (pure Go) ..."
CGO_ENABLED=0 "$GO_BIN" build -o "$BIN_DIR/astcencgo_pure" ./cmd/astcencgo

echo "Building cmd/astcencgo (native CGO) ..."
CGO_ENABLED=1 "$GO_BIN" build -tags astcenc_native -o "$BIN_DIR/astcencgo_native" ./cmd/astcencgo

QUALITIES=(medium thorough)
IMPLS=(go native)

BLOCK_LIST=()
if [[ -n "$BLOCKS" ]]; then
  read -r -a BLOCK_LIST <<<"$BLOCKS"
elif [[ -n "$BLOCK" ]]; then
  BLOCK_LIST=("$BLOCK")
else
  BLOCK_LIST=("${DEFAULT_BLOCKS[@]}")
fi

PROFILE_LIST=()
if [[ -n "$PROFILES" ]]; then
  read -r -a PROFILE_LIST <<<"$PROFILES"
else
  PROFILE_LIST=("$PROFILE")
fi

shafile() {
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    return 1
  fi
}

MANIFEST="$OUT_DIR/manifest.tsv"
echo -e "sha256\tfile" >"$MANIFEST"

echo "Running encode/decode ..."
echo "  blocks:   ${BLOCK_LIST[*]}"
echo "  profiles: ${PROFILE_LIST[*]}"

shopt -s nullglob
INPUTS=("$TEX_DIR"/*.png)
if [[ "${#INPUTS[@]}" -eq 0 ]]; then
  echo "samples/run_compare.sh: no input PNGs found in $TEX_DIR (run ./samples/fetch_polyhaven.sh first)" >&2
  exit 2
fi

for inpng in "${INPUTS[@]}"; do
  base="$(basename "$inpng")"
  name="${base%.png}"

  for prof in "${PROFILE_LIST[@]}"; do
    for block in "${BLOCK_LIST[@]}"; do
      for q in "${QUALITIES[@]}"; do
        for enc in "${IMPLS[@]}"; do
          astc_out="$OUT_DIR/${name}__${block}__${prof}__${q}__enc-${enc}.astc"

          if [[ "$enc" == "go" ]]; then
            GOMAXPROCS="$GOMAXPROCS" "$BIN_DIR/astcencgo_pure" -encode -impl go -in "$inpng" -out "$astc_out" -block "$block" -profile "$prof" -quality "$q"
          else
            GOMAXPROCS="$GOMAXPROCS" "$BIN_DIR/astcencgo_native" -encode -impl native -in "$inpng" -out "$astc_out" -block "$block" -profile "$prof" -quality "$q"
          fi
          echo -e "$(shafile "$astc_out")\t$(basename "$astc_out")" >>"$MANIFEST"

          for dec in "${IMPLS[@]}"; do
            png_out="$OUT_DIR/${name}__${block}__${prof}__${q}__enc-${enc}__dec-${dec}.png"
            if [[ "$dec" == "go" ]]; then
              GOMAXPROCS="$GOMAXPROCS" "$BIN_DIR/astcencgo_pure" -decode -impl go -in "$astc_out" -out "$png_out" -profile "$prof"
            else
              GOMAXPROCS="$GOMAXPROCS" "$BIN_DIR/astcencgo_native" -decode -impl native -in "$astc_out" -out "$png_out" -profile "$prof"
            fi
            echo -e "$(shafile "$png_out")\t$(basename "$png_out")" >>"$MANIFEST"
          done
        done
      done
    done
  done
done

echo "Done."
echo "Outputs:   $OUT_DIR"
echo "Manifest:  $MANIFEST"
