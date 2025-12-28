#include "bridge.h"

#include <stdint.h>
#include <string.h>

#if defined(_MSC_VER)
  #include <malloc.h>
  #define ASTC_NATIVE_ALLOCA _alloca
#else
  #include <alloca.h>
  #define ASTC_NATIVE_ALLOCA alloca
#endif

#include "astcenc.h"

// Go callback bridge for astcenc_config::progress_callback.
//
// The upstream callback does not provide user data, so we route a uintptr_t
// "handle" using thread-local storage. This is safe because the library
// contract is that progress callbacks are invoked from user-managed worker
// threads (i.e. from within astcenc_compress_image()).
extern "C" void astc_native_go_progress(uintptr_t handle, float progress);

static thread_local uintptr_t astc_native_tls_progress_handle = 0;

static void astc_native_progress_callback(float progress)
{
	uintptr_t handle = astc_native_tls_progress_handle;
	if (handle)
	{
		astc_native_go_progress(handle, progress);
	}
}

extern "C" const char* astc_native_error_string(int err)
{
	return astcenc_get_error_string(static_cast<astcenc_error>(err));
}

extern "C" int astc_native_context_create(
	int profile,
	unsigned int block_x,
	unsigned int block_y,
	unsigned int block_z,
	float quality,
	unsigned int flags,
	unsigned int thread_count,
	void** out_ctx
) {
	if (!out_ctx)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_config config;
	astcenc_error err = astcenc_config_init(
		static_cast<astcenc_profile>(profile),
		block_x, block_y, block_z,
		quality,
		flags,
		&config);
	if (err != ASTCENC_SUCCESS)
	{
		return err;
	}

	astcenc_context* ctx = nullptr;
	err = astcenc_context_alloc(&config, thread_count, &ctx);
	if (err != ASTCENC_SUCCESS)
	{
		return err;
	}

	*out_ctx = ctx;
	return ASTCENC_SUCCESS;
}

extern "C" int astc_native_config_init_data(
	int profile,
	unsigned int block_x,
	unsigned int block_y,
	unsigned int block_z,
	float quality,
	unsigned int flags,
	astc_native_config_data* out_cfg
) {
	if (!out_cfg)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_config cfg;
	astcenc_error err = astcenc_config_init(
		static_cast<astcenc_profile>(profile),
		block_x, block_y, block_z,
		quality,
		flags,
		&cfg);
	if (err != ASTCENC_SUCCESS)
	{
		return err;
	}

	out_cfg->profile = static_cast<int>(cfg.profile);
	out_cfg->flags = cfg.flags;
	out_cfg->block_x = cfg.block_x;
	out_cfg->block_y = cfg.block_y;
	out_cfg->block_z = cfg.block_z;

	out_cfg->cw_r_weight = cfg.cw_r_weight;
	out_cfg->cw_g_weight = cfg.cw_g_weight;
	out_cfg->cw_b_weight = cfg.cw_b_weight;
	out_cfg->cw_a_weight = cfg.cw_a_weight;
	out_cfg->a_scale_radius = cfg.a_scale_radius;
	out_cfg->rgbm_m_scale = cfg.rgbm_m_scale;

	out_cfg->tune_partition_count_limit = cfg.tune_partition_count_limit;
	out_cfg->tune_2partition_index_limit = cfg.tune_2partition_index_limit;
	out_cfg->tune_3partition_index_limit = cfg.tune_3partition_index_limit;
	out_cfg->tune_4partition_index_limit = cfg.tune_4partition_index_limit;
	out_cfg->tune_block_mode_limit = cfg.tune_block_mode_limit;
	out_cfg->tune_refinement_limit = cfg.tune_refinement_limit;
	out_cfg->tune_candidate_limit = cfg.tune_candidate_limit;
	out_cfg->tune_2partitioning_candidate_limit = cfg.tune_2partitioning_candidate_limit;
	out_cfg->tune_3partitioning_candidate_limit = cfg.tune_3partitioning_candidate_limit;
	out_cfg->tune_4partitioning_candidate_limit = cfg.tune_4partitioning_candidate_limit;
	out_cfg->tune_db_limit = cfg.tune_db_limit;
	out_cfg->tune_mse_overshoot = cfg.tune_mse_overshoot;
	out_cfg->tune_2partition_early_out_limit_factor = cfg.tune_2partition_early_out_limit_factor;
	out_cfg->tune_3partition_early_out_limit_factor = cfg.tune_3partition_early_out_limit_factor;
	out_cfg->tune_2plane_early_out_limit_correlation = cfg.tune_2plane_early_out_limit_correlation;
	out_cfg->tune_search_mode0_enable = cfg.tune_search_mode0_enable;

	return ASTCENC_SUCCESS;
}

