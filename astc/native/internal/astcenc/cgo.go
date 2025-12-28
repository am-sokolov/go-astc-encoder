//go:build astcenc_native && cgo

package astcenc

/*
#cgo CXXFLAGS: -O3 -std=c++14 -I${SRCDIR}/upstream
#cgo darwin LDFLAGS: -lm
#cgo linux LDFLAGS: -lstdc++ -lm -pthread

#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

func ErrorString(code int) string {
	if code == 0 {
		return ""
	}
	s := C.astc_native_error_string(C.int(code))
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

func Realloc(p unsafe.Pointer, size int) unsafe.Pointer {
	if size <= 0 {
		return nil
	}
	return C.realloc(p, C.size_t(size))
}

func Free(p unsafe.Pointer) {
	if p != nil {
		C.free(p)
	}
}

func ContextCreate(profile int, blockX, blockY, blockZ int, quality float32, flags uint32, threadCount int) (unsafe.Pointer, int) {
	var ctx unsafe.Pointer
	code := C.astc_native_context_create(
		C.int(profile),
		C.uint(blockX),
		C.uint(blockY),
		C.uint(blockZ),
		C.float(quality),
		C.uint(flags),
		C.uint(threadCount),
		(*unsafe.Pointer)(unsafe.Pointer(&ctx)),
	)
	return ctx, int(code)
}

func ContextDestroy(ctx unsafe.Pointer) {
	if ctx != nil {
		C.astc_native_context_destroy(ctx)
	}
}

func ImageCreateU8() (unsafe.Pointer, int) {
	var img unsafe.Pointer
	code := C.astc_native_image_create_u8((*unsafe.Pointer)(unsafe.Pointer(&img)))
	return img, int(code)
}

func ImageCreateF16() (unsafe.Pointer, int) {
	var img unsafe.Pointer
	code := C.astc_native_image_create_f16((*unsafe.Pointer)(unsafe.Pointer(&img)))
	return img, int(code)
}

func ImageCreateF32() (unsafe.Pointer, int) {
	var img unsafe.Pointer
	code := C.astc_native_image_create_f32((*unsafe.Pointer)(unsafe.Pointer(&img)))
	return img, int(code)
}

func ImageDestroy(img unsafe.Pointer) {
	if img != nil {
		C.astc_native_image_destroy(img)
	}
}

func ImageInitU8(img unsafe.Pointer, width, height, depth int, rgba unsafe.Pointer) int {
	return int(C.astc_native_image_init_u8(img, C.uint(width), C.uint(height), C.uint(depth), rgba))
}

func ImageInitF16(img unsafe.Pointer, width, height, depth int, rgba unsafe.Pointer) int {
	return int(C.astc_native_image_init_f16(img, C.uint(width), C.uint(height), C.uint(depth), rgba))
}

func ImageInitF32(img unsafe.Pointer, width, height, depth int, rgba unsafe.Pointer) int {
	return int(C.astc_native_image_init_f32(img, C.uint(width), C.uint(height), C.uint(depth), rgba))
}

func CompressImage(ctx, img, outData unsafe.Pointer, outLen int, threadIndex int) int {
	return int(C.astc_native_compress_image(ctx, img, outData, C.size_t(outLen), C.uint(threadIndex)))
}

func CompressReset(ctx unsafe.Pointer) int {
	return int(C.astc_native_compress_reset(ctx))
}

func DecompressImageRGBA8(ctx, data unsafe.Pointer, dataLen int, width, height, depth int, outRGBA unsafe.Pointer, outLen int, threadIndex int) int {
	return int(C.astc_native_decompress_image_rgba8(
		ctx,
		data,
		C.size_t(dataLen),
		C.uint(width),
		C.uint(height),
		C.uint(depth),
		outRGBA,
		C.size_t(outLen),
		C.uint(threadIndex),
	))
}

func DecompressImageRGBAF32(ctx, data unsafe.Pointer, dataLen int, width, height, depth int, outRGBA unsafe.Pointer, outLen int, threadIndex int) int {
	return int(C.astc_native_decompress_image_rgba32f(
		ctx,
		data,
		C.size_t(dataLen),
		C.uint(width),
		C.uint(height),
		C.uint(depth),
		outRGBA,
		C.size_t(outLen),
		C.uint(threadIndex),
	))
}

func DecompressReset(ctx unsafe.Pointer) int {
	return int(C.astc_native_decompress_reset(ctx))
}

// ----------------------------------------------------------------------------
// Raw-ish bindings for full astcenc feature exposure.
// ----------------------------------------------------------------------------

type ConfigData = C.astc_native_config_data
type Swizzle = C.astc_native_swizzle
type BlockInfo = C.astc_native_block_info

func ConfigInitData(profile int, blockX, blockY, blockZ int, quality float32, flags uint32) (ConfigData, int) {
	var cfg ConfigData
	code := C.astc_native_config_init_data(
		C.int(profile),
		C.uint(blockX),
		C.uint(blockY),
		C.uint(blockZ),
		C.float(quality),
		C.uint(flags),
		&cfg,
	)
	return cfg, int(code)
}

func ContextAllocFromData(cfg *ConfigData, threadCount int, enableProgress bool) (unsafe.Pointer, int) {
	if cfg == nil {
		return nil, 3 // ASTCENC_ERR_BAD_PARAM
	}
	var ctx unsafe.Pointer
	ep := 0
	if enableProgress {
		ep = 1
	}
	code := C.astc_native_context_alloc_from_data(
		cfg,
		C.uint(threadCount),
		C.int(ep),
		(*unsafe.Pointer)(unsafe.Pointer(&ctx)),
	)
	return ctx, int(code)
}

func CompressImageEx(ctx unsafe.Pointer, dataType int, width, height, depth int, rgba unsafe.Pointer, swizzle *Swizzle, outData unsafe.Pointer, outLen int, threadIndex int, progressHandle uintptr) int {
	return int(C.astc_native_compress_image_ex(
		ctx,
		C.int(dataType),
		C.uint(width),
		C.uint(height),
		C.uint(depth),
		rgba,
		swizzle,
		outData,
		C.size_t(outLen),
		C.uint(threadIndex),
		C.uintptr_t(progressHandle),
	))
}

func CompressCancel(ctx unsafe.Pointer) int {
	return int(C.astc_native_compress_cancel(ctx))
}

func DecompressImageEx(ctx unsafe.Pointer, data unsafe.Pointer, dataLen int, outType int, width, height, depth int, outRGBA unsafe.Pointer, outLen int, swizzle *Swizzle, threadIndex int) int {
	return int(C.astc_native_decompress_image_ex(
		ctx,
		data,
		C.size_t(dataLen),
		C.int(outType),
		C.uint(width),
		C.uint(height),
		C.uint(depth),
		outRGBA,
		C.size_t(outLen),
		swizzle,
		C.uint(threadIndex),
	))
}

func GetBlockInfo(ctx unsafe.Pointer, block unsafe.Pointer, info *BlockInfo) int {
	if block == nil || info == nil {
		return 3 // ASTCENC_ERR_BAD_PARAM
	}
	return int(C.astc_native_get_block_info(ctx, (*C.uint8_t)(block), info))
}

//export astc_native_go_progress
func astc_native_go_progress(handle C.uintptr_t, progress C.float) {
	if handle == 0 {
		return
	}
	defer func() { _ = recover() }()

	h := cgo.Handle(handle)
	v := h.Value()
	cb, ok := v.(func(float32))
	if !ok || cb == nil {
		return
	}
	cb(float32(progress))
}
