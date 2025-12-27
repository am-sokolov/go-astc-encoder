package astc

import "math"

func flt2intRTN(v float32) int {
	// Port of astc::flt2int_rtn(): round-to-nearest and convert to int.
	// Note: This intentionally uses (v + 0.5) truncation semantics (including for negative v),
	// matching the upstream behavior.
	return int(v + 0.5)
}

func clampF32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func absF32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func quantColorUquant(q quantMethod, value uint8) uint8 {
	if q < quant6 || q > quant256 {
		return 0
	}
	qi := int(q) - int(quant6)
	return colorQuantizeUquantLUT[qi][value]
}

func quantizeAndUnquantizeRetainTopTwoBits(q quantMethod, value uint8) uint8 {
	// Scalar port of quantize_and_unquantize_retain_top_two_bits().
	v := value
	for i := 0; i < 256; i++ {
		quantval := quantColorUquant(q, v)
		if (v & 0xC0) == (quantval & 0xC0) {
			return quantval
		}
		v--
	}
	return quantColorUquant(q, value)
}

func quantizeAndUnquantizeRetainTopFourBits(q quantMethod, value uint8) uint8 {
	// Scalar port of quantize_and_unquantize_retain_top_four_bits().
	v := value
	for i := 0; i < 256; i++ {
		quantval := quantColorUquant(q, v)
		if (v & 0xF0) == (quantval & 0xF0) {
			return quantval
		}
		v--
	}
	return quantColorUquant(q, value)
}

func quantizeHDRA(alpha0, alpha1 float32, q quantMethod) (out [2]uint8) {
	// Scalar port of quantize_hdr_alpha() in Source/astcenc_color_quantize.cpp.
	alpha0 = clampF32(alpha0, 0, 65280)
	alpha1 = clampF32(alpha1, 0, 65280)

	ialpha0 := flt2intRTN(alpha0)
	ialpha1 := flt2intRTN(alpha1)

	// Try to encode one of the delta submodes, in decreasing-precision order.
	for i := 2; i >= 0; i-- {
		val0 := (ialpha0 + (128 >> i)) >> (8 - i)
		val1 := (ialpha1 + (128 >> i)) >> (8 - i)

		v6 := (val0 & 0x7F) | ((i & 1) << 7)
		v6e := quantColorUquant(q, uint8(v6))
		v6d := int(v6e)
		if ((v6 ^ v6d) & 0x80) != 0 {
			continue
		}

		val0 = (val0 & ^0x7F) | (v6d & 0x7F)
		diffval := val1 - val0

		cutoff := 32 >> i
		mask := 2*cutoff - 1
		if diffval < -cutoff || diffval >= cutoff {
			continue
		}

		v7 := ((i & 2) << 6) | ((val0 >> 7) << (6 - i)) | (diffval & mask)
		v7e := quantColorUquant(q, uint8(v7))
		v7d := int(v7e)

		testbits := [...]int{0xE0, 0xF0, 0xF8}
		if ((v7 ^ v7d) & testbits[i]) != 0 {
			continue
		}

		out[0] = v6e
		out[1] = v7e
		return out
	}

	// Could not encode any of the delta modes; instead encode a flat value.
	val0 := (ialpha0 + 256) >> 9
	val1 := (ialpha1 + 256) >> 9
	v6 := val0 | 0x80
	v7 := val1 | 0x80
	out[0] = quantColorUquant(q, uint8(v6))
	out[1] = quantColorUquant(q, uint8(v7))
	return out
}

