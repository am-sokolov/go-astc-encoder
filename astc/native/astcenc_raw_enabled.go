//go:build astcenc_native && cgo

package native

/*
#include "internal/astcenc/bridge.h"
*/
import "C"

import (
	"errors"
	"runtime"
	"runtime/cgo"
	"unsafe"

	"github.com/arm-software/astc-encoder/astc"
)

// ConfigInit populates a config using upstream astcenc_config_init defaults.
func ConfigInit(profile astc.Profile, blockX, blockY, blockZ int, quality float32, flags Flags) (Config, error) {
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 || blockX > 255 || blockY > 255 || blockZ > 255 {
		return Config{}, errors.New("astc/native: invalid block dimensions")
	}
	if blockX*blockY*blockZ > 216 {
		return Config{}, errors.New("astc/native: invalid block dimensions")
	}

	cProf, err := profileToC(profile)
	if err != nil {
		return Config{}, err
	}

	var cfg C.astc_native_config_data
	code := C.astc_native_config_init_data(
		C.int(cProf),
		C.uint(blockX),
		C.uint(blockY),
		C.uint(blockZ),
		C.float(quality),
		C.uint(flags),
		&cfg,
	)
	if err := errFromCode(int(code), "astcenc_config_init"); err != nil {
		return Config{}, err
	}

	return configFromC(cfg), nil
}

// ContextAlloc allocates a reusable codec context based on a config.
func ContextAlloc(cfg *Config, threadCount int) (*Context, error) {
	if cfg == nil {
		return nil, errors.New("astc/native: nil config")
	}
	if threadCount <= 0 {
		threadCount = runtime.GOMAXPROCS(0)
	}
	if threadCount < 1 {
		threadCount = 1
	}

	cCfg := configToC(*cfg)

	var ctxp unsafe.Pointer
	enableProgress := cfg.ProgressCallback != nil
	code := C.astc_native_context_alloc_from_data(
		&cCfg,
		C.uint(threadCount),
		boolToCInt(enableProgress),
		(*unsafe.Pointer)(unsafe.Pointer(&ctxp)),
	)
	if err := errFromCode(int(code), "astcenc_context_alloc"); err != nil {
		return nil, err
	}

	return &Context{
		ctx:         ctxp,
		cfg:         *cfg,
		threadCount: threadCount,
	}, nil
}

func (c *Context) Close() error {
	if c == nil {
		return nil
	}
	if c.ctx != nil {
		C.astc_native_context_destroy(c.ctx)
		c.ctx = nil
	}
	return nil
}

func (c *Context) CompressImage(img *Image, swizzle Swizzle, out []byte, threadIndex int) error {
	if c == nil || c.ctx == nil {
		return errors.New("astc/native: nil context")
	}
	if img == nil {
		return errors.New("astc/native: nil image")
	}

	blockX, blockY, blockZ := int(c.cfg.BlockX), int(c.cfg.BlockY), int(c.cfg.BlockZ)
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 {
		return errors.New("astc/native: invalid context block dimensions")
	}

	blocksX := (img.DimX + blockX - 1) / blockX
	blocksY := (img.DimY + blockY - 1) / blockY
	blocksZ := (img.DimZ + blockZ - 1) / blockZ
	if blocksX <= 0 || blocksY <= 0 || blocksZ <= 0 {
		return errors.New("astc/native: invalid image dimensions")
	}
	needBlocks := blocksX * blocksY * blocksZ * astc.BlockBytes
	if len(out) < needBlocks {
		return errors.New("astc/native: output buffer too small")
	}

	dataPtr, dataType, err := imageDataPtrAndType(img)
	if err != nil {
		return err
	}

	var cSwz C.astc_native_swizzle = swizzleToC(swizzle)

	var progressHandle uintptr
	if cb := c.cfg.ProgressCallback; cb != nil {
		h := cgo.NewHandle(func(p float32) { cb(p) })
		defer h.Delete()
		progressHandle = uintptr(h)
	}

	code := C.astc_native_compress_image_ex(
		c.ctx,
		C.int(dataType),
		C.uint(img.DimX),
		C.uint(img.DimY),
		C.uint(img.DimZ),
		dataPtr,
		&cSwz,
		unsafe.Pointer(&out[0]),
		C.size_t(needBlocks),
		C.uint(threadIndex),
		C.uintptr_t(progressHandle),
	)
	if err := errFromCode(int(code), "astcenc_compress_image"); err != nil {
		return err
	}
	return nil
}

