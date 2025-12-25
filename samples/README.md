## Samples

This folder contains scripts and metadata for downloading a small set of free (CC0) texture images
and generating encoder/decoder outputs for manual comparison.

- Textures live in `samples/textures/`.
- Generated outputs are written to `results/` (override with `OUT_DIR=...`).

### Quick start

1. Download CC0 textures:

`./samples/fetch_polyhaven.sh`

2. Generate ASTC + decoded PNG outputs for both implementations:

`./samples/run_compare.sh`

You can override defaults:

`BLOCK=6x6 PROFILE=ldr ./samples/run_compare.sh`

Or run multiple block sizes / profiles:

`BLOCKS='4x4 5x5 6x6 8x8' PROFILES='ldr srgb' ./samples/run_compare.sh`
