package astc

// Endpoint formats. Values are specified by ASTC and must not be reordered.
const (
	fmtLuminance              = 0
	fmtLuminanceDelta         = 1
	fmtHDRLuminanceLargeRange = 2
	fmtHDRLuminanceSmallRange = 3
	fmtLuminanceAlpha         = 4
	fmtLuminanceAlphaDelta    = 5
	fmtRGBScale               = 6
	fmtHDRRGBScale            = 7
	fmtRGB                    = 8
	fmtRGBDelta               = 9
	fmtRGBScaleAlpha          = 10
	fmtHDRRGB                 = 11
	fmtRGBA                   = 12
	fmtRGBADelta              = 13
	fmtHDRRGBLDRAlpha         = 14
	fmtHDRRGBA                = 15
)

type int4 [4]int

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func haddRGB(v int4) int { return v[0] + v[1] + v[2] }

func uncontractColor(input int4) int4 {
	blue := input[2]
	input[0] = (input[0] + blue) >> 1
	input[1] = (input[1] + blue) >> 1
	return input
}

func bitTransferSigned(input0, input1 int4) (int4, int4) {
	// Ported from bit_transfer_signed() in Source/astcenc_vecmathlib_common_4.h.
	for i := 0; i < 4; i++ {
		input1[i] = (input1[i] >> 1) | (input0[i] & 0x80)
		input0[i] = (input0[i] >> 1) & 0x3F
		if (input0[i] & 0x20) != 0 {
			input0[i] -= 0x40
		}
	}
	return input0, input1
}

func rgbaDeltaUnpack(input0, input1 int4) (output0, output1 int4) {
	// Ported from rgba_delta_unpack() in Source/astcenc_color_unquantize.cpp.
	input1, input0 = bitTransferSigned(input1, input0)

	rgbSum := haddRGB(input1)
	for i := 0; i < 4; i++ {
		input1[i] = input1[i] + input0[i]
	}

	if rgbSum < 0 {
		input0 = uncontractColor(input0)
		input1 = uncontractColor(input1)
		input0, input1 = input1, input0
	}

	for i := 0; i < 4; i++ {
		output0[i] = clampInt(input0[i], 0, 255)
		output1[i] = clampInt(input1[i], 0, 255)
	}
	return output0, output1
}

func rgbDeltaUnpack(input0, input1 int4) (output0, output1 int4) {
	output0, output1 = rgbaDeltaUnpack(input0, input1)
	output0[3] = 255
	output1[3] = 255
	return output0, output1
}

func rgbaUnpack(input0, input1 int4) (output0, output1 int4) {
	// Ported from rgba_unpack() in Source/astcenc_color_unquantize.cpp.
	if haddRGB(input0) > haddRGB(input1) {
		input0 = uncontractColor(input0)
		input1 = uncontractColor(input1)
		input0, input1 = input1, input0
	}
	return input0, input1
}

func rgbUnpack(input0, input1 int4) (output0, output1 int4) {
	output0, output1 = rgbaUnpack(input0, input1)
	output0[3] = 255
	output1[3] = 255
	return output0, output1
}

func rgbScaleAlphaUnpack(input0 int4, alpha1 uint8, scale uint8) (output0, output1 int4) {
	// Ported from rgb_scale_alpha_unpack() in Source/astcenc_color_unquantize.cpp.
	output1 = input0
	output1[3] = int(alpha1)

	for i := 0; i < 4; i++ {
		output0[i] = (input0[i] * int(scale)) >> 8
	}
	output0[3] = input0[3]
	return output0, output1
}

func rgbScaleUnpack(input0 int4, scale int) (output0, output1 int4) {
	// Ported from rgb_scale_unpack() in Source/astcenc_color_unquantize.cpp.
	output1 = input0
	output1[3] = 255

	for i := 0; i < 4; i++ {
		output0[i] = (input0[i] * scale) >> 8
	}
	output0[3] = 255
	return output0, output1
}

func luminanceUnpack(input []uint8) (output0, output1 int4) {
	lum0 := int(input[0])
	lum1 := int(input[1])
	output0 = int4{lum0, lum0, lum0, 255}
	output1 = int4{lum1, lum1, lum1, 255}
	return output0, output1
}

func luminanceDeltaUnpack(input []uint8) (output0, output1 int4) {
	v0 := int(input[0])
	v1 := int(input[1])
	l0 := (v0 >> 2) | (v1 & 0xC0)
	l1 := l0 + (v1 & 0x3F)
	if l1 > 255 {
		l1 = 255
	}
	output0 = int4{l0, l0, l0, 255}
	output1 = int4{l1, l1, l1, 255}
	return output0, output1
}

