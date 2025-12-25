package astc

import (
	"encoding/binary"
	"math/bits"
)

type symBlockType uint8

const (
	symBlockNonConst symBlockType = iota
	symBlockConstU16
	symBlockConstF16
	symBlockError
)

type symbolicBlock struct {
	blockType symBlockType

	blockMode       uint16
	partitionCount  uint8
	partitionIndex  uint16
	colorFormats    [blockMaxPartitions]uint8
	colorValues     [blockMaxPartitions][blockMaxColorValues]uint8
	quantMode       quantMethod // Color endpoint quantization.
	plane2Component int8

	weights        [blockMaxWeights]uint8 // Unquantized weights (0..64), with plane 2 at +32.
	constantColor  [4]uint16
	formatsMatched bool
}

func physicalToSymbolic(block []byte, blockX, blockY, blockZ int) (scb symbolicBlock) {
	ctx := getDecodeContext(blockX, blockY, blockZ)
	return physicalToSymbolicWithCtx(block, ctx)
}

func physicalToSymbolicWithCtx(block []byte, ctx *decodeContext) (scb symbolicBlock) {
	if len(block) < BlockBytes {
		scb.blockType = symBlockError
		return scb
	}

	blockZ := ctx.blockZ

	blockMode := int(readBits(11, 0, block))
	if (blockMode & 0x1FF) == 0x1FC {
		// Constant color block (including void-extent blocks).
		scb.blockType = symBlockConstU16
		if (blockMode & 0x200) != 0 {
			scb.blockType = symBlockConstF16
		}

		scb.partitionCount = 0
		for i := 0; i < 4; i++ {
			scb.constantColor[i] = binary.LittleEndian.Uint16(block[8+2*i : 10+2*i])
		}

		// Validate void-extent fields (matches reference decoder behavior).
		if blockZ == 1 {
			rsv := int(readBits(2, 10, block))
			if rsv != 3 {
				scb.blockType = symBlockError
				return scb
			}

			vxLowS := int(readBits(8, 12, block)) | (int(readBits(5, 12+8, block)) << 8)
			vxHighS := int(readBits(13, 25, block))
			vxLowT := int(readBits(8, 38, block)) | (int(readBits(5, 38+8, block)) << 8)
			vxHighT := int(readBits(13, 51, block))

			allOnes := vxLowS == 0x1FFF && vxHighS == 0x1FFF && vxLowT == 0x1FFF && vxHighT == 0x1FFF
			if (vxLowS >= vxHighS || vxLowT >= vxHighT) && !allOnes {
				scb.blockType = symBlockError
			}
		} else {
			vxLowS := int(readBits(9, 10, block))
			vxHighS := int(readBits(9, 19, block))
			vxLowT := int(readBits(9, 28, block))
			vxHighT := int(readBits(9, 37, block))
			vxLowR := int(readBits(9, 46, block))
			vxHighR := int(readBits(9, 55, block))

			allOnes := vxLowS == 0x1FF && vxHighS == 0x1FF &&
				vxLowT == 0x1FF && vxHighT == 0x1FF &&
				vxLowR == 0x1FF && vxHighR == 0x1FF

			if (vxLowS >= vxHighS || vxLowT >= vxHighT || vxLowR >= vxHighR) && !allOnes {
				scb.blockType = symBlockError
			}
		}

		return scb
	}

	scb.blockType = symBlockNonConst

	// Decode block mode and validate for the given block size.
	bmi := ctx.blockModes[blockMode]
	if !bmi.ok {
		scb.blockType = symBlockError
		return scb
	}

	isDualPlane := bmi.isDualPlane
	weightQuant := bmi.weightQuant
	weightBits := int(bmi.weightBits)
	weightCount := int(bmi.weightCount)
	realWeightCount := int(bmi.realWeightCnt)

	partitionCount := int(readBits(2, 11, block)) + 1
	if partitionCount <= 0 || partitionCount > blockMaxPartitions {
		scb.blockType = symBlockError
		return scb
	}

	scb.blockMode = uint16(blockMode)
	scb.partitionCount = uint8(partitionCount)

	// Decode weights. The weight stream is stored reversed in the physical block.
	loBlock := binary.LittleEndian.Uint64(block[0:8])
	hiBlock := binary.LittleEndian.Uint64(block[8:16])

	bitsForWeights := weightBits
	belowWeightsPos := 128 - bitsForWeights

	var indices [blockMaxWeights]uint8
	loW := bits.Reverse64(hiBlock)
	hiW := bits.Reverse64(loBlock)
	btqW := btqCounts[weightQuant]
	decodeISE128(int(btqW.bits), btqW.trits, btqW.quints, realWeightCount, loW, hiW, 0, indices[:])

	uqMap := weightUnscrambleAndUnquantMap[weightQuant]
	if isDualPlane {
		for i := 0; i < weightCount; i++ {
			scb.weights[i] = uqMap[indices[2*i]]
			scb.weights[i+weightsPlane2Offset] = uqMap[indices[2*i+1]]
		}
	} else {
		for i := 0; i < weightCount; i++ {
			scb.weights[i] = uqMap[indices[i]]
		}
	}

	if isDualPlane && partitionCount == 4 {
		scb.blockType = symBlockError
		return scb
	}

	colorFormats := [blockMaxPartitions]int{}
	encodedTypeHighPartSize := 0
	if partitionCount == 1 {
		colorFormats[0] = int(readBits(4, 13, block))
		scb.partitionIndex = 0
	} else {
		encodedTypeHighPartSize = (3 * partitionCount) - 4
		belowWeightsPos -= encodedTypeHighPartSize
		encodedType := int(readBits(6, 13+partitionIndexBits, block)) |
			(int(readBits(encodedTypeHighPartSize, belowWeightsPos, block)) << 6)
		baseclass := encodedType & 0x3
		if baseclass == 0 {
			for i := 0; i < partitionCount; i++ {
				colorFormats[i] = (encodedType >> 2) & 0xF
			}
			belowWeightsPos += encodedTypeHighPartSize
			scb.formatsMatched = true
			encodedTypeHighPartSize = 0
		} else {
			bitpos := 2
			baseclass--
			for i := 0; i < partitionCount; i++ {
				colorFormats[i] = (((encodedType >> bitpos) & 1) + baseclass) << 2
				bitpos++
			}
			for i := 0; i < partitionCount; i++ {
				colorFormats[i] |= (encodedType >> bitpos) & 3
				bitpos += 2
			}
		}

		scb.partitionIndex = uint16(readBits(partitionIndexBits, 13, block))
	}

	for i := 0; i < partitionCount; i++ {
		scb.colorFormats[i] = uint8(colorFormats[i])
	}

	colorIntCount := 0
	for i := 0; i < partitionCount; i++ {
		endpointClass := colorFormats[i] >> 2
		colorIntCount += (endpointClass + 1) * 2
	}
	if colorIntCount > blockMaxColorInts {
		scb.blockType = symBlockError
		return scb
	}

	colorBitsArr := [...]int{-1, 115 - 4, 113 - 4 - partitionIndexBits, 113 - 4 - partitionIndexBits, 113 - 4 - partitionIndexBits}
	colorBits := colorBitsArr[partitionCount] - bitsForWeights - encodedTypeHighPartSize
	if isDualPlane {
		colorBits -= 2
	}
	if colorBits < 0 {
		colorBits = 0
	}

	colorQuantLevel := quantLevelForISE(colorIntCount, colorBits)
	if colorQuantLevel < int(quant6) {
		scb.blockType = symBlockError
		return scb
	}
	scb.quantMode = quantMethod(colorQuantLevel)

	var valuesToDecode [blockMaxColorIntsBuf]uint8
	startBit := 17
	if partitionCount != 1 {
		startBit = 19 + partitionIndexBits
	}
	btqC := btqCounts[scb.quantMode]
	decodeISE128(int(btqC.bits), btqC.trits, btqC.quints, colorIntCount, loBlock, hiBlock, startBit, valuesToDecode[:])

	unpackTable := colorScrambledPquantToUquantTables[int(scb.quantMode)-int(quant6)]
	valueOff := 0
	for i := 0; i < partitionCount; i++ {
		vals := 2*(colorFormats[i]>>2) + 2
		for j := 0; j < vals; j++ {
			scb.colorValues[i][j] = unpackTable[valuesToDecode[valueOff+j]]
		}
		valueOff += vals
	}

	scb.plane2Component = -1
	if isDualPlane {
		scb.plane2Component = int8(readBits(2, belowWeightsPos-2, block))
	}

	return scb
}
