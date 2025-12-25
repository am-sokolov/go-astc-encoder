package astc

// Integer sequence encoding (ISE) encoder.
//
// This is a scalar port of encode_ise() in Source/astcenc_integer_sequence.cpp.

var integerOfQuints [5][5][5]uint8
var integerOfTrits [3][3][3][3][3]uint8

func init() {
	// Build inverse tables from the decoder tables. Multiple packed integers can map to the
	// same unpacked tuple; any stable choice round-trips correctly for our use cases.
	for packed := 0; packed < len(quintsOfInteger); packed++ {
		q := quintsOfInteger[packed]
		integerOfQuints[q[2]][q[1]][q[0]] = uint8(packed)
	}
	for packed := 0; packed < len(tritsOfInteger); packed++ {
		t := tritsOfInteger[packed]
		integerOfTrits[t[4]][t[3]][t[2]][t[1]][t[0]] = uint8(packed)
	}
}

func writeBits(bitCount int, bitOffset int, data []byte, value uint32) {
	if bitCount <= 0 {
		return
	}
	mask := uint32((1 << uint(bitCount)) - 1)
	value &= mask

	byteOff := bitOffset >> 3
	shift := uint(bitOffset & 7)

	value <<= shift
	mask <<= shift
	mask = ^mask

	if byteOff < len(data) {
		data[byteOff] = (data[byteOff] & byte(mask)) | byte(value)
	}
	if byteOff+1 < len(data) {
		data[byteOff+1] = (data[byteOff+1] & byte(mask>>8)) | byte(value>>8)
	}
}

func encodeISE(q quantMethod, charCount int, input []uint8, output []byte, bitOffset int) {
	if charCount <= 0 {
		panic("astc: encodeISE: charCount must be > 0")
	}
	if len(input) < charCount {
		panic("astc: encodeISE: input too small")
	}

	btq := btqCounts[q]
	bits := int(btq.bits)
	trits := btq.trits
	quints := btq.quints

	mask := uint8(0)
	if bits != 0 {
		mask = uint8((1 << uint(bits)) - 1)
	}

	// Write out trits and bits.
	if trits {
		i := 0
		fullBlocks := charCount / 5
		for j := 0; j < fullBlocks; j++ {
			i4 := input[i+4] >> bits
			i3 := input[i+3] >> bits
			i2 := input[i+2] >> bits
			i1 := input[i+1] >> bits
			i0 := input[i+0] >> bits

			T := integerOfTrits[i4][i3][i2][i1][i0]

			// Element 0 + T0 + T1
			pack := (input[i] & mask) | (((T >> 0) & 0x3) << bits)
			i++
			writeBits(bits+2, bitOffset, output, uint32(pack))
			bitOffset += bits + 2

			// Element 1 + T2 + T3
			pack = (input[i] & mask) | (((T >> 2) & 0x3) << bits)
			i++
			writeBits(bits+2, bitOffset, output, uint32(pack))
			bitOffset += bits + 2

			// Element 2 + T4
			pack = (input[i] & mask) | (((T >> 4) & 0x1) << bits)
			i++
			writeBits(bits+1, bitOffset, output, uint32(pack))
			bitOffset += bits + 1

			// Element 3 + T5 + T6
			pack = (input[i] & mask) | (((T >> 5) & 0x3) << bits)
			i++
			writeBits(bits+2, bitOffset, output, uint32(pack))
			bitOffset += bits + 2

			// Element 4 + T7
			pack = (input[i] & mask) | (((T >> 7) & 0x1) << bits)
			i++
			writeBits(bits+1, bitOffset, output, uint32(pack))
			bitOffset += bits + 1
		}

		// Loop tail for a partial block.
		if i != charCount {
			i4 := uint8(0)
			i3 := uint8(0)
			i2 := uint8(0)
			i1 := uint8(0)
			i0 := input[i+0] >> bits
			if i+3 < charCount {
				i3 = input[i+3] >> bits
			}
			if i+2 < charCount {
				i2 = input[i+2] >> bits
			}
			if i+1 < charCount {
				i1 = input[i+1] >> bits
			}

			T := integerOfTrits[i4][i3][i2][i1][i0]

			// Truncated table as this iteration is always partial.
			tbits := [...]int{2, 2, 1, 2}
			tshift := [...]int{0, 2, 4, 5}

			j := 0
			for ; i < charCount; i, j = i+1, j+1 {
				pack := (input[i] & mask) | (((T >> uint(tshift[j])) & uint8((1<<uint(tbits[j]))-1)) << bits)
				writeBits(bits+tbits[j], bitOffset, output, uint32(pack))
				bitOffset += bits + tbits[j]
			}
		}
		return
	}

	// Write out quints and bits.
	if quints {
		i := 0
		fullBlocks := charCount / 3
		for j := 0; j < fullBlocks; j++ {
			i2 := input[i+2] >> bits
			i1 := input[i+1] >> bits
			i0 := input[i+0] >> bits

			T := integerOfQuints[i2][i1][i0]

			// Element 0
			pack := (input[i] & mask) | (((T >> 0) & 0x7) << bits)
			i++
			writeBits(bits+3, bitOffset, output, uint32(pack))
			bitOffset += bits + 3

			// Element 1
			pack = (input[i] & mask) | (((T >> 3) & 0x3) << bits)
			i++
			writeBits(bits+2, bitOffset, output, uint32(pack))
			bitOffset += bits + 2

			// Element 2
			pack = (input[i] & mask) | (((T >> 5) & 0x3) << bits)
			i++
			writeBits(bits+2, bitOffset, output, uint32(pack))
			bitOffset += bits + 2
		}

		// Loop tail for a partial block.
		if i != charCount {
			i2 := uint8(0)
			i1 := uint8(0)
			i0 := input[i+0] >> bits
			if i+1 < charCount {
				i1 = input[i+1] >> bits
			}

			T := integerOfQuints[i2][i1][i0]

			tbits := [...]int{3, 2}
			tshift := [...]int{0, 3}

			j := 0
			for ; i < charCount; i, j = i+1, j+1 {
				pack := (input[i] & mask) | (((T >> uint(tshift[j])) & uint8((1<<uint(tbits[j]))-1)) << bits)
				writeBits(bits+tbits[j], bitOffset, output, uint32(pack))
				bitOffset += bits + tbits[j]
			}
		}
		return
	}

	// Write out just bits.
	for i := 0; i < charCount; i++ {
		writeBits(bits, bitOffset, output, uint32(input[i]))
		bitOffset += bits
	}
}