func luminanceAlphaUnpack(input []uint8) (output0, output1 int4) {
	lum0 := int(input[0])
	lum1 := int(input[1])
	alpha0 := int(input[2])
	alpha1 := int(input[3])
	output0 = int4{lum0, lum0, lum0, alpha0}
	output1 = int4{lum1, lum1, lum1, alpha1}
	return output0, output1
}

func luminanceAlphaDeltaUnpack(input []uint8) (output0, output1 int4) {
	lum0 := int(input[0])
	lum1 := int(input[1])
	alpha0 := int(input[2])
	alpha1 := int(input[3])

	lum0 |= (lum1 & 0x80) << 1
	alpha0 |= (alpha1 & 0x80) << 1
	lum1 &= 0x7F
	alpha1 &= 0x7F

	if (lum1 & 0x40) != 0 {
		lum1 -= 0x80
	}
	if (alpha1 & 0x40) != 0 {
		alpha1 -= 0x80
	}

	lum0 >>= 1
	lum1 >>= 1
	alpha0 >>= 1
	alpha1 >>= 1

	lum1 += lum0
	alpha1 += alpha0

	lum1 = clampInt(lum1, 0, 255)
	alpha1 = clampInt(alpha1, 0, 255)

	output0 = int4{lum0, lum0, lum0, alpha0}
	output1 = int4{lum1, lum1, lum1, alpha1}
	return output0, output1
}

func safeSignedLsh32(val int32, shift int) int32 {
	return int32(uint32(val) << uint(shift))
}

func hdrRgboUnpack(input []uint8) (output0, output1 int4) {
	// Ported from hdr_rgbo_unpack() in Source/astcenc_color_unquantize.cpp.
	v0 := int(input[0])
	v1 := int(input[1])
	v2 := int(input[2])
	v3 := int(input[3])

	modeval := ((v0 & 0xC0) >> 6) | (((v1 & 0x80) >> 7) << 2) | (((v2 & 0x80) >> 7) << 3)

	majcomp := 0
	mode := 0
	if (modeval & 0xC) != 0xC {
		majcomp = modeval >> 2
		mode = modeval & 3
	} else if modeval != 0xF {
		majcomp = modeval & 3
		mode = 4
	} else {
		majcomp = 0
		mode = 5
	}

	red := v0 & 0x3F
	green := v1 & 0x1F
	blue := v2 & 0x1F
	scale := v3 & 0x1F

	bit0 := (v1 >> 6) & 1
	bit1 := (v1 >> 5) & 1
	bit2 := (v2 >> 6) & 1
	bit3 := (v2 >> 5) & 1
	bit4 := (v3 >> 7) & 1
	bit5 := (v3 >> 6) & 1
	bit6 := (v3 >> 5) & 1

	ohcomp := 1 << mode

	if (ohcomp & 0x30) != 0 {
		green |= bit0 << 6
	}
	if (ohcomp & 0x3A) != 0 {
		green |= bit1 << 5
	}
	if (ohcomp & 0x30) != 0 {
		blue |= bit2 << 6
	}
	if (ohcomp & 0x3A) != 0 {
		blue |= bit3 << 5
	}
	if (ohcomp & 0x3D) != 0 {
		scale |= bit6 << 5
	}
	if (ohcomp & 0x2D) != 0 {
		scale |= bit5 << 6
	}
	if (ohcomp & 0x04) != 0 {
		scale |= bit4 << 7
	}
	if (ohcomp & 0x3B) != 0 {
		red |= bit4 << 6
	}
	if (ohcomp & 0x04) != 0 {
		red |= bit3 << 6
	}
	if (ohcomp & 0x10) != 0 {
		red |= bit5 << 7
	}
	if (ohcomp & 0x0F) != 0 {
		red |= bit2 << 7
	}
	if (ohcomp & 0x05) != 0 {
		red |= bit1 << 8
	}
	if (ohcomp & 0x0A) != 0 {
		red |= bit0 << 8
	}
	if (ohcomp & 0x05) != 0 {
		red |= bit0 << 9
	}
	if (ohcomp & 0x02) != 0 {
		red |= bit6 << 9
	}
	if (ohcomp & 0x01) != 0 {
		red |= bit3 << 10
	}
	if (ohcomp & 0x02) != 0 {
		red |= bit5 << 10
	}

	// expand to 12 bits.
	shamts := [...]int{1, 1, 2, 3, 4, 5}
	shamt := shamts[mode]
	red <<= shamt
	green <<= shamt
	blue <<= shamt
	scale <<= shamt

	// on modes 0 to 4, the values stored for "green" and "blue" are differentials
	if mode != 5 {
		green = red - green
		blue = red - blue
	}

	// switch around components.
	switch majcomp {
	case 1:
		red, green = green, red
	case 2:
		red, blue = blue, red
	}

	red0 := red - scale
	green0 := green - scale
	blue0 := blue - scale

	red = maxInt(red, 0)
	green = maxInt(green, 0)
	blue = maxInt(blue, 0)

	red0 = maxInt(red0, 0)
	green0 = maxInt(green0, 0)
	blue0 = maxInt(blue0, 0)

	output0 = int4{red0 << 4, green0 << 4, blue0 << 4, 0x7800}
	output1 = int4{red << 4, green << 4, blue << 4, 0x7800}
	return output0, output1
}

