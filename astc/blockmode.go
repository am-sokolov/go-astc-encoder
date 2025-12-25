package astc

const (
	blockMaxWeights      = 64
	blockMinWeightBits   = 24
	blockMaxWeightBits   = 96
	partitionIndexBits   = 10
	weightsPlane2Offset  = 32
	blockMaxPartitions   = 4
	blockMaxTexels       = 216
	blockMaxColorValues  = 8
	blockMaxColorInts    = 18
	blockMaxColorIntsBuf = 32
)

// decodeBlockMode2D decodes the properties of an encoded 2D block mode.
//
// This is a direct scalar port of decode_block_mode_2d() in Source/astcenc_block_sizes.cpp.
func decodeBlockMode2D(blockMode int) (xWeights, yWeights int, isDualPlane bool, quantMode quantMethod, weightBits int, ok bool) {
	baseQuantMode := (blockMode >> 4) & 1
	H := (blockMode >> 9) & 1
	D := (blockMode >> 10) & 1
	A := (blockMode >> 5) & 0x3

	xWeights = 0
	yWeights = 0

	if (blockMode & 3) != 0 {
		baseQuantMode |= (blockMode & 3) << 1
		B := (blockMode >> 7) & 3
		switch (blockMode >> 2) & 3 {
		case 0:
			xWeights = B + 4
			yWeights = A + 2
		case 1:
			xWeights = B + 8
			yWeights = A + 2
		case 2:
			xWeights = A + 2
			yWeights = B + 8
		case 3:
			B &= 1
			if (blockMode & 0x100) != 0 {
				xWeights = B + 2
				yWeights = A + 2
			} else {
				xWeights = A + 2
				yWeights = B + 6
			}
		}
	} else {
		baseQuantMode |= ((blockMode >> 2) & 3) << 1
		if ((blockMode >> 2) & 3) == 0 {
			return 0, 0, false, 0, 0, false
		}

		B := (blockMode >> 9) & 3
		switch (blockMode >> 7) & 3 {
		case 0:
			xWeights = 12
			yWeights = A + 2
		case 1:
			xWeights = A + 2
			yWeights = 12
		case 2:
			xWeights = A + 6
			yWeights = B + 6
			D = 0
			H = 0
		case 3:
			switch (blockMode >> 5) & 3 {
			case 0:
				xWeights = 6
				yWeights = 10
			case 1:
				xWeights = 10
				yWeights = 6
			case 2, 3:
				return 0, 0, false, 0, 0, false
			}
		}
	}

	weightCount := xWeights * yWeights * (D + 1)
	qm := (baseQuantMode - 2) + 6*H
	if qm < 0 || qm > int(quant32) {
		return 0, 0, false, 0, 0, false
	}

	quantMode = quantMethod(qm)
	isDualPlane = D != 0

	weightBits = iseSequenceBitCount(weightCount, quantMode)
	if weightCount > blockMaxWeights || weightBits < blockMinWeightBits || weightBits > blockMaxWeightBits {
		return 0, 0, false, 0, 0, false
	}
	return xWeights, yWeights, isDualPlane, quantMode, weightBits, true
}

// decodeBlockMode3D decodes the properties of an encoded 3D block mode.
//
// This is a direct scalar port of decode_block_mode_3d() in Source/astcenc_block_sizes.cpp.
func decodeBlockMode3D(blockMode int) (xWeights, yWeights, zWeights int, isDualPlane bool, quantMode quantMethod, weightBits int, ok bool) {
	baseQuantMode := (blockMode >> 4) & 1
	H := (blockMode >> 9) & 1
	D := (blockMode >> 10) & 1
	A := (blockMode >> 5) & 0x3

	xWeights = 0
	yWeights = 0
	zWeights = 0

	if (blockMode & 3) != 0 {
		baseQuantMode |= (blockMode & 3) << 1
		B := (blockMode >> 7) & 3
		C := (blockMode >> 2) & 0x3
		xWeights = A + 2
		yWeights = B + 2
		zWeights = C + 2
	} else {
		baseQuantMode |= ((blockMode >> 2) & 3) << 1
		if ((blockMode >> 2) & 3) == 0 {
			return 0, 0, 0, false, 0, 0, false
		}

		B := (blockMode >> 9) & 3
		if ((blockMode >> 7) & 3) != 3 {
			D = 0
			H = 0
		}
		switch (blockMode >> 7) & 3 {
		case 0:
			xWeights = 6
			yWeights = B + 2
			zWeights = A + 2
		case 1:
			xWeights = A + 2
			yWeights = 6
			zWeights = B + 2
		case 2:
			xWeights = A + 2
			yWeights = B + 2
			zWeights = 6
		case 3:
			xWeights = 2
			yWeights = 2
			zWeights = 2
			switch (blockMode >> 5) & 3 {
			case 0:
				xWeights = 6
			case 1:
				yWeights = 6
			case 2:
				zWeights = 6
			case 3:
				return 0, 0, 0, false, 0, 0, false
			}
		}
	}

	weightCount := xWeights * yWeights * zWeights * (D + 1)
	qm := (baseQuantMode - 2) + 6*H
	if qm < 0 || qm > int(quant32) {
		return 0, 0, 0, false, 0, 0, false
	}

	quantMode = quantMethod(qm)
	isDualPlane = D != 0

	weightBits = iseSequenceBitCount(weightCount, quantMode)
	if weightCount > blockMaxWeights || weightBits < blockMinWeightBits || weightBits > blockMaxWeightBits {
		return 0, 0, 0, false, 0, 0, false
	}
	return xWeights, yWeights, zWeights, isDualPlane, quantMode, weightBits, true
}
