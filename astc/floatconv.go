package astc

import "math/bits"

func unorm16ToSF16(p uint16) uint16 {
	// Scalar port of unorm16_to_sf16() in Source/astcenc_vecmathlib.h.
	if p == 0xFFFF {
		return 0x3C00 // FP16 1.0
	}
	if p < 4 {
		// Small values are handled with a simple shift which is exact.
		return p << 8
	}

	// lz is clz(p) - 16, clamped. For p in [0x0004, 0xFFFE], this is in [0, 13].
	lz := bits.LeadingZeros32(uint32(p)) - 16
	if lz < 0 {
		lz = 0
	} else if lz > 32 {
		lz = 32
	}

	// p = p * 2^(lz+1), but kept in 16-bit range.
	p32 := uint32(p) * (1 << uint(lz+1))
	p32 &= 0xFFFF
	p32 >>= 6

	// Exponent bits.
	exp := uint32(14 - lz)
	p32 |= exp << 10
	return uint16(p32)
}

func lnsToSF16(p uint16) uint16 {
	// Scalar port of lns_to_sf16() in Source/astcenc_vecmathlib.h.
	mc := int(p & 0x7FF)
	ec := int(p >> 11)

	var mt int
	if mc < 512 {
		mt = mc * 3
	} else if mc < 1536 {
		mt = mc*4 - 512
	} else {
		mt = mc*5 - 2048
	}

	res := (ec << 10) | (mt >> 3)
	if res > 0x7BFF {
		res = 0x7BFF
	}
	return uint16(res)
}