func quantizeHDRRGB(color0, color1 [4]float32, q quantMethod) (out [6]uint8) {
	// Scalar port of quantize_hdr_rgb() in Source/astcenc_color_quantize.cpp.
	for i := 0; i < 3; i++ {
		color0[i] = clampF32(color0[i], 0, 65535)
		color1[i] = clampF32(color1[i], 0, 65535)
	}

	color0Bak := color0
	color1Bak := color1

	majcomp := 2
	if color1[0] > color1[1] && color1[0] > color1[2] {
		majcomp = 0
	} else if color1[1] > color1[2] {
		majcomp = 1
	}

	// Swizzle components.
	switch majcomp {
	case 1:
		color0[0], color0[1] = color0[1], color0[0]
		color1[0], color1[1] = color1[1], color1[0]
	case 2:
		color0[0], color0[2] = color0[2], color0[0]
		color1[0], color1[2] = color1[2], color1[0]
	}

	aBase := clampF32(color1[0], 0, 65535)

	b0Base := aBase - color1[1]
	b1Base := aBase - color1[2]
	cBase := aBase - color0[0]
	d0Base := aBase - b0Base - cBase - color0[1]
	d1Base := aBase - b1Base - cBase - color0[2]

	modeBits := [8][4]int{
		{9, 7, 6, 7},
		{9, 8, 6, 6},
		{10, 6, 7, 7},
		{10, 7, 7, 6},
		{11, 8, 6, 5},
		{11, 6, 8, 6},
		{12, 7, 7, 5},
		{12, 6, 7, 6},
	}

	modeCutoffs := [8][4]float32{
		{16384, 8192, 8192, 8},
		{32768, 8192, 4096, 8},
		{4096, 8192, 4096, 4},
		{8192, 8192, 2048, 4},
		{8192, 2048, 512, 2},
		{2048, 8192, 1024, 2},
		{2048, 2048, 256, 1},
		{1024, 2048, 512, 1},
	}

	modeScales := [8]float32{
		1.0 / 128.0,
		1.0 / 128.0,
		1.0 / 64.0,
		1.0 / 64.0,
		1.0 / 32.0,
		1.0 / 32.0,
		1.0 / 16.0,
		1.0 / 16.0,
	}

	modeRscales := [8]float32{
		128.0,
		128.0,
		64.0,
		64.0,
		32.0,
		32.0,
		16.0,
		16.0,
	}

	for mode := 7; mode >= 0; mode-- {
		bCutoff := modeCutoffs[mode][0]
		cCutoff := modeCutoffs[mode][1]
		dCutoff := modeCutoffs[mode][2]
		if b0Base > bCutoff || b1Base > bCutoff || cBase > cCutoff || absF32(d0Base) > dCutoff || absF32(d1Base) > dCutoff {
			continue
		}

		modeScale := modeScales[mode]
		modeRscale := modeRscales[mode]

		bIntCutoff := 1 << modeBits[mode][1]
		cIntCutoff := 1 << modeBits[mode][2]
		dIntCutoff := 1 << (modeBits[mode][3] - 1)

		aInt := flt2intRTN(aBase * modeScale)
		aLow := aInt & 0xFF
		aQuant := int(quantColorUquant(q, uint8(aLow)))
		aInt = (aInt & ^0xFF) | aQuant
		aF := float32(aInt) * modeRscale

		cF := clampF32(aF-color0[0], 0, 65535)
		cInt := flt2intRTN(cF * modeScale)
		if cInt >= cIntCutoff {
			continue
		}

		cLow := cInt & 0x3F
		cLow |= (mode & 1) << 7
		cLow |= (aInt & 0x100) >> 2

		cQuant := quantizeAndUnquantizeRetainTopTwoBits(q, uint8(cLow))
		cInt = (cInt & ^0x3F) | int(cQuant&0x3F)
		cF = float32(cInt) * modeRscale

		b0F := clampF32(aF-color1[1], 0, 65535)
		b1F := clampF32(aF-color1[2], 0, 65535)
		b0Int := flt2intRTN(b0F * modeScale)
		b1Int := flt2intRTN(b1F * modeScale)
		if b0Int >= bIntCutoff || b1Int >= bIntCutoff {
			continue
		}

		b0Low := b0Int & 0x3F
		b1Low := b1Int & 0x3F

		bit0 := 0
		bit1 := 0
		switch mode {
		case 0, 1, 3, 4, 6:
			bit0 = (b0Int >> 6) & 1
		case 2, 5, 7:
			bit0 = (aInt >> 9) & 1
		}
		switch mode {
		case 0, 1, 3, 4, 6:
			bit1 = (b1Int >> 6) & 1
		case 2:
			bit1 = (cInt >> 6) & 1
		case 5, 7:
			bit1 = (aInt >> 10) & 1
		}

		b0Low |= bit0 << 6
		b1Low |= bit1 << 6
		b0Low |= ((mode >> 1) & 1) << 7
		b1Low |= ((mode >> 2) & 1) << 7

		b0Quant := quantizeAndUnquantizeRetainTopTwoBits(q, uint8(b0Low))
		b1Quant := quantizeAndUnquantizeRetainTopTwoBits(q, uint8(b1Low))

		b0Int = (b0Int & ^0x3F) | int(b0Quant&0x3F)
		b1Int = (b1Int & ^0x3F) | int(b1Quant&0x3F)
		b0F = float32(b0Int) * modeRscale
		b1F = float32(b1Int) * modeRscale

		d0F := clampF32(aF-b0F-cF-color0[1], -65535, 65535)
		d1F := clampF32(aF-b1F-cF-color0[2], -65535, 65535)

		d0Int := flt2intRTN(d0F * modeScale)
		d1Int := flt2intRTN(d1F * modeScale)
		if absInt(d0Int) >= dIntCutoff || absInt(d1Int) >= dIntCutoff {
			continue
		}

		d0Low := d0Int & 0x1F
		d1Low := d1Int & 0x1F

		bit2 := 0
		bit3 := 0
		switch mode {
		case 0, 2:
			bit2 = (d0Int >> 6) & 1
		case 1, 4:
			bit2 = (b0Int >> 7) & 1
		case 3:
			bit2 = (aInt >> 9) & 1
		case 5:
			bit2 = (cInt >> 7) & 1
		case 6, 7:
			bit2 = (aInt >> 11) & 1
		}
		switch mode {
		case 0, 2:
			bit3 = (d1Int >> 6) & 1
		case 1, 4:
			bit3 = (b1Int >> 7) & 1
		case 3, 5, 6, 7:
			bit3 = (cInt >> 6) & 1
		}

		bit4 := 0
		bit5 := 0
		switch mode {
		case 4, 6:
			bit4 = (aInt >> 9) & 1
			bit5 = (aInt >> 10) & 1
		default:
			bit4 = (d0Int >> 5) & 1
			bit5 = (d1Int >> 5) & 1
		}

		d0Low |= bit2 << 6
		d1Low |= bit3 << 6
		d0Low |= bit4 << 5
		d1Low |= bit5 << 5

		d0Low |= (majcomp & 1) << 7
		d1Low |= ((majcomp >> 1) & 1) << 7

		d0Quant := quantizeAndUnquantizeRetainTopFourBits(q, uint8(d0Low))
		d1Quant := quantizeAndUnquantizeRetainTopFourBits(q, uint8(d1Low))

		out[0] = uint8(aQuant)
		out[1] = cQuant
		out[2] = b0Quant
		out[3] = b1Quant
		out[4] = d0Quant
		out[5] = d1Quant
		return out
	}

	// Fallback encoding.
	vals := [6]float32{
		color0Bak[0], color1Bak[0],
		color0Bak[1], color1Bak[1],
		color0Bak[2], color1Bak[2],
	}

	for i := 0; i < 6; i++ {
		vals[i] = clampF32(vals[i], 0, 65020)
	}

	for i := 0; i < 4; i++ {
		idx := flt2intRTN(vals[i] / 256.0)
		out[i] = quantColorUquant(q, uint8(idx))
	}

	for i := 4; i < 6; i++ {
		idx := flt2intRTN(vals[i]/512.0) + 128
		out[i] = quantizeAndUnquantizeRetainTopTwoBits(q, uint8(idx))
	}

	return out
}