func (c *Context) CompressReset() error {
	if c == nil || c.ctx == nil {
		return errors.New("astc/native: nil context")
	}
	code := C.astc_native_compress_reset(c.ctx)
	return errFromCode(int(code), "astcenc_compress_reset")
}

func (c *Context) CompressCancel() error {
	if c == nil || c.ctx == nil {
		return errors.New("astc/native: nil context")
	}
	code := C.astc_native_compress_cancel(c.ctx)
	return errFromCode(int(code), "astcenc_compress_cancel")
}

func (c *Context) DecompressImage(data []byte, imgOut *Image, swizzle Swizzle, threadIndex int) error {
	if c == nil || c.ctx == nil {
		return errors.New("astc/native: nil context")
	}
	if imgOut == nil {
		return errors.New("astc/native: nil output image")
	}
	if len(data) == 0 {
		return errors.New("astc/native: empty input data")
	}

	blockX, blockY, blockZ := int(c.cfg.BlockX), int(c.cfg.BlockY), int(c.cfg.BlockZ)
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 {
		return errors.New("astc/native: invalid context block dimensions")
	}

	blocksX := (imgOut.DimX + blockX - 1) / blockX
	blocksY := (imgOut.DimY + blockY - 1) / blockY
	blocksZ := (imgOut.DimZ + blockZ - 1) / blockZ
	if blocksX <= 0 || blocksY <= 0 || blocksZ <= 0 {
		return errors.New("astc/native: invalid image dimensions")
	}
	needBlocks := blocksX * blocksY * blocksZ * astc.BlockBytes
	if len(data) < needBlocks {
		return errors.New("astc/native: block buffer too small")
	}

	outPtr, outType, outLenBytes, err := imageOutPtrTypeAndLen(imgOut)
	if err != nil {
		return err
	}

	var cSwz C.astc_native_swizzle = swizzleToC(swizzle)
	code := C.astc_native_decompress_image_ex(
		c.ctx,
		unsafe.Pointer(&data[0]),
		C.size_t(needBlocks),
		C.int(outType),
		C.uint(imgOut.DimX),
		C.uint(imgOut.DimY),
		C.uint(imgOut.DimZ),
		outPtr,
		C.size_t(outLenBytes),
		&cSwz,
		C.uint(threadIndex),
	)
	if err := errFromCode(int(code), "astcenc_decompress_image"); err != nil {
		return err
	}
	return nil
}

func (c *Context) DecompressReset() error {
	if c == nil || c.ctx == nil {
		return errors.New("astc/native: nil context")
	}
	code := C.astc_native_decompress_reset(c.ctx)
	return errFromCode(int(code), "astcenc_decompress_reset")
}

func (c *Context) GetBlockInfo(block [astc.BlockBytes]byte) (BlockInfo, error) {
	if c == nil || c.ctx == nil {
		return BlockInfo{}, errors.New("astc/native: nil context")
	}
	var info C.astc_native_block_info
	code := C.astc_native_get_block_info(c.ctx, (*C.uint8_t)(unsafe.Pointer(&block[0])), &info)
	if err := errFromCode(int(code), "astcenc_get_block_info"); err != nil {
		return BlockInfo{}, err
	}
	return blockInfoFromC(info), nil
}

func boolToCInt(v bool) C.int {
	if v {
		return 1
	}
	return 0
}

