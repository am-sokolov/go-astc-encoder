package astc

import "encoding/binary"

// btqCount describes the element packing for an integer sequence quantization mode.
type btqCount struct {
	bits   uint8
	trits  bool
	quints bool
}

var btqCounts = [...]btqCount{
	{bits: 1},               // quant2
	{bits: 0, trits: true},  // quant3
	{bits: 2},               // quant4
	{bits: 0, quints: true}, // quant5
	{bits: 1, trits: true},  // quant6
	{bits: 3},               // quant8
	{bits: 1, quints: true}, // quant10
	{bits: 2, trits: true},  // quant12
	{bits: 4},               // quant16
	{bits: 2, quints: true}, // quant20
	{bits: 3, trits: true},  // quant24
	{bits: 5},               // quant32
	{bits: 3, quints: true}, // quant40
	{bits: 4, trits: true},  // quant48
	{bits: 6},               // quant64
	{bits: 4, quints: true}, // quant80
	{bits: 5, trits: true},  // quant96
	{bits: 7},               // quant128
	{bits: 5, quints: true}, // quant160
	{bits: 6, trits: true},  // quant192
	{bits: 8},               // quant256
}

type iseSize struct {
	scale   uint8
	divisor uint8 // encoded as ((divisor<<1)+1)
}

var iseSizes = [...]iseSize{
	{scale: 1, divisor: 0},  // quant2
	{scale: 8, divisor: 2},  // quant3
	{scale: 2, divisor: 0},  // quant4
	{scale: 7, divisor: 1},  // quant5
	{scale: 13, divisor: 2}, // quant6
	{scale: 3, divisor: 0},  // quant8
	{scale: 10, divisor: 1}, // quant10
	{scale: 18, divisor: 2}, // quant12
	{scale: 4, divisor: 0},  // quant16
	{scale: 13, divisor: 1}, // quant20
	{scale: 23, divisor: 2}, // quant24
	{scale: 5, divisor: 0},  // quant32
	{scale: 16, divisor: 1}, // quant40
	{scale: 28, divisor: 2}, // quant48
	{scale: 6, divisor: 0},  // quant64
	{scale: 19, divisor: 1}, // quant80
	{scale: 33, divisor: 2}, // quant96
	{scale: 7, divisor: 0},  // quant128
	{scale: 22, divisor: 1}, // quant160
	{scale: 38, divisor: 2}, // quant192
	{scale: 8, divisor: 0},  // quant256
}

func iseSequenceBitCount(charCount int, q quantMethod) int {
	if int(q) < 0 || int(q) >= len(iseSizes) {
		return 1024
	}
	e := iseSizes[q]
	divisor := int((e.divisor << 1) + 1)
	return (int(e.scale)*charCount + divisor - 1) / divisor
}

var tritBitsToRead = [...]uint8{2, 2, 1, 2, 1}
var tritBlockShift = [...]uint8{0, 2, 4, 5, 7}
var tritNextLCounter = [...]uint8{1, 2, 3, 4, 0}
var tritHCounterIncr = [...]uint8{0, 0, 0, 0, 1}

var quintBitsToRead = [...]uint8{3, 2, 2}
var quintBlockShift = [...]uint8{0, 3, 5}
var quintNextLCounter = [...]uint8{1, 2, 0}
var quintHCounterIncr = [...]uint8{0, 0, 1}

var bitMasksU64 = [...]uint64{
	0x0000,
	0x0001, 0x0003, 0x0007, 0x000F,
	0x001F, 0x003F, 0x007F, 0x00FF,
	0x01FF, 0x03FF, 0x07FF, 0x0FFF,
	0x1FFF, 0x3FFF, 0x7FFF, 0xFFFF,
}

