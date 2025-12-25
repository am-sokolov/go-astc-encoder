#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! command -v rsync >/dev/null 2>&1; then
  echo "scripts/make_package.sh: missing 'rsync'" >&2
  exit 2
fi
if ! command -v tar >/dev/null 2>&1; then
  echo "scripts/make_package.sh: missing 'tar'" >&2
  exit 2
fi

VERSION="${VERSION:-}"
if [[ -z "$VERSION" ]]; then
  GIT_SHA="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null || true)"
  DATE="$(date +%Y%m%d)"
  if [[ -n "$GIT_SHA" ]]; then
    VERSION="${DATE}-${GIT_SHA}"
  else
    VERSION="${DATE}"
  fi
fi

STAGE_NAME="${STAGE_NAME:-go-astc-encoder}"
ARCHIVE_NAME="${ARCHIVE_NAME:-go-astc-encoder-${VERSION}}"
OUT_DIR="${OUT_DIR:-$ROOT/dist}"
STAGE_DIR="${STAGE_DIR:-$OUT_DIR/stage}"
PKG_DIR="$STAGE_DIR/$STAGE_NAME"

mkdir -p "$OUT_DIR" "$STAGE_DIR"
rm -rf "$PKG_DIR"
mkdir -p "$PKG_DIR"

echo "Staging into: $PKG_DIR"

# Copy everything needed for a source redistribution, excluding local build artifacts.
rsync -a --delete \
  --exclude '.git/' \
  --exclude '.github/' \
  --exclude '.cache/' \
  --exclude 'dist/' \
  --exclude 'build*/' \
  --exclude 'Binaries/' \
  --exclude 'bin/' \
  --exclude 'lib/' \
  --exclude 'samples/bin/' \
  --exclude '*.o' \
  --exclude '*.a' \
  --exclude 'astc.test' \
  "$ROOT/" "$PKG_DIR/"

TEX_GLOB="$PKG_DIR/samples/textures/"'*.png'
if ! ls $TEX_GLOB >/dev/null 2>&1; then
  echo "warning: no sample textures found in samples/textures/ (run ./samples/fetch_polyhaven.sh before packaging)" >&2
fi

if [[ ! -d "$PKG_DIR/results" ]]; then
  echo "warning: no results/ directory found (run ./samples/run_compare.sh before packaging to include outputs)" >&2
fi

ARCHIVE_TGZ="$OUT_DIR/$ARCHIVE_NAME.tar.gz"
ARCHIVE_ZIP="$OUT_DIR/$ARCHIVE_NAME.zip"

echo "Creating: $ARCHIVE_TGZ"
tar -C "$STAGE_DIR" -czf "$ARCHIVE_TGZ" "$STAGE_NAME"

if command -v zip >/dev/null 2>&1; then
  echo "Creating: $ARCHIVE_ZIP"
  (cd "$STAGE_DIR" && zip -qr "$ARCHIVE_ZIP" "$STAGE_NAME")
fi

if command -v shasum >/dev/null 2>&1; then
  (cd "$OUT_DIR" && shasum -a 256 "$(basename "$ARCHIVE_TGZ")" >"$ARCHIVE_NAME.tar.gz.sha256")
  if [[ -f "$ARCHIVE_ZIP" ]]; then
    (cd "$OUT_DIR" && shasum -a 256 "$(basename "$ARCHIVE_ZIP")" >"$ARCHIVE_NAME.zip.sha256")
  fi
fi

echo "Done."
echo "Artifacts:"
echo "  $ARCHIVE_TGZ"
if [[ -f "$ARCHIVE_ZIP" ]]; then
  echo "  $ARCHIVE_ZIP"
fi
