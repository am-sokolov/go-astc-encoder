package astc

import (
	"errors"
	"math"
	"sort"
	"sync"
)

// EncodeQuality controls encoder search effort.
type EncodeQuality uint8

const (
	EncodeFastest EncodeQuality = iota
	EncodeFast
	EncodeMedium
	EncodeThorough
	EncodeVeryThorough
	EncodeExhaustive
)

type blockModeDesc struct {
	mode        int
	xWeights    int
	yWeights    int
	zWeights    int
	isDualPlane bool
	weightQuant quantMethod
	weightBits  int

	// sampleTexelIndices maps each weight-grid point to a representative texel index.
	// It matches sampleWeightGrid2D/3D behavior but is precomputed per block size+mode.
	sampleTexelIndices []uint16
}

type blockModeCacheKey uint32

func makeBlockModeCacheKey(blockX, blockY, blockZ int) blockModeCacheKey {
	return blockModeCacheKey(uint32(blockX) | (uint32(blockY) << 8) | (uint32(blockZ) << 16))
}

var (
	blockModeCacheMu sync.RWMutex
	blockModeCache   = map[blockModeCacheKey][]blockModeDesc{}
)

func validBlockModes(blockX, blockY, blockZ int) []blockModeDesc {
	key := makeBlockModeCacheKey(blockX, blockY, blockZ)

	blockModeCacheMu.RLock()
	if got, ok := blockModeCache[key]; ok {
		blockModeCacheMu.RUnlock()
		return got
	}
	blockModeCacheMu.RUnlock()

	var modes []blockModeDesc
	for mode := 0; mode < (1 << 11); mode++ {
		if blockZ == 1 {
			xw, yw, dp, q, wb, ok := decodeBlockMode2D(mode)
			if !ok || xw > blockX || yw > blockY {
				continue
			}
			modes = append(modes, blockModeDesc{
				mode:               mode,
				xWeights:           xw,
				yWeights:           yw,
				zWeights:           1,
				isDualPlane:        dp,
				weightQuant:        q,
				weightBits:         wb,
				sampleTexelIndices: makeWeightGridSampleMap(blockX, blockY, blockZ, xw, yw, 1),
			})
		} else {
			xw, yw, zw, dp, q, wb, ok := decodeBlockMode3D(mode)
			if !ok || xw > blockX || yw > blockY || zw > blockZ {
				continue
			}
			modes = append(modes, blockModeDesc{
				mode:               mode,
				xWeights:           xw,
				yWeights:           yw,
				zWeights:           zw,
				isDualPlane:        dp,
				weightQuant:        q,
				weightBits:         wb,
				sampleTexelIndices: makeWeightGridSampleMap(blockX, blockY, blockZ, xw, yw, zw),
			})
		}
	}

	// Sort by a crude "quality" heuristic to make quality presets deterministic.
	sort.Slice(modes, func(i, j int) bool {
		ai := modes[i].xWeights * modes[i].yWeights * modes[i].zWeights
		aj := modes[j].xWeights * modes[j].yWeights * modes[j].zWeights
		if ai != aj {
			return ai > aj
		}
		if modes[i].weightQuant != modes[j].weightQuant {
			return modes[i].weightQuant > modes[j].weightQuant
		}
		return modes[i].weightBits < modes[j].weightBits
	})

	blockModeCacheMu.Lock()
	// Another goroutine may have populated it; keep the first.
	if got, ok := blockModeCache[key]; ok {
		blockModeCacheMu.Unlock()
		return got
	}
	blockModeCache[key] = modes
	blockModeCacheMu.Unlock()

	return modes
}