func decodeISE(q quantMethod, charCount int, input []byte, bitOffset int, output []uint8) {
	if charCount <= 0 {
		panic("astc: decodeISE: charCount must be > 0")
	}
	if len(output) < charCount {
		panic("astc: decodeISE: output too small")
	}

	btq := btqCounts[q]
	bits := int(btq.bits)
	trits := btq.trits
	quints := btq.quints

	// Fast path for ASTC blocks: all uses in this package pass 16-byte input blocks.
	if len(input) >= BlockBytes {
		lo := binary.LittleEndian.Uint64(input[0:8])
		hi := binary.LittleEndian.Uint64(input[8:16])
		if !trits && !quints {
			decodeISE128BitsOnly(bits, charCount, lo, hi, bitOffset, output)
			return
		}
		decodeISE128(bits, trits, quints, charCount, lo, hi, bitOffset, output)
		return
	}

	decodeISESlow(bits, trits, quints, charCount, input, bitOffset, output)
}

func decodeISESlow(bits int, trits bool, quints bool, charCount int, input []byte, bitOffset int, output []uint8) {
	var results [68]uint8
	var tqBlocks [22]uint8

	lcounter := 0
	hcounter := 0

	for i := 0; i < charCount; i++ {
		if bits > 0 {
			results[i] = uint8(readBits(bits, bitOffset, input))
			bitOffset += bits
		} else {
			results[i] = 0
		}

		if trits {
			// These tables are copied from the reference encoder.
			br := int(tritBitsToRead[lcounter])
			tdata := readBits(br, bitOffset, input)
			bitOffset += br
			tqBlocks[hcounter] |= uint8(tdata) << tritBlockShift[lcounter]
			hcounter += int(tritHCounterIncr[lcounter])
			lcounter = int(tritNextLCounter[lcounter])
		}

		if quints {
			br := int(quintBitsToRead[lcounter])
			tdata := readBits(br, bitOffset, input)
			bitOffset += br
			tqBlocks[hcounter] |= uint8(tdata) << quintBlockShift[lcounter]
			hcounter += int(quintHCounterIncr[lcounter])
			lcounter = int(quintNextLCounter[lcounter])
		}
	}

	if trits {
		tritBlocks := (charCount + 4) / 5
		for i := 0; i < tritBlocks; i++ {
			t := tritsOfInteger[tqBlocks[i]]
			results[5*i+0] |= t[0] << bits
			results[5*i+1] |= t[1] << bits
			results[5*i+2] |= t[2] << bits
			results[5*i+3] |= t[3] << bits
			results[5*i+4] |= t[4] << bits
		}
	}

	if quints {
		quintBlocks := (charCount + 2) / 3
		for i := 0; i < quintBlocks; i++ {
			qv := quintsOfInteger[tqBlocks[i]]
			results[3*i+0] |= qv[0] << bits
			results[3*i+1] |= qv[1] << bits
			results[3*i+2] |= qv[2] << bits
		}
	}

	copy(output[:charCount], results[:charCount])
}

func decodeISE128(bits int, trits bool, quints bool, charCount int, lo, hi uint64, bitOffset int, output []uint8) {
	if trits {
		decodeISE128Trits(bits, charCount, lo, hi, bitOffset, output)
		return
	}
	if quints {
		decodeISE128Quints(bits, charCount, lo, hi, bitOffset, output)
		return
	}
	decodeISE128BitsOnly(bits, charCount, lo, hi, bitOffset, output)
}

func decodeISE128BitsOnly(bits int, charCount int, lo, hi uint64, bitOffset int, output []uint8) {
	mask := bitMasksU64[bits]

	bit := uint(bitOffset)
	i := 0
	if bit < 64 {
		for ; i < charCount && bit < 64; i++ {
			v := (lo >> bit) | (hi << (64 - bit))
			output[i] = uint8(v & mask)
			bit += uint(bits)
		}
		if bit >= 64 {
			bit -= 64
		}
	}
	for ; i < charCount; i++ {
		output[i] = uint8((hi >> bit) & mask)
		bit += uint(bits)
	}
}

func readBits128U(bitCount uint, bit *uint, lo, hi uint64) uint64 {
	mask := bitMasksU64[bitCount]
	b := *bit
	var v uint64
	if b < 64 {
		v = (lo >> b) | (hi << (64 - b))
	} else {
		v = hi >> (b - 64)
	}
	*bit = b + bitCount
	return v & mask
}