extern "C" int astc_native_context_alloc_from_data(
	const astc_native_config_data* in_cfg,
	unsigned int thread_count,
	int enable_progress_callback,
	void** out_ctx
) {
	if (!in_cfg || !out_ctx)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_config cfg;
	cfg.profile = static_cast<astcenc_profile>(in_cfg->profile);
	cfg.flags = in_cfg->flags;
	cfg.block_x = in_cfg->block_x;
	cfg.block_y = in_cfg->block_y;
	cfg.block_z = in_cfg->block_z;

	cfg.cw_r_weight = in_cfg->cw_r_weight;
	cfg.cw_g_weight = in_cfg->cw_g_weight;
	cfg.cw_b_weight = in_cfg->cw_b_weight;
	cfg.cw_a_weight = in_cfg->cw_a_weight;
	cfg.a_scale_radius = in_cfg->a_scale_radius;
	cfg.rgbm_m_scale = in_cfg->rgbm_m_scale;

	cfg.tune_partition_count_limit = in_cfg->tune_partition_count_limit;
	cfg.tune_2partition_index_limit = in_cfg->tune_2partition_index_limit;
	cfg.tune_3partition_index_limit = in_cfg->tune_3partition_index_limit;
	cfg.tune_4partition_index_limit = in_cfg->tune_4partition_index_limit;
	cfg.tune_block_mode_limit = in_cfg->tune_block_mode_limit;
	cfg.tune_refinement_limit = in_cfg->tune_refinement_limit;
	cfg.tune_candidate_limit = in_cfg->tune_candidate_limit;
	cfg.tune_2partitioning_candidate_limit = in_cfg->tune_2partitioning_candidate_limit;
	cfg.tune_3partitioning_candidate_limit = in_cfg->tune_3partitioning_candidate_limit;
	cfg.tune_4partitioning_candidate_limit = in_cfg->tune_4partitioning_candidate_limit;
	cfg.tune_db_limit = in_cfg->tune_db_limit;
	cfg.tune_mse_overshoot = in_cfg->tune_mse_overshoot;
	cfg.tune_2partition_early_out_limit_factor = in_cfg->tune_2partition_early_out_limit_factor;
	cfg.tune_3partition_early_out_limit_factor = in_cfg->tune_3partition_early_out_limit_factor;
	cfg.tune_2plane_early_out_limit_correlation = in_cfg->tune_2plane_early_out_limit_correlation;
	cfg.tune_search_mode0_enable = in_cfg->tune_search_mode0_enable;

	cfg.progress_callback = enable_progress_callback ? astc_native_progress_callback : nullptr;

#if defined(ASTCENC_DIAGNOSTICS)
	cfg.trace_file_path = nullptr;
#endif

	astcenc_context* ctx = nullptr;
	astcenc_error err = astcenc_context_alloc(&cfg, thread_count, &ctx);
	if (err != ASTCENC_SUCCESS)
	{
		return err;
	}

	*out_ctx = ctx;
	return ASTCENC_SUCCESS;
}

extern "C" void astc_native_context_destroy(void* ctx)
{
	if (!ctx)
	{
		return;
	}
	astcenc_context_free(static_cast<astcenc_context*>(ctx));
}

extern "C" int astc_native_image_create_u8(void** out_img)
{
	if (!out_img)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_image* img = new astcenc_image();
	img->dim_x = 0;
	img->dim_y = 0;
	img->dim_z = 0;
	img->data_type = ASTCENC_TYPE_U8;
	img->data = nullptr;

	*out_img = img;
	return ASTCENC_SUCCESS;
}

extern "C" int astc_native_image_create_f16(void** out_img)
{
	if (!out_img)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_image* img = new astcenc_image();
	img->dim_x = 0;
	img->dim_y = 0;
	img->dim_z = 0;
	img->data_type = ASTCENC_TYPE_F16;
	img->data = nullptr;

	*out_img = img;
	return ASTCENC_SUCCESS;
}

extern "C" int astc_native_image_create_f32(void** out_img)
{
	if (!out_img)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_image* img = new astcenc_image();
	img->dim_x = 0;
	img->dim_y = 0;
	img->dim_z = 0;
	img->data_type = ASTCENC_TYPE_F32;
	img->data = nullptr;

	*out_img = img;
	return ASTCENC_SUCCESS;
}