func quantizeHDRRGBA(color0, color1 [4]float32, q quantMethod) (out [8]uint8) {
	rgb := quantizeHDRRGB(color0, color1, q)
	copy(out[0:6], rgb[:])
	alpha := quantizeHDRA(color0[3], color1[3], q)
	out[6] = alpha[0]
	out[7] = alpha[1]
	return out
}

func quantizeHDRRGBLDRAlpha(color0, color1 [4]float32, q quantMethod) (out [8]uint8) {
	// Port of quantize_hdr_rgb_ldr_alpha() in Source/astcenc_color_quantize.cpp.
	a0 := clampF32(color0[3]/257.0, 0, 255)
	a1 := clampF32(color1[3]/257.0, 0, 255)
	out[6] = quantColorUquant(q, uint8(clampInt(flt2intRTN(a0), 0, 255)))
	out[7] = quantColorUquant(q, uint8(clampInt(flt2intRTN(a1), 0, 255)))
	rgb := quantizeHDRRGB(color0, color1, q)
	copy(out[0:6], rgb[:])
	return out
}

func quantizeHDRRGBScale(rgbo [4]float32, q quantMethod) (out [4]uint8) {
	// Scalar port of quantize_hdr_rgbo() in Source/astcenc_color_quantize.cpp.
	//
	// The input is RGB + offset (O): the high endpoint is (R+O, G+O, B+O) and the low endpoint is
	// reconstructed from the encoded scale during decode.
	rgbo[0] += rgbo[3]
	rgbo[1] += rgbo[3]
	rgbo[2] += rgbo[3]

	for i := 0; i < 4; i++ {
		rgbo[i] = clampF32(rgbo[i], 0, 65535)
	}

	rgboBak := rgbo

	majcomp := 2
	if rgbo[0] > rgbo[1] && rgbo[0] > rgbo[2] {
		majcomp = 0
	} else if rgbo[1] > rgbo[2] {
		majcomp = 1
	}

	switch majcomp {
	case 1:
		rgbo[0], rgbo[1] = rgbo[1], rgbo[0]
	case 2:
		rgbo[0], rgbo[2] = rgbo[2], rgbo[0]
	}

	modeBits := [5][3]int{
		{11, 5, 7},
		{11, 6, 5},
		{10, 5, 8},
		{9, 6, 7},
		{8, 7, 6},
	}

	modeCutoffs := [5][2]float32{
		{1024, 4096},
		{2048, 1024},
		{2048, 16384},
		{8192, 16384},
		{32768, 16384},
	}

	modeRscales := [5]float32{
		32.0,
		32.0,
		64.0,
		128.0,
		256.0,
	}

	modeScales := [5]float32{
		1.0 / 32.0,
		1.0 / 32.0,
		1.0 / 64.0,
		1.0 / 128.0,
		1.0 / 256.0,
	}

	rBase := rgbo[0]
	gBase := rgbo[0] - rgbo[1]
	bBase := rgbo[0] - rgbo[2]
	sBase := rgbo[3]

	for mode := 0; mode < 5; mode++ {
		if gBase > modeCutoffs[mode][0] || bBase > modeCutoffs[mode][0] || sBase > modeCutoffs[mode][1] {
			continue
		}

		modeEnc := 0
		if mode < 4 {
			modeEnc = mode | (majcomp << 2)
		} else {
			modeEnc = majcomp | 0xC
		}

		modeScale := modeScales[mode]
		modeRscale := modeRscales[mode]

		gbIntCutoff := 1 << modeBits[mode][1]
		sIntCutoff := 1 << modeBits[mode][2]

		rInt := flt2intRTN(rBase * modeScale)
		rLow := rInt & 0x3F
		rLow |= (modeEnc & 3) << 6

		rQuant := quantizeAndUnquantizeRetainTopTwoBits(q, uint8(rLow))
		rInt = (rInt & ^0x3F) | int(rQuant&0x3F)
		rF := float32(rInt) * modeRscale

		gF := clampF32(rF-rgbo[1], 0, 65535)
		bF := clampF32(rF-rgbo[2], 0, 65535)
		gInt := flt2intRTN(gF * modeScale)
		bInt := flt2intRTN(bF * modeScale)
		if gInt >= gbIntCutoff || bInt >= gbIntCutoff {
			continue
		}

		gLow := gInt & 0x1F
		bLow := bInt & 0x1F

		bit0, bit1, bit2, bit3 := 0, 0, 0, 0

		switch mode {
		case 0, 2:
			bit0 = (rInt >> 9) & 1
		case 1, 3:
			bit0 = (rInt >> 8) & 1
		case 4:
			bit0 = (gInt >> 6) & 1
		}

		switch mode {
		case 0, 1, 2, 3:
			bit2 = (rInt >> 7) & 1
		case 4:
			bit2 = (bInt >> 6) & 1
		}

		switch mode {
		case 0, 2:
			bit1 = (rInt >> 8) & 1
		case 1, 3, 4:
			bit1 = (gInt >> 5) & 1
		}

		switch mode {
		case 0:
			bit3 = (rInt >> 10) & 1
		case 2:
			bit3 = (rInt >> 6) & 1
		case 1, 3, 4:
			bit3 = (bInt >> 5) & 1
		}

		gLow |= (modeEnc & 0x4) << 5
		bLow |= (modeEnc & 0x8) << 4
		gLow |= bit0 << 6
		gLow |= bit1 << 5
		bLow |= bit2 << 6
		bLow |= bit3 << 5

		gQuant := quantizeAndUnquantizeRetainTopFourBits(q, uint8(gLow))
		bQuant := quantizeAndUnquantizeRetainTopFourBits(q, uint8(bLow))
		gInt = (gInt & ^0x1F) | int(gQuant&0x1F)
		bInt = (bInt & ^0x1F) | int(bQuant&0x1F)

		gF = float32(gInt) * modeRscale
		bF = float32(bInt) * modeRscale

		rgbErrSum := (rF - rgbo[0]) + (rF - gF - rgbo[1]) + (rF - bF - rgbo[2])
		sF := sBase + rgbErrSum*(1.0/3.0)
		sF = clampF32(sF, 0, 1e9)

		sInt := flt2intRTN(sF * modeScale)
		if sInt >= sIntCutoff {
			continue
		}

		sLow := sInt & 0x1F
		bit4, bit5, bit6 := 0, 0, 0

		if mode == 1 {
			bit6 = (rInt >> 9) & 1
		} else {
			bit6 = (sInt >> 5) & 1
		}

		if mode == 4 {
			bit5 = (rInt >> 7) & 1
		} else if mode == 1 {
			bit5 = (rInt >> 10) & 1
		} else {
			bit5 = (sInt >> 6) & 1
		}

		if mode == 2 {
			bit4 = (sInt >> 7) & 1
		} else {
			bit4 = (rInt >> 6) & 1
		}

		sLow |= bit6 << 5
		sLow |= bit5 << 6
		sLow |= bit4 << 7

		sQuant := quantizeAndUnquantizeRetainTopFourBits(q, uint8(sLow))

		out[0] = rQuant
		out[1] = gQuant
		out[2] = bQuant
		out[3] = sQuant
		return out
	}

	// Failed to encode any of the modes above? In that case encode using mode #5.
	vals := [4]float32{
		rgboBak[0],
		rgboBak[1],
		rgboBak[2],
		rgboBak[3],
	}
	var ivals [4]int
	var cvals [3]float32

	for i := 0; i < 3; i++ {
		vals[i] = clampF32(vals[i], 0, 65020)
		ivals[i] = flt2intRTN(vals[i] * (1.0 / 512.0))
		cvals[i] = float32(ivals[i]) * 512.0
	}

	rgbErrSum := (cvals[0] - vals[0]) + (cvals[1] - vals[1]) + (cvals[2] - vals[2])
	vals[3] += rgbErrSum * (1.0 / 3.0)

	vals[3] = clampF32(vals[3], 0, 65020)
	ivals[3] = flt2intRTN(vals[3] * (1.0 / 512.0))

	encvals := [4]int{}
	encvals[0] = (ivals[0] & 0x3F) | 0xC0
	encvals[1] = (ivals[1] & 0x7F) | 0x80
	encvals[2] = (ivals[2] & 0x7F) | 0x80
	encvals[3] = (ivals[3] & 0x7F) | ((ivals[0] & 0x40) << 1)

	for i := 0; i < 4; i++ {
		out[i] = quantizeAndUnquantizeRetainTopFourBits(q, uint8(encvals[i]))
	}

	return out
}

