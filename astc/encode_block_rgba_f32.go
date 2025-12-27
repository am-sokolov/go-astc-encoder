package astc

import (
	"errors"
	"math"
)

func isConstBlockRGBAF32(texels []float32) (r, g, b, a uint16, ok bool) {
	if len(texels) < 4 {
		return 0, 0, 0, 0, false
	}
	r0 := float32ToHalf(texels[0])
	g0 := float32ToHalf(texels[1])
	b0 := float32ToHalf(texels[2])
	a0 := float32ToHalf(texels[3])
	for i := 4; i < len(texels); i += 4 {
		if float32ToHalf(texels[i+0]) != r0 ||
			float32ToHalf(texels[i+1]) != g0 ||
			float32ToHalf(texels[i+2]) != b0 ||
			float32ToHalf(texels[i+3]) != a0 {
			return 0, 0, 0, 0, false
		}
	}
	return r0, g0, b0, a0, true
}

func alphaRGBAbsCorrelationU16(texels [][4]uint16) float64 {
	n := len(texels)
	if n <= 1 {
		return 1
	}

	var sumL, sumA int64
	var sumLL, sumAA int64
	var sumLA int64

	for i := 0; i < n; i++ {
		l := int64(texels[i][0]) + int64(texels[i][1]) + int64(texels[i][2])
		a := int64(texels[i][3])
		sumL += l
		sumA += a
		sumLL += l * l
		sumAA += a * a
		sumLA += l * a
	}

	nn := int64(n)
	varL := sumLL*nn - sumL*sumL
	varA := sumAA*nn - sumA*sumA
	if varL <= 0 || varA <= 0 {
		// No variance -> a single weight plane is sufficient.
		return 1
	}

	cov := sumLA*nn - sumL*sumA
	corr := float64(cov) / math.Sqrt(float64(varL)*float64(varA))
	if corr < 0 {
		corr = -corr
	}
	if corr > 1 {
		corr = 1
	}
	return corr
}

func componentRestAbsCorrelationU16(texels [][4]uint16, component int) float64 {
	n := len(texels)
	if n <= 1 || component < 0 || component > 3 {
		return 1
	}

	var sumX, sumY int64
	var sumXX, sumYY int64
	var sumXY int64

	for i := 0; i < n; i++ {
		x := int64(texels[i][component])
		y := int64(texels[i][0]) + int64(texels[i][1]) + int64(texels[i][2]) + int64(texels[i][3]) - x

		sumX += x
		sumY += y
		sumXX += x * x
		sumYY += y * y
		sumXY += x * y
	}

	nn := int64(n)
	varX := sumXX*nn - sumX*sumX
	varY := sumYY*nn - sumY*sumY
	if varX <= 0 || varY <= 0 {
		// No variance -> a single weight plane is sufficient.
		return 1
	}

	cov := sumXY*nn - sumX*sumY
	corr := float64(cov) / math.Sqrt(float64(varX)*float64(varY))
	if corr < 0 {
		corr = -corr
	}
	if corr > 1 {
		corr = 1
	}
	return corr
}

func endpointIntCount(format uint8) int {
	return (int(format>>2) + 1) * 2
}

