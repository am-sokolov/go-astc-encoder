//go:build goexperiment.simd && amd64

package astc

import (
	"simd/archsimd"
	"unsafe"
)

var (
	avgIdxR = archsimd.LoadInt8x16(&[16]int8{0, 4, 8, 12, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1})
	avgIdxG = archsimd.LoadInt8x16(&[16]int8{1, 5, 9, 13, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1})
	avgIdxB = archsimd.LoadInt8x16(&[16]int8{2, 6, 10, 14, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1})
	avgIdxA = archsimd.LoadInt8x16(&[16]int8{3, 7, 11, 15, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1})
	avgZero = archsimd.LoadUint8x16(&[16]uint8{})
)

func avgBlockRGBA8(pix []byte, width, height, x0, y0, blockX, blockY int) (r, g, b, a uint8) {
	// The current simd/archsimd implementation is amd64-only (Go experiment, late 2025).
	// Keep scalar fallback for correctness and for older CPUs without AVX.
	if blockX != 4 || blockY != 4 || x0+4 > width || y0+4 > height || !archsimd.X86.AVX() {
		return avgBlockRGBA8Scalar(pix, width, height, x0, y0, blockX, blockY)
	}

	stride := width * 4
	base := y0*stride + x0*4

	var sumR, sumG, sumB, sumA uint32
	for row := 0; row < 4; row++ {
		off := base + row*stride
		v := archsimd.LoadUint8x16((*[16]uint8)(unsafe.Pointer(&pix[off])))

		sumR += sum4Bytes(v.PermuteOrZero(avgIdxR))
		sumG += sum4Bytes(v.PermuteOrZero(avgIdxG))
		sumB += sum4Bytes(v.PermuteOrZero(avgIdxB))
		sumA += sum4Bytes(v.PermuteOrZero(avgIdxA))
	}

	// 16 pixels.
	return uint8((sumR + 8) / 16),
		uint8((sumG + 8) / 16),
		uint8((sumB + 8) / 16),
		uint8((sumA + 8) / 16)
}

func sum4Bytes(v archsimd.Uint8x16) uint32 {
	// v has 4 values in lanes 0..3, and zeros elsewhere.
	s := v.SumAbsDiff(avgZero)
	return uint32(s.GetElem(0)) + uint32(s.GetElem(4))
}