func quantizeHDRLuminanceLargeRange(color0, color1 [4]float32, q quantMethod) (out [2]uint8) {
	// Scalar port of quantize_hdr_luminance_large_range() in Source/astcenc_color_quantize.cpp.
	lum0 := (color0[0] + color0[1] + color0[2]) * (1.0 / 3.0)
	lum1 := (color1[0] + color1[1] + color1[2]) * (1.0 / 3.0)
	if lum1 < lum0 {
		avg := (lum0 + lum1) * 0.5
		lum0 = avg
		lum1 = avg
	}

	ilum1 := flt2intRTN(lum1)
	ilum0 := flt2intRTN(lum0)

	upperV0 := (ilum0 + 128) >> 8
	upperV1 := (ilum1 + 128) >> 8
	upperV0 = clampInt(upperV0, 0, 255)
	upperV1 = clampInt(upperV1, 0, 255)

	lowerV0 := (ilum1 + 256) >> 8
	lowerV1 := ilum0 >> 8
	lowerV0 = clampInt(lowerV0, 0, 255)
	lowerV1 = clampInt(lowerV1, 0, 255)

	upper0Dec := upperV0 << 8
	upper1Dec := upperV1 << 8
	lower0Dec := (lowerV1 << 8) + 128
	lower1Dec := (lowerV0 << 8) - 128

	upper0Diff := upper0Dec - ilum0
	upper1Diff := upper1Dec - ilum1
	lower0Diff := lower0Dec - ilum0
	lower1Diff := lower1Dec - ilum1

	upperErr := upper0Diff*upper0Diff + upper1Diff*upper1Diff
	lowerErr := lower0Diff*lower0Diff + lower1Diff*lower1Diff

	v0, v1 := upperV0, upperV1
	if lowerErr <= upperErr {
		v0, v1 = lowerV0, lowerV1
	}

	out[0] = quantColorUquant(q, uint8(v0))
	out[1] = quantColorUquant(q, uint8(v1))
	return out
}