func encodeBlockRGBAF32HDR(profile Profile, blockX, blockY, blockZ int, texels []float32, quality EncodeQuality, channelWeight [4]float32, tuneOverride *encoderTuning) ([BlockBytes]byte, error) {
	if profile != ProfileHDR && profile != ProfileHDRRGBLDRAlpha {
		return [BlockBytes]byte{}, errors.New("astc: EncodeRGBAF32* only supports HDR profiles")
	}

	if r, g, b, a, ok := isConstBlockRGBAF32(texels); ok {
		return EncodeConstBlockF16(r, g, b, a), nil
	}

	texelCount := blockX * blockY * blockZ

	// Candidate list.
	modes := validBlockModes(blockX, blockY, blockZ)
	if len(modes) == 0 {
		// Fallback: constant average.
		var sr, sg, sb, sa float64
		for t := 0; t < texelCount; t++ {
			off := t * 4
			sr += float64(texels[off+0])
			sg += float64(texels[off+1])
			sb += float64(texels[off+2])
			sa += float64(texels[off+3])
		}
		inv := 1.0 / float64(texelCount)
		return EncodeConstBlockF16(
			float32ToHalf(float32(sr*inv)),
			float32ToHalf(float32(sg*inv)),
			float32ToHalf(float32(sb*inv)),
			float32ToHalf(float32(sa*inv)),
		), nil
	}

	tune := encoderTuningFor(quality, texelCount)
	if tuneOverride != nil {
		tune = *tuneOverride
	}
	modeLimit := tune.modeLimit
	if modeLimit <= 0 || modeLimit > len(modes) {
		modeLimit = len(modes)
	}
	modes = modes[:modeLimit]

	// Convert texels into "code space" (UNORM16 or LNS), matching upstream encoder behavior.
	var srcCodesArr [blockMaxTexels][4]uint16
	srcCodes := srcCodesArr[:texelCount]
	var texelLumaArr [blockMaxTexels]float32
	var texelAlphaArr [blockMaxTexels]float32
	texelLuma := texelLumaArr[:texelCount]
	texelAlpha := texelAlphaArr[:texelCount]

	var codeMin [4]uint16
	var codeMax [4]uint16
	for c := 0; c < 4; c++ {
		codeMin[c] = 0xFFFF
		codeMax[c] = 0
	}
	for t := 0; t < texelCount; t++ {
		off := t * 4
		r := texels[off+0]
		g := texels[off+1]
		b := texels[off+2]
		a := texels[off+3]

		texelLuma[t] = r + g + b
		texelAlpha[t] = a

		srcCodes[t][0] = hdrTexelToLNS(r)
		srcCodes[t][1] = hdrTexelToLNS(g)
		srcCodes[t][2] = hdrTexelToLNS(b)
		if profile == ProfileHDR {
			srcCodes[t][3] = hdrTexelToLNS(a)
		} else {
			// HDR RGB + LDR alpha uses UNORM16 alpha endpoints.
			av := a
			if !(av >= 0) {
				av = 0
			}
			if av <= 0 {
				av = 0
			} else if av >= 1 {
				av = 1
			}
			srcCodes[t][3] = uint16(flt2intRTN(av * 65535.0))
		}

		for c := 0; c < 4; c++ {
			v := srcCodes[t][c]
			if v < codeMin[c] {
				codeMin[c] = v
			}
			if v > codeMax[c] {
				codeMax[c] = v
			}
		}
	}

	alphaMin := codeMin[3]
	alphaMax := codeMax[3]
	alphaVary := alphaMin != alphaMax

	var dualPlaneComponentsArr [4]int
	dualPlaneComponentCount := 0
	if quality >= EncodeThorough {
		thresh := tune.dualPlaneCorrelationThreshold
		for c := 0; c < 4; c++ {
			if codeMin[c] == codeMax[c] {
				continue
			}
			if thresh > 0 && componentRestAbsCorrelationU16(srcCodes, c) >= float64(thresh) {
				continue
			}
			dualPlaneComponentsArr[dualPlaneComponentCount] = c
			dualPlaneComponentCount++
		}
	} else if alphaVary {
		dualPlaneComponentsArr[0] = 3
		dualPlaneComponentCount = 1
	}
	dualPlaneComponents := dualPlaneComponentsArr[:dualPlaneComponentCount]
	allowDualPlane := len(dualPlaneComponents) != 0

	wR := float64(channelWeight[0])
	wG := float64(channelWeight[1])
	wB := float64(channelWeight[2])
	wA := float64(channelWeight[3])

	bestErr := math.Inf(1)
	var bestMode blockModeDesc
	bestPartitionCount := 0
	bestPartitionIndex := 0
	bestPlane2Component := -1
	bestEndpointFormat := uint8(0)
	bestColorQuant := quantMethod(0)

	var endpointPquantBufA [32]uint8
	var endpointPquantBufB [32]uint8
	currEndpointPquantBuf := endpointPquantBufA[:]
	bestEndpointPquantBuf := endpointPquantBufB[:]
	bestEndpointLen := 0

	var weightPquantBufA [blockMaxWeights]uint8
	var weightPquantBufB [blockMaxWeights]uint8
	currWeightPquantBuf := weightPquantBufA[:]
	bestWeightPquantBuf := weightPquantBufB[:]
	bestWeightLen := 0

	var weightsUQArr [blockMaxWeights]uint8
	var texelWeightsArr [blockMaxTexels]int
	var texelWeights2Arr [blockMaxTexels]int
	texelWeights := texelWeightsArr[:texelCount]
	texelWeights2 := texelWeights2Arr[:texelCount]

	var evalEp0 [blockMaxPartitions][4]int32
	var evalEpd [blockMaxPartitions][4]int32

	var partitionCountsArr [blockMaxPartitions]int
	partitionCountsArr[0] = 1
	partitionCountLen := 1
	for pc := 2; pc <= tune.maxPartitionCount && pc <= blockMaxPartitions; pc++ {
		partitionCountsArr[partitionCountLen] = pc
		partitionCountLen++
	}
	partitionCounts := partitionCountsArr[:partitionCountLen]

	var pt2 *partitionTable
	var pt3 *partitionTable
	var pt4 *partitionTable
	if tune.maxPartitionCount >= 2 {
		pt2 = getPartitionTable(blockX, blockY, blockZ, 2)
	}
	if tune.maxPartitionCount >= 3 {
		pt3 = getPartitionTable(blockX, blockY, blockZ, 3)
	}
	if tune.maxPartitionCount >= 4 {
		pt4 = getPartitionTable(blockX, blockY, blockZ, 4)
	}

	partIndexLimit2 := tune.partitionIndexLimit[2]
	partIndexLimit3 := tune.partitionIndexLimit[3]
	partIndexLimit4 := tune.partitionIndexLimit[4]
	if partIndexLimit2 > (1 << partitionIndexBits) {
		partIndexLimit2 = 1 << partitionIndexBits
	}
	if partIndexLimit3 > (1 << partitionIndexBits) {
		partIndexLimit3 = 1 << partitionIndexBits
	}
	if partIndexLimit4 > (1 << partitionIndexBits) {
		partIndexLimit4 = 1 << partitionIndexBits
	}

	var candidates2Arr [128]int
	var candidates3Arr [128]int
	var candidates4Arr [128]int

	var candidates2 []int
	var candidates3 []int
	var candidates4 []int

	candidates2Count := 0
	candidates3Count := 0
	candidates4Count := 0

	if pt2 != nil {
		want := tune.partitionCandidateLimit[2]
		if want > partIndexLimit2 {
			want = partIndexLimit2
		}
		if want > len(candidates2Arr) {
			want = len(candidates2Arr)
		}
		if want > 0 && partIndexLimit2 > 0 {
			candidates2 = candidates2Arr[:want]
			candidates2Count = selectBestPartitionIndicesU16(candidates2, srcCodes, pt2, 2, partIndexLimit2, alphaVary)
		}
	}
	if pt3 != nil {
		want := tune.partitionCandidateLimit[3]
		if want > partIndexLimit3 {
			want = partIndexLimit3
		}
		if want > len(candidates3Arr) {
			want = len(candidates3Arr)
		}
		if want > 0 && partIndexLimit3 > 0 {
			candidates3 = candidates3Arr[:want]
			candidates3Count = selectBestPartitionIndicesU16(candidates3, srcCodes, pt3, 3, partIndexLimit3, alphaVary)
		}
	}
	if pt4 != nil {
		want := tune.partitionCandidateLimit[4]
		if want > partIndexLimit4 {
			want = partIndexLimit4
		}
		if want > len(candidates4Arr) {
			want = len(candidates4Arr)
		}
		if want > 0 && partIndexLimit4 > 0 {
			candidates4 = candidates4Arr[:want]
			candidates4Count = selectBestPartitionIndicesU16(candidates4, srcCodes, pt4, 4, partIndexLimit4, alphaVary)
		}
	}

	for _, mode := range modes {
		if mode.isDualPlane && !allowDualPlane {
			continue
		}

		weightCountPerPlane := mode.xWeights * mode.yWeights * mode.zWeights
		if weightCountPerPlane <= 0 || weightCountPerPlane > blockMaxWeights {
			continue
		}
		noDecimation := weightCountPerPlane == texelCount
		realWeightCount := weightCountPerPlane
		if mode.isDualPlane {
			realWeightCount *= 2
		}

		dec := getDecimationTable(blockX, blockY, blockZ, mode.xWeights, mode.yWeights, mode.zWeights)
		wQuantLUT := &weightQuantizeScrambledLUT[mode.weightQuant]
		uqMap := weightUnscrambleAndUnquantMap[mode.weightQuant]
		weightsUQ := weightsUQArr[:]
		sampleMap := mode.sampleTexelIndices

		belowWeightsPos := 128 - mode.weightBits
		for _, partitionCount := range partitionCounts {
			if mode.isDualPlane && partitionCount == 4 {
				continue
			}
			startBit := 17
			if partitionCount != 1 {
				startBit = 19 + partitionIndexBits
			}

			bitsAvailable := belowWeightsPos - startBit
			if mode.isDualPlane {
				bitsAvailable -= 2
			}
			if bitsAvailable <= 0 {
				continue
			}

			var pt *partitionTable
			var candidates []int
			candidateCount := 0
			partIndexLimit := 1
			switch partitionCount {
			case 1:
				// none
			case 2:
				pt = pt2
				candidates = candidates2
				candidateCount = candidates2Count
				partIndexLimit = partIndexLimit2
			case 3:
				pt = pt3
				candidates = candidates3
				candidateCount = candidates3Count
				partIndexLimit = partIndexLimit3
			case 4:
				pt = pt4
				candidates = candidates4
				candidateCount = candidates4Count
				partIndexLimit = partIndexLimit4
			default:
				continue
			}
			if partitionCount != 1 && pt == nil {
				continue
			}

			var idxListArr [1]int
			idxList := ([]int)(nil)
			if partitionCount == 1 {
				idxListArr[0] = 0
				idxList = idxListArr[:]
			} else if candidateCount > 0 && tuneOverride == nil {
				idxList = candidates[:candidateCount]
			}

			iterCount := partIndexLimit
			if idxList != nil {
				iterCount = len(idxList)
			}

			for i := 0; i < iterCount; i++ {
				partitionIndex := i
				if idxList != nil {
					partitionIndex = idxList[i]
				}

				var assign []uint8
				if partitionCount != 1 {
					assign = pt.partitionsForIndex(partitionIndex)
				}

				var minIdx [blockMaxPartitions]int
				var maxIdx [blockMaxPartitions]int
				var minL [blockMaxPartitions]float32
				var maxL [blockMaxPartitions]float32
				var minA [blockMaxPartitions]float32
				var maxA [blockMaxPartitions]float32
				var count [blockMaxPartitions]uint16

				for p := 0; p < partitionCount; p++ {
					minL[p] = float32(math.Inf(1))
					maxL[p] = float32(math.Inf(-1))
					minA[p] = float32(math.Inf(1))
					maxA[p] = float32(math.Inf(-1))
					minIdx[p] = 0
					maxIdx[p] = 0
					count[p] = 0
				}

				for t := 0; t < texelCount; t++ {
					part := 0
					if assign != nil {
						part = int(assign[t])
					}
					count[part]++

					l := texelLuma[t]
					a := texelAlpha[t]
					if l < minL[part] || (l == minL[part] && a < minA[part]) {
						minL[part] = l
						minA[part] = a
						minIdx[part] = t
					}
					if l > maxL[part] || (l == maxL[part] && a > maxA[part]) {
						maxL[part] = l
						maxA[part] = a
						maxIdx[part] = t
					}
				}

				if partitionCount != 1 {
					degenerate := false
					for p := 0; p < partitionCount; p++ {
						if count[p] == 0 {
							degenerate = true
							break
						}
					}
					if degenerate {
						continue
					}
				}

				var formatsArr [5]uint8
				formatCount := 0
				alphaConst := !alphaVary
				if profile == ProfileHDR {
					formatsArr[formatCount] = fmtHDRRGBA
					formatCount++
					if alphaConst && alphaMin == 0x7800 {
						formatsArr[formatCount] = fmtHDRRGB
						formatCount++
						formatsArr[formatCount] = fmtHDRRGBScale
						formatCount++
						formatsArr[formatCount] = fmtHDRLuminanceSmallRange
						formatCount++
						formatsArr[formatCount] = fmtHDRLuminanceLargeRange
						formatCount++
					}
				} else {
					formatsArr[formatCount] = fmtHDRRGBLDRAlpha
					formatCount++
					if alphaConst && alphaMin == 0xFFFF {
						formatsArr[formatCount] = fmtHDRRGB
						formatCount++
						formatsArr[formatCount] = fmtHDRRGBScale
						formatCount++
						formatsArr[formatCount] = fmtHDRLuminanceSmallRange
						formatCount++
						formatsArr[formatCount] = fmtHDRLuminanceLargeRange
						formatCount++
					}
				}

				for fi := 0; fi < formatCount; fi++ {
					endpointFormat := formatsArr[fi]
					endpointStride := endpointIntCount(endpointFormat)

					colorIntCount := partitionCount * endpointStride
					qLevel := quantLevelForISE(colorIntCount, bitsAvailable)
					if qLevel < int(quant6) {
						continue
					}
					colorQuant := quantMethod(qLevel)
					qi := int(colorQuant) - int(quant6)

					// Scratch endpoints. These may swap if a new best is found.
					endpointPquant := currEndpointPquantBuf[:partitionCount*endpointStride]

					formatOK := true
					for p := 0; p < partitionCount; p++ {
						e0Src := srcCodes[minIdx[p]]
						e1Src := srcCodes[maxIdx[p]]
						color0 := [4]float32{
							float32(e0Src[0]),
							float32(e0Src[1]),
							float32(e0Src[2]),
							float32(e0Src[3]),
						}
						color1 := [4]float32{
							float32(e1Src[0]),
							float32(e1Src[1]),
							float32(e1Src[2]),
							float32(e1Src[3]),
						}

						base := p * endpointStride
						switch endpointFormat {
						case fmtHDRRGBA:
							uq := quantizeHDRRGBA(color0, color1, colorQuant)
							for j := 0; j < 8; j++ {
								endpointPquant[base+j] = colorQuantizePquantLUT[qi][uq[j]]
							}
							_, _, e0, e1 := unpackColorEndpoints(profile, endpointFormat, uq[:])
							evalEp0[p][0] = int32(e0[0])
							evalEp0[p][1] = int32(e0[1])
							evalEp0[p][2] = int32(e0[2])
							evalEp0[p][3] = int32(e0[3])
							evalEpd[p][0] = int32(e1[0] - e0[0])
							evalEpd[p][1] = int32(e1[1] - e0[1])
							evalEpd[p][2] = int32(e1[2] - e0[2])
							evalEpd[p][3] = int32(e1[3] - e0[3])
						case fmtHDRRGBLDRAlpha:
							uq := quantizeHDRRGBLDRAlpha(color0, color1, colorQuant)
							for j := 0; j < 8; j++ {
								endpointPquant[base+j] = colorQuantizePquantLUT[qi][uq[j]]
							}
							_, _, e0, e1 := unpackColorEndpoints(profile, endpointFormat, uq[:])
							evalEp0[p][0] = int32(e0[0])
							evalEp0[p][1] = int32(e0[1])
							evalEp0[p][2] = int32(e0[2])
							evalEp0[p][3] = int32(e0[3])
							evalEpd[p][0] = int32(e1[0] - e0[0])
							evalEpd[p][1] = int32(e1[1] - e0[1])
							evalEpd[p][2] = int32(e1[2] - e0[2])
							evalEpd[p][3] = int32(e1[3] - e0[3])
						case fmtHDRRGBScale:
							loR := float32(e0Src[0])
							loG := float32(e0Src[1])
							loB := float32(e0Src[2])
							hiR := float32(e1Src[0])
							hiG := float32(e1Src[1])
							hiB := float32(e1Src[2])
							if hiR < loR {
								loR, hiR = hiR, loR
							}
							if hiG < loG {
								loG, hiG = hiG, loG
							}
							if hiB < loB {
								loB, hiB = hiB, loB
							}
							scale := ((hiR - loR) + (hiG - loG) + (hiB - loB)) * (1.0 / 3.0)
							uq := quantizeHDRRGBScale([4]float32{loR, loG, loB, scale}, colorQuant)
							for j := 0; j < 4; j++ {
								endpointPquant[base+j] = colorQuantizePquantLUT[qi][uq[j]]
							}
							_, _, e0, e1 := unpackColorEndpoints(profile, endpointFormat, uq[:])
							evalEp0[p][0] = int32(e0[0])
							evalEp0[p][1] = int32(e0[1])
							evalEp0[p][2] = int32(e0[2])
							evalEp0[p][3] = int32(e0[3])
							evalEpd[p][0] = int32(e1[0] - e0[0])
							evalEpd[p][1] = int32(e1[1] - e0[1])
							evalEpd[p][2] = int32(e1[2] - e0[2])
							evalEpd[p][3] = int32(e1[3] - e0[3])
						case fmtHDRLuminanceSmallRange:
							uq, ok := tryQuantizeHDRLuminanceSmallRange(color0, color1, colorQuant)
							if !ok {
								formatOK = false
								break
							}
							for j := 0; j < 2; j++ {
								endpointPquant[base+j] = colorQuantizePquantLUT[qi][uq[j]]
							}
							_, _, e0, e1 := unpackColorEndpoints(profile, endpointFormat, uq[:])
							evalEp0[p][0] = int32(e0[0])
							evalEp0[p][1] = int32(e0[1])
							evalEp0[p][2] = int32(e0[2])
							evalEp0[p][3] = int32(e0[3])
							evalEpd[p][0] = int32(e1[0] - e0[0])
							evalEpd[p][1] = int32(e1[1] - e0[1])
							evalEpd[p][2] = int32(e1[2] - e0[2])
							evalEpd[p][3] = int32(e1[3] - e0[3])
						case fmtHDRLuminanceLargeRange:
							uq := quantizeHDRLuminanceLargeRange(color0, color1, colorQuant)
							for j := 0; j < 2; j++ {
								endpointPquant[base+j] = colorQuantizePquantLUT[qi][uq[j]]
							}
							_, _, e0, e1 := unpackColorEndpoints(profile, endpointFormat, uq[:])
							evalEp0[p][0] = int32(e0[0])
							evalEp0[p][1] = int32(e0[1])
							evalEp0[p][2] = int32(e0[2])
							evalEp0[p][3] = int32(e0[3])
							evalEpd[p][0] = int32(e1[0] - e0[0])
							evalEpd[p][1] = int32(e1[1] - e0[1])
							evalEpd[p][2] = int32(e1[2] - e0[2])
							evalEpd[p][3] = int32(e1[3] - e0[3])
						case fmtHDRRGB:
							uq6 := quantizeHDRRGB(color0, color1, colorQuant)
							for j := 0; j < 6; j++ {
								endpointPquant[base+j] = colorQuantizePquantLUT[qi][uq6[j]]
							}
							_, _, e0, e1 := unpackColorEndpoints(profile, endpointFormat, uq6[:])
							evalEp0[p][0] = int32(e0[0])
							evalEp0[p][1] = int32(e0[1])
							evalEp0[p][2] = int32(e0[2])
							evalEp0[p][3] = int32(e0[3])
							evalEpd[p][0] = int32(e1[0] - e0[0])
							evalEpd[p][1] = int32(e1[1] - e0[1])
							evalEpd[p][2] = int32(e1[2] - e0[2])
							evalEpd[p][3] = int32(e1[3] - e0[3])
						default:
							formatOK = false
						}
						if !formatOK {
							break
						}
					}
					if !formatOK {
						continue
					}

					if !mode.isDualPlane {
						plane2Component := -1

						// Scratch weights. These may swap if a new best is found.
						weightPquant := currWeightPquantBuf[:realWeightCount]

						// Compute per-texel weights (0..64) in code space.
						for t := 0; t < texelCount; t++ {
							part := 0
							if assign != nil {
								part = int(assign[t])
							}
							d0 := int64(evalEpd[part][0])
							d1 := int64(evalEpd[part][1])
							d2 := int64(evalEpd[part][2])
							d3 := int64(evalEpd[part][3])
							den := d0*d0 + d1*d1 + d2*d2 + d3*d3
							if den == 0 {
								texelWeights[t] = 0
								continue
							}

							c0 := int64(srcCodes[t][0]) - int64(evalEp0[part][0])
							c1 := int64(srcCodes[t][1]) - int64(evalEp0[part][1])
							c2 := int64(srcCodes[t][2]) - int64(evalEp0[part][2])
							c3 := int64(srcCodes[t][3]) - int64(evalEp0[part][3])
							num := c0*d0 + c1*d1 + c2*d2 + c3*d3
							if num <= 0 {
								texelWeights[t] = 0
							} else if num >= den {
								texelWeights[t] = 64
							} else {
								texelWeights[t] = int((num*64 + den/2) / den)
							}
						}

						for wi := 0; wi < weightCountPerPlane; wi++ {
							p := (*wQuantLUT)[texelWeights[int(sampleMap[wi])]]
							weightPquant[wi] = p
							weightsUQ[wi] = uqMap[p]
						}
						var errv float64
						if noDecimation {
							for t := 0; t < texelCount; t++ {
								part := 0
								if assign != nil {
									part = int(assign[t])
								}
								w := int32(weightsUQ[t])

								rv := clampI32(int(evalEp0[part][0]+((evalEpd[part][0]*w+32)>>6)), 0, 0xFFFF)
								gv := clampI32(int(evalEp0[part][1]+((evalEpd[part][1]*w+32)>>6)), 0, 0xFFFF)
								bv := clampI32(int(evalEp0[part][2]+((evalEpd[part][2]*w+32)>>6)), 0, 0xFFFF)
								av := clampI32(int(evalEp0[part][3]+((evalEpd[part][3]*w+32)>>6)), 0, 0xFFFF)

								dr := float64(int32(srcCodes[t][0]) - int32(rv))
								dg := float64(int32(srcCodes[t][1]) - int32(gv))
								db := float64(int32(srcCodes[t][2]) - int32(bv))
								da := float64(int32(srcCodes[t][3]) - int32(av))
								errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da

								if errv >= bestErr {
									break
								}
							}
						} else {
							for t := 0; t < texelCount; t++ {
								part := 0
								if assign != nil {
									part = int(assign[t])
								}

								e := dec[t]
								sum := uint32(8)
								sum += uint32(weightsUQ[e.idx[0]]) * uint32(e.w[0])
								sum += uint32(weightsUQ[e.idx[1]]) * uint32(e.w[1])
								sum += uint32(weightsUQ[e.idx[2]]) * uint32(e.w[2])
								sum += uint32(weightsUQ[e.idx[3]]) * uint32(e.w[3])
								w := int32(sum >> 4)

								rv := clampI32(int(evalEp0[part][0]+((evalEpd[part][0]*w+32)>>6)), 0, 0xFFFF)
								gv := clampI32(int(evalEp0[part][1]+((evalEpd[part][1]*w+32)>>6)), 0, 0xFFFF)
								bv := clampI32(int(evalEp0[part][2]+((evalEpd[part][2]*w+32)>>6)), 0, 0xFFFF)
								av := clampI32(int(evalEp0[part][3]+((evalEpd[part][3]*w+32)>>6)), 0, 0xFFFF)

								dr := float64(int32(srcCodes[t][0]) - int32(rv))
								dg := float64(int32(srcCodes[t][1]) - int32(gv))
								db := float64(int32(srcCodes[t][2]) - int32(bv))
								da := float64(int32(srcCodes[t][3]) - int32(av))
								errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da

								if errv >= bestErr {
									break
								}
							}
						}
						if errv < bestErr {
							bestErr = errv
							bestMode = mode
							bestPartitionCount = partitionCount
							bestPartitionIndex = partitionIndex
							bestPlane2Component = plane2Component
							bestEndpointFormat = endpointFormat
							bestColorQuant = colorQuant
							bestEndpointLen = partitionCount * endpointStride
							bestWeightLen = realWeightCount
							currEndpointPquantBuf, bestEndpointPquantBuf = bestEndpointPquantBuf, currEndpointPquantBuf
							currWeightPquantBuf, bestWeightPquantBuf = bestWeightPquantBuf, currWeightPquantBuf

							if bestErr == 0 {
								block, err := buildPhysicalBlock(bestMode, blockX, blockY, blockZ, bestPartitionCount, bestPartitionIndex, bestPlane2Component, bestEndpointFormat, bestColorQuant, bestEndpointPquantBuf[:bestEndpointLen], bestWeightPquantBuf[:bestWeightLen])
								if err == nil {
									return block, nil
								}
							}
						}
					} else {
						for _, plane2Component := range dualPlaneComponents {
							// Scratch weights. These may swap if a new best is found.
							weightPquant := currWeightPquantBuf[:realWeightCount]

							// Compute per-texel weights (0..64) in code space.
							var plane1Comp [3]int
							pi := 0
							for c := 0; c < 4; c++ {
								if c == plane2Component {
									continue
								}
								plane1Comp[pi] = c
								pi++
							}
							c0 := plane1Comp[0]
							c1 := plane1Comp[1]
							c2 := plane1Comp[2]

							for t := 0; t < texelCount; t++ {
								part := 0
								if assign != nil {
									part = int(assign[t])
								}

								d0 := int64(evalEpd[part][c0])
								d1 := int64(evalEpd[part][c1])
								d2 := int64(evalEpd[part][c2])
								den := d0*d0 + d1*d1 + d2*d2
								if den == 0 {
									texelWeights[t] = 0
								} else {
									v0 := int64(srcCodes[t][c0]) - int64(evalEp0[part][c0])
									v1 := int64(srcCodes[t][c1]) - int64(evalEp0[part][c1])
									v2 := int64(srcCodes[t][c2]) - int64(evalEp0[part][c2])
									num := v0*d0 + v1*d1 + v2*d2
									if num <= 0 {
										texelWeights[t] = 0
									} else if num >= den {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int((num*64 + den/2) / den)
									}
								}

								dP2 := int64(evalEpd[part][plane2Component])
								denP2 := dP2
								signP2 := int64(1)
								if denP2 < 0 {
									denP2 = -denP2
									signP2 = -1
								}
								if denP2 == 0 {
									texelWeights2[t] = 0
								} else {
									num := (int64(srcCodes[t][plane2Component]) - int64(evalEp0[part][plane2Component])) * signP2
									if num <= 0 {
										texelWeights2[t] = 0
									} else if num >= denP2 {
										texelWeights2[t] = 64
									} else {
										texelWeights2[t] = int((num*64 + denP2/2) / denP2)
									}
								}
							}

							for wi := 0; wi < weightCountPerPlane; wi++ {
								tix := int(sampleMap[wi])
								p1 := (*wQuantLUT)[texelWeights[tix]]
								p2 := (*wQuantLUT)[texelWeights2[tix]]
								weightPquant[2*wi] = p1
								weightPquant[2*wi+1] = p2
								weightsUQ[wi] = uqMap[p1]
								weightsUQ[wi+weightsPlane2Offset] = uqMap[p2]
							}

							var errv float64
							if noDecimation {
								for t := 0; t < texelCount; t++ {
									part := 0
									if assign != nil {
										part = int(assign[t])
									}
									w1 := int32(weightsUQ[t])
									w2 := int32(weightsUQ[t+weightsPlane2Offset])

									w := w1
									if plane2Component == 0 {
										w = w2
									}
									rv := clampI32(int(evalEp0[part][0]+((evalEpd[part][0]*w+32)>>6)), 0, 0xFFFF)

									w = w1
									if plane2Component == 1 {
										w = w2
									}
									gv := clampI32(int(evalEp0[part][1]+((evalEpd[part][1]*w+32)>>6)), 0, 0xFFFF)

									w = w1
									if plane2Component == 2 {
										w = w2
									}
									bv := clampI32(int(evalEp0[part][2]+((evalEpd[part][2]*w+32)>>6)), 0, 0xFFFF)

									w = w1
									if plane2Component == 3 {
										w = w2
									}
									av := clampI32(int(evalEp0[part][3]+((evalEpd[part][3]*w+32)>>6)), 0, 0xFFFF)

									dr := float64(int32(srcCodes[t][0]) - int32(rv))
									dg := float64(int32(srcCodes[t][1]) - int32(gv))
									db := float64(int32(srcCodes[t][2]) - int32(bv))
									da := float64(int32(srcCodes[t][3]) - int32(av))
									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da

									if errv >= bestErr {
										break
									}
								}
							} else {
								for t := 0; t < texelCount; t++ {
									part := 0
									if assign != nil {
										part = int(assign[t])
									}

									e := dec[t]
									sum1 := uint32(8)
									sum2 := uint32(8)
									sum1 += uint32(weightsUQ[e.idx[0]]) * uint32(e.w[0])
									sum1 += uint32(weightsUQ[e.idx[1]]) * uint32(e.w[1])
									sum1 += uint32(weightsUQ[e.idx[2]]) * uint32(e.w[2])
									sum1 += uint32(weightsUQ[e.idx[3]]) * uint32(e.w[3])

									sum2 += uint32(weightsUQ[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
									sum2 += uint32(weightsUQ[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
									sum2 += uint32(weightsUQ[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
									sum2 += uint32(weightsUQ[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

									w1 := int32(sum1 >> 4)
									w2 := int32(sum2 >> 4)

									w := w1
									if plane2Component == 0 {
										w = w2
									}
									rv := clampI32(int(evalEp0[part][0]+((evalEpd[part][0]*w+32)>>6)), 0, 0xFFFF)

									w = w1
									if plane2Component == 1 {
										w = w2
									}
									gv := clampI32(int(evalEp0[part][1]+((evalEpd[part][1]*w+32)>>6)), 0, 0xFFFF)

									w = w1
									if plane2Component == 2 {
										w = w2
									}
									bv := clampI32(int(evalEp0[part][2]+((evalEpd[part][2]*w+32)>>6)), 0, 0xFFFF)

									w = w1
									if plane2Component == 3 {
										w = w2
									}
									av := clampI32(int(evalEp0[part][3]+((evalEpd[part][3]*w+32)>>6)), 0, 0xFFFF)

									dr := float64(int32(srcCodes[t][0]) - int32(rv))
									dg := float64(int32(srcCodes[t][1]) - int32(gv))
									db := float64(int32(srcCodes[t][2]) - int32(bv))
									da := float64(int32(srcCodes[t][3]) - int32(av))
									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da

									if errv >= bestErr {
										break
									}
								}
							}

							if errv < bestErr {
								bestErr = errv
								bestMode = mode
								bestPartitionCount = partitionCount
								bestPartitionIndex = partitionIndex
								bestPlane2Component = plane2Component
								bestEndpointFormat = endpointFormat
								bestColorQuant = colorQuant
								bestEndpointLen = partitionCount * endpointStride
								bestWeightLen = realWeightCount
								copy(bestEndpointPquantBuf[:bestEndpointLen], endpointPquant[:bestEndpointLen])
								currWeightPquantBuf, bestWeightPquantBuf = bestWeightPquantBuf, currWeightPquantBuf

								if bestErr == 0 {
									block, err := buildPhysicalBlock(bestMode, blockX, blockY, blockZ, bestPartitionCount, bestPartitionIndex, bestPlane2Component, bestEndpointFormat, bestColorQuant, bestEndpointPquantBuf[:bestEndpointLen], bestWeightPquantBuf[:bestWeightLen])
									if err == nil {
										return block, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if !math.IsInf(bestErr, 0) {
		block, err := buildPhysicalBlock(bestMode, blockX, blockY, blockZ, bestPartitionCount, bestPartitionIndex, bestPlane2Component, bestEndpointFormat, bestColorQuant, bestEndpointPquantBuf[:bestEndpointLen], bestWeightPquantBuf[:bestWeightLen])
		if err == nil {
			return block, nil
		}
	}

	// Fallback: constant average.
	var sr, sg, sb, sa float64
	for t := 0; t < texelCount; t++ {
		off := t * 4
		sr += float64(texels[off+0])
		sg += float64(texels[off+1])
		sb += float64(texels[off+2])
		sa += float64(texels[off+3])
	}
	inv := 1.0 / float64(texelCount)
	return EncodeConstBlockF16(
		float32ToHalf(float32(sr*inv)),
		float32ToHalf(float32(sg*inv)),
		float32ToHalf(float32(sb*inv)),
		float32ToHalf(float32(sa*inv)),
	), nil
}
