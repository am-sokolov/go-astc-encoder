package astc

// quantLevelForISELUT provides an O(1) lookup for quantLevelForISE().
//
// It maps (charCount, bitsAvailable) -> best quantMethod numeric value, or -1 if none fits.
// This is heavily used by the decoder when unpacking endpoint streams.
const (
	iseQuantLUTMaxChars = blockMaxColorIntsBuf
	iseQuantLUTMaxBits  = 128
)

var quantLevelForISELUT [iseQuantLUTMaxChars + 1][iseQuantLUTMaxBits + 1]int16

func init() {
	for cc := 0; cc <= iseQuantLUTMaxChars; cc++ {
		for b := 0; b <= iseQuantLUTMaxBits; b++ {
			quantLevelForISELUT[cc][b] = -1
		}
	}

	for cc := 1; cc <= iseQuantLUTMaxChars; cc++ {
		for b := 0; b <= iseQuantLUTMaxBits; b++ {
			best := int16(-1)
			for q := int(quant256); q >= int(quant2); q-- {
				if iseSequenceBitCount(cc, quantMethod(q)) <= b {
					best = int16(q)
					break
				}
			}
			quantLevelForISELUT[cc][b] = best
		}
	}
}

func quantLevelForISE(charCount, bitsAvailable int) int {
	// Find the highest-precision quant level that fits into bitsAvailable.
	if charCount <= 0 {
		return -1
	}
	if bitsAvailable < 0 {
		return -1
	}
	if bitsAvailable > iseQuantLUTMaxBits {
		bitsAvailable = iseQuantLUTMaxBits
	}
	if charCount <= iseQuantLUTMaxChars {
		return int(quantLevelForISELUT[charCount][bitsAvailable])
	}

	// Fallback (should not be hit by the current encoder/decoder).
	best := -1
	for q := int(quant256); q >= int(quant2); q-- {
		if iseSequenceBitCount(charCount, quantMethod(q)) <= bitsAvailable {
			best = q
			break
		}
	}
	return best
}