func tryQuantizeHDRLuminanceSmallRange(color0, color1 [4]float32, q quantMethod) (out [2]uint8, ok bool) {
	// Scalar port of try_quantize_hdr_luminance_small_range() in Source/astcenc_color_quantize.cpp.
	lum0 := (color0[0] + color0[1] + color0[2]) * (1.0 / 3.0)
	lum1 := (color1[0] + color1[1] + color1[2]) * (1.0 / 3.0)
	if lum1 < lum0 {
		avg := (lum0 + lum1) * 0.5
		lum0 = avg
		lum1 = avg
	}

	ilum1 := flt2intRTN(lum1)
	ilum0 := flt2intRTN(lum0)

	// Difference of more than a factor-of-2 results in immediate failure.
	if ilum1-ilum0 > 2048 {
		return out, false
	}

	// High-precision submode.
	lowval := (ilum0 + 16) >> 5
	highval := (ilum1 + 16) >> 5

	lowval = clampInt(lowval, 0, 2047)
	highval = clampInt(highval, 0, 2047)

	v0 := lowval & 0x7F
	v0e := quantColorUquant(q, uint8(v0))
	v0d := int(v0e)
	if v0d < 0x80 {
		lowval = (lowval & ^0x7F) | v0d
		diffval := highval - lowval
		if diffval >= 0 && diffval <= 15 {
			v1 := ((lowval >> 3) & 0xF0) | diffval
			v1e := quantColorUquant(q, uint8(v1))
			v1d := int(v1e)
			if (v1d & 0xF0) == (v1 & 0xF0) {
				out[0] = v0e
				out[1] = v1e
				return out, true
			}
		}
	}

	// Low-precision submode.
	lowval = (ilum0 + 32) >> 6
	highval = (ilum1 + 32) >> 6

	lowval = clampInt(lowval, 0, 1023)
	highval = clampInt(highval, 0, 1023)

	v0 = (lowval & 0x7F) | 0x80
	v0e = quantColorUquant(q, uint8(v0))
	v0d = int(v0e)
	if (v0d & 0x80) == 0 {
		return out, false
	}

	lowval = (lowval & ^0x7F) | (v0d & 0x7F)
	diffval := highval - lowval
	if diffval < 0 || diffval > 31 {
		return out, false
	}

	v1 := ((lowval >> 2) & 0xE0) | diffval
	v1e := quantColorUquant(q, uint8(v1))
	v1d := int(v1e)
	if (v1d & 0xE0) != (v1 & 0xE0) {
		return out, false
	}

	out[0] = v0e
	out[1] = v1e
	return out, true
}