func configFromC(cfg C.astc_native_config_data) Config {
	prof := astc.Profile(cfg.profile)
	// Map upstream astcenc_profile enum values into astc.Profile values.
	// Upstream: 0=LDR_SRGB, 1=LDR; astc: 0=LDR, 1=LDR_SRGB.
	switch int(cfg.profile) {
	case 0:
		prof = astc.ProfileLDRSRGB
	case 1:
		prof = astc.ProfileLDR
	case 2:
		prof = astc.ProfileHDRRGBLDRAlpha
	case 3:
		prof = astc.ProfileHDR
	}

	return Config{
		Profile: prof,
		Flags:   Flags(cfg.flags),

		BlockX: uint32(cfg.block_x),
		BlockY: uint32(cfg.block_y),
		BlockZ: uint32(cfg.block_z),

		CWRWeight: float32(cfg.cw_r_weight),
		CWGWeight: float32(cfg.cw_g_weight),
		CWBWeight: float32(cfg.cw_b_weight),
		CWAWeight: float32(cfg.cw_a_weight),

		AScaleRadius: uint32(cfg.a_scale_radius),
		RGBMMScale:   float32(cfg.rgbm_m_scale),

		TunePartitionCountLimit:            uint32(cfg.tune_partition_count_limit),
		Tune2PartitionIndexLimit:           uint32(cfg.tune_2partition_index_limit),
		Tune3PartitionIndexLimit:           uint32(cfg.tune_3partition_index_limit),
		Tune4PartitionIndexLimit:           uint32(cfg.tune_4partition_index_limit),
		TuneBlockModeLimit:                 uint32(cfg.tune_block_mode_limit),
		TuneRefinementLimit:                uint32(cfg.tune_refinement_limit),
		TuneCandidateLimit:                 uint32(cfg.tune_candidate_limit),
		Tune2PartitioningCandidateLimit:    uint32(cfg.tune_2partitioning_candidate_limit),
		Tune3PartitioningCandidateLimit:    uint32(cfg.tune_3partitioning_candidate_limit),
		Tune4PartitioningCandidateLimit:    uint32(cfg.tune_4partitioning_candidate_limit),
		TuneDBLimit:                        float32(cfg.tune_db_limit),
		TuneMSEOvershoot:                   float32(cfg.tune_mse_overshoot),
		Tune2PartitionEarlyOutLimitFactor:  float32(cfg.tune_2partition_early_out_limit_factor),
		Tune3PartitionEarlyOutLimitFactor:  float32(cfg.tune_3partition_early_out_limit_factor),
		Tune2PlaneEarlyOutLimitCorrelation: float32(cfg.tune_2plane_early_out_limit_correlation),
		TuneSearchMode0Enable:              float32(cfg.tune_search_mode0_enable),
	}
}

func configToC(cfg Config) C.astc_native_config_data {
	prof := int(cfg.Profile)
	switch cfg.Profile {
	case astc.ProfileLDR:
		prof = 1 // ASTCENC_PRF_LDR
	case astc.ProfileLDRSRGB:
		prof = 0 // ASTCENC_PRF_LDR_SRGB
	case astc.ProfileHDRRGBLDRAlpha:
		prof = 2 // ASTCENC_PRF_HDR_RGB_LDR_A
	case astc.ProfileHDR:
		prof = 3 // ASTCENC_PRF_HDR
	}

	return C.astc_native_config_data{
		profile: C.int(prof),
		flags:   C.uint(cfg.Flags),

		block_x: C.uint(cfg.BlockX),
		block_y: C.uint(cfg.BlockY),
		block_z: C.uint(cfg.BlockZ),

		cw_r_weight: C.float(cfg.CWRWeight),
		cw_g_weight: C.float(cfg.CWGWeight),
		cw_b_weight: C.float(cfg.CWBWeight),
		cw_a_weight: C.float(cfg.CWAWeight),

		a_scale_radius: C.uint(cfg.AScaleRadius),
		rgbm_m_scale:   C.float(cfg.RGBMMScale),

		tune_partition_count_limit:              C.uint(cfg.TunePartitionCountLimit),
		tune_2partition_index_limit:             C.uint(cfg.Tune2PartitionIndexLimit),
		tune_3partition_index_limit:             C.uint(cfg.Tune3PartitionIndexLimit),
		tune_4partition_index_limit:             C.uint(cfg.Tune4PartitionIndexLimit),
		tune_block_mode_limit:                   C.uint(cfg.TuneBlockModeLimit),
		tune_refinement_limit:                   C.uint(cfg.TuneRefinementLimit),
		tune_candidate_limit:                    C.uint(cfg.TuneCandidateLimit),
		tune_2partitioning_candidate_limit:      C.uint(cfg.Tune2PartitioningCandidateLimit),
		tune_3partitioning_candidate_limit:      C.uint(cfg.Tune3PartitioningCandidateLimit),
		tune_4partitioning_candidate_limit:      C.uint(cfg.Tune4PartitioningCandidateLimit),
		tune_db_limit:                           C.float(cfg.TuneDBLimit),
		tune_mse_overshoot:                      C.float(cfg.TuneMSEOvershoot),
		tune_2partition_early_out_limit_factor:  C.float(cfg.Tune2PartitionEarlyOutLimitFactor),
		tune_3partition_early_out_limit_factor:  C.float(cfg.Tune3PartitionEarlyOutLimitFactor),
		tune_2plane_early_out_limit_correlation: C.float(cfg.Tune2PlaneEarlyOutLimitCorrelation),
		tune_search_mode0_enable:                C.float(cfg.TuneSearchMode0Enable),
	}
}

func swizzleToC(swz Swizzle) C.astc_native_swizzle {
	return C.astc_native_swizzle{
		r: C.astc_native_swz(swz.R),
		g: C.astc_native_swz(swz.G),
		b: C.astc_native_swz(swz.B),
		a: C.astc_native_swz(swz.A),
	}
}

