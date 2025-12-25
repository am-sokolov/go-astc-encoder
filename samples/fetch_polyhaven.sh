#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEX_DIR="$ROOT/textures"

if ! command -v curl >/dev/null 2>&1; then
  echo "samples/fetch_polyhaven.sh: missing 'curl'" >&2
  exit 2
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "samples/fetch_polyhaven.sh: missing 'jq' (used to parse Poly Haven API JSON)" >&2
  exit 2
fi

mkdir -p "$TEX_DIR"

# Poly Haven CC0 textures (Diffuse 1k PNG).
ASSETS=(
  aerial_asphalt_01
  brick_wall_001
  cobblestone_01
  denim_fabric
  old_wood_floor
)

SOURCES_TSV="$TEX_DIR/SOURCES.tsv"
echo -e "asset_id\tmap\tresolution\tformat\tfile\turl\tlicense" >"$SOURCES_TSV"

for asset in "${ASSETS[@]}"; do
  echo "Downloading $asset (Diffuse 1k PNG) ..."
  url="$(curl -fsSL "https://api.polyhaven.com/files/$asset" | jq -r '.Diffuse["1k"].png.url')"
  if [[ -z "$url" || "$url" == "null" ]]; then
    echo "samples/fetch_polyhaven.sh: failed to get download URL for $asset" >&2
    exit 1
  fi

  out="$TEX_DIR/${asset}_diffuse_1k.png"
  if [[ -s "$out" ]]; then
    echo "  already have $(basename "$out"); skipping download"
  else
    tmp="$out.tmp"
    curl -fL --retry 3 --retry-delay 1 -o "$tmp" "$url"
    mv "$tmp" "$out"
  fi

  echo -e "${asset}\tDiffuse\t1k\tpng\t$(basename "$out")\t${url}\tCC0-1.0" >>"$SOURCES_TSV"
done

echo "Done. Textures are in: $TEX_DIR"