func hdrRgbUnpack(input []uint8) (output0, output1 int4) {
	// Ported from hdr_rgb_unpack() in Source/astcenc_color_unquantize.cpp.
	v0 := int(input[0])
	v1 := int(input[1])
	v2 := int(input[2])
	v3 := int(input[3])
	v4 := int(input[4])
	v5 := int(input[5])

	modeval := ((v1 & 0x80) >> 7) | (((v2 & 0x80) >> 7) << 1) | (((v3 & 0x80) >> 7) << 2)
	majcomp := ((v4 & 0x80) >> 7) | (((v5 & 0x80) >> 7) << 1)

	if majcomp == 3 {
		output0 = int4{v0 << 8, v2 << 8, (v4 & 0x7F) << 9, 0x7800}
		output1 = int4{v1 << 8, v3 << 8, (v5 & 0x7F) << 9, 0x7800}
		return output0, output1
	}

	a := v0 | ((v1 & 0x40) << 2)
	b0 := v2 & 0x3F
	b1 := v3 & 0x3F
	c := v1 & 0x3F
	d0 := v4 & 0x7F
	d1 := v5 & 0x7F

	dbitsTab := [...]int{7, 6, 7, 6, 5, 6, 5, 6}
	dbits := dbitsTab[modeval]

	bit0 := (v2 >> 6) & 1
	bit1 := (v3 >> 6) & 1
	bit2 := (v4 >> 6) & 1
	bit3 := (v5 >> 6) & 1
	bit4 := (v4 >> 5) & 1
	bit5 := (v5 >> 5) & 1

	ohmod := 1 << modeval
	if (ohmod & 0xA4) != 0 {
		a |= bit0 << 9
	}
	if (ohmod & 0x8) != 0 {
		a |= bit2 << 9
	}
	if (ohmod & 0x50) != 0 {
		a |= bit4 << 9
	}
	if (ohmod & 0x50) != 0 {
		a |= bit5 << 10
	}
	if (ohmod & 0xA0) != 0 {
		a |= bit1 << 10
	}
	if (ohmod & 0xC0) != 0 {
		a |= bit2 << 11
	}
	if (ohmod & 0x4) != 0 {
		c |= bit1 << 6
	}
	if (ohmod & 0xE8) != 0 {
		c |= bit3 << 6
	}
	if (ohmod & 0x20) != 0 {
		c |= bit2 << 7
	}
	if (ohmod & 0x5B) != 0 {
		b0 |= bit0 << 6
		b1 |= bit1 << 6
	}
	if (ohmod & 0x12) != 0 {
		b0 |= bit2 << 7
		b1 |= bit3 << 7
	}
	if (ohmod & 0xAF) != 0 {
		d0 |= bit4 << 5
		d1 |= bit5 << 5
	}
	if (ohmod & 0x5) != 0 {
		d0 |= bit2 << 6
		d1 |= bit3 << 6
	}

	// sign-extend d0 and d1
	d0x := int32(d0)
	d1x := int32(d1)
	sxShamt := 32 - dbits
	d0x = safeSignedLsh32(d0x, sxShamt)
	d0x >>= uint(sxShamt)
	d1x = safeSignedLsh32(d1x, sxShamt)
	d1x >>= uint(sxShamt)
	d0 = int(d0x)
	d1 = int(d1x)

	valShamt := (modeval >> 1) ^ 3
	a = int(safeSignedLsh32(int32(a), valShamt))
	b0 = int(safeSignedLsh32(int32(b0), valShamt))
	b1 = int(safeSignedLsh32(int32(b1), valShamt))
	c = int(safeSignedLsh32(int32(c), valShamt))
	d0 = int(safeSignedLsh32(int32(d0), valShamt))
	d1 = int(safeSignedLsh32(int32(d1), valShamt))

	red1 := a
	green1 := a - b0
	blue1 := a - b1
	red0 := a - c
	green0 := a - b0 - c - d0
	blue0 := a - b1 - c - d1

	red0 = clampInt(red0, 0, 4095)
	green0 = clampInt(green0, 0, 4095)
	blue0 = clampInt(blue0, 0, 4095)
	red1 = clampInt(red1, 0, 4095)
	green1 = clampInt(green1, 0, 4095)
	blue1 = clampInt(blue1, 0, 4095)

	switch majcomp {
	case 1:
		red0, green0 = green0, red0
		red1, green1 = green1, red1
	case 2:
		red0, blue0 = blue0, red0
		red1, blue1 = blue1, red1
	}

	output0 = int4{red0 << 4, green0 << 4, blue0 << 4, 0x7800}
	output1 = int4{red1 << 4, green1 << 4, blue1 << 4, 0x7800}
	return output0, output1
}

