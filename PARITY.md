# Upstream `astcenc` parity checklist

Upstream reference: `astc/native/internal/astcenc/upstream/astcenc.h`

Legend:
- ✅ implemented
- ⚠️ partially implemented
- ❌ missing

## Public API parity (`astcenc.h`)

| Feature | CGO (`astc/native`) | Pure-Go (`astc`) | Tests |
|---|---:|---:|---|
| `astcenc_config_init` equivalent | ✅ (`native.ConfigInit`) | ✅ (`astc.ConfigInit`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `astcenc_context_alloc/free` equivalent | ✅ (`native.ContextAlloc`, `(*Context).Close`) | ✅ (`astc.ContextAlloc`, `(*Context).Close`) | ✅ (`astc/astcenc_api_test.go`) |
| `astcenc_compress_image/reset` equivalent | ✅ (`(*Context).CompressImage/CompressReset`) | ✅ (`(*Context).CompressImage/CompressReset`) | ✅ (`astc/astcenc_api_test.go`) |
| `astcenc_compress_cancel` equivalent | ✅ (`(*Context).CompressCancel`) | ✅ (`(*Context).CompressCancel`) | ✅ (`astc/astcenc_cancel_test.go`) |
| `astcenc_decompress_image/reset` equivalent | ✅ (`(*Context).DecompressImage/DecompressReset`) | ✅ (`(*Context).DecompressImage/DecompressReset`) | ✅ (`astc/astcenc_api_test.go`) |
| `astcenc_get_block_info` equivalent | ✅ (`(*Context).GetBlockInfo`) | ✅ (`(*Context).GetBlockInfo`) | ✅ (`astc/astcenc_api_test.go`, `astc/native/astcenc_raw_test.go`) |
| `astcenc_get_error_string` equivalent | ✅ (`native/internal/astcenc.ErrorString`) | ✅ (`astc.ErrorString`, `astc.ErrorCodeOf`) | ✅ (`astc/errors_test.go`) |

## Flags (`ASTCENC_FLG_*`)

| Flag | CGO (`astc/native`) | Pure-Go (`astc`) | Tests |
|---|---:|---:|---|
| `ASTCENC_FLG_MAP_NORMAL` | ✅ (`native.Flags`) | ✅ (L+A blocks + angular normal error metric) | ✅ (`astc/astcenc_api_native_test.go`, `astc/normal_map_parity_native_test.go`) |
| `ASTCENC_FLG_USE_DECODE_UNORM8` | ✅ (`native.Flags`) | ✅ (affects encoder evaluation rounding) | ✅ (`astc/astcenc_api_native_test.go`, `astc/decode_unorm8_parity_native_test.go`) |
| `ASTCENC_FLG_USE_ALPHA_WEIGHT` | ✅ (`native.Flags`) | ✅ (RGB weight scaling + `a_scale_radius` alpha-scale RDO parity) | ✅ (`astc/astcenc_api_native_test.go`) |
| `ASTCENC_FLG_USE_PERCEPTUAL` | ✅ (`native.Flags`) | ✅ (sets perceptual default `cw_*_weight` like upstream) | ✅ (`astc/astcenc_api_native_test.go`) |
| `ASTCENC_FLG_DECOMPRESS_ONLY` | ✅ (`native.Flags`) | ✅ (enforced in `CompressImage`) | ✅ (`astc/astcenc_api_test.go`) |
| `ASTCENC_FLG_SELF_DECOMPRESS_ONLY` | ✅ (`native.Flags`) | ✅ (accepted; currently a no-op in decoder) | ✅ (`astc/astcenc_api_native_test.go`) |
| `ASTCENC_FLG_MAP_RGBM` | ✅ (`native.Flags`) | ✅ (RGBM-aware error metric + M==0 rejection) | ✅ (`astc/astcenc_api_native_test.go`, `astc/rgbm_parity_native_test.go`) |

## Config fields (`astcenc_config`)

| Field | CGO (`astc/native`) | Pure-Go (`astc`) | Tests |
|---|---:|---:|---|
| `cw_*_weight` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `a_scale_radius` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `rgbm_m_scale` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_partition_count_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_*partition*_index_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_block_mode_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_refinement_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_candidate_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_*partitioning_candidate_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_db_limit` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_mse_overshoot` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_*partition_early_out_limit_factor` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_2plane_early_out_limit_correlation` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `tune_search_mode0_enable` | ✅ (`native.Config`) | ✅ (`astc.Config`) | ✅ (`astc/astcenc_api_native_test.go`) |
| `progress_callback` | ✅ (`native.Config.ProgressCallback`) | ✅ (upstream-like callback throttling) | ✅ (`astc/astcenc_progress_test.go`, `astc/native/astcenc_raw_test.go`) |

## Codec capabilities (data types, profiles, dimensions)

| Capability | CGO (`astc/native`) | Pure-Go (`astc`) | Tests |
|---|---:|---:|---|
| Encode RGBA8 (2D+3D) | ✅ | ✅ | ✅ |
| Encode RGBAF16 (2D+3D) | ✅ | ✅ (via `astc.Context` API) | ✅ (`astc/astcenc_f16_test.go`) |
| Encode RGBAF32 (2D+3D) | ✅ | ✅ | ✅ |
| Decode to RGBA8 (2D+3D) | ✅ (LDR only) | ✅ (LDR only) | ✅ |
| Decode to RGBAF16 | ✅ (raw API) | ✅ (via `astc.Context` API) | ✅ (`astc/astcenc_f16_test.go`) |
| Decode to RGBAF32 (2D+3D) | ✅ | ✅ | ✅ |
| LDR profiles | ✅ | ✅ | ✅ |
| HDR profiles | ✅ | ✅ | ✅ |
| Swizzle support | ✅ (raw API) | ✅ (via `astc.Context` API) | ✅ (`astc/astcenc_api_test.go`, `astc/native/astcenc_raw_test.go`) |

## Algorithmic parity (encoder search space)

| Feature | CGO (`astc/native`) | Pure-Go (`astc`) | Tests |
|---|---:|---:|---|
| Partitions (2–4) | ✅ | ✅ (LDR); ⚠️ (HDR F32: supported, simplified search) | ⚠️ |
| Dual-plane | ✅ | ✅ (LDR+HDR; plane2 component selectable) | ✅ (`astc/hdr_dual_plane_component_native_test.go`) |
| Endpoint-mode selection | ✅ | ⚠️ (LDR limited; HDR F32: HDRRGBA/HDRRGB/HDRRGBScale/HDR luminance only) | ⚠️ |
| Full HDR “true float source encode” | ✅ | ⚠️ (valid blocks, not full search) | ✅ (sanity + decode parity) |