extern "C" void astc_native_image_destroy(void* imgp)
{
	astcenc_image* img = static_cast<astcenc_image*>(imgp);
	if (!img)
	{
		return;
	}

	delete[] img->data;
	img->data = nullptr;
	delete img;
}

extern "C" int astc_native_image_init_u8(
	void* imgp,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba
) {
	astcenc_image* img = static_cast<astcenc_image*>(imgp);
	if (!img || !rgba || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	if (!img->data || img->dim_z != dim_z)
	{
		delete[] img->data;
		img->data = new void*[dim_z];
	}

	img->dim_x = dim_x;
	img->dim_y = dim_y;
	img->dim_z = dim_z;
	img->data_type = ASTCENC_TYPE_U8;

	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;
	uint8_t* base = static_cast<uint8_t*>(rgba);
	for (unsigned int z = 0; z < dim_z; z++)
	{
		img->data[z] = base + static_cast<size_t>(z) * slice_stride;
	}

	return ASTCENC_SUCCESS;
}

extern "C" int astc_native_image_init_f16(
	void* imgp,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba
) {
	astcenc_image* img = static_cast<astcenc_image*>(imgp);
	if (!img || !rgba || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	if (!img->data || img->dim_z != dim_z)
	{
		delete[] img->data;
		img->data = new void*[dim_z];
	}

	img->dim_x = dim_x;
	img->dim_y = dim_y;
	img->dim_z = dim_z;
	img->data_type = ASTCENC_TYPE_F16;

	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;
	uint16_t* base = static_cast<uint16_t*>(rgba);
	for (unsigned int z = 0; z < dim_z; z++)
	{
		img->data[z] = base + static_cast<size_t>(z) * slice_stride;
	}

	return ASTCENC_SUCCESS;
}

extern "C" int astc_native_image_init_f32(
	void* imgp,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba
) {
	astcenc_image* img = static_cast<astcenc_image*>(imgp);
	if (!img || !rgba || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	if (!img->data || img->dim_z != dim_z)
	{
		delete[] img->data;
		img->data = new void*[dim_z];
	}

	img->dim_x = dim_x;
	img->dim_y = dim_y;
	img->dim_z = dim_z;
	img->data_type = ASTCENC_TYPE_F32;

	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;
	float* base = static_cast<float*>(rgba);
	for (unsigned int z = 0; z < dim_z; z++)
	{
		img->data[z] = base + static_cast<size_t>(z) * slice_stride;
	}

	return ASTCENC_SUCCESS;
}

static astcenc_swizzle makeSwizzle(const astc_native_swizzle* in_swz)
{
	if (!in_swz)
	{
		return astcenc_swizzle { ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };
	}

	return astcenc_swizzle {
		static_cast<astcenc_swz>(in_swz->r),
		static_cast<astcenc_swz>(in_swz->g),
		static_cast<astcenc_swz>(in_swz->b),
		static_cast<astcenc_swz>(in_swz->a),
	};
}

extern "C" int astc_native_compress_image_ex(
	void* ctxp,
	int data_type,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* rgba,
	const astc_native_swizzle* in_swz,
	void* out_data,
	size_t out_len,
	unsigned int thread_index,
	uintptr_t progress_handle
) {
	if (!ctxp || !rgba || !out_data || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_image img;
	img.dim_x = dim_x;
	img.dim_y = dim_y;
	img.dim_z = dim_z;
	img.data_type = static_cast<astcenc_type>(data_type);

	void** slices = static_cast<void**>(ASTC_NATIVE_ALLOCA(sizeof(void*) * dim_z));
	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;

	if (data_type == ASTCENC_TYPE_U8)
	{
		uint8_t* base = static_cast<uint8_t*>(rgba);
		for (unsigned int z = 0; z < dim_z; z++)
		{
			slices[z] = base + static_cast<size_t>(z) * slice_stride;
		}
	}
	else if (data_type == ASTCENC_TYPE_F16)
	{
		uint16_t* base = static_cast<uint16_t*>(rgba);
		for (unsigned int z = 0; z < dim_z; z++)
		{
			slices[z] = base + static_cast<size_t>(z) * slice_stride;
		}
	}
	else if (data_type == ASTCENC_TYPE_F32)
	{
		float* base = static_cast<float*>(rgba);
		for (unsigned int z = 0; z < dim_z; z++)
		{
			slices[z] = base + static_cast<size_t>(z) * slice_stride;
		}
	}
	else
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	img.data = slices;

	astcenc_swizzle swz = makeSwizzle(in_swz);

	uintptr_t old_handle = astc_native_tls_progress_handle;
	astc_native_tls_progress_handle = progress_handle;
	astcenc_error err = astcenc_compress_image(
		static_cast<astcenc_context*>(ctxp),
		&img,
		&swz,
		static_cast<uint8_t*>(out_data),
		out_len,
		thread_index);
	astc_native_tls_progress_handle = old_handle;
	return err;
}

extern "C" int astc_native_compress_image(
	void* ctxp,
	void* imgp,
	void* out_data,
	size_t out_len,
	unsigned int thread_index
) {
	if (!ctxp || !imgp || !out_data)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_swizzle swz { ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };

	return astcenc_compress_image(
		static_cast<astcenc_context*>(ctxp),
		static_cast<astcenc_image*>(imgp),
		&swz,
		static_cast<uint8_t*>(out_data),
		out_len,
		thread_index);
}

extern "C" int astc_native_compress_reset(void* ctxp)
{
	if (!ctxp)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}
	return astcenc_compress_reset(static_cast<astcenc_context*>(ctxp));
}

extern "C" int astc_native_compress_cancel(void* ctxp)
{
	if (!ctxp)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}
	return astcenc_compress_cancel(static_cast<astcenc_context*>(ctxp));
}

extern "C" int astc_native_decompress_image_ex(
	void* ctxp,
	const void* data,
	size_t data_len,
	int out_type,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* out_rgba,
	size_t out_len,
	const astc_native_swizzle* in_swz,
	unsigned int thread_index
) {
	if (!ctxp || !data || !out_rgba || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	size_t texel_count = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * static_cast<size_t>(dim_z);
	size_t need = 0;
	if (out_type == ASTCENC_TYPE_U8)
	{
		need = texel_count * 4;
	}
	else if (out_type == ASTCENC_TYPE_F16)
	{
		need = texel_count * 4 * sizeof(uint16_t);
	}
	else if (out_type == ASTCENC_TYPE_F32)
	{
		need = texel_count * 4 * sizeof(float);
	}
	else
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	if (out_len < need)
	{
		return ASTCENC_ERR_OUT_OF_MEM;
	}

	astcenc_image img;
	img.dim_x = dim_x;
	img.dim_y = dim_y;
	img.dim_z = dim_z;
	img.data_type = static_cast<astcenc_type>(out_type);

	void** slices = static_cast<void**>(ASTC_NATIVE_ALLOCA(sizeof(void*) * dim_z));
	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;

	if (out_type == ASTCENC_TYPE_U8)
	{
		uint8_t* base = static_cast<uint8_t*>(out_rgba);
		for (unsigned int z = 0; z < dim_z; z++)
		{
			slices[z] = base + static_cast<size_t>(z) * slice_stride;
		}
	}
	else if (out_type == ASTCENC_TYPE_F16)
	{
		uint16_t* base = static_cast<uint16_t*>(out_rgba);
		for (unsigned int z = 0; z < dim_z; z++)
		{
			slices[z] = base + static_cast<size_t>(z) * slice_stride;
		}
	}
	else // ASTCENC_TYPE_F32
	{
		float* base = static_cast<float*>(out_rgba);
		for (unsigned int z = 0; z < dim_z; z++)
		{
			slices[z] = base + static_cast<size_t>(z) * slice_stride;
		}
	}

	img.data = slices;

	astcenc_swizzle swz = makeSwizzle(in_swz);

	return astcenc_decompress_image(
		static_cast<astcenc_context*>(ctxp),
		static_cast<const uint8_t*>(data),
		data_len,
		&img,
		&swz,
		thread_index);
}

extern "C" int astc_native_decompress_image_rgba8(
	void* ctxp,
	const void* data,
	size_t data_len,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* out_rgba,
	size_t out_len,
	unsigned int thread_index
) {
	if (!ctxp || !data || !out_rgba || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	size_t need = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * static_cast<size_t>(dim_z) * 4;
	if (out_len < need)
	{
		return ASTCENC_ERR_OUT_OF_MEM;
	}

	astcenc_image img;
	img.dim_x = dim_x;
	img.dim_y = dim_y;
	img.dim_z = dim_z;
	img.data_type = ASTCENC_TYPE_U8;

	void** slices = static_cast<void**>(ASTC_NATIVE_ALLOCA(sizeof(void*) * dim_z));

	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;
	uint8_t* base = static_cast<uint8_t*>(out_rgba);
	for (unsigned int z = 0; z < dim_z; z++)
	{
		slices[z] = base + static_cast<size_t>(z) * slice_stride;
	}
	img.data = slices;

	astcenc_swizzle swz { ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };

	return astcenc_decompress_image(
		static_cast<astcenc_context*>(ctxp),
		static_cast<const uint8_t*>(data),
		data_len,
		&img,
		&swz,
		thread_index);
}

extern "C" int astc_native_decompress_image_rgba32f(
	void* ctxp,
	const void* data,
	size_t data_len,
	unsigned int dim_x,
	unsigned int dim_y,
	unsigned int dim_z,
	void* out_rgba,
	size_t out_len,
	unsigned int thread_index
) {
	if (!ctxp || !data || !out_rgba || dim_x == 0 || dim_y == 0 || dim_z == 0)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	size_t need = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * static_cast<size_t>(dim_z) * 4 * sizeof(float);
	if (out_len < need)
	{
		return ASTCENC_ERR_OUT_OF_MEM;
	}

	astcenc_image img;
	img.dim_x = dim_x;
	img.dim_y = dim_y;
	img.dim_z = dim_z;
	img.data_type = ASTCENC_TYPE_F32;

	void** slices = static_cast<void**>(ASTC_NATIVE_ALLOCA(sizeof(void*) * dim_z));

	size_t slice_stride = static_cast<size_t>(dim_x) * static_cast<size_t>(dim_y) * 4;
	float* base = static_cast<float*>(out_rgba);
	for (unsigned int z = 0; z < dim_z; z++)
	{
		slices[z] = base + static_cast<size_t>(z) * slice_stride;
	}
	img.data = slices;

	astcenc_swizzle swz { ASTCENC_SWZ_R, ASTCENC_SWZ_G, ASTCENC_SWZ_B, ASTCENC_SWZ_A };

	return astcenc_decompress_image(
		static_cast<astcenc_context*>(ctxp),
		static_cast<const uint8_t*>(data),
		data_len,
		&img,
		&swz,
		thread_index);
}

extern "C" int astc_native_decompress_reset(void* ctxp)
{
	if (!ctxp)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}
	return astcenc_decompress_reset(static_cast<astcenc_context*>(ctxp));
}

extern "C" int astc_native_get_block_info(void* ctxp, const uint8_t data[16], astc_native_block_info* out_info)
{
	if (!ctxp || !data || !out_info)
	{
		return ASTCENC_ERR_BAD_PARAM;
	}

	astcenc_block_info info;
	astcenc_error err = astcenc_get_block_info(
		static_cast<astcenc_context*>(ctxp),
		data,
		&info);
	if (err != ASTCENC_SUCCESS)
	{
		return err;
	}

	out_info->profile = static_cast<int>(info.profile);
	out_info->block_x = info.block_x;
	out_info->block_y = info.block_y;
	out_info->block_z = info.block_z;
	out_info->texel_count = info.texel_count;

	out_info->is_error_block = info.is_error_block ? 1 : 0;
	out_info->is_constant_block = info.is_constant_block ? 1 : 0;
	out_info->is_hdr_block = info.is_hdr_block ? 1 : 0;
	out_info->is_dual_plane_block = info.is_dual_plane_block ? 1 : 0;

	out_info->partition_count = info.partition_count;
	out_info->partition_index = info.partition_index;
	out_info->dual_plane_component = info.dual_plane_component;

	memcpy(out_info->color_endpoint_modes, info.color_endpoint_modes, sizeof(out_info->color_endpoint_modes));
	out_info->color_level_count = info.color_level_count;
	out_info->weight_level_count = info.weight_level_count;
	out_info->weight_x = info.weight_x;
	out_info->weight_y = info.weight_y;
	out_info->weight_z = info.weight_z;

	memcpy(out_info->color_endpoints, info.color_endpoints, sizeof(out_info->color_endpoints));
	memcpy(out_info->weight_values_plane1, info.weight_values_plane1, sizeof(out_info->weight_values_plane1));
	memcpy(out_info->weight_values_plane2, info.weight_values_plane2, sizeof(out_info->weight_values_plane2));
	memcpy(out_info->partition_assignment, info.partition_assignment, sizeof(out_info->partition_assignment));

	return ASTCENC_SUCCESS;
}
