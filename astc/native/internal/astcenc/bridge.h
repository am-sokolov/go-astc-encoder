#ifndef ASTC_NATIVE_BRIDGE_H_INCLUDED
#define ASTC_NATIVE_BRIDGE_H_INCLUDED

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// NOTE: This header is included by cgo and must be C-compatible. Keep it free
// of any C++-only declarations.

const char* astc_native_error_string(int err);

int astc_native_context_create(
	int profile,
	unsigned int block_x,
	unsigned int block_y,
	unsigned int block_z,
	float quality,
	unsigned int flags,
	unsigned int thread_count,
	void** out_ctx);

void astc_native_context_destroy(void* ctx);

// ----------------------------------------------------------------------------
// Raw astcenc-ish API surface (for 1:1 bindings)
// ----------------------------------------------------------------------------

// Swizzle selector values; these are required to match upstream astcenc_swz
// values (ASTCENC_SWZ_*).
typedef enum astc_native_swz
{
	ASTC_NATIVE_SWZ_R = 0,
	ASTC_NATIVE_SWZ_G = 1,
	ASTC_NATIVE_SWZ_B = 2,
	ASTC_NATIVE_SWZ_A = 3,
	ASTC_NATIVE_SWZ_0 = 4,
	ASTC_NATIVE_SWZ_1 = 5,
	ASTC_NATIVE_SWZ_Z = 6,
} astc_native_swz;

typedef struct astc_native_swizzle
{
	astc_native_swz r;
	astc_native_swz g;
	astc_native_swz b;
	astc_native_swz a;
} astc_native_swizzle;

// A POD representation of upstream astcenc_config, excluding any hidden fields.
typedef struct astc_native_config_data
{
	int profile;
	unsigned int flags;

	unsigned int block_x;
	unsigned int block_y;
	unsigned int block_z;

	float cw_r_weight;
	float cw_g_weight;
	float cw_b_weight;
	float cw_a_weight;

	unsigned int a_scale_radius;
	float rgbm_m_scale;

	unsigned int tune_partition_count_limit;
	unsigned int tune_2partition_index_limit;
	unsigned int tune_3partition_index_limit;
	unsigned int tune_4partition_index_limit;
	unsigned int tune_block_mode_limit;
	unsigned int tune_refinement_limit;
	unsigned int tune_candidate_limit;
	unsigned int tune_2partitioning_candidate_limit;
	unsigned int tune_3partitioning_candidate_limit;
	unsigned int tune_4partitioning_candidate_limit;
	float tune_db_limit;
	float tune_mse_overshoot;
	float tune_2partition_early_out_limit_factor;
	float tune_3partition_early_out_limit_factor;
	float tune_2plane_early_out_limit_correlation;
	float tune_search_mode0_enable;
} astc_native_config_data;

// Populate a config using upstream astcenc_config_init defaults.
int astc_native_config_init_data(
	int profile,
	unsigned int block_x,
	unsigned int block_y,
	unsigned int block_z,
	float quality,
	unsigned int flags,
	astc_native_config_data* out_cfg);

// Allocate a codec context from a fully specified config.
int astc_native_context_alloc_from_data(
	const astc_native_config_data* cfg,
	unsigned int thread_count,
	int enable_progress_callback,
	void** out_ctx);

// Compress an image using a transient astcenc_image built from the provided
// tightly-packed RGBA buffer. The data pointer must remain valid for the
// duration of the call (including when called concurrently from multiple
// threads).
int astc_native_compress_image_ex(
	void* ctx,
	int data_type, // 0=u8, 1=f16, 2=f32 (matches ASTCENC_TYPE_*)
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba,
	const astc_native_swizzle* swizzle,
	void* out_data,
	size_t out_len,
	unsigned int thread_index,
	uintptr_t progress_handle);

int astc_native_compress_cancel(void* ctx);

// Decompress an image into a tightly-packed RGBA output buffer.
int astc_native_decompress_image_ex(
	void* ctx,
	const void* data,
	size_t data_len,
	int out_type, // 0=u8, 1=f16, 2=f32 (matches ASTCENC_TYPE_*)
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* out_rgba,
	size_t out_len,
	const astc_native_swizzle* swizzle,
	unsigned int thread_index);

// A POD representation of upstream astcenc_block_info.
typedef struct astc_native_block_info
{
	int profile;
	unsigned int block_x;
	unsigned int block_y;
	unsigned int block_z;
	unsigned int texel_count;

	uint8_t is_error_block;
	uint8_t is_constant_block;
	uint8_t is_hdr_block;
	uint8_t is_dual_plane_block;

	unsigned int partition_count;
	unsigned int partition_index;
	unsigned int dual_plane_component;
	unsigned int color_endpoint_modes[4];
	unsigned int color_level_count;
	unsigned int weight_level_count;
	unsigned int weight_x;
	unsigned int weight_y;
	unsigned int weight_z;
	float color_endpoints[4][2][4];
	float weight_values_plane1[216];
	float weight_values_plane2[216];
	uint8_t partition_assignment[216];
} astc_native_block_info;

int astc_native_get_block_info(void* ctx, const uint8_t data[16], astc_native_block_info* info);

// ----------------------------------------------------------------------------
// Existing high-level helper surface (kept for backward compatibility)
// ----------------------------------------------------------------------------

int astc_native_image_create_u8(void** out_img);

void astc_native_image_destroy(void* img);

int astc_native_image_init_u8(
	void* img,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba);

int astc_native_image_create_f16(void** out_img);

int astc_native_image_init_f16(
	void* img,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba);

int astc_native_image_create_f32(void** out_img);

int astc_native_image_init_f32(
	void* img,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba);

int astc_native_compress_image(
	void* ctx,
	void* img,
	void* out_data,
	size_t out_len,
	unsigned int thread_index);

int astc_native_compress_reset(void* ctx);

int astc_native_decompress_image_rgba8(
	void* ctx,
	const void* data,
	size_t data_len,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* out_rgba,
	size_t out_len,
	unsigned int thread_index);

int astc_native_decompress_image_rgba32f(
	void* ctx,
	const void* data,
	size_t data_len,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* out_rgba,
	size_t out_len,
	unsigned int thread_index);

int astc_native_decompress_reset(void* ctx);

#ifdef __cplusplus
} // extern "C"
#endif

#endif // ASTC_NATIVE_BRIDGE_H_INCLUDED