func hdrTexelToLNS(v float32) uint16 {
	// Scalar port of float_to_lns() in Source/astcenc_vecmathlib.h.
	//
	// This returns a 16-bit LNS code value (0..65535) suitable for HDR profiles.
	if !(v > (1.0 / 67108864.0)) { // Underflow/NaN/negative
		return 0
	}
	if v >= 65536.0 {
		return 0xFFFF
	}

	a := v
	mant, exp := math.Frexp(float64(a))
	// frexp returns a = mant * 2^exp, with mant in [0.5, 1).
	// Upstream uses a custom frexp() returning exp unbias by 126, which matches exp here.

	// If input is smaller than 2^-14, multiply by 2^25 and don't bias.
	if exp < -13 {
		a = float32(a * 33554432.0)
		exp = 0
	} else {
		a = float32((mant - 0.5) * 4096.0)
		exp = exp + 14
	}

	if a < 384.0 {
		a = a * (4.0 / 3.0)
	} else if a <= 1408.0 {
		a = a + 128.0
	} else {
		a = (a + 512.0) * (4.0 / 5.0)
	}

	a = a + float32(exp)*2048.0 + 1.0
	if a <= 0 {
		return 0
	}
	if a >= 65535.0 {
		return 0xFFFF
	}
	return uint16(flt2intRTN(a))
}