func hdrRgbLdrAlphaUnpack(input []uint8) (output0, output1 int4) {
	output0, output1 = hdrRgbUnpack(input[:6])
	output0[3] = int(input[6])
	output1[3] = int(input[7])
	return output0, output1
}

func hdrLuminanceSmallRangeUnpack(input []uint8) (output0, output1 int4) {
	v0 := int(input[0])
	v1 := int(input[1])

	y0, y1 := 0, 0
	if (v0 & 0x80) != 0 {
		y0 = ((v1 & 0xE0) << 4) | ((v0 & 0x7F) << 2)
		y1 = (v1 & 0x1F) << 2
	} else {
		y0 = ((v1 & 0xF0) << 4) | ((v0 & 0x7F) << 1)
		y1 = (v1 & 0xF) << 1
	}

	y1 += y0
	if y1 > 0xFFF {
		y1 = 0xFFF
	}

	output0 = int4{y0 << 4, y0 << 4, y0 << 4, 0x7800}
	output1 = int4{y1 << 4, y1 << 4, y1 << 4, 0x7800}
	return output0, output1
}

func hdrLuminanceLargeRangeUnpack(input []uint8) (output0, output1 int4) {
	v0 := int(input[0])
	v1 := int(input[1])

	y0, y1 := 0, 0
	if v1 >= v0 {
		y0 = v0 << 4
		y1 = v1 << 4
	} else {
		y0 = (v1 << 4) + 8
		y1 = (v0 << 4) - 8
	}

	output0 = int4{y0 << 4, y0 << 4, y0 << 4, 0x7800}
	output1 = int4{y1 << 4, y1 << 4, y1 << 4, 0x7800}
	return output0, output1
}

func hdrAlphaUnpack(input []uint8) (output0, output1 int) {
	v6 := int(input[0])
	v7 := int(input[1])

	selector := ((v6 >> 7) & 1) | ((v7 >> 6) & 2)
	v6 &= 0x7F
	v7 &= 0x7F
	if selector == 3 {
		output0 = v6 << 5
		output1 = v7 << 5
	} else {
		v6 |= (v7 << (selector + 1)) & 0x780
		v7 &= (0x3F >> selector)
		v7 ^= 32 >> selector
		v7 -= 32 >> selector
		v6 <<= (4 - selector)
		v7 <<= (4 - selector)
		v7 += v6

		if v7 < 0 {
			v7 = 0
		} else if v7 > 0xFFF {
			v7 = 0xFFF
		}

		output0 = v6
		output1 = v7
	}

	output0 <<= 4
	output1 <<= 4
	return output0, output1
}