func imageDataPtrAndType(img *Image) (unsafe.Pointer, int, error) {
	if img.DimX <= 0 || img.DimY <= 0 || img.DimZ <= 0 {
		return nil, 0, errors.New("astc/native: invalid image dimensions")
	}

	texelCount := img.DimX * img.DimY * img.DimZ
	if texelCount <= 0 {
		return nil, 0, errors.New("astc/native: invalid image dimensions")
	}

	switch img.DataType {
	case TypeU8:
		if len(img.DataU8) != texelCount*4 {
			return nil, 0, errors.New("astc/native: invalid RGBA8 buffer length")
		}
		return unsafe.Pointer(&img.DataU8[0]), 0, nil
	case TypeF16:
		if len(img.DataF16) != texelCount*4 {
			return nil, 0, errors.New("astc/native: invalid RGBAF16 buffer length")
		}
		return unsafe.Pointer(&img.DataF16[0]), 1, nil
	case TypeF32:
		if len(img.DataF32) != texelCount*4 {
			return nil, 0, errors.New("astc/native: invalid RGBAF32 buffer length")
		}
		return unsafe.Pointer(&img.DataF32[0]), 2, nil
	default:
		return nil, 0, errors.New("astc/native: unknown image data type")
	}
}

func imageOutPtrTypeAndLen(img *Image) (outPtr unsafe.Pointer, outType int, outLenBytes int, err error) {
	if img.DimX <= 0 || img.DimY <= 0 || img.DimZ <= 0 {
		return nil, 0, 0, errors.New("astc/native: invalid image dimensions")
	}

	texelCount := img.DimX * img.DimY * img.DimZ
	if texelCount <= 0 {
		return nil, 0, 0, errors.New("astc/native: invalid image dimensions")
	}

	switch img.DataType {
	case TypeU8:
		if len(img.DataU8) != texelCount*4 {
			return nil, 0, 0, errors.New("astc/native: invalid RGBA8 buffer length")
		}
		return unsafe.Pointer(&img.DataU8[0]), 0, len(img.DataU8), nil
	case TypeF16:
		if len(img.DataF16) != texelCount*4 {
			return nil, 0, 0, errors.New("astc/native: invalid RGBAF16 buffer length")
		}
		return unsafe.Pointer(&img.DataF16[0]), 1, len(img.DataF16) * 2, nil
	case TypeF32:
		if len(img.DataF32) != texelCount*4 {
			return nil, 0, 0, errors.New("astc/native: invalid RGBAF32 buffer length")
		}
		return unsafe.Pointer(&img.DataF32[0]), 2, len(img.DataF32) * 4, nil
	default:
		return nil, 0, 0, errors.New("astc/native: unknown image data type")
	}
}

func blockInfoFromC(info C.astc_native_block_info) BlockInfo {
	var out BlockInfo

	out.Profile = astc.Profile(info.profile)
	out.BlockX = uint32(info.block_x)
	out.BlockY = uint32(info.block_y)
	out.BlockZ = uint32(info.block_z)
	out.TexelCount = uint32(info.texel_count)

	out.IsErrorBlock = info.is_error_block != 0
	out.IsConstantBlock = info.is_constant_block != 0
	out.IsHDRBlock = info.is_hdr_block != 0
	out.IsDualPlaneBlock = info.is_dual_plane_block != 0

	out.PartitionCount = uint32(info.partition_count)
	out.PartitionIndex = uint32(info.partition_index)
	out.DualPlaneComponent = uint32(info.dual_plane_component)

	for i := 0; i < 4; i++ {
		out.ColorEndpointModes[i] = uint32(info.color_endpoint_modes[i])
	}
	out.ColorLevelCount = uint32(info.color_level_count)
	out.WeightLevelCount = uint32(info.weight_level_count)
	out.WeightX = uint32(info.weight_x)
	out.WeightY = uint32(info.weight_y)
	out.WeightZ = uint32(info.weight_z)

	for p := 0; p < 4; p++ {
		for e := 0; e < 2; e++ {
			for c := 0; c < 4; c++ {
				out.ColorEndpoints[p][e][c] = float32(info.color_endpoints[p][e][c])
			}
		}
	}

	for i := 0; i < 216; i++ {
		out.WeightValuesPlane1[i] = float32(info.weight_values_plane1[i])
		out.WeightValuesPlane2[i] = float32(info.weight_values_plane2[i])
		out.PartitionAssignment[i] = uint8(info.partition_assignment[i])
	}

	return out
}