func decodeISE128Trits(bits int, charCount int, lo, hi uint64, bitOffset int, output []uint8) {
	bit := uint(bitOffset)
	shift := uint(bits)

	i := 0
	for ; i+4 < charCount; i += 5 {
		var base0, base1, base2, base3, base4 uint8
		if bits > 0 {
			base0 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t0 := uint8(readBits128U(2, &bit, lo, hi))

		if bits > 0 {
			base1 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t1 := uint8(readBits128U(2, &bit, lo, hi))

		if bits > 0 {
			base2 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t2 := uint8(readBits128U(1, &bit, lo, hi))

		if bits > 0 {
			base3 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t3 := uint8(readBits128U(2, &bit, lo, hi))

		if bits > 0 {
			base4 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t4 := uint8(readBits128U(1, &bit, lo, hi))

		T := t0 | (t1 << 2) | (t2 << 4) | (t3 << 5) | (t4 << 7)
		tv := tritsOfInteger[T]

		output[i+0] = base0 | (tv[0] << shift)
		output[i+1] = base1 | (tv[1] << shift)
		output[i+2] = base2 | (tv[2] << shift)
		output[i+3] = base3 | (tv[3] << shift)
		output[i+4] = base4 | (tv[4] << shift)
	}

	if i >= charCount {
		return
	}

	rem := charCount - i
	var base [5]uint8
	var T uint8

	for j := 0; j < rem; j++ {
		if bits > 0 {
			base[j] = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}

		switch j {
		case 0:
			T |= uint8(readBits128U(2, &bit, lo, hi)) << 0
		case 1:
			T |= uint8(readBits128U(2, &bit, lo, hi)) << 2
		case 2:
			T |= uint8(readBits128U(1, &bit, lo, hi)) << 4
		case 3:
			T |= uint8(readBits128U(2, &bit, lo, hi)) << 5
		case 4:
			T |= uint8(readBits128U(1, &bit, lo, hi)) << 7
		}
	}

	tv := tritsOfInteger[T]
	for j := 0; j < rem; j++ {
		output[i+j] = base[j] | (tv[j] << shift)
	}
}

func decodeISE128Quints(bits int, charCount int, lo, hi uint64, bitOffset int, output []uint8) {
	bit := uint(bitOffset)
	shift := uint(bits)

	i := 0
	for ; i+2 < charCount; i += 3 {
		var base0, base1, base2 uint8
		if bits > 0 {
			base0 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t0 := uint8(readBits128U(3, &bit, lo, hi))

		if bits > 0 {
			base1 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t1 := uint8(readBits128U(2, &bit, lo, hi))

		if bits > 0 {
			base2 = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		t2 := uint8(readBits128U(2, &bit, lo, hi))

		T := t0 | (t1 << 3) | (t2 << 5)
		qv := quintsOfInteger[T]

		output[i+0] = base0 | (qv[0] << shift)
		output[i+1] = base1 | (qv[1] << shift)
		output[i+2] = base2 | (qv[2] << shift)
	}

	if i >= charCount {
		return
	}

	rem := charCount - i
	var base [3]uint8
	var T uint8

	for j := 0; j < rem; j++ {
		if bits > 0 {
			base[j] = uint8(readBits128U(uint(bits), &bit, lo, hi))
		}
		switch j {
		case 0:
			T |= uint8(readBits128U(3, &bit, lo, hi)) << 0
		case 1:
			T |= uint8(readBits128U(2, &bit, lo, hi)) << 3
		case 2:
			T |= uint8(readBits128U(2, &bit, lo, hi)) << 5
		}
	}

	qv := quintsOfInteger[T]
	for j := 0; j < rem; j++ {
		output[i+j] = base[j] | (qv[j] << shift)
	}
}

func readBits(bitCount int, bitOffset int, data []byte) uint32 {
	if bitCount == 0 {
		return 0
	}
	mask := uint32((1 << uint(bitCount)) - 1)

	byteOff := bitOffset >> 3
	shift := uint(bitOffset & 7)

	if byteOff+1 < len(data) {
		v := uint32(data[byteOff]) | (uint32(data[byteOff+1]) << 8)
		return (v >> shift) & mask
	}

	var v uint32
	if byteOff < len(data) {
		v |= uint32(data[byteOff])
	}
	if byteOff+1 < len(data) {
		v |= uint32(data[byteOff+1]) << 8
	}
	return (v >> shift) & mask
}
