package astc

// Weight quantization only uses quant methods QUANT_2 .. QUANT_32.
// These tables are copied from Source/astcenc_weight_quant_xfer_tables.cpp.

var weightQuantToUnquant = [12][32]uint8{
	// quant2
	{0, 64},
	// quant3
	{0, 32, 64},
	// quant4
	{0, 21, 43, 64},
	// quant5
	{0, 16, 32, 48, 64},
	// quant6
	{0, 12, 25, 39, 52, 64},
	// quant8
	{0, 9, 18, 27, 37, 46, 55, 64},
	// quant10
	{0, 7, 14, 21, 28, 36, 43, 50, 57, 64},
	// quant12
	{0, 5, 11, 17, 23, 28, 36, 41, 47, 53, 59, 64},
	// quant16
	{0, 4, 8, 12, 17, 21, 25, 29, 35, 39, 43, 47, 52, 56, 60, 64},
	// quant20
	{0, 3, 6, 9, 13, 16, 19, 23, 26, 29, 35, 38, 41, 45, 48, 51, 55, 58, 61, 64},
	// quant24
	{0, 2, 5, 8, 11, 13, 16, 19, 22, 24, 27, 30, 34, 37, 40, 42, 45, 48, 51, 53, 56, 59, 62, 64},
	// quant32
	{0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58, 60, 62, 64},
}

var weightScrambleMap = [12][32]uint8{
	// quant2
	{0, 1},
	// quant3
	{0, 1, 2},
	// quant4
	{0, 1, 2, 3},
	// quant5
	{0, 1, 2, 3, 4},
	// quant6
	{0, 2, 4, 5, 3, 1},
	// quant8
	{0, 1, 2, 3, 4, 5, 6, 7},
	// quant10
	{0, 2, 4, 6, 8, 9, 7, 5, 3, 1},
	// quant12
	{0, 4, 8, 2, 6, 10, 11, 7, 3, 9, 5, 1},
	// quant16
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	// quant20
	{0, 4, 8, 12, 16, 2, 6, 10, 14, 18, 19, 15, 11, 7, 3, 17, 13, 9, 5, 1},
	// quant24
	{0, 8, 16, 2, 10, 18, 4, 12, 20, 6, 14, 22, 23, 15, 7, 21, 13, 5, 19, 11, 3, 17, 9, 1},
	// quant32
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
}

var weightUnscrambleAndUnquantMap [12][32]uint8

func init() {
	for q := quantMethod(0); q <= quant32; q++ {
		if q > quant32 {
			continue
		}

		levels := quantLevel(q)
		for i := 0; i < levels; i++ {
			scr := weightScrambleMap[q][i]
			weightUnscrambleAndUnquantMap[q][scr] = weightQuantToUnquant[q][i]
		}
	}
}

func quantLevel(q quantMethod) int {
	switch q {
	case quant2:
		return 2
	case quant3:
		return 3
	case quant4:
		return 4
	case quant5:
		return 5
	case quant6:
		return 6
	case quant8:
		return 8
	case quant10:
		return 10
	case quant12:
		return 12
	case quant16:
		return 16
	case quant20:
		return 20
	case quant24:
		return 24
	case quant32:
		return 32
	case quant40:
		return 40
	case quant48:
		return 48
	case quant64:
		return 64
	case quant80:
		return 80
	case quant96:
		return 96
	case quant128:
		return 128
	case quant160:
		return 160
	case quant192:
		return 192
	case quant256:
		return 256
	default:
		return 0
	}
}