func hdrRgbHdrAlphaUnpack(input []uint8) (output0, output1 int4) {
	output0, output1 = hdrRgbUnpack(input[:6])
	alpha0, alpha1 := hdrAlphaUnpack(input[6:8])
	output0[3] = alpha0
	output1[3] = alpha1
	return output0, output1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// unpackColorEndpoints unpacks and expands ASTC endpoint colors.
//
// This is a scalar port of unpack_color_endpoints() in Source/astcenc_color_unquantize.cpp.
func unpackColorEndpoints(profile Profile, format uint8, input []uint8) (rgbHDR, alphaHDR bool, output0, output1 int4) {
	rgbHDR = false
	alphaHDR = false
	alphaHDRDefault := false

	switch int(format) {
	case fmtLuminance:
		output0, output1 = luminanceUnpack(input[:2])
	case fmtLuminanceDelta:
		output0, output1 = luminanceDeltaUnpack(input[:2])
	case fmtHDRLuminanceSmallRange:
		rgbHDR = true
		alphaHDRDefault = true
		output0, output1 = hdrLuminanceSmallRangeUnpack(input[:2])
	case fmtHDRLuminanceLargeRange:
		rgbHDR = true
		alphaHDRDefault = true
		output0, output1 = hdrLuminanceLargeRangeUnpack(input[:2])
	case fmtLuminanceAlpha:
		output0, output1 = luminanceAlphaUnpack(input[:4])
	case fmtLuminanceAlphaDelta:
		output0, output1 = luminanceAlphaDeltaUnpack(input[:4])
	case fmtRGBScale:
		input0 := int4{int(input[0]), int(input[1]), int(input[2]), 0}
		output0, output1 = rgbScaleUnpack(input0, int(input[3]))
	case fmtRGBScaleAlpha:
		input0 := int4{int(input[0]), int(input[1]), int(input[2]), int(input[4])}
		output0, output1 = rgbScaleAlphaUnpack(input0, input[5], input[3])
	case fmtHDRRGBScale:
		rgbHDR = true
		alphaHDRDefault = true
		output0, output1 = hdrRgboUnpack(input[:4])
	case fmtRGB:
		input0 := int4{int(input[0]), int(input[2]), int(input[4]), 0}
		input1 := int4{int(input[1]), int(input[3]), int(input[5]), 0}
		output0, output1 = rgbUnpack(input0, input1)
	case fmtRGBDelta:
		input0 := int4{int(input[0]), int(input[2]), int(input[4]), 0}
		input1 := int4{int(input[1]), int(input[3]), int(input[5]), 0}
		output0, output1 = rgbDeltaUnpack(input0, input1)
	case fmtHDRRGB:
		rgbHDR = true
		alphaHDRDefault = true
		output0, output1 = hdrRgbUnpack(input[:6])
	case fmtRGBA:
		input0 := int4{int(input[0]), int(input[2]), int(input[4]), int(input[6])}
		input1 := int4{int(input[1]), int(input[3]), int(input[5]), int(input[7])}
		output0, output1 = rgbaUnpack(input0, input1)
	case fmtRGBADelta:
		input0 := int4{int(input[0]), int(input[2]), int(input[4]), int(input[6])}
		input1 := int4{int(input[1]), int(input[3]), int(input[5]), int(input[7])}
		output0, output1 = rgbaDeltaUnpack(input0, input1)
	case fmtHDRRGBLDRAlpha:
		rgbHDR = true
		output0, output1 = hdrRgbLdrAlphaUnpack(input[:8])
	case fmtHDRRGBA:
		rgbHDR = true
		alphaHDR = true
		output0, output1 = hdrRgbHdrAlphaUnpack(input[:8])
	default:
		// Unknown format => treat as error.
		output0 = int4{0xFF, 0x00, 0xFF, 0xFF}
		output1 = output0
		rgbHDR = false
		alphaHDR = false
		return rgbHDR, alphaHDR, output0, output1
	}

	if alphaHDRDefault {
		if profile == ProfileHDR {
			output0[3] = 0x7800
			output1[3] = 0x7800
			alphaHDR = true
		} else {
			output0[3] = 0x00FF
			output1[3] = 0x00FF
			alphaHDR = false
		}
	}

	// Handle endpoint errors and expansion.
	if profile == ProfileLDR {
		if rgbHDR || alphaHDR {
			output0 = int4{0xFF, 0x00, 0xFF, 0xFF}
			output1 = output0
			rgbHDR = false
			alphaHDR = false
		}

		for i := 0; i < 4; i++ {
			output0[i] *= 257
			output1[i] *= 257
		}
	} else if profile == ProfileLDRSRGB {
		if rgbHDR || alphaHDR {
			output0 = int4{0xFF, 0x00, 0xFF, 0xFF}
			output1 = output0
			rgbHDR = false
			alphaHDR = false
		}

		for i := 0; i < 4; i++ {
			output0[i] = (output0[i] << 8) | 0x80
			output1[i] = (output1[i] << 8) | 0x80
		}
	} else {
		// HDR decode profile, but endpoints may be LDR.
		for i := 0; i < 4; i++ {
			scale := 257
			if i < 3 {
				if rgbHDR {
					scale = 1
				}
			} else {
				if alphaHDR {
					scale = 1
				}
			}
			output0[i] *= scale
			output1[i] *= scale
		}
	}

	return rgbHDR, alphaHDR, output0, output1
}