func makeWeightGridSampleMap(blockX, blockY, blockZ, xWeights, yWeights, zWeights int) []uint16 {
	weightsPerPlane := xWeights * yWeights * zWeights
	out := make([]uint16, weightsPerPlane)
	if weightsPerPlane <= 0 {
		return out
	}

	if blockZ == 1 {
		xDen := xWeights - 1
		yDen := yWeights - 1
		for wy := 0; wy < yWeights; wy++ {
			y := 0
			if yDen > 0 {
				y = (wy*(blockY-1) + yDen/2) / yDen
			}
			for wx := 0; wx < xWeights; wx++ {
				x := 0
				if xDen > 0 {
					x = (wx*(blockX-1) + xDen/2) / xDen
				}
				out[wy*xWeights+wx] = uint16(y*blockX + x)
			}
		}
		return out
	}

	xy := blockX * blockY
	xDen := xWeights - 1
	yDen := yWeights - 1
	zDen := zWeights - 1
	for wz := 0; wz < zWeights; wz++ {
		z := 0
		if zDen > 0 {
			z = (wz*(blockZ-1) + zDen/2) / zDen
		}
		for wy := 0; wy < yWeights; wy++ {
			y := 0
			if yDen > 0 {
				y = (wy*(blockY-1) + yDen/2) / yDen
			}
			for wx := 0; wx < xWeights; wx++ {
				x := 0
				if xDen > 0 {
					x = (wx*(blockX-1) + xDen/2) / xDen
				}
				out[(wz*yWeights+wy)*xWeights+wx] = uint16(z*xy + y*blockX + x)
			}
		}
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clampI32(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func colorQuantize(q quantMethod, u uint8) (pquantScrambled uint8, uquant uint8) {
	if q < quant6 || q > quant256 {
		return 0, 0
	}
	qi := int(q) - int(quant6)
	return colorQuantizePquantLUT[qi][u], colorQuantizeUquantLUT[qi][u]
}

var (
	colorQuantizePquantLUT [int(quant256) - int(quant6) + 1][256]uint8
	colorQuantizeUquantLUT [int(quant256) - int(quant6) + 1][256]uint8
)

var weightQuantizeScrambledLUT [int(quant32) + 1][65]uint8

var (
	endpointExpandLDR  [256]int32
	endpointExpandSRGB [256]int32
	normalXYZLUT       [256][256][3]float32
)

func init() {
	for u := 0; u < 256; u++ {
		endpointExpandLDR[u] = int32(u * 257)
		endpointExpandSRGB[u] = int32((u << 8) | 0x80)
	}

	for q := quantMethod(quant6); q <= quant256; q++ {
		qi := int(q) - int(quant6)
		table := colorScrambledPquantToUquantTables[qi]
		for u := 0; u <= 255; u++ {
			best := 0
			bestDiff := 0x7FFFFFFF
			for i := 0; i < len(table); i++ {
				d := absInt(int(table[i]) - u)
				if d < bestDiff {
					bestDiff = d
					best = i
					if d == 0 {
						break
					}
				}
			}
			colorQuantizePquantLUT[qi][u] = uint8(best)
			colorQuantizeUquantLUT[qi][u] = table[best]
		}
	}

	for q := quantMethod(0); q <= quant32; q++ {
		levels := quantLevel(q)
		if levels <= 0 {
			continue
		}
		for u := 0; u <= 64; u++ {
			best := 0
			bestDiff := 0x7FFFFFFF
			for i := 0; i < levels; i++ {
				d := absInt(int(weightQuantToUnquant[q][i]) - u)
				if d < bestDiff {
					bestDiff = d
					best = i
					if d == 0 {
						break
					}
				}
			}
			weightQuantizeScrambledLUT[q][u] = weightScrambleMap[q][best]
		}
	}

	// Precompute normalized vectors for normal-map angular error evaluation (MAP_NORMAL mode).
	for r := 0; r < 256; r++ {
		x := float32(r)*(2.0/255.0) - 1.0
		for a := 0; a < 256; a++ {
			y := float32(a)*(2.0/255.0) - 1.0
			z2 := 1.0 - x*x - y*y
			if z2 < 0 {
				z2 = 0
			}
			z := float32(math.Sqrt(float64(z2)))
			n2 := x*x + y*y + z*z
			if n2 > 0 {
				invN := float32(1.0 / math.Sqrt(float64(n2)))
				normalXYZLUT[r][a][0] = x * invN
				normalXYZLUT[r][a][1] = y * invN
				normalXYZLUT[r][a][2] = z * invN
			}
		}
	}
}

func weightQuantizeScrambled(q quantMethod, u int) uint8 {
	if q > quant32 {
		return 0
	}
	u = clampI32(u, 0, 64)
	return weightQuantizeScrambledLUT[q][u]
}

func bitAt(data []byte, bitIndex int) uint8 {
	return (data[bitIndex>>3] >> uint(bitIndex&7)) & 1
}

func setBit(data []byte, bitIndex int) {
	data[bitIndex>>3] |= 1 << uint(bitIndex&7)
}

func blockErrorRGBA8(a, b []byte) uint64 {
	var sum uint64
	for i := 0; i < len(a); i++ {
		d := int(a[i]) - int(b[i])
		sum += uint64(d * d)
	}
	return sum
}

func normalMapAngularError(origR, origA, decR, decA uint8) float64 {
	ref := normalXYZLUT[origR][origA]
	dec := normalXYZLUT[decR][decA]
	dot := float64(ref[0]*dec[0] + ref[1]*dec[1] + ref[2]*dec[2])
	if dot > 1 {
		dot = 1
	} else if dot < -1 {
		dot = -1
	}
	return 1.0 - dot
}

func extractBlockRGBA8(pix []byte, width, height, x0, y0, blockX, blockY int, dst []byte) {
	for by := 0; by < blockY; by++ {
		y := y0 + by
		if y >= height {
			y = height - 1
		}
		row := y * width * 4
		for bx := 0; bx < blockX; bx++ {
			x := x0 + bx
			if x >= width {
				x = width - 1
			}
			src := row + x*4
			dstOff := (by*blockX + bx) * 4
			dst[dstOff+0] = pix[src+0]
			dst[dstOff+1] = pix[src+1]
			dst[dstOff+2] = pix[src+2]
			dst[dstOff+3] = pix[src+3]
		}
	}
}

func isConstBlockRGBA8(texels []byte) (r, g, b, a uint8, ok bool) {
	if len(texels) < 4 {
		return 0, 0, 0, 0, false
	}
	r0 := texels[0]
	g0 := texels[1]
	b0 := texels[2]
	a0 := texels[3]
	for i := 4; i < len(texels); i += 4 {
		if texels[i+0] != r0 || texels[i+1] != g0 || texels[i+2] != b0 || texels[i+3] != a0 {
			return 0, 0, 0, 0, false
		}
	}
	return r0, g0, b0, a0, true
}

type partitionEndpointsRGBA struct {
	// Quantized uquant endpoints, ordered to avoid triggering rgbaUnpack swapping.
	e0 [4]uint8
	e1 [4]uint8

	// Scrambled pquant values in ASTC endpoint order:
	// r0,r1,g0,g1,b0,b1,a0,a1
	pquant [8]uint8
}

func luma(r, g, b uint8) int {
	return int(r) + int(g) + int(b)
}

func selectEndpointsRGBA(texels []byte, blockX, blockY int, partAssign []uint8, part int) (e0, e1 [4]uint8) {
	minL := math.MaxInt
	maxL := math.MinInt
	minA := math.MaxInt
	maxA := math.MinInt
	minIdx := 0
	maxIdx := 0

	texelCount := len(texels) / 4
	for t := 0; t < texelCount; t++ {
		if partAssign != nil && int(partAssign[t]) != part {
			continue
		}

		off := t * 4
		r := texels[off+0]
		g := texels[off+1]
		b := texels[off+2]
		a := texels[off+3]
		l := luma(r, g, b)

		ai := int(a)
		if l < minL || (l == minL && ai < minA) {
			minL = l
			minA = ai
			minIdx = t
		}
		if l > maxL || (l == maxL && ai > maxA) {
			maxL = l
			maxA = ai
			maxIdx = t
		}
	}

	off0 := minIdx * 4
	off1 := maxIdx * 4
	e0 = [4]uint8{texels[off0+0], texels[off0+1], texels[off0+2], texels[off0+3]}
	e1 = [4]uint8{texels[off1+0], texels[off1+1], texels[off1+2], texels[off1+3]}
	return e0, e1
}

func quantizeEndpointsRGBA(q quantMethod, e0, e1 [4]uint8) partitionEndpointsRGBA {
	return quantizeEndpointsRGBABytes(q, e0[0], e0[1], e0[2], e0[3], e1[0], e1[1], e1[2], e1[3])
}

func quantizeEndpointsRGBABytes(q quantMethod, r0, g0, b0, a0, r1, g1, b1, a1 uint8) partitionEndpointsRGBA {
	var out partitionEndpointsRGBA

	// Quantize per component.
	pR0, uR0 := colorQuantize(q, r0)
	pR1, uR1 := colorQuantize(q, r1)
	pG0, uG0 := colorQuantize(q, g0)
	pG1, uG1 := colorQuantize(q, g1)
	pB0, uB0 := colorQuantize(q, b0)
	pB1, uB1 := colorQuantize(q, b1)
	pA0, uA0 := colorQuantize(q, a0)
	pA1, uA1 := colorQuantize(q, a1)

	out.e0 = [4]uint8{uR0, uG0, uB0, uA0}
	out.e1 = [4]uint8{uR1, uG1, uB1, uA1}
	out.pquant = [8]uint8{pR0, pR1, pG0, pG1, pB0, pB1, pA0, pA1}

	// Ensure we won't trigger rgbaUnpack swapping in the decoder.
	if luma(out.e0[0], out.e0[1], out.e0[2]) > luma(out.e1[0], out.e1[1], out.e1[2]) {
		out.e0, out.e1 = out.e1, out.e0
		out.pquant = [8]uint8{pR1, pR0, pG1, pG0, pB1, pB0, pA1, pA0}
	}
	return out
}

func computeTexelWeightsRGBA(texels []byte, partAssign []uint8, endpoints []partitionEndpointsRGBA, outWeights []int) {
	texelCount := len(texels) / 4
	for t := 0; t < texelCount; t++ {
		part := 0
		if partAssign != nil {
			part = int(partAssign[t])
		}

		e0 := endpoints[part].e0
		e1 := endpoints[part].e1

		d0 := int64(int(e1[0]) - int(e0[0]))
		d1 := int64(int(e1[1]) - int(e0[1]))
		d2 := int64(int(e1[2]) - int(e0[2]))
		d3 := int64(int(e1[3]) - int(e0[3]))
		den := d0*d0 + d1*d1 + d2*d2 + d3*d3
		if den == 0 {
			outWeights[t] = 0
			continue
		}

		off := t * 4
		c0 := int64(int(texels[off+0]) - int(e0[0]))
		c1 := int64(int(texels[off+1]) - int(e0[1]))
		c2 := int64(int(texels[off+2]) - int(e0[2]))
		c3 := int64(int(texels[off+3]) - int(e0[3]))

		num := c0*d0 + c1*d1 + c2*d2 + c3*d3
		if num <= 0 {
			outWeights[t] = 0
			continue
		}
		if num >= den {
			outWeights[t] = 64
			continue
		}

		// Round to nearest: floor(num*64/den + 0.5).
		outWeights[t] = int((num*64 + den/2) / den)
	}
}

func computeTexelWeightsRGB(texels []byte, partAssign []uint8, endpoints []partitionEndpointsRGBA, outWeights []int) {
	texelCount := len(texels) / 4
	for t := 0; t < texelCount; t++ {
		part := 0
		if partAssign != nil {
			part = int(partAssign[t])
		}

		e0 := endpoints[part].e0
		e1 := endpoints[part].e1

		d0 := int64(int(e1[0]) - int(e0[0]))
		d1 := int64(int(e1[1]) - int(e0[1]))
		d2 := int64(int(e1[2]) - int(e0[2]))
		den := d0*d0 + d1*d1 + d2*d2
		if den == 0 {
			outWeights[t] = 0
			continue
		}

		off := t * 4
		c0 := int64(int(texels[off+0]) - int(e0[0]))
		c1 := int64(int(texels[off+1]) - int(e0[1]))
		c2 := int64(int(texels[off+2]) - int(e0[2]))

		num := c0*d0 + c1*d1 + c2*d2
		if num <= 0 {
			outWeights[t] = 0
			continue
		}
		if num >= den {
			outWeights[t] = 64
			continue
		}

		outWeights[t] = int((num*64 + den/2) / den)
	}
}

func computeTexelWeightsAlpha(texels []byte, partAssign []uint8, endpoints []partitionEndpointsRGBA, outWeights []int) {
	texelCount := len(texels) / 4
	for t := 0; t < texelCount; t++ {
		part := 0
		if partAssign != nil {
			part = int(partAssign[t])
		}

		e0a := int(endpoints[part].e0[3])
		e1a := int(endpoints[part].e1[3])
		den := int64(e1a - e0a)
		if den == 0 {
			outWeights[t] = 0
			continue
		}

		off := t*4 + 3
		a := int(texels[off])

		num := int64(a - e0a)
		if den < 0 {
			den = -den
			num = -num
		}
		if num <= 0 {
			outWeights[t] = 0
			continue
		}
		if num >= den {
			outWeights[t] = 64
			continue
		}
		outWeights[t] = int((num*64 + den/2) / den)
	}
}

func sampleWeightGrid2D(blockX, blockY, xWeights, yWeights int, texelWeights []int, gridWeights []int) {
	for wy := 0; wy < yWeights; wy++ {
		y := (wy*(blockY-1) + (yWeights-1)/2) / (yWeights - 1)
		for wx := 0; wx < xWeights; wx++ {
			x := (wx*(blockX-1) + (xWeights-1)/2) / (xWeights - 1)
			gridWeights[wy*xWeights+wx] = texelWeights[y*blockX+x]
		}
	}
}

func sampleWeightGrid3D(blockX, blockY, blockZ, xWeights, yWeights, zWeights int, texelWeights []int, gridWeights []int) {
	xy := blockX * blockY
	for wz := 0; wz < zWeights; wz++ {
		z := (wz*(blockZ-1) + (zWeights-1)/2) / (zWeights - 1)
		for wy := 0; wy < yWeights; wy++ {
			y := (wy*(blockY-1) + (yWeights-1)/2) / (yWeights - 1)
			for wx := 0; wx < xWeights; wx++ {
				x := (wx*(blockX-1) + (xWeights-1)/2) / (xWeights - 1)
				gridWeights[(wz*yWeights+wy)*xWeights+wx] = texelWeights[z*xy+y*blockX+x]
			}
		}
	}
}

func buildPhysicalBlock(
	mode blockModeDesc,
	blockX, blockY, blockZ int,
	partitionCount int,
	partitionIndex int,
	plane2Component int,
	endpointFormat uint8,
	colorQuant quantMethod,
	endpointPquant []uint8,
	weightPquant []uint8,
) ([BlockBytes]byte, error) {
	var block [BlockBytes]byte

	if partitionCount < 1 || partitionCount > 4 {
		return block, errors.New("astc: encoder: unsupported partition count")
	}
	if colorQuant < quant6 {
		return block, errors.New("astc: encoder: invalid color quant")
	}

	// Common header.
	writeBits(11, 0, block[:], uint32(mode.mode))
	writeBits(2, 11, block[:], uint32(partitionCount-1))

	belowWeightsPos := 128 - mode.weightBits
	if mode.isDualPlane {
		if partitionCount == 4 {
			return block, errors.New("astc: encoder: dual-plane blocks cannot use 4 partitions")
		}
		if plane2Component < 0 || plane2Component > 3 {
			return block, errors.New("astc: encoder: invalid dual-plane component")
		}
		writeBits(2, belowWeightsPos-2, block[:], uint32(plane2Component))
	}

	startBit := 0
	if partitionCount == 1 {
		// Color format directly.
		writeBits(4, 13, block[:], uint32(endpointFormat))
		startBit = 17
	} else {
		// Partition index.
		writeBits(partitionIndexBits, 13, block[:], uint32(partitionIndex))

		// Matched formats. Set baseclass = 0 and format = endpointFormat.
		encodedType := uint32(endpointFormat) << 2
		writeBits(6, 13+partitionIndexBits, block[:], encodedType)
		startBit = 19 + partitionIndexBits
	}

	encodeISE(colorQuant, len(endpointPquant), endpointPquant, block[:], startBit)

	// Weights: write into a temporary bitstream and then map into the MSB bit-reversed region.
	var weightBits [BlockBytes]byte
	encodeISE(mode.weightQuant, len(weightPquant), weightPquant, weightBits[:], 0)
	for k := 0; k < mode.weightBits; k++ {
		if bitAt(weightBits[:], k) != 0 {
			setBit(block[:], 127-k)
		}
	}

	// Sanity: ensure the block round-trips in our parser.
	scb := physicalToSymbolic(block[:], blockX, blockY, blockZ)
	if scb.blockType == symBlockError {
		return block, errors.New("astc: encoder: produced invalid block")
	}
	return block, nil
}

func buildPhysicalBlockRGBA(
	mode blockModeDesc,
	blockX, blockY, blockZ int,
	partitionCount int,
	partitionIndex int,
	plane2Component int,
	colorQuant quantMethod,
	endpointPquant []uint8,
	weightPquant []uint8,
) ([BlockBytes]byte, error) {
	return buildPhysicalBlock(mode, blockX, blockY, blockZ, partitionCount, partitionIndex, plane2Component, fmtRGBA, colorQuant, endpointPquant, weightPquant)
}

func encodeBlockRGBA8LDR(profile Profile, blockX, blockY, blockZ int, texels []byte, quality EncodeQuality, channelWeight [4]float32, flags Flags, rgbmScale float32, tuneOverride *encoderTuning) ([BlockBytes]byte, error) {
	if profile != ProfileLDR && profile != ProfileLDRSRGB && profile != ProfileHDRRGBLDRAlpha && profile != ProfileHDR {
		return [BlockBytes]byte{}, errors.New("astc: invalid profile")
	}

	if r, g, b, a, ok := isConstBlockRGBA8(texels); ok {
		return EncodeConstBlockRGBA8(r, g, b, a), nil
	}

	texelCount := blockX * blockY * blockZ

	normalMap := (flags & FlagMapNormal) != 0
	rgbmMap := (flags & FlagMapRGBM) != 0
	useU8 := (flags&FlagUseDecodeUNORM8) != 0 || profile == ProfileLDRSRGB
	if rgbmMap && rgbmScale < 1 {
		rgbmScale = 1
	}
	endpointFormat := uint8(fmtRGBA)
	endpointStride := 8
	if normalMap {
		endpointFormat = fmtLuminanceAlpha
		endpointStride = 4
	}

	// Candidate list.
	modes := validBlockModes(blockX, blockY, blockZ)
	if len(modes) == 0 {
		// Fallback: constant average.
		r, g, b, a := avgBlockRGBA8(texels, blockX, blockY*blockZ, 0, 0, blockX, blockY*blockZ)
		return EncodeConstBlockRGBA8(r, g, b, a), nil
	}

	tune := encoderTuningFor(quality, texelCount)
	if tuneOverride != nil {
		tune = *tuneOverride
	}
	if tuneOverride == nil && normalMap && quality >= EncodeMedium {
		// The upstream encoder increases effort and partitioning for normal maps because L+A blocks
		// need fewer endpoint bits. Our medium preset is intentionally conservative to preserve
		// existing regression fixtures for generic color data, so bump limits here to better match
		// reference output for ASTCENC_FLG_MAP_NORMAL.
		if tune.modeLimit < 94 {
			tune.modeLimit = 94
		}
		if tune.maxPartitionCount < 4 {
			tune.maxPartitionCount = 4
		}
		if tune.partitionIndexLimit[2] < 82 {
			tune.partitionIndexLimit[2] = 82
		}
		if tune.partitionIndexLimit[3] < 60 {
			tune.partitionIndexLimit[3] = 60
		}
		if tune.partitionIndexLimit[4] < 30 {
			tune.partitionIndexLimit[4] = 30
		}
		if tune.partitionCandidateLimit[2] < 3 {
			tune.partitionCandidateLimit[2] = 3
		}
		if tune.partitionCandidateLimit[3] < 2 {
			tune.partitionCandidateLimit[3] = 2
		}
		if tune.partitionCandidateLimit[4] < 2 {
			tune.partitionCandidateLimit[4] = 2
		}
	} else if tuneOverride == nil && normalMap && tune.maxPartitionCount < 4 {
		// Lower presets: still allow a little more partitioning headroom.
		tune.maxPartitionCount++
	}
	modeLimit := tune.modeLimit
	if modeLimit <= 0 || modeLimit > len(modes) {
		modeLimit = len(modes)
	}
	modes = modes[:modeLimit]

	// For higher presets we can use faster (approximate) weight projection to reduce division overhead.
	// This does not affect the medium preset used by regression fixtures.
	useFloatWeights := quality >= EncodeThorough

	expandEndpoint := &endpointExpandLDR
	if profile == ProfileLDRSRGB {
		expandEndpoint = &endpointExpandSRGB
	}

	var partitionCountsArr [blockMaxPartitions]int
	partitionCountsArr[0] = 1
	partitionCountLen := 1
	for pc := 2; pc <= tune.maxPartitionCount && pc <= blockMaxPartitions; pc++ {
		partitionCountsArr[partitionCountLen] = pc
		partitionCountLen++
	}
	partitionCounts := partitionCountsArr[:partitionCountLen]

	var texelLumaArr [blockMaxTexels]int
	var texelAlphaArr [blockMaxTexels]int
	texelLuma := texelLumaArr[:texelCount]
	texelAlpha := texelAlphaArr[:texelCount]

	alphaMin := uint8(255)
	alphaMax := uint8(0)
	for t := 0; t < texelCount; t++ {
		off := t * 4
		r := texels[off+0]
		g := texels[off+1]
		b := texels[off+2]
		a := texels[off+3]

		l := int(r) + int(g) + int(b)
		if rgbmMap {
			l *= int(a)
		}
		texelLuma[t] = l
		texelAlpha[t] = int(a)

		if a < alphaMin {
			alphaMin = a
		}
		if a > alphaMax {
			alphaMax = a
		}
	}
	alphaVary := alphaMin != alphaMax

	allowDualPlane := alphaVary
	if allowDualPlane && quality >= EncodeThorough {
		thresh := tune.dualPlaneCorrelationThreshold
		if normalMap && thresh < 0.99 {
			thresh = 0.99
		}
		if thresh > 0 {
			if alphaRGBAbsCorrelation(texels) >= float64(thresh) {
				allowDualPlane = false
			}
		}
	}

	var texelWeightsArr [blockMaxTexels]int
	var texelWeights2Arr [blockMaxTexels]int
	texelWeights := texelWeightsArr[:texelCount]
	texelWeights2 := texelWeights2Arr[:texelCount]

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
			candidates2Count = selectBestPartitionIndices(candidates2, texels, pt2, 2, partIndexLimit2, alphaVary)
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
			candidates3Count = selectBestPartitionIndices(candidates3, texels, pt3, 3, partIndexLimit3, alphaVary)
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
			candidates4Count = selectBestPartitionIndices(candidates4, texels, pt4, 4, partIndexLimit4, alphaVary)
		}
	}

	wR := float64(channelWeight[0])
	wG := float64(channelWeight[1])
	wB := float64(channelWeight[2])
	wA := float64(channelWeight[3])
	rgbmScale64 := float64(rgbmScale)

	bestErr := math.Inf(1)
	var bestMode blockModeDesc
	bestPartitionCount := 0
	bestPartitionIndex := 0
	bestPlane2Component := -1
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
	var endpointsArr [4]partitionEndpointsRGBA
	var evalEp0 [4][4]int32
	var evalEpd [4][4]int32

	for _, mode := range modes {
		if mode.isDualPlane && !allowDualPlane {
			continue
		}

		dec := getDecimationTable(blockX, blockY, blockZ, mode.xWeights, mode.yWeights, mode.zWeights)

		weightCountPerPlane := mode.xWeights * mode.yWeights * mode.zWeights
		noDecimation := weightCountPerPlane == texelCount
		sampleMap := mode.sampleTexelIndices
		realWeightCount := weightCountPerPlane
		if mode.isDualPlane {
			realWeightCount *= 2
		}
		weightsUQ := weightsUQArr[:]
		uqMap := weightUnscrambleAndUnquantMap[mode.weightQuant]
		wQuantLUT := &weightQuantizeScrambledLUT[mode.weightQuant]

		belowWeightsPos := 128 - mode.weightBits

		for _, partitionCount := range partitionCounts {
			if mode.isDualPlane && partitionCount == 4 {
				// Invalid per spec; matches reference encoder behavior.
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

			colorIntCount := partitionCount * endpointStride
			qLevel := quantLevelForISE(colorIntCount, bitsAvailable)
			if qLevel < int(quant6) {
				continue
			}
			colorQuant := quantMethod(qLevel)

			var pt *partitionTable
			var candidates []int
			candidateCount := 0
			partIndexLimit := 1
			switch partitionCount {
			case 1:
				// no partition table
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

			endpoints := endpointsArr[:partitionCount]

			var idxListArr [1]int
			idxList := ([]int)(nil)
			if partitionCount == 1 {
				idxListArr[0] = 0
				idxList = idxListArr[:]
			} else if candidateCount > 0 && !normalMap && tuneOverride == nil {
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

				// Slices into scratch buffers. These buffers may swap when a new best candidate is found.
				endpointPquant := currEndpointPquantBuf[:partitionCount*endpointStride]
				weightPquant := currWeightPquantBuf[:realWeightCount]

				var assign []uint8
				if partitionCount != 1 {
					assign = pt.partitionsForIndex(partitionIndex)
				}

				// Endpoint selection in one pass for all partitions.
				var count [4]uint16
				var minL [4]int
				var maxL [4]int
				var minA [4]int
				var maxA [4]int
				var minIdx [4]int
				var maxIdx [4]int
				var sumX [4]float64
				var sumY [4]float64
				var sumXX [4]float64
				var sumYY [4]float64
				var sumXY [4]float64

				if assign == nil {
					// partitionCount == 1
					count[0] = 0
					minL[0] = math.MaxInt
					maxL[0] = math.MinInt
					minA[0] = math.MaxInt
					maxA[0] = math.MinInt
					minIdx[0] = 0
					maxIdx[0] = 0

					for t := 0; t < texelCount; t++ {
						count[0]++
						l := texelLuma[t]
						ai := texelAlpha[t]
						if normalMap {
							off := t * 4
							x := float64(texels[off+0])
							y := float64(texels[off+3])
							sumX[0] += x
							sumY[0] += y
							sumXX[0] += x * x
							sumYY[0] += y * y
							sumXY[0] += x * y
						}
						if l < minL[0] || (l == minL[0] && ai < minA[0]) {
							minL[0] = l
							minA[0] = ai
							minIdx[0] = t
						}
						if l > maxL[0] || (l == maxL[0] && ai > maxA[0]) {
							maxL[0] = l
							maxA[0] = ai
							maxIdx[0] = t
						}
					}
				} else {
					for p := 0; p < partitionCount; p++ {
						count[p] = 0
						minL[p] = math.MaxInt
						maxL[p] = math.MinInt
						minA[p] = math.MaxInt
						maxA[p] = math.MinInt
						minIdx[p] = 0
						maxIdx[p] = 0
					}

					for t := 0; t < texelCount; t++ {
						part := int(assign[t])
						count[part]++

						l := texelLuma[t]
						ai := texelAlpha[t]
						if normalMap {
							off := t * 4
							x := float64(texels[off+0])
							y := float64(texels[off+3])
							sumX[part] += x
							sumY[part] += y
							sumXX[part] += x * x
							sumYY[part] += y * y
							sumXY[part] += x * y
						}
						if l < minL[part] || (l == minL[part] && ai < minA[part]) {
							minL[part] = l
							minA[part] = ai
							minIdx[part] = t
						}
						if l > maxL[part] || (l == maxL[part] && ai > maxA[part]) {
							maxL[part] = l
							maxA[part] = ai
							maxIdx[part] = t
						}
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

				if normalMap {
					// Use a simple 2D PCA on (R, A) to pick endpoints for the L+A line. This better matches
					// reference behavior for ASTCENC_FLG_MAP_NORMAL than luma-only endpoint selection.
					var meanX [4]float64
					var meanY [4]float64
					var dirX [4]float64
					var dirY [4]float64

					for p := 0; p < partitionCount; p++ {
						n := float64(count[p])
						if n <= 0 {
							continue
						}

						mx := sumX[p] / n
						my := sumY[p] / n
						meanX[p] = mx
						meanY[p] = my

						// Covariance matrix of centered data.
						cov00 := sumXX[p]/n - mx*mx
						cov11 := sumYY[p]/n - my*my
						cov01 := sumXY[p]/n - mx*my

						// Principal axis direction for 2x2 covariance.
						dx, dy := 1.0, 0.0
						if cov01 != 0 || cov00 != cov11 {
							theta := 0.5 * math.Atan2(2*cov01, cov00-cov11)
							dx = math.Cos(theta)
							dy = math.Sin(theta)
						} else if cov11 > cov00 {
							dx, dy = 0.0, 1.0
						}
						dirX[p] = dx
						dirY[p] = dy
					}

					minProj := [4]float64{math.Inf(1), math.Inf(1), math.Inf(1), math.Inf(1)}
					maxProj := [4]float64{math.Inf(-1), math.Inf(-1), math.Inf(-1), math.Inf(-1)}
					for t := 0; t < texelCount; t++ {
						part := 0
						if assign != nil {
							part = int(assign[t])
						}
						off := t * 4
						x := float64(texels[off+0]) - meanX[part]
						y := float64(texels[off+3]) - meanY[part]
						proj := x*dirX[part] + y*dirY[part]
						if proj < minProj[part] {
							minProj[part] = proj
							minIdx[part] = t
						}
						if proj > maxProj[part] {
							maxProj[part] = proj
							maxIdx[part] = t
						}
					}
				}

				for p := 0; p < partitionCount; p++ {
					off0 := minIdx[p] * 4
					off1 := maxIdx[p] * 4
					var ep partitionEndpointsRGBA
					if normalMap {
						lum0 := texels[off0+0]
						lum1 := texels[off1+0]
						a0 := texels[off0+3]
						a1 := texels[off1+3]
						ep = quantizeEndpointsRGBABytes(colorQuant, lum0, lum0, lum0, a0, lum1, lum1, lum1, a1)
					} else {
						ep = quantizeEndpointsRGBABytes(
							colorQuant,
							texels[off0+0], texels[off0+1], texels[off0+2], texels[off0+3],
							texels[off1+0], texels[off1+1], texels[off1+2], texels[off1+3],
						)
					}
					endpoints[p] = ep
					base := p * endpointStride
					pp := ep.pquant
					if normalMap {
						endpointPquant[base+0] = pp[0]
						endpointPquant[base+1] = pp[1]
						endpointPquant[base+2] = pp[6]
						endpointPquant[base+3] = pp[7]
					} else {
						endpointPquant[base+0] = pp[0]
						endpointPquant[base+1] = pp[1]
						endpointPquant[base+2] = pp[2]
						endpointPquant[base+3] = pp[3]
						endpointPquant[base+4] = pp[4]
						endpointPquant[base+5] = pp[5]
						endpointPquant[base+6] = pp[6]
						endpointPquant[base+7] = pp[7]
					}
				}

				plane2Component := -1
				if mode.isDualPlane {
					plane2Component = 3 // Alpha
					switch partitionCount {
					case 1:
						e0u := endpoints[0].e0
						e1u := endpoints[0].e1

						dr0 := int64(int(e1u[0]) - int(e0u[0]))
						dr1 := int64(int(e1u[1]) - int(e0u[1]))
						dr2 := int64(int(e1u[2]) - int(e0u[2]))
						denRGB := dr0*dr0 + dr1*dr1 + dr2*dr2
						e0r0 := int64(e0u[0])
						e0r1 := int64(e0u[1])
						e0r2 := int64(e0u[2])

						a0 := int64(e0u[3])
						a1 := int64(e1u[3])
						denA := a1 - a0
						signA := int64(1)
						if denA < 0 {
							denA = -denA
							signA = -1
						}

						for t := 0; t < texelCount; t++ {
							off := t * 4
							if denRGB == 0 {
								texelWeights[t] = 0
							} else {
								c0 := int64(int(texels[off+0])) - e0r0
								c1 := int64(int(texels[off+1])) - e0r1
								c2 := int64(int(texels[off+2])) - e0r2
								num := c0*dr0 + c1*dr1 + c2*dr2
								if num <= 0 {
									texelWeights[t] = 0
								} else if num >= denRGB {
									texelWeights[t] = 64
								} else {
									texelWeights[t] = int((num*64 + denRGB/2) / denRGB)
								}
							}

							if denA == 0 {
								texelWeights2[t] = 0
							} else {
								num := (int64(int(texels[off+3])) - a0) * signA
								if num <= 0 {
									texelWeights2[t] = 0
								} else if num >= denA {
									texelWeights2[t] = 64
								} else {
									texelWeights2[t] = int((num*64 + denA/2) / denA)
								}
							}
						}
					case 2:
						e00 := endpoints[0].e0
						e10 := endpoints[0].e1
						e01 := endpoints[1].e0
						e11 := endpoints[1].e1

						dr00 := int64(int(e10[0]) - int(e00[0]))
						dr01 := int64(int(e10[1]) - int(e00[1]))
						dr02 := int64(int(e10[2]) - int(e00[2]))
						denRGB0 := dr00*dr00 + dr01*dr01 + dr02*dr02
						e0r00 := int64(e00[0])
						e0r01 := int64(e00[1])
						e0r02 := int64(e00[2])

						dr10 := int64(int(e11[0]) - int(e01[0]))
						dr11 := int64(int(e11[1]) - int(e01[1]))
						dr12 := int64(int(e11[2]) - int(e01[2]))
						denRGB1 := dr10*dr10 + dr11*dr11 + dr12*dr12
						e0r10 := int64(e01[0])
						e0r11 := int64(e01[1])
						e0r12 := int64(e01[2])

						a00 := int64(e00[3])
						a10 := int64(e10[3])
						denA0 := a10 - a00
						signA0 := int64(1)
						if denA0 < 0 {
							denA0 = -denA0
							signA0 = -1
						}

						a01 := int64(e01[3])
						a11 := int64(e11[3])
						denA1 := a11 - a01
						signA1 := int64(1)
						if denA1 < 0 {
							denA1 = -denA1
							signA1 = -1
						}

						for t := 0; t < texelCount; t++ {
							off := t * 4
							if assign[t] == 0 {
								if denRGB0 == 0 {
									texelWeights[t] = 0
								} else {
									c0 := int64(int(texels[off+0])) - e0r00
									c1 := int64(int(texels[off+1])) - e0r01
									c2 := int64(int(texels[off+2])) - e0r02
									num := c0*dr00 + c1*dr01 + c2*dr02
									if num <= 0 {
										texelWeights[t] = 0
									} else if num >= denRGB0 {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int((num*64 + denRGB0/2) / denRGB0)
									}
								}

								if denA0 == 0 {
									texelWeights2[t] = 0
								} else {
									num := (int64(int(texels[off+3])) - a00) * signA0
									if num <= 0 {
										texelWeights2[t] = 0
									} else if num >= denA0 {
										texelWeights2[t] = 64
									} else {
										texelWeights2[t] = int((num*64 + denA0/2) / denA0)
									}
								}
							} else {
								if denRGB1 == 0 {
									texelWeights[t] = 0
								} else {
									c0 := int64(int(texels[off+0])) - e0r10
									c1 := int64(int(texels[off+1])) - e0r11
									c2 := int64(int(texels[off+2])) - e0r12
									num := c0*dr10 + c1*dr11 + c2*dr12
									if num <= 0 {
										texelWeights[t] = 0
									} else if num >= denRGB1 {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int((num*64 + denRGB1/2) / denRGB1)
									}
								}

								if denA1 == 0 {
									texelWeights2[t] = 0
								} else {
									num := (int64(int(texels[off+3])) - a01) * signA1
									if num <= 0 {
										texelWeights2[t] = 0
									} else if num >= denA1 {
										texelWeights2[t] = 64
									} else {
										texelWeights2[t] = int((num*64 + denA1/2) / denA1)
									}
								}
							}
						}
					default:
						var e0rgb [4][3]int64
						var drgb [4][3]int64
						var denrgb [4]int64

						var e0a [4]int64
						var dena [4]int64
						var signa [4]int64

						for p := 0; p < partitionCount; p++ {
							e0u := endpoints[p].e0
							e1u := endpoints[p].e1

							d0 := int64(int(e1u[0]) - int(e0u[0]))
							d1 := int64(int(e1u[1]) - int(e0u[1]))
							d2 := int64(int(e1u[2]) - int(e0u[2]))

							e0rgb[p][0] = int64(e0u[0])
							e0rgb[p][1] = int64(e0u[1])
							e0rgb[p][2] = int64(e0u[2])
							drgb[p][0] = d0
							drgb[p][1] = d1
							drgb[p][2] = d2
							denrgb[p] = d0*d0 + d1*d1 + d2*d2

							a0 := int64(e0u[3])
							a1 := int64(e1u[3])
							den := a1 - a0
							sign := int64(1)
							if den < 0 {
								den = -den
								sign = -1
							}
							e0a[p] = a0
							dena[p] = den
							signa[p] = sign
						}

						for t := 0; t < texelCount; t++ {
							part := int(assign[t])
							off := t * 4

							den := denrgb[part]
							if den == 0 {
								texelWeights[t] = 0
							} else {
								c0 := int64(int(texels[off+0])) - e0rgb[part][0]
								c1 := int64(int(texels[off+1])) - e0rgb[part][1]
								c2 := int64(int(texels[off+2])) - e0rgb[part][2]
								num := c0*drgb[part][0] + c1*drgb[part][1] + c2*drgb[part][2]
								if num <= 0 {
									texelWeights[t] = 0
								} else if num >= den {
									texelWeights[t] = 64
								} else {
									texelWeights[t] = int((num*64 + den/2) / den)
								}
							}

							den = dena[part]
							if den == 0 {
								texelWeights2[t] = 0
							} else {
								num := (int64(int(texels[off+3])) - e0a[part]) * signa[part]
								if num <= 0 {
									texelWeights2[t] = 0
								} else if num >= den {
									texelWeights2[t] = 64
								} else {
									texelWeights2[t] = int((num*64 + den/2) / den)
								}
							}
						}
					}

					for i := 0; i < weightCountPerPlane; i++ {
						tix := int(sampleMap[i])
						p1 := (*wQuantLUT)[texelWeights[tix]]
						p2 := (*wQuantLUT)[texelWeights2[tix]]
						weightPquant[2*i] = p1
						weightPquant[2*i+1] = p2
						weightsUQ[i] = uqMap[p1]
						weightsUQ[i+weightsPlane2Offset] = uqMap[p2]
					}
				} else {
					useWeightedProjection := !(channelWeight[0] == 1 && channelWeight[1] == 1 && channelWeight[2] == 1 && channelWeight[3] == 1)
					useFloatProjection := useFloatWeights || useWeightedProjection

					if useFloatProjection {
						switch partitionCount {
						case 1:
							e0u := endpoints[0].e0
							e1u := endpoints[0].e1

							d0 := float32(int(e1u[0]) - int(e0u[0]))
							d1 := float32(int(e1u[1]) - int(e0u[1]))
							d2 := float32(int(e1u[2]) - int(e0u[2]))
							d3 := float32(int(e1u[3]) - int(e0u[3]))
							w0 := channelWeight[0]
							w1 := channelWeight[1]
							w2 := channelWeight[2]
							w3 := channelWeight[3]
							if !useWeightedProjection {
								w0, w1, w2, w3 = 1, 1, 1, 1
							}
							den := d0*d0*w0 + d1*d1*w1 + d2*d2*w2 + d3*d3*w3
							if den <= 0 {
								for t := 0; t < texelCount; t++ {
									texelWeights[t] = 0
								}
								break
							}
							invDen := float32(64) / den
							e00 := float32(e0u[0])
							e01 := float32(e0u[1])
							e02 := float32(e0u[2])
							e03 := float32(e0u[3])

							for t := 0; t < texelCount; t++ {
								off := t * 4
								c0 := float32(texels[off+0]) - e00
								c1 := float32(texels[off+1]) - e01
								c2 := float32(texels[off+2]) - e02
								c3 := float32(texels[off+3]) - e03
								w := (c0*d0*w0 + c1*d1*w1 + c2*d2*w2 + c3*d3*w3) * invDen
								if w <= 0 {
									texelWeights[t] = 0
								} else if w >= 64 {
									texelWeights[t] = 64
								} else {
									texelWeights[t] = int(w + 0.5)
								}
							}
						case 2:
							e00 := endpoints[0].e0
							e10 := endpoints[0].e1
							e01 := endpoints[1].e0
							e11 := endpoints[1].e1

							d00 := float32(int(e10[0]) - int(e00[0]))
							d01 := float32(int(e10[1]) - int(e00[1]))
							d02 := float32(int(e10[2]) - int(e00[2]))
							d03 := float32(int(e10[3]) - int(e00[3]))
							w0 := channelWeight[0]
							w1 := channelWeight[1]
							w2 := channelWeight[2]
							w3 := channelWeight[3]
							if !useWeightedProjection {
								w0, w1, w2, w3 = 1, 1, 1, 1
							}
							den0 := d00*d00*w0 + d01*d01*w1 + d02*d02*w2 + d03*d03*w3
							invDen0 := float32(0)
							if den0 > 0 {
								invDen0 = float32(64) / den0
							}
							e0r00 := float32(e00[0])
							e0r01 := float32(e00[1])
							e0r02 := float32(e00[2])
							e0r03 := float32(e00[3])

							d10 := float32(int(e11[0]) - int(e01[0]))
							d11 := float32(int(e11[1]) - int(e01[1]))
							d12 := float32(int(e11[2]) - int(e01[2]))
							d13 := float32(int(e11[3]) - int(e01[3]))
							den1 := d10*d10*w0 + d11*d11*w1 + d12*d12*w2 + d13*d13*w3
							invDen1 := float32(0)
							if den1 > 0 {
								invDen1 = float32(64) / den1
							}
							e0r10 := float32(e01[0])
							e0r11 := float32(e01[1])
							e0r12 := float32(e01[2])
							e0r13 := float32(e01[3])

							for t := 0; t < texelCount; t++ {
								off := t * 4
								if assign[t] == 0 {
									if invDen0 == 0 {
										texelWeights[t] = 0
										continue
									}
									c0 := float32(texels[off+0]) - e0r00
									c1 := float32(texels[off+1]) - e0r01
									c2 := float32(texels[off+2]) - e0r02
									c3 := float32(texels[off+3]) - e0r03
									w := (c0*d00*w0 + c1*d01*w1 + c2*d02*w2 + c3*d03*w3) * invDen0
									if w <= 0 {
										texelWeights[t] = 0
									} else if w >= 64 {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int(w + 0.5)
									}
								} else {
									if invDen1 == 0 {
										texelWeights[t] = 0
										continue
									}
									c0 := float32(texels[off+0]) - e0r10
									c1 := float32(texels[off+1]) - e0r11
									c2 := float32(texels[off+2]) - e0r12
									c3 := float32(texels[off+3]) - e0r13
									w := (c0*d10*w0 + c1*d11*w1 + c2*d12*w2 + c3*d13*w3) * invDen1
									if w <= 0 {
										texelWeights[t] = 0
									} else if w >= 64 {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int(w + 0.5)
									}
								}
							}
						default:
							var e0v [4][4]float32
							var dv [4][4]float32
							var invDen [4]float32
							w0 := channelWeight[0]
							w1 := channelWeight[1]
							w2 := channelWeight[2]
							w3 := channelWeight[3]
							if !useWeightedProjection {
								w0, w1, w2, w3 = 1, 1, 1, 1
							}

							for p := 0; p < partitionCount; p++ {
								e0u := endpoints[p].e0
								e1u := endpoints[p].e1

								d0 := float32(int(e1u[0]) - int(e0u[0]))
								d1 := float32(int(e1u[1]) - int(e0u[1]))
								d2 := float32(int(e1u[2]) - int(e0u[2]))
								d3 := float32(int(e1u[3]) - int(e0u[3]))
								den := d0*d0*w0 + d1*d1*w1 + d2*d2*w2 + d3*d3*w3
								if den > 0 {
									invDen[p] = float32(64) / den
								} else {
									invDen[p] = 0
								}

								e0v[p][0] = float32(e0u[0])
								e0v[p][1] = float32(e0u[1])
								e0v[p][2] = float32(e0u[2])
								e0v[p][3] = float32(e0u[3])
								dv[p][0] = d0
								dv[p][1] = d1
								dv[p][2] = d2
								dv[p][3] = d3
							}

							for t := 0; t < texelCount; t++ {
								part := int(assign[t])
								id := invDen[part]
								if id == 0 {
									texelWeights[t] = 0
									continue
								}
								off := t * 4
								c0 := float32(texels[off+0]) - e0v[part][0]
								c1 := float32(texels[off+1]) - e0v[part][1]
								c2 := float32(texels[off+2]) - e0v[part][2]
								c3 := float32(texels[off+3]) - e0v[part][3]
								w := (c0*dv[part][0]*w0 + c1*dv[part][1]*w1 + c2*dv[part][2]*w2 + c3*dv[part][3]*w3) * id
								if w <= 0 {
									texelWeights[t] = 0
								} else if w >= 64 {
									texelWeights[t] = 64
								} else {
									texelWeights[t] = int(w + 0.5)
								}
							}
						}
					} else {
						switch partitionCount {
						case 1:
							e0u := endpoints[0].e0
							e1u := endpoints[0].e1

							d0 := int32(int(e1u[0]) - int(e0u[0]))
							d1 := int32(int(e1u[1]) - int(e0u[1]))
							d2 := int32(int(e1u[2]) - int(e0u[2]))
							d3 := int32(int(e1u[3]) - int(e0u[3]))
							den := d0*d0 + d1*d1 + d2*d2 + d3*d3
							e00 := int32(e0u[0])
							e01 := int32(e0u[1])
							e02 := int32(e0u[2])
							e03 := int32(e0u[3])

							for t := 0; t < texelCount; t++ {
								off := t * 4
								if den == 0 {
									texelWeights[t] = 0
									continue
								}
								c0 := int32(texels[off+0]) - e00
								c1 := int32(texels[off+1]) - e01
								c2 := int32(texels[off+2]) - e02
								c3 := int32(texels[off+3]) - e03
								num := c0*d0 + c1*d1 + c2*d2 + c3*d3
								if num <= 0 {
									texelWeights[t] = 0
								} else if num >= den {
									texelWeights[t] = 64
								} else {
									texelWeights[t] = int((num*64 + den/2) / den)
								}
							}
						case 2:
							e00 := endpoints[0].e0
							e10 := endpoints[0].e1
							e01 := endpoints[1].e0
							e11 := endpoints[1].e1

							d00 := int32(int(e10[0]) - int(e00[0]))
							d01 := int32(int(e10[1]) - int(e00[1]))
							d02 := int32(int(e10[2]) - int(e00[2]))
							d03 := int32(int(e10[3]) - int(e00[3]))
							den0 := d00*d00 + d01*d01 + d02*d02 + d03*d03
							e0r00 := int32(e00[0])
							e0r01 := int32(e00[1])
							e0r02 := int32(e00[2])
							e0r03 := int32(e00[3])

							d10 := int32(int(e11[0]) - int(e01[0]))
							d11 := int32(int(e11[1]) - int(e01[1]))
							d12 := int32(int(e11[2]) - int(e01[2]))
							d13 := int32(int(e11[3]) - int(e01[3]))
							den1 := d10*d10 + d11*d11 + d12*d12 + d13*d13
							e0r10 := int32(e01[0])
							e0r11 := int32(e01[1])
							e0r12 := int32(e01[2])
							e0r13 := int32(e01[3])

							for t := 0; t < texelCount; t++ {
								off := t * 4
								if assign[t] == 0 {
									if den0 == 0 {
										texelWeights[t] = 0
										continue
									}
									c0 := int32(texels[off+0]) - e0r00
									c1 := int32(texels[off+1]) - e0r01
									c2 := int32(texels[off+2]) - e0r02
									c3 := int32(texels[off+3]) - e0r03
									num := c0*d00 + c1*d01 + c2*d02 + c3*d03
									if num <= 0 {
										texelWeights[t] = 0
									} else if num >= den0 {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int((num*64 + den0/2) / den0)
									}
								} else {
									if den1 == 0 {
										texelWeights[t] = 0
										continue
									}
									c0 := int32(texels[off+0]) - e0r10
									c1 := int32(texels[off+1]) - e0r11
									c2 := int32(texels[off+2]) - e0r12
									c3 := int32(texels[off+3]) - e0r13
									num := c0*d10 + c1*d11 + c2*d12 + c3*d13
									if num <= 0 {
										texelWeights[t] = 0
									} else if num >= den1 {
										texelWeights[t] = 64
									} else {
										texelWeights[t] = int((num*64 + den1/2) / den1)
									}
								}
							}
						default:
							var e0v [4][4]int32
							var dv [4][4]int32
							var denv [4]int32

							for p := 0; p < partitionCount; p++ {
								e0u := endpoints[p].e0
								e1u := endpoints[p].e1

								d0 := int32(int(e1u[0]) - int(e0u[0]))
								d1 := int32(int(e1u[1]) - int(e0u[1]))
								d2 := int32(int(e1u[2]) - int(e0u[2]))
								d3 := int32(int(e1u[3]) - int(e0u[3]))

								e0v[p][0] = int32(e0u[0])
								e0v[p][1] = int32(e0u[1])
								e0v[p][2] = int32(e0u[2])
								e0v[p][3] = int32(e0u[3])
								dv[p][0] = d0
								dv[p][1] = d1
								dv[p][2] = d2
								dv[p][3] = d3
								denv[p] = d0*d0 + d1*d1 + d2*d2 + d3*d3
							}

							for t := 0; t < texelCount; t++ {
								part := int(assign[t])
								den := denv[part]
								if den == 0 {
									texelWeights[t] = 0
									continue
								}
								off := t * 4
								c0 := int32(texels[off+0]) - e0v[part][0]
								c1 := int32(texels[off+1]) - e0v[part][1]
								c2 := int32(texels[off+2]) - e0v[part][2]
								c3 := int32(texels[off+3]) - e0v[part][3]
								num := c0*dv[part][0] + c1*dv[part][1] + c2*dv[part][2] + c3*dv[part][3]
								if num <= 0 {
									texelWeights[t] = 0
								} else if num >= den {
									texelWeights[t] = 64
								} else {
									texelWeights[t] = int((num*64 + den/2) / den)
								}
							}
						}
					}

					for i := 0; i < weightCountPerPlane; i++ {
						p := (*wQuantLUT)[texelWeights[int(sampleMap[i])]]
						weightPquant[i] = p
						weightsUQ[i] = uqMap[p]
					}
				}

				for p := 0; p < partitionCount; p++ {
					e0u := endpoints[p].e0
					e1u := endpoints[p].e1

					e0r := (*expandEndpoint)[e0u[0]]
					e1r := (*expandEndpoint)[e1u[0]]
					e0g := (*expandEndpoint)[e0u[1]]
					e1g := (*expandEndpoint)[e1u[1]]
					e0b := (*expandEndpoint)[e0u[2]]
					e1b := (*expandEndpoint)[e1u[2]]
					e0a := (*expandEndpoint)[e0u[3]]
					e1a := (*expandEndpoint)[e1u[3]]

					evalEp0[p][0] = e0r
					evalEpd[p][0] = e1r - e0r
					evalEp0[p][1] = e0g
					evalEpd[p][1] = e1g - e0g
					evalEp0[p][2] = e0b
					evalEpd[p][2] = e1b - e0b
					evalEp0[p][3] = e0a
					evalEpd[p][3] = e1a - e0a
				}

				var errv float64
				if !mode.isDualPlane {
					if assign == nil {
						e0 := evalEp0[0]
						d := evalEpd[0]
						if noDecimation {
							for t := 0; t < texelCount; t++ {
								w1 := int32(weightsUQ[t])
								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w1 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						} else {
							for t := 0; t < texelCount; t++ {
								e := dec[t]
								sum1 := uint32(8)
								sum1 += uint32(weightsUQ[e.idx[0]]) * uint32(e.w[0])
								sum1 += uint32(weightsUQ[e.idx[1]]) * uint32(e.w[1])
								sum1 += uint32(weightsUQ[e.idx[2]]) * uint32(e.w[2])
								sum1 += uint32(weightsUQ[e.idx[3]]) * uint32(e.w[3])
								w1 := int32(sum1 >> 4)

								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w1 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						}
					} else {
						if noDecimation {
							for t := 0; t < texelCount; t++ {
								w1 := int32(weightsUQ[t])

								part := int(assign[t])
								e0 := evalEp0[part]
								d := evalEpd[part]
								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w1 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						} else {
							for t := 0; t < texelCount; t++ {
								e := dec[t]
								sum1 := uint32(8)
								sum1 += uint32(weightsUQ[e.idx[0]]) * uint32(e.w[0])
								sum1 += uint32(weightsUQ[e.idx[1]]) * uint32(e.w[1])
								sum1 += uint32(weightsUQ[e.idx[2]]) * uint32(e.w[2])
								sum1 += uint32(weightsUQ[e.idx[3]]) * uint32(e.w[3])
								w1 := int32(sum1 >> 4)

								part := int(assign[t])
								e0 := evalEp0[part]
								d := evalEpd[part]
								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w1 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						}
					}
				} else {
					// Current encoder only evaluates dual-plane with plane2Component = Alpha.
					if assign == nil {
						e0 := evalEp0[0]
						d := evalEpd[0]
						if noDecimation {
							for t := 0; t < texelCount; t++ {
								w1 := int32(weightsUQ[t])
								w2 := int32(weightsUQ[t+weightsPlane2Offset])

								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w2 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						} else {
							for t := 0; t < texelCount; t++ {
								e := dec[t]
								sum1 := uint32(8)
								sum1 += uint32(weightsUQ[e.idx[0]]) * uint32(e.w[0])
								sum1 += uint32(weightsUQ[e.idx[1]]) * uint32(e.w[1])
								sum1 += uint32(weightsUQ[e.idx[2]]) * uint32(e.w[2])
								sum1 += uint32(weightsUQ[e.idx[3]]) * uint32(e.w[3])
								w1 := int32(sum1 >> 4)

								sum2 := uint32(8)
								sum2 += uint32(weightsUQ[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
								sum2 += uint32(weightsUQ[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
								sum2 += uint32(weightsUQ[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
								sum2 += uint32(weightsUQ[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])
								w2 := int32(sum2 >> 4)

								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w2 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						}
					} else {
						if noDecimation {
							for t := 0; t < texelCount; t++ {
								w1 := int32(weightsUQ[t])
								w2 := int32(weightsUQ[t+weightsPlane2Offset])

								part := int(assign[t])
								e0 := evalEp0[part]
								d := evalEpd[part]
								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w2 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						} else {
							for t := 0; t < texelCount; t++ {
								e := dec[t]
								sum1 := uint32(8)
								sum1 += uint32(weightsUQ[e.idx[0]]) * uint32(e.w[0])
								sum1 += uint32(weightsUQ[e.idx[1]]) * uint32(e.w[1])
								sum1 += uint32(weightsUQ[e.idx[2]]) * uint32(e.w[2])
								sum1 += uint32(weightsUQ[e.idx[3]]) * uint32(e.w[3])
								w1 := int32(sum1 >> 4)

								sum2 := uint32(8)
								sum2 += uint32(weightsUQ[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
								sum2 += uint32(weightsUQ[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
								sum2 += uint32(weightsUQ[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
								sum2 += uint32(weightsUQ[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])
								w2 := int32(sum2 >> 4)

								part := int(assign[t])
								e0 := evalEp0[part]
								d := evalEpd[part]
								off := t * 4

								r16 := e0[0] + ((d[0]*w1 + 32) >> 6)
								g16 := e0[1] + ((d[1]*w1 + 32) >> 6)
								b16 := e0[2] + ((d[2]*w1 + 32) >> 6)
								a16 := e0[3] + ((d[3]*w2 + 32) >> 6)
								if useU8 {
									r16 = u16ToU8ReplicatedI32(r16)
									g16 = u16ToU8ReplicatedI32(g16)
									b16 = u16ToU8ReplicatedI32(b16)
									a16 = u16ToU8ReplicatedI32(a16)
								}
								if normalMap {
									r8 := uint8(r16 >> 8)
									a8 := uint8(a16 >> 8)
									errv += normalMapAngularError(texels[off+0], texels[off+3], r8, a8)
								} else if rgbmMap {
									if a16 == 0 {
										errv = math.Inf(1)
										break
									}

									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									srcR := float64(srcR16) * float64(srcA16) * rgbmScale64
									srcG := float64(srcG16) * float64(srcA16) * rgbmScale64
									srcB := float64(srcB16) * float64(srcA16) * rgbmScale64
									decR := float64(r16) * float64(a16) * rgbmScale64
									decG := float64(g16) * float64(a16) * rgbmScale64
									decB := float64(b16) * float64(a16) * rgbmScale64

									errR := math.Abs(srcR - decR)
									errG := math.Abs(srcG - decG)
									errB := math.Abs(srcB - decB)

									if errR > 1e15 {
										errR = 1e15
									}
									if errG > 1e15 {
										errG = 1e15
									}
									if errB > 1e15 {
										errB = 1e15
									}

									errTex := wR*errR*errR + wG*errG*errG + wB*errB*errB
									if errTex > 1e30 {
										errTex = 1e30
									}
									errv += errTex
								} else {
									srcR16 := u8ToU16ReplicatedI32(texels[off+0])
									srcG16 := u8ToU16ReplicatedI32(texels[off+1])
									srcB16 := u8ToU16ReplicatedI32(texels[off+2])
									srcA16 := u8ToU16ReplicatedI32(texels[off+3])

									dr := float64(srcR16 - r16)
									dg := float64(srcG16 - g16)
									db := float64(srcB16 - b16)
									da := float64(srcA16 - a16)

									errv += wR*dr*dr + wG*dg*dg + wB*db*db + wA*da*da
								}

								if errv >= bestErr {
									break
								}
							}
						}
					}
				}

				if errv < bestErr {
					bestErr = errv
					bestMode = mode
					bestPartitionCount = partitionCount
					bestPartitionIndex = partitionIndex
					bestPlane2Component = plane2Component
					bestColorQuant = colorQuant
					bestEndpointLen = partitionCount * endpointStride
					bestWeightLen = realWeightCount
					currEndpointPquantBuf, bestEndpointPquantBuf = bestEndpointPquantBuf, currEndpointPquantBuf
					currWeightPquantBuf, bestWeightPquantBuf = bestWeightPquantBuf, currWeightPquantBuf

					if bestErr == 0 {
						block, err := buildPhysicalBlock(bestMode, blockX, blockY, blockZ, bestPartitionCount, bestPartitionIndex, bestPlane2Component, endpointFormat, bestColorQuant, bestEndpointPquantBuf[:bestEndpointLen], bestWeightPquantBuf[:bestWeightLen])
						if err != nil {
							break
						}
						return block, nil
					}
				}
			}
		}
	}

	if math.IsInf(bestErr, 1) {
		// Fallback: constant average.
		r, g, b, a := avgBlockRGBA8(texels, blockX, blockY*blockZ, 0, 0, blockX, blockY*blockZ)
		return EncodeConstBlockRGBA8(r, g, b, a), nil
	}
	block, err := buildPhysicalBlock(bestMode, blockX, blockY, blockZ, bestPartitionCount, bestPartitionIndex, bestPlane2Component, endpointFormat, bestColorQuant, bestEndpointPquantBuf[:bestEndpointLen], bestWeightPquantBuf[:bestWeightLen])
	if err != nil {
		r, g, b, a := avgBlockRGBA8(texels, blockX, blockY*blockZ, 0, 0, blockX, blockY*blockZ)
		return EncodeConstBlockRGBA8(r, g, b, a), nil
	}
	return block, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func u8ToU16ReplicatedI32(v uint8) int32 {
	return int32(v) * 257
}

func clampU16I32(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 0xFFFF {
		return 0xFFFF
	}
	return v
}

func u16ToU8ReplicatedI32(v int32) int32 {
	v = clampU16I32(v)
	return (v >> 8) * 257
}
