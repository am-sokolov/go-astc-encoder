package astc

// hash52 is the hash function used for procedural partition assignment.
//
// Ported from Source/astcenc_partition_tables.cpp.
func hash52(inp uint32) uint32 {
	inp ^= inp >> 15
	inp *= 0xEEDE0891
	inp ^= inp >> 5
	inp += inp << 16
	inp ^= inp >> 7
	inp ^= inp >> 3
	inp ^= inp << 6
	inp ^= inp >> 17
	return inp
}

// selectPartition selects the partition index for a single texel coordinate.
//
// Ported from Source/astcenc_partition_tables.cpp.
func selectPartition(seed, x, y, z, partitionCount int, smallBlock bool) uint8 {
	if smallBlock {
		x <<= 1
		y <<= 1
		z <<= 1
	}

	seed += (partitionCount - 1) * 1024
	rnum := hash52(uint32(seed))

	seed1 := uint8(rnum & 0xF)
	seed2 := uint8((rnum >> 4) & 0xF)
	seed3 := uint8((rnum >> 8) & 0xF)
	seed4 := uint8((rnum >> 12) & 0xF)
	seed5 := uint8((rnum >> 16) & 0xF)
	seed6 := uint8((rnum >> 20) & 0xF)
	seed7 := uint8((rnum >> 24) & 0xF)
	seed8 := uint8((rnum >> 28) & 0xF)
	seed9 := uint8((rnum >> 18) & 0xF)
	seed10 := uint8((rnum >> 22) & 0xF)
	seed11 := uint8((rnum >> 26) & 0xF)
	seed12 := uint8(((rnum >> 30) | (rnum << 2)) & 0xF)

	seed1 *= seed1
	seed2 *= seed2
	seed3 *= seed3
	seed4 *= seed4
	seed5 *= seed5
	seed6 *= seed6
	seed7 *= seed7
	seed8 *= seed8
	seed9 *= seed9
	seed10 *= seed10
	seed11 *= seed11
	seed12 *= seed12

	var sh1, sh2 int
	if (seed & 1) != 0 {
		if (seed & 2) != 0 {
			sh1 = 4
		} else {
			sh1 = 5
		}
		if partitionCount == 3 {
			sh2 = 6
		} else {
			sh2 = 5
		}
	} else {
		if partitionCount == 3 {
			sh1 = 6
		} else {
			sh1 = 5
		}
		if (seed & 2) != 0 {
			sh2 = 4
		} else {
			sh2 = 5
		}
	}

	sh3 := sh2
	if (seed & 0x10) != 0 {
		sh3 = sh1
	}

	seed1 >>= uint8(sh1)
	seed2 >>= uint8(sh2)
	seed3 >>= uint8(sh1)
	seed4 >>= uint8(sh2)
	seed5 >>= uint8(sh1)
	seed6 >>= uint8(sh2)
	seed7 >>= uint8(sh1)
	seed8 >>= uint8(sh2)

	seed9 >>= uint8(sh3)
	seed10 >>= uint8(sh3)
	seed11 >>= uint8(sh3)
	seed12 >>= uint8(sh3)

	a := int(seed1)*x + int(seed2)*y + int(seed11)*z + int(rnum>>14)
	b := int(seed3)*x + int(seed4)*y + int(seed12)*z + int(rnum>>10)
	c := int(seed5)*x + int(seed6)*y + int(seed9)*z + int(rnum>>6)
	d := int(seed7)*x + int(seed8)*y + int(seed10)*z + int(rnum>>2)

	a &= 0x3F
	b &= 0x3F
	c &= 0x3F
	d &= 0x3F

	if partitionCount <= 3 {
		d = 0
	}
	if partitionCount <= 2 {
		c = 0
	}
	if partitionCount <= 1 {
		b = 0
	}

	if a >= b && a >= c && a >= d {
		return 0
	} else if b >= c && b >= d {
		return 1
	} else if c >= d {
		return 2
	}
	return 3
}
