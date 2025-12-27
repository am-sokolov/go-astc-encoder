package astc

import (
	"math"
	"runtime"
)

// ConfigInit populates a Config using defaults equivalent to upstream astcenc_config_init.
func ConfigInit(profile Profile, blockX, blockY, blockZ int, quality float32, flags Flags) (Config, error) {
	if blockZ == 0 {
		// Upstream accepts Z==0 for 2D and normalizes to 1.
		blockZ = 1
	}

	if quality < 0 || quality > 100 {
		return Config{}, newError(ErrBadQuality, "astc: invalid quality")
	}
	if err := validateBlockSize(blockX, blockY, blockZ); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Profile: profile,
		Flags:   flags,
		BlockX:  uint32(blockX),
		BlockY:  uint32(blockY),
		BlockZ:  uint32(blockZ),

		// Defaults; may be overridden by profile/flags below.
		CWRWeight: 1,
		CWGWeight: 1,
		CWBWeight: 1,
		CWAWeight: 1,
	}

	if err := validateProfile(profile); err != nil {
		return Config{}, err
	}
	if err := validateFlags(profile, flags); err != nil {
		return Config{}, err
	}

	texels := float64(blockX * blockY * blockZ)
	ltexels := math.Log10(texels)

	// Pick the preset table based on texel count.
	presets := presetConfigsLow
	if texels < 25 {
		presets = presetConfigsHigh
	} else if texels < 64 {
		presets = presetConfigsMid
	}

	// Determine which preset nodes to use (or interpolate between).
	end := 0
	for end < len(presets) && presets[end].quality < quality {
		end++
	}
	start := 0
	if end > 0 {
		start = end - 1
	}
	if end >= len(presets) {
		end = len(presets) - 1
		start = end
	}

	if start == end {
		cfg.TunePartitionCountLimit = presets[start].tunePartitionCountLimit
		cfg.Tune2PartitionIndexLimit = presets[start].tune2PartitionIndexLimit
		cfg.Tune3PartitionIndexLimit = presets[start].tune3PartitionIndexLimit
		cfg.Tune4PartitionIndexLimit = presets[start].tune4PartitionIndexLimit
		cfg.TuneBlockModeLimit = presets[start].tuneBlockModeLimit
		cfg.TuneRefinementLimit = presets[start].tuneRefinementLimit
		cfg.TuneCandidateLimit = presets[start].tuneCandidateLimit
		cfg.Tune2PartitioningCandidateLimit = presets[start].tune2PartitioningCandidateLimit
		cfg.Tune3PartitioningCandidateLimit = presets[start].tune3PartitioningCandidateLimit
		cfg.Tune4PartitioningCandidateLimit = presets[start].tune4PartitioningCandidateLimit

		cfg.TuneDBLimit = float32(math.Max(
			float64(presets[start].tuneDBLimitABase)-35*ltexels,
			float64(presets[start].tuneDBLimitBBase)-19*ltexels,
		))

		cfg.TuneMSEOvershoot = presets[start].tuneMSEOvershoot
		cfg.Tune2PartitionEarlyOutLimitFactor = presets[start].tune2PartitionEarlyOutLimitFactor
		cfg.Tune3PartitionEarlyOutLimitFactor = presets[start].tune3PartitionEarlyOutLimitFactor
		cfg.Tune2PlaneEarlyOutLimitCorrelation = presets[start].tune2PlaneEarlyOutLimitCorrelation
		cfg.TuneSearchMode0Enable = presets[start].tuneSearchMode0Enable
	} else {
		a := presets[start]
		b := presets[end]
		wtRange := b.quality - a.quality
		if wtRange <= 0 {
			return Config{}, newError(ErrBadQuality, "astc: invalid quality preset table")
		}

		wtA := (b.quality - quality) / wtRange
		wtB := (quality - a.quality) / wtRange

		lerp := func(av, bv float32) float32 { return av*wtA + bv*wtB }
		lerpi := func(av, bv uint32) uint32 {
			v := float32(av)*wtA + float32(bv)*wtB
			return uint32(int(v + 0.5))
		}

		cfg.TunePartitionCountLimit = lerpi(a.tunePartitionCountLimit, b.tunePartitionCountLimit)
		cfg.Tune2PartitionIndexLimit = lerpi(a.tune2PartitionIndexLimit, b.tune2PartitionIndexLimit)
		cfg.Tune3PartitionIndexLimit = lerpi(a.tune3PartitionIndexLimit, b.tune3PartitionIndexLimit)
		cfg.Tune4PartitionIndexLimit = lerpi(a.tune4PartitionIndexLimit, b.tune4PartitionIndexLimit)
		cfg.TuneBlockModeLimit = lerpi(a.tuneBlockModeLimit, b.tuneBlockModeLimit)
		cfg.TuneRefinementLimit = lerpi(a.tuneRefinementLimit, b.tuneRefinementLimit)
		cfg.TuneCandidateLimit = lerpi(a.tuneCandidateLimit, b.tuneCandidateLimit)
		cfg.Tune2PartitioningCandidateLimit = lerpi(a.tune2PartitioningCandidateLimit, b.tune2PartitioningCandidateLimit)
		cfg.Tune3PartitioningCandidateLimit = lerpi(a.tune3PartitioningCandidateLimit, b.tune3PartitioningCandidateLimit)
		cfg.Tune4PartitioningCandidateLimit = lerpi(a.tune4PartitioningCandidateLimit, b.tune4PartitioningCandidateLimit)

		cfg.TuneDBLimit = float32(math.Max(
			float64(lerp(a.tuneDBLimitABase, b.tuneDBLimitABase))-35*ltexels,
			float64(lerp(a.tuneDBLimitBBase, b.tuneDBLimitBBase))-19*ltexels,
		))

		cfg.TuneMSEOvershoot = lerp(a.tuneMSEOvershoot, b.tuneMSEOvershoot)
		cfg.Tune2PartitionEarlyOutLimitFactor = lerp(a.tune2PartitionEarlyOutLimitFactor, b.tune2PartitionEarlyOutLimitFactor)
		cfg.Tune3PartitionEarlyOutLimitFactor = lerp(a.tune3PartitionEarlyOutLimitFactor, b.tune3PartitionEarlyOutLimitFactor)
		cfg.Tune2PlaneEarlyOutLimitCorrelation = lerp(a.tune2PlaneEarlyOutLimitCorrelation, b.tune2PlaneEarlyOutLimitCorrelation)
		cfg.TuneSearchMode0Enable = lerp(a.tuneSearchMode0Enable, b.tuneSearchMode0Enable)
	}

	// Profile-specific defaults.
	switch profile {
	case ProfileLDR, ProfileLDRSRGB:
		// LDR defaults are fine.
	case ProfileHDRRGBLDRAlpha, ProfileHDR:
		cfg.TuneDBLimit = 999.0
		cfg.TuneSearchMode0Enable = 0
	default:
		return Config{}, newError(ErrBadProfile, "astc: invalid profile")
	}

	// Flag-specific defaults.
	if (flags & FlagMapNormal) != 0 {
		// Normal map encoding uses L+A blocks, so allow one more partitioning than normal.
		if cfg.TunePartitionCountLimit < 4 {
			cfg.TunePartitionCountLimit++
		}

		cfg.CWGWeight = 0
		cfg.CWBWeight = 0
		cfg.Tune2PartitionEarlyOutLimitFactor *= 1.5
		cfg.Tune3PartitionEarlyOutLimitFactor *= 1.5
		cfg.Tune2PlaneEarlyOutLimitCorrelation = 0.99

		// Normals are prone to blocking artifacts on smooth curves so try harder.
		cfg.TuneDBLimit *= 1.03
	} else if (flags & FlagMapRGBM) != 0 {
		cfg.RGBMMScale = 5.0
		cfg.CWAWeight = 2.0 * cfg.RGBMMScale
	} else {
		// Perceptual weights for RGB color data.
		if (flags & FlagUsePerceptual) != 0 {
			cfg.CWRWeight = 0.30 * 2.25
			cfg.CWGWeight = 0.59 * 2.25
			cfg.CWBWeight = 0.11 * 2.25
		}
	}

	return cfg, nil
}

// ContextAlloc creates a reusable codec context based on a config, mirroring upstream
// astcenc_context_alloc semantics.
func ContextAlloc(cfg *Config, threadCount int) (*Context, error) {
	if cfg == nil {
		return nil, newError(ErrBadParam, "astc: nil config")
	}
	if threadCount <= 0 {
		return nil, newError(ErrBadParam, "astc: invalid thread count")
	}

	blockX := int(cfg.BlockX)
	blockY := int(cfg.BlockY)
	blockZ := int(cfg.BlockZ)
	if err := validateBlockSize(blockX, blockY, blockZ); err != nil {
		return nil, err
	}

	// Copy config for context internal use and validate+clamp it (matches upstream).
	cfgi := *cfg
	if err := validateAndClampConfig(&cfgi); err != nil {
		return nil, err
	}

	ctx := &Context{
		cfg:         cfgi,
		threadCount: threadCount,
		blockX:      blockX,
		blockY:      blockY,
		blockZ:      blockZ,
		decodeCtx:   getDecodeContext(blockX, blockY, blockZ),
	}
	ctx.state.Store(uint32(ctxIdle))

	// Contexts configured for single threaded use implicitly reset between images.
	// We implement this by starting in a "reset" state.
	ctx.compress.needsReset.Store(0)
	ctx.decompress.needsReset.Store(0)

	// Mirror upstream: convert TuneDBLimit from dB to linear MSE threshold for LDR profiles.
	ctx.tuneDBLimitInternal = cfgi.TuneDBLimit
	if (cfgi.Profile == ProfileLDR) || (cfgi.Profile == ProfileLDRSRGB) {
		ctx.tuneDBLimitInternal = float32(math.Pow(0.1, float64(cfgi.TuneDBLimit)*0.1) * 65535.0 * 65535.0)
	} else {
		ctx.tuneDBLimitInternal = 0
	}

	return ctx, nil
}

func (c *Context) Close() error {
	if c == nil {
		return nil
	}
	// Pure-Go context has no external resources; keep Close for API parity.
	return nil
}

func (c *Context) CompressImage(img *Image, swizzle Swizzle, out []byte, threadIndex int) error {
	if c == nil {
		return newError(ErrBadContext, "astc: nil context")
	}
	if img == nil {
		return newError(ErrBadParam, "astc: nil image")
	}
	if c.cfg.Flags&FlagDecompressOnly != 0 {
		return newError(ErrBadContext, "astc: context is decompress-only")
	}
	if threadIndex < 0 || threadIndex >= c.threadCount {
		return newError(ErrBadParam, "astc: invalid thread index")
	}
	if err := validateCompressionSwizzle(swizzle); err != nil {
		return err
	}

	// Single-threaded contexts implicitly reset between images (matches upstream).
	if c.threadCount == 1 {
		_ = c.CompressReset()
	}

	inType, err := validateImageIn(img)
	if err != nil {
		return err
	}

	// Encode blocks using dynamic scheduling.
	blockX, blockY, blockZ := c.blockX, c.blockY, c.blockZ
	blocksX := (img.DimX + blockX - 1) / blockX
	blocksY := (img.DimY + blockY - 1) / blockY
	blocksZ := (img.DimZ + blockZ - 1) / blockZ
	if blocksX <= 0 || blocksY <= 0 || blocksZ <= 0 {
		return newError(ErrBadParam, "astc: invalid image dimensions")
	}
	totalBlocks := blocksX * blocksY * blocksZ
	needOut := totalBlocks * BlockBytes
	if len(out) < needOut {
		return newError(ErrOutOfMem, "astc: output buffer too small")
	}

	if err := c.beginCompress(uint32(totalBlocks), img, swizzle, inType); err != nil {
		return err
	}
	defer c.endCompress()

	planeBlocks := blocksX * blocksY

	texelCount := blockX * blockY * blockZ
	u8BlockTexels := make([]byte, texelCount*4)
	f32BlockTexels := make([]float32, texelCount*4)

	quality := encodeQualityFromConfig(c.cfg)
	baseWeight := [4]float32{c.cfg.CWRWeight, c.cfg.CWGWeight, c.cfg.CWBWeight, c.cfg.CWAWeight}
	tune := encoderTuningFromConfig(c.cfg)

	total := int(c.compress.totalBlocks.Load())
	for {
		if c.compress.cancel.Load() != 0 {
			break
		}
		i := int(c.compress.nextBlock.Add(1) - 1)
		if i < 0 || i >= total {
			break
		}

		bz := i / planeBlocks
		rem := i - bz*planeBlocks
		by := rem / blocksX
		bx := rem - by*blocksX

		x0 := bx * blockX
		y0 := by * blockY
		z0 := bz * blockZ

		dstOff := i * BlockBytes
		dst := out[dstOff : dstOff+BlockBytes]

		var blk [BlockBytes]byte
		useFullBlock := true
		if c.cfg.AScaleRadius != 0 && blockZ == 1 {
			switch swizzle.A {
			case Swz1:
				useFullBlock = true
			case Swz0:
				useFullBlock = false
			default:
				alphaAverages := c.compress.inputAlphaAverages
				if alphaAverages != nil {
					startX := x0
					endX := x0 + blockX
					if endX > img.DimX {
						endX = img.DimX
					}
					startY := y0
					endY := y0 + blockY
					if endY > img.DimY {
						endY = img.DimY
					}

					ext := int(c.cfg.AScaleRadius) - 1
					if ext < 0 {
						ext = 0
					}
					xFootprint := blockX + 2*ext
					yFootprint := blockY + 2*ext
					footprint := float32(xFootprint * yFootprint)
					threshold := float32(0.0)
					if footprint > 0 {
						threshold = 0.9 / (255.0 * footprint)
					}

					useFullBlock = false
					zBase := z0 * img.DimY * img.DimX
					for ay := startY; ay < endY && !useFullBlock; ay++ {
						rowBase := zBase + ay*img.DimX
						for ax := startX; ax < endX; ax++ {
							if alphaAverages[rowBase+ax] > threshold {
								useFullBlock = true
								break
							}
						}
					}
				}
			}
		}

		if !useFullBlock {
			if c.cfg.Profile == ProfileLDR || c.cfg.Profile == ProfileLDRSRGB {
				blk = EncodeConstBlockRGBA8(0, 0, 0, 0)
			} else {
				blk = EncodeConstBlockF16(0, 0, 0, 0)
			}
			err = nil
		} else {
			switch inType {
			case TypeU8:
				extractBlockRGBA8Volume(img.DataU8, img.DimX, img.DimY, img.DimZ, x0, y0, z0, blockX, blockY, blockZ, u8BlockTexels)
				applySwizzleRGBA8InPlace(u8BlockTexels[:texelCount*4], swizzle)

				blockWeight := baseWeight
				if (c.cfg.Flags & FlagUseAlphaWeight) != 0 {
					maxA := uint8(0)
					for t := 0; t < texelCount; t++ {
						a := u8BlockTexels[t*4+3]
						if a > maxA {
							maxA = a
						}
					}
					alphaScale := float32(maxA) * (1.0 / 255.0)
					blockWeight[0] *= alphaScale
					blockWeight[1] *= alphaScale
					blockWeight[2] *= alphaScale
				}

				blk, err = encodeBlockRGBA8LDR(c.cfg.Profile, blockX, blockY, blockZ, u8BlockTexels[:texelCount*4], quality, blockWeight, c.cfg.Flags, c.cfg.RGBMMScale, &tune)
			case TypeF16:
				extractBlockRGBAF16ToF32Volume(img.DataF16, img.DimX, img.DimY, img.DimZ, x0, y0, z0, blockX, blockY, blockZ, f32BlockTexels)
				applySwizzleRGBAF32InPlace(f32BlockTexels[:texelCount*4], swizzle)

				blockWeight := baseWeight
				if (c.cfg.Flags & FlagUseAlphaWeight) != 0 {
					alphaScale := float32(0)
					if c.cfg.Profile == ProfileHDR {
						maxCode := uint16(0)
						for t := 0; t < texelCount; t++ {
							code := hdrTexelToLNS(f32BlockTexels[t*4+3])
							if code > maxCode {
								maxCode = code
							}
						}
						alphaScale = float32(maxCode) * (1.0 / 65535.0)
					} else {
						for t := 0; t < texelCount; t++ {
							a := f32BlockTexels[t*4+3]
							if a > alphaScale {
								alphaScale = a
							}
						}
						if !(alphaScale >= 0) {
							alphaScale = 0
						}
					}
					blockWeight[0] *= alphaScale
					blockWeight[1] *= alphaScale
					blockWeight[2] *= alphaScale
				}

				blk, err = encodeBlockForF32Input(c.cfg.Profile, blockX, blockY, blockZ, f32BlockTexels[:texelCount*4], quality, blockWeight, c.cfg.Flags, c.cfg.RGBMMScale, &tune)
			case TypeF32:
				extractBlockRGBAF32Volume(img.DataF32, img.DimX, img.DimY, img.DimZ, x0, y0, z0, blockX, blockY, blockZ, f32BlockTexels)
				applySwizzleRGBAF32InPlace(f32BlockTexels[:texelCount*4], swizzle)

				blockWeight := baseWeight
				if (c.cfg.Flags & FlagUseAlphaWeight) != 0 {
					alphaScale := float32(0)
					if c.cfg.Profile == ProfileHDR {
						maxCode := uint16(0)
						for t := 0; t < texelCount; t++ {
							code := hdrTexelToLNS(f32BlockTexels[t*4+3])
							if code > maxCode {
								maxCode = code
							}
						}
						alphaScale = float32(maxCode) * (1.0 / 65535.0)
					} else {
						for t := 0; t < texelCount; t++ {
							a := f32BlockTexels[t*4+3]
							if a > alphaScale {
								alphaScale = a
							}
						}
						if !(alphaScale >= 0) {
							alphaScale = 0
						}
					}
					blockWeight[0] *= alphaScale
					blockWeight[1] *= alphaScale
					blockWeight[2] *= alphaScale
				}

				blk, err = encodeBlockForF32Input(c.cfg.Profile, blockX, blockY, blockZ, f32BlockTexels[:texelCount*4], quality, blockWeight, c.cfg.Flags, c.cfg.RGBMMScale, &tune)
			default:
				return newError(ErrBadParam, "astc: unsupported image data type")
			}
		}

		if err != nil {
			return err
		}
		copy(dst, blk[:])

		done := c.compress.doneBlocks.Add(1)
		c.maybeReportProgress(done, uint32(total), c.cfg.ProgressCallback)
	}

	return nil
}

func (c *Context) CompressReset() error {
	if c == nil {
		return newError(ErrBadContext, "astc: nil context")
	}
	if c.compress.workers.Load() != 0 {
		return newError(ErrBadContext, "astc: compress reset while compress active")
	}
	c.compress.needsReset.Store(0)
	c.compress.cancel.Store(0)
	c.compress.initState.Store(0)
	return nil
}

func (c *Context) CompressCancel() error {
	if c == nil {
		return newError(ErrBadContext, "astc: nil context")
	}
	c.compress.cancel.Store(1)
	return nil
}

func (c *Context) DecompressImage(data []byte, imgOut *Image, swizzle Swizzle, threadIndex int) error {
	if c == nil {
		return newError(ErrBadContext, "astc: nil context")
	}
	if imgOut == nil {
		return newError(ErrBadParam, "astc: nil output image")
	}
	if len(data) == 0 {
		return newError(ErrBadParam, "astc: empty input data")
	}
	if threadIndex < 0 || threadIndex >= c.threadCount {
		return newError(ErrBadParam, "astc: invalid thread index")
	}
	if err := validateDecompressionSwizzle(swizzle); err != nil {
		return err
	}

	// Single-threaded contexts implicitly reset between images (matches upstream).
	if c.threadCount == 1 {
		_ = c.DecompressReset()
	}

	if _, err := validateImageOut(imgOut); err != nil {
		return err
	}

	blockX, blockY, blockZ := c.blockX, c.blockY, c.blockZ
	blocksX := (imgOut.DimX + blockX - 1) / blockX
	blocksY := (imgOut.DimY + blockY - 1) / blockY
	blocksZ := (imgOut.DimZ + blockZ - 1) / blockZ
	if blocksX <= 0 || blocksY <= 0 || blocksZ <= 0 {
		return newError(ErrBadParam, "astc: invalid image dimensions")
	}
	totalBlocks := blocksX * blocksY * blocksZ
	needBlocks := totalBlocks * BlockBytes
	if len(data) < needBlocks {
		return newError(ErrOutOfMem, "astc: block buffer too small")
	}

	if err := c.beginDecompress(uint32(totalBlocks)); err != nil {
		return err
	}
	defer c.endDecompress()

	planeBlocks := blocksX * blocksY

	texelCount := blockX * blockY * blockZ
	u8Decoded := make([]byte, texelCount*4)
	f32Decoded := make([]float32, texelCount*4)

	// All threads run until no work remaining.
	total := int(c.decompress.totalBlocks.Load())
	for {
		i := int(c.decompress.nextBlock.Add(1) - 1)
		if i < 0 || i >= total {
			break
		}

		bz := i / planeBlocks
		rem := i - bz*planeBlocks
		by := rem / blocksX
		bx := rem - by*blocksX

		x0 := bx * blockX
		y0 := by * blockY
		z0 := bz * blockZ

		srcOff := i * BlockBytes
		block := data[srcOff : srcOff+BlockBytes]

		switch imgOut.DataType {
		case TypeU8:
			if c.cfg.Profile == ProfileLDR || c.cfg.Profile == ProfileLDRSRGB {
				decodeBlockToRGBA8(c.cfg.Profile, c.decodeCtx, block, u8Decoded)
			} else {
				// HDR decode to U8: decode to float and quantize.
				decodeBlockToRGBAF32(c.cfg.Profile, c.decodeCtx, block, f32Decoded)
				quantizeRGBAF32ToU8(f32Decoded, u8Decoded)
			}
			applySwizzleRGBA8InPlace(u8Decoded[:texelCount*4], swizzle)
			storeBlockRGBA8Volume(imgOut.DataU8, imgOut.DimX, imgOut.DimY, imgOut.DimZ, x0, y0, z0, blockX, blockY, blockZ, u8Decoded)
		case TypeF32:
			decodeBlockToRGBAF32(c.cfg.Profile, c.decodeCtx, block, f32Decoded)
			applySwizzleRGBAF32InPlace(f32Decoded[:texelCount*4], swizzle)
			storeBlockRGBAF32Volume(imgOut.DataF32, imgOut.DimX, imgOut.DimY, imgOut.DimZ, x0, y0, z0, blockX, blockY, blockZ, f32Decoded)
		case TypeF16:
			decodeBlockToRGBAF32(c.cfg.Profile, c.decodeCtx, block, f32Decoded)
			applySwizzleRGBAF32InPlace(f32Decoded[:texelCount*4], swizzle)
			storeBlockRGBAF32AsF16Volume(imgOut.DataF16, imgOut.DimX, imgOut.DimY, imgOut.DimZ, x0, y0, z0, blockX, blockY, blockZ, f32Decoded)
		default:
			return newError(ErrBadParam, "astc: unsupported output image type")
		}
	}

	return nil
}

func (c *Context) DecompressReset() error {
	if c == nil {
		return newError(ErrBadContext, "astc: nil context")
	}
	if c.decompress.workers.Load() != 0 {
		return newError(ErrBadContext, "astc: decompress reset while decompress active")
	}
	c.decompress.needsReset.Store(0)
	c.decompress.initState.Store(0)
	return nil
}

func (c *Context) GetBlockInfo(block [BlockBytes]byte) (BlockInfo, error) {
	if c == nil {
		return BlockInfo{}, newError(ErrBadContext, "astc: nil context")
	}

	info := BlockInfo{}
	info.Profile = c.cfg.Profile
	info.BlockX = uint32(c.blockX)
	info.BlockY = uint32(c.blockY)
	info.BlockZ = uint32(c.blockZ)
	info.TexelCount = uint32(c.blockX * c.blockY * c.blockZ)

	scb := physicalToSymbolicWithCtx(block[:], c.decodeCtx)
	info.IsErrorBlock = scb.blockType == symBlockError
	if info.IsErrorBlock {
		return info, nil
	}

	info.IsConstantBlock = scb.blockType == symBlockConstU16 || scb.blockType == symBlockConstF16
	if info.IsConstantBlock {
		return info, nil
	}

	bmi := c.decodeCtx.blockModes[scb.blockMode]
	if !bmi.ok {
		info.IsErrorBlock = true
		return info, nil
	}

	info.IsDualPlaneBlock = bmi.isDualPlane
	info.PartitionCount = uint32(scb.partitionCount)
	info.PartitionIndex = uint32(scb.partitionIndex)
	info.DualPlaneComponent = uint32(scb.plane2Component)

	info.WeightX = uint32(bmi.xWeights)
	info.WeightY = uint32(bmi.yWeights)
	info.WeightZ = uint32(bmi.zWeights)

	info.ColorLevelCount = uint32(quantLevel(scb.quantMode))
	info.WeightLevelCount = uint32(quantLevel(bmi.weightQuant))

	// Unpack color endpoints.
	for p := 0; p < int(scb.partitionCount); p++ {
		format := scb.colorFormats[p]
		info.ColorEndpointModes[p] = uint32(format)

		rgbHDR, alphaHDR, e0, e1 := unpackColorEndpoints(c.cfg.Profile, format, scb.colorValues[p][:])
		info.IsHDRBlock = info.IsHDRBlock || rgbHDR || alphaHDR

		for j := 0; j < 2; j++ {
			var v int4
			if j == 0 {
				v = e0
			} else {
				v = e1
			}
			for cch := 0; cch < 4; cch++ {
				u := uint16(v[cch])
				if cch < 3 {
					if rgbHDR {
						info.ColorEndpoints[p][j][cch] = lnsToFloat32Table[u]
					} else {
						info.ColorEndpoints[p][j][cch] = unorm16ToFloat32Table[u]
					}
				} else {
					if alphaHDR {
						info.ColorEndpoints[p][j][cch] = lnsToFloat32Table[u]
					} else {
						info.ColorEndpoints[p][j][cch] = unorm16ToFloat32Table[u]
					}
				}
			}
		}
	}

	// Unpack per-texel weights.
	texelCount := c.decodeCtx.texelCount
	if bmi.noDecimation {
		for t := 0; t < texelCount; t++ {
			info.WeightValuesPlane1[t] = float32(scb.weights[t]) * (1.0 / 64.0)
			if info.IsDualPlaneBlock {
				info.WeightValuesPlane2[t] = float32(scb.weights[weightsPlane2Offset+t]) * (1.0 / 64.0)
			}
		}
	} else {
		dec := bmi.decimation
		wvals := scb.weights[:]
		for t := 0; t < texelCount; t++ {
			e := dec[t]
			sum1 := uint32(8)
			sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
			sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
			sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
			sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])
			w1 := int(sum1 >> 4)
			info.WeightValuesPlane1[t] = float32(w1) * (1.0 / 64.0)

			if info.IsDualPlaneBlock {
				sum2 := uint32(8)
				sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
				sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
				sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
				sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])
				w2 := int(sum2 >> 4)
				info.WeightValuesPlane2[t] = float32(w2) * (1.0 / 64.0)
			}
		}
	}

	// Unpack partition assignments.
	if pc := int(scb.partitionCount); pc >= 2 && pc <= blockMaxPartitions {
		if pt := c.decodeCtx.partitionTables[pc]; pt != nil {
			assign := pt.partitionsForIndex(int(scb.partitionIndex))
			for t := 0; t < texelCount; t++ {
				info.PartitionAssignment[t] = assign[t]
			}
		}
	}

	return info, nil
}

// -----------------------------------------------------------------------------
// Pure-Go helpers (ported from upstream config init/validation logic)
// -----------------------------------------------------------------------------

type presetConfig struct {
	quality float32

	tunePartitionCountLimit            uint32
	tune2PartitionIndexLimit           uint32
	tune3PartitionIndexLimit           uint32
	tune4PartitionIndexLimit           uint32
	tuneBlockModeLimit                 uint32
	tuneRefinementLimit                uint32
	tuneCandidateLimit                 uint32
	tune2PartitioningCandidateLimit    uint32
	tune3PartitioningCandidateLimit    uint32
	tune4PartitioningCandidateLimit    uint32
	tuneDBLimitABase                   float32
	tuneDBLimitBBase                   float32
	tuneMSEOvershoot                   float32
	tune2PartitionEarlyOutLimitFactor  float32
	tune3PartitionEarlyOutLimitFactor  float32
	tune2PlaneEarlyOutLimitCorrelation float32
	tuneSearchMode0Enable              float32
}

var presetConfigsHigh = []presetConfig{
	{0, 2, 10, 6, 4, 43, 2, 2, 2, 2, 2, 85.2, 63.2, 3.5, 1.0, 1.0, 0.85, 0.0},
	{10, 3, 18, 10, 8, 55, 3, 3, 2, 2, 2, 85.2, 63.2, 3.5, 1.0, 1.0, 0.90, 0.0},
	{60, 4, 34, 28, 16, 77, 3, 3, 2, 2, 2, 95.0, 70.0, 2.5, 1.1, 1.05, 0.95, 0.0},
	{98, 4, 82, 60, 30, 94, 4, 4, 3, 2, 2, 105.0, 77.0, 10.0, 1.35, 1.15, 0.97, 0.0},
	{99, 4, 256, 128, 64, 98, 4, 6, 8, 6, 4, 200.0, 200.0, 10.0, 1.6, 1.4, 0.98, 0.0},
	{100, 4, 512, 512, 512, 100, 4, 8, 8, 8, 8, 200.0, 200.0, 10.0, 2.0, 2.0, 0.99, 0.0},
}

var presetConfigsMid = []presetConfig{
	{0, 2, 10, 6, 4, 43, 2, 2, 2, 2, 2, 85.2, 63.2, 3.5, 1.0, 1.0, 0.80, 1.0},
	{10, 3, 18, 12, 10, 55, 3, 3, 2, 2, 2, 85.2, 63.2, 3.5, 1.0, 1.0, 0.85, 1.0},
	{60, 3, 34, 28, 16, 77, 3, 3, 2, 2, 2, 95.0, 70.0, 3.0, 1.1, 1.05, 0.90, 1.0},
	{98, 4, 82, 60, 30, 94, 4, 4, 3, 2, 2, 105.0, 77.0, 10.0, 1.4, 1.2, 0.95, 0.0},
	{99, 4, 256, 128, 64, 98, 4, 6, 8, 6, 3, 200.0, 200.0, 10.0, 1.6, 1.4, 0.98, 0.0},
	{100, 4, 256, 256, 256, 100, 4, 8, 8, 8, 8, 200.0, 200.0, 10.0, 2.0, 2.0, 0.99, 0.0},
}

var presetConfigsLow = []presetConfig{
	{0, 2, 10, 6, 4, 40, 2, 2, 2, 2, 2, 85.0, 63.0, 3.5, 1.0, 1.0, 0.80, 1.0},
	{10, 2, 18, 12, 10, 55, 3, 3, 2, 2, 2, 85.0, 63.0, 3.5, 1.0, 1.0, 0.85, 1.0},
	{60, 3, 34, 28, 16, 77, 3, 3, 2, 2, 2, 95.0, 70.0, 3.5, 1.1, 1.05, 0.90, 1.0},
	{98, 4, 82, 60, 30, 93, 4, 4, 3, 2, 2, 105.0, 77.0, 10.0, 1.3, 1.2, 0.97, 1.0},
	{99, 4, 256, 128, 64, 98, 4, 6, 8, 5, 2, 200.0, 200.0, 10.0, 1.6, 1.4, 0.98, 1.0},
	{100, 4, 256, 256, 256, 100, 4, 8, 8, 8, 8, 200.0, 200.0, 10.0, 2.0, 2.0, 0.99, 1.0},
}

func validateProfile(profile Profile) error {
	switch profile {
	case ProfileLDR, ProfileLDRSRGB, ProfileHDRRGBLDRAlpha, ProfileHDR:
		return nil
	default:
		return newError(ErrBadProfile, "astc: invalid profile")
	}
}

func validateFlags(profile Profile, flags Flags) error {
	if flags&^FlagAll != 0 {
		return newError(ErrBadFlags, "astc: invalid flags")
	}
	mapMask := FlagMapNormal | FlagMapRGBM
	if bitsOnes32(uint32(flags&mapMask)) > 1 {
		return newError(ErrBadFlags, "astc: invalid flags")
	}
	if (flags & FlagUseDecodeUNORM8) != 0 {
		if profile == ProfileHDR || profile == ProfileHDRRGBLDRAlpha {
			return newError(ErrBadDecodeMode, "astc: invalid decode mode for HDR profile")
		}
	}
	return nil
}

func validateBlockSize(blockX, blockY, blockZ int) error {
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 {
		return newError(ErrBadBlockSize, "astc: invalid block dimensions")
	}
	if blockX > 255 || blockY > 255 || blockZ > 255 {
		return newError(ErrBadBlockSize, "astc: invalid block dimensions")
	}
	if blockX*blockY*blockZ > blockMaxTexels {
		return newError(ErrBadBlockSize, "astc: invalid block dimensions")
	}
	if blockZ <= 1 {
		if !isLegal2DBlockSize(blockX, blockY) {
			return newError(ErrBadBlockSize, "astc: invalid block dimensions")
		}
		return nil
	}
	if !isLegal3DBlockSize(blockX, blockY, blockZ) {
		return newError(ErrBadBlockSize, "astc: invalid block dimensions")
	}
	return nil
}

func isLegal2DBlockSize(xdim, ydim int) bool {
	switch (xdim << 8) | ydim {
	case 0x0404,
		0x0504,
		0x0505,
		0x0605,
		0x0606,
		0x0805,
		0x0806,
		0x0808,
		0x0A05,
		0x0A06,
		0x0A08,
		0x0A0A,
		0x0C0A,
		0x0C0C:
		return true
	default:
		return false
	}
}

func isLegal3DBlockSize(xdim, ydim, zdim int) bool {
	switch (xdim << 16) | (ydim << 8) | zdim {
	case 0x030303,
		0x040303,
		0x040403,
		0x040404,
		0x050404,
		0x050504,
		0x050505,
		0x060505,
		0x060605,
		0x060606:
		return true
	default:
		return false
	}
}

func validateAndClampConfig(cfg *Config) error {
	if cfg == nil {
		return newError(ErrBadParam, "astc: nil config")
	}
	if err := validateProfile(cfg.Profile); err != nil {
		return err
	}
	if err := validateFlags(cfg.Profile, cfg.Flags); err != nil {
		return err
	}
	if err := validateBlockSize(int(cfg.BlockX), int(cfg.BlockY), int(cfg.BlockZ)); err != nil {
		return err
	}

	if cfg.RGBMMScale < 1 {
		cfg.RGBMMScale = 1
	}

	cfg.TunePartitionCountLimit = clampU32(cfg.TunePartitionCountLimit, 1, 4)
	cfg.Tune2PartitionIndexLimit = clampU32(cfg.Tune2PartitionIndexLimit, 1, 1024)
	cfg.Tune3PartitionIndexLimit = clampU32(cfg.Tune3PartitionIndexLimit, 1, 1024)
	cfg.Tune4PartitionIndexLimit = clampU32(cfg.Tune4PartitionIndexLimit, 1, 1024)
	cfg.TuneBlockModeLimit = clampU32(cfg.TuneBlockModeLimit, 1, 100)
	if cfg.TuneRefinementLimit < 1 {
		cfg.TuneRefinementLimit = 1
	}
	cfg.TuneCandidateLimit = clampU32(cfg.TuneCandidateLimit, 1, 8)
	cfg.Tune2PartitioningCandidateLimit = clampU32(cfg.Tune2PartitioningCandidateLimit, 1, 8)
	cfg.Tune3PartitioningCandidateLimit = clampU32(cfg.Tune3PartitioningCandidateLimit, 1, 8)
	cfg.Tune4PartitioningCandidateLimit = clampU32(cfg.Tune4PartitioningCandidateLimit, 1, 8)

	if cfg.TuneDBLimit < 0 {
		cfg.TuneDBLimit = 0
	}
	if cfg.TuneMSEOvershoot < 1 {
		cfg.TuneMSEOvershoot = 1
	}
	if cfg.Tune2PartitionEarlyOutLimitFactor < 0 {
		cfg.Tune2PartitionEarlyOutLimitFactor = 0
	}
	if cfg.Tune3PartitionEarlyOutLimitFactor < 0 {
		cfg.Tune3PartitionEarlyOutLimitFactor = 0
	}
	if cfg.Tune2PlaneEarlyOutLimitCorrelation < 0 {
		cfg.Tune2PlaneEarlyOutLimitCorrelation = 0
	}

	maxWeight := max4(cfg.CWRWeight, cfg.CWGWeight, cfg.CWBWeight, cfg.CWAWeight)
	if !(maxWeight > 0) {
		return newError(ErrBadParam, "astc: invalid component weights")
	}
	minWeight := maxWeight / 1000.0
	if cfg.CWRWeight < minWeight {
		cfg.CWRWeight = minWeight
	}
	if cfg.CWGWeight < minWeight {
		cfg.CWGWeight = minWeight
	}
	if cfg.CWBWeight < minWeight {
		cfg.CWBWeight = minWeight
	}
	if cfg.CWAWeight < minWeight {
		cfg.CWAWeight = minWeight
	}

	return nil
}

func clampU32(v, lo, hi uint32) uint32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max4(a, b, c, d float32) float32 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	if d > m {
		m = d
	}
	return m
}

func bitsOnes32(v uint32) int {
	// Inline popcount to avoid importing math/bits just for one use.
	v = v - ((v >> 1) & 0x55555555)
	v = (v & 0x33333333) + ((v >> 2) & 0x33333333)
	return int((((v + (v >> 4)) & 0x0F0F0F0F) * 0x01010101) >> 24)
}

func validateCompressionSwizzle(swz Swizzle) error {
	// Matches upstream validate_compression_swizzle: SWZ_Z is invalid for compression.
	if swz.R > Swz1 || swz.G > Swz1 || swz.B > Swz1 || swz.A > Swz1 {
		return newError(ErrBadSwizzle, "astc: invalid swizzle")
	}
	return nil
}

func validateDecompressionSwizzle(swz Swizzle) error {
	// Matches upstream validate_decompression_swizzle: SWZ_Z is valid for decompression.
	if swz.R > SwzZ || swz.G > SwzZ || swz.B > SwzZ || swz.A > SwzZ {
		return newError(ErrBadSwizzle, "astc: invalid swizzle")
	}
	return nil
}

func encodeQualityFromConfig(cfg Config) EncodeQuality {
	// Heuristic mapping based on tune_block_mode_limit from upstream presets.
	v := cfg.TuneBlockModeLimit
	switch {
	case v <= 43:
		return EncodeFastest
	case v <= 55:
		return EncodeFast
	case v <= 77:
		return EncodeMedium
	case v <= 94:
		return EncodeThorough
	case v <= 98:
		return EncodeVeryThorough
	default:
		return EncodeExhaustive
	}
}

// -----------------------------------------------------------------------------
// Job scheduling and pixel helpers
// -----------------------------------------------------------------------------

func (c *Context) maybeReportProgress(done, total uint32, cb func(float32)) {
	if cb == nil || total == 0 {
		return
	}

	// Ensure the progress bar hits 100% (matches upstream).
	if done >= total {
		c.compress.progressMu.Lock()
		last := math.Float32frombits(c.compress.progressLastValueBits.Load())
		if last != 100.0 {
			cb(100.0)
			c.compress.progressLastValueBits.Store(math.Float32bits(100.0))
		}
		c.compress.progressMu.Unlock()
		return
	}

	minDiff := math.Float32frombits(c.compress.progressMinDiffBits.Load())
	last := math.Float32frombits(c.compress.progressLastValueBits.Load())
	thisValue := (float32(done) / float32(total)) * 100.0

	// Initial lockless test - have we progressed enough to emit?
	if (thisValue - last) <= minDiff {
		return
	}

	// Recheck under lock, because another thread might report first.
	c.compress.progressMu.Lock()
	last = math.Float32frombits(c.compress.progressLastValueBits.Load())
	if (thisValue - last) > minDiff {
		cb(thisValue)
		c.compress.progressLastValueBits.Store(math.Float32bits(thisValue))
	}
	c.compress.progressMu.Unlock()
}

func (c *Context) beginCompress(totalBlocks uint32, img *Image, swizzle Swizzle, inType DataType) error {
	if c.compress.needsReset.Load() != 0 {
		return newError(ErrBadContext, "astc: compress requires reset")
	}

	for {
		switch contextState(c.state.Load()) {
		case ctxIdle:
			if c.state.CompareAndSwap(uint32(ctxIdle), uint32(ctxCompressActive)) {
				// Acquired.
			} else {
				continue
			}
		case ctxCompressActive:
			// Join.
		default:
			return newError(ErrBadContext, "astc: context busy")
		}
		break
	}

	for {
		st := c.compress.initState.Load()
		if st == 2 {
			break
		}
		if st == 0 && c.compress.initState.CompareAndSwap(0, 1) {
			c.compress.totalBlocks.Store(totalBlocks)
			c.compress.nextBlock.Store(0)
			c.compress.doneBlocks.Store(0)
			c.compress.cancel.Store(0)
			c.compress.inputAlphaAverages = nil

			// Report every 1% or 4096 blocks, whichever is larger (matches upstream).
			minDiff := float32(1.0)
			if totalBlocks != 0 {
				d := (4096.0 / float32(totalBlocks)) * 100.0
				if d > minDiff {
					minDiff = d
				}
			}
			c.compress.progressMinDiffBits.Store(math.Float32bits(minDiff))
			c.compress.progressLastValueBits.Store(math.Float32bits(0.0))

			// Precompute alpha averages for alpha-scale RDO (matches upstream input_alpha_averages).
			if c.cfg.AScaleRadius != 0 && c.blockZ == 1 && swizzle.A != Swz0 && swizzle.A != Swz1 {
				c.compress.inputAlphaAverages = computeInputAlphaAverages(img, inType, swizzle.A, int(c.cfg.AScaleRadius))
			}

			c.compress.initState.Store(2)
			break
		}
		runtime.Gosched()
	}

	c.compress.workers.Add(1)
	return nil
}

func (c *Context) endCompress() {
	if c.compress.workers.Add(-1) != 0 {
		return
	}

	if c.threadCount > 1 {
		c.compress.needsReset.Store(1)
	}

	c.compress.inputAlphaAverages = nil
	c.compress.initState.Store(0)
	c.state.Store(uint32(ctxIdle))
}

func (c *Context) beginDecompress(totalBlocks uint32) error {
	if c.decompress.needsReset.Load() != 0 {
		return newError(ErrBadContext, "astc: decompress requires reset")
	}

	for {
		switch contextState(c.state.Load()) {
		case ctxIdle:
			if c.state.CompareAndSwap(uint32(ctxIdle), uint32(ctxDecompressActive)) {
				// Acquired.
			} else {
				continue
			}
		case ctxDecompressActive:
			// Join.
		default:
			return newError(ErrBadContext, "astc: context busy")
		}
		break
	}

	for {
		st := c.decompress.initState.Load()
		if st == 2 {
			break
		}
		if st == 0 && c.decompress.initState.CompareAndSwap(0, 1) {
			c.decompress.totalBlocks.Store(totalBlocks)
			c.decompress.nextBlock.Store(0)
			c.decompress.doneBlocks.Store(0)
			c.decompress.initState.Store(2)
			break
		}
		runtime.Gosched()
	}

	c.decompress.workers.Add(1)
	return nil
}

func (c *Context) endDecompress() {
	if c.decompress.workers.Add(-1) != 0 {
		return
	}

	if c.threadCount > 1 {
		c.decompress.needsReset.Store(1)
	}

	c.decompress.initState.Store(0)
	c.state.Store(uint32(ctxIdle))
}

func validateImageIn(img *Image) (DataType, error) {
	if img.DimX <= 0 || img.DimY <= 0 || img.DimZ <= 0 {
		return 0, newError(ErrBadParam, "astc: invalid image dimensions")
	}
	texelCount := img.DimX * img.DimY * img.DimZ
	if texelCount <= 0 {
		return 0, newError(ErrBadParam, "astc: invalid image dimensions")
	}

	switch img.DataType {
	case TypeU8:
		if len(img.DataU8) != texelCount*4 {
			return 0, newError(ErrBadParam, "astc: invalid RGBA8 buffer length")
		}
		return TypeU8, nil
	case TypeF16:
		if len(img.DataF16) != texelCount*4 {
			return 0, newError(ErrBadParam, "astc: invalid RGBAF16 buffer length")
		}
		return TypeF16, nil
	case TypeF32:
		if len(img.DataF32) != texelCount*4 {
			return 0, newError(ErrBadParam, "astc: invalid RGBAF32 buffer length")
		}
		return TypeF32, nil
	default:
		return 0, newError(ErrBadParam, "astc: unknown image data type")
	}
}

func validateImageOut(img *Image) (DataType, error) {
	if img.DimX <= 0 || img.DimY <= 0 || img.DimZ <= 0 {
		return 0, newError(ErrBadParam, "astc: invalid image dimensions")
	}
	texelCount := img.DimX * img.DimY * img.DimZ
	if texelCount <= 0 {
		return 0, newError(ErrBadParam, "astc: invalid image dimensions")
	}

	switch img.DataType {
	case TypeU8:
		if len(img.DataU8) != texelCount*4 {
			return 0, newError(ErrBadParam, "astc: invalid RGBA8 buffer length")
		}
		return TypeU8, nil
	case TypeF16:
		if len(img.DataF16) != texelCount*4 {
			return 0, newError(ErrBadParam, "astc: invalid RGBAF16 buffer length")
		}
		return TypeF16, nil
	case TypeF32:
		if len(img.DataF32) != texelCount*4 {
			return 0, newError(ErrBadParam, "astc: invalid RGBAF32 buffer length")
		}
		return TypeF32, nil
	default:
		return 0, newError(ErrBadParam, "astc: unknown image data type")
	}
}

func applySwizzleRGBA8InPlace(pix []byte, swz Swizzle) {
	if swz == SwizzleRGBA {
		return
	}
	for i := 0; i < len(pix); i += 4 {
		r0 := pix[i+0]
		g0 := pix[i+1]
		b0 := pix[i+2]
		a0 := pix[i+3]
		pix[i+0] = swzU8(swz.R, r0, g0, b0, a0)
		pix[i+1] = swzU8(swz.G, r0, g0, b0, a0)
		pix[i+2] = swzU8(swz.B, r0, g0, b0, a0)
		pix[i+3] = swzU8(swz.A, r0, g0, b0, a0)
	}
}

func swzU8(s Swz, r, g, b, a byte) byte {
	switch s {
	case SwzR:
		return r
	case SwzG:
		return g
	case SwzB:
		return b
	case SwzA:
		return a
	case Swz0:
		return 0
	case Swz1:
		return 255
	case SwzZ:
		xN := (float32(r) * (2.0 / 255.0)) - 1.0
		yN := (float32(a) * (2.0 / 255.0)) - 1.0
		zN := 1.0 - xN*xN - yN*yN
		if zN < 0 {
			zN = 0
		}
		z := float32(math.Sqrt(float64(zN)))*0.5 + 0.5
		if z > 1 {
			z = 1
		}
		return uint8(flt2intRTN(z * 255.0))
	default:
		return 0
	}
}

func computeInputAlphaAverages(img *Image, inType DataType, alphaSwz Swz, radius int) []float32 {
	if img == nil || radius <= 0 {
		return nil
	}

	width, height, depth := img.DimX, img.DimY, img.DimZ
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil
	}
	texelCount := width * height * depth
	if texelCount <= 0 {
		return nil
	}

	// Upstream treats dim_z==1 as 2D for averaging purposes (no Z footprint).
	haveZ := depth > 1

	alpha := make([]float32, texelCount)
	switch inType {
	case TypeU8:
		const inv255 = 1.0 / 255.0
		for i := 0; i < texelCount; i++ {
			off := i * 4
			r := img.DataU8[off+0]
			g := img.DataU8[off+1]
			b := img.DataU8[off+2]
			a := img.DataU8[off+3]
			alpha[i] = float32(swzU8(alphaSwz, r, g, b, a)) * inv255
		}
	case TypeF16:
		for i := 0; i < texelCount; i++ {
			off := i * 4
			switch alphaSwz {
			case SwzR:
				alpha[i] = halfToFloat32(img.DataF16[off+0])
			case SwzG:
				alpha[i] = halfToFloat32(img.DataF16[off+1])
			case SwzB:
				alpha[i] = halfToFloat32(img.DataF16[off+2])
			case SwzA:
				alpha[i] = halfToFloat32(img.DataF16[off+3])
			case Swz1:
				alpha[i] = 1
			default:
				alpha[i] = 0
			}
		}
	case TypeF32:
		for i := 0; i < texelCount; i++ {
			off := i * 4
			switch alphaSwz {
			case SwzR:
				alpha[i] = img.DataF32[off+0]
			case SwzG:
				alpha[i] = img.DataF32[off+1]
			case SwzB:
				alpha[i] = img.DataF32[off+2]
			case SwzA:
				alpha[i] = img.DataF32[off+3]
			case Swz1:
				alpha[i] = 1
			default:
				alpha[i] = 0
			}
		}
	default:
		return nil
	}

	rad := radius
	kdim := 2*rad + 1
	if kdim <= 1 {
		return alpha
	}

	tmp := make([]float32, texelCount)

	// Pass 1: X box filter with edge replication.
	planeSize := width * height
	for z := 0; z < depth; z++ {
		zBase := z * planeSize
		for y := 0; y < height; y++ {
			rowBase := zBase + y*width

			sum := float32(0)
			for dx := -rad; dx <= rad; dx++ {
				sx := dx
				if sx < 0 {
					sx = 0
				} else if sx >= width {
					sx = width - 1
				}
				sum += alpha[rowBase+sx]
			}
			tmp[rowBase+0] = sum

			for x := 1; x < width; x++ {
				removeX := x - rad - 1
				if removeX < 0 {
					removeX = 0
				} else if removeX >= width {
					removeX = width - 1
				}
				addX := x + rad
				if addX < 0 {
					addX = 0
				} else if addX >= width {
					addX = width - 1
				}
				sum += alpha[rowBase+addX] - alpha[rowBase+removeX]
				tmp[rowBase+x] = sum
			}
		}
	}

	// Pass 2: Y box filter with edge replication (reuse alpha for output).
	for z := 0; z < depth; z++ {
		zBase := z * planeSize
		for x := 0; x < width; x++ {
			sum := float32(0)
			for dy := -rad; dy <= rad; dy++ {
				sy := dy
				if sy < 0 {
					sy = 0
				} else if sy >= height {
					sy = height - 1
				}
				sum += tmp[zBase+sy*width+x]
			}
			alpha[zBase+0*width+x] = sum

			for y := 1; y < height; y++ {
				removeY := y - rad - 1
				if removeY < 0 {
					removeY = 0
				} else if removeY >= height {
					removeY = height - 1
				}
				addY := y + rad
				if addY < 0 {
					addY = 0
				} else if addY >= height {
					addY = height - 1
				}
				sum += tmp[zBase+addY*width+x] - tmp[zBase+removeY*width+x]
				alpha[zBase+y*width+x] = sum
			}
		}
	}

	// Pass 3: Z box filter for 3D images (output into tmp), otherwise finalize as 2D.
	if haveZ {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				sum := float32(0)
				for dz := -rad; dz <= rad; dz++ {
					sz := dz
					if sz < 0 {
						sz = 0
					} else if sz >= depth {
						sz = depth - 1
					}
					sum += alpha[sz*planeSize+y*width+x]
				}
				tmp[0*planeSize+y*width+x] = sum

				for z := 1; z < depth; z++ {
					removeZ := z - rad - 1
					if removeZ < 0 {
						removeZ = 0
					} else if removeZ >= depth {
						removeZ = depth - 1
					}
					addZ := z + rad
					if addZ < 0 {
						addZ = 0
					} else if addZ >= depth {
						addZ = depth - 1
					}
					sum += alpha[addZ*planeSize+y*width+x] - alpha[removeZ*planeSize+y*width+x]
					tmp[z*planeSize+y*width+x] = sum
				}
			}
		}

		inv := 1.0 / float32(kdim*kdim*kdim)
		for i := range tmp {
			tmp[i] *= inv
		}
		return tmp
	}

	inv := 1.0 / float32(kdim*kdim)
	for i := range alpha {
		alpha[i] *= inv
	}
	return alpha
}

func alphaFootprintHasAnyU8(pix []byte, width, height, depth, x0, y0, z0, blockX, blockY int, aScaleRadius uint32, alphaSwz Swz) bool {
	if alphaSwz == Swz1 {
		return true
	}
	if alphaSwz == Swz0 {
		return false
	}
	if aScaleRadius == 0 {
		return true
	}
	ext := int(aScaleRadius) - 1
	if ext < 0 {
		ext = 0
	}

	startX := x0 - ext
	if startX < 0 {
		startX = 0
	}
	endX := x0 + blockX + ext
	if endX > width {
		endX = width
	}

	startY := y0 - ext
	if startY < 0 {
		startY = 0
	}
	endY := y0 + blockY + ext
	if endY > height {
		endY = height
	}

	z := z0
	if z < 0 {
		z = 0
	} else if z >= depth {
		z = depth - 1
	}

	rowStride := width * 4
	sliceStride := height * rowStride
	sliceBase := z * sliceStride

	for y := startY; y < endY; y++ {
		rowBase := sliceBase + y*rowStride
		for x := startX; x < endX; x++ {
			off := rowBase + x*4
			r := pix[off+0]
			g := pix[off+1]
			b := pix[off+2]
			a := pix[off+3]
			if swzU8(alphaSwz, r, g, b, a) != 0 {
				return true
			}
		}
	}

	return false
}

func applySwizzleRGBAF32InPlace(pix []float32, swz Swizzle) {
	if swz == SwizzleRGBA {
		return
	}
	for i := 0; i < len(pix); i += 4 {
		r0 := pix[i+0]
		g0 := pix[i+1]
		b0 := pix[i+2]
		a0 := pix[i+3]
		pix[i+0] = swzF32(swz.R, r0, g0, b0, a0)
		pix[i+1] = swzF32(swz.G, r0, g0, b0, a0)
		pix[i+2] = swzF32(swz.B, r0, g0, b0, a0)
		pix[i+3] = swzF32(swz.A, r0, g0, b0, a0)
	}
}

func swzF32(s Swz, r, g, b, a float32) float32 {
	switch s {
	case SwzR:
		return r
	case SwzG:
		return g
	case SwzB:
		return b
	case SwzA:
		return a
	case Swz0:
		return 0
	case Swz1:
		return 1
	case SwzZ:
		xN := (r * 2.0) - 1.0
		yN := (a * 2.0) - 1.0
		zN := 1.0 - xN*xN - yN*yN
		if zN < 0 {
			zN = 0
		}
		z := float32(math.Sqrt(float64(zN)))*0.5 + 0.5
		if z > 1 {
			z = 1
		}
		return z
	default:
		return 0
	}
}

func extractBlockRGBAF16ToF32Volume(pix []uint16, width, height, depth, x0, y0, z0, blockX, blockY, blockZ int, dst []float32) {
	texelCount := blockX * blockY * blockZ
	if len(dst) < texelCount*4 {
		return
	}

	// Clamp origin to image bounds.
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if z0 < 0 {
		z0 = 0
	}

	for zz := 0; zz < blockZ; zz++ {
		z := z0 + zz
		if z >= depth {
			z = depth - 1
		}
		for yy := 0; yy < blockY; yy++ {
			y := y0 + yy
			if y >= height {
				y = height - 1
			}
			for xx := 0; xx < blockX; xx++ {
				x := x0 + xx
				if x >= width {
					x = width - 1
				}

				srcOff := ((z*height+y)*width + x) * 4
				dstOff := ((zz*blockY+yy)*blockX + xx) * 4
				dst[dstOff+0] = halfToFloat32(pix[srcOff+0])
				dst[dstOff+1] = halfToFloat32(pix[srcOff+1])
				dst[dstOff+2] = halfToFloat32(pix[srcOff+2])
				dst[dstOff+3] = halfToFloat32(pix[srcOff+3])
			}
		}
	}
}

func encodeBlockForF32Input(profile Profile, blockX, blockY, blockZ int, texels []float32, quality EncodeQuality, channelWeight [4]float32, flags Flags, rgbmScale float32, tuneOverride *encoderTuning) ([BlockBytes]byte, error) {
	if profile == ProfileHDR || profile == ProfileHDRRGBLDRAlpha {
		return encodeBlockRGBAF32HDR(profile, blockX, blockY, blockZ, texels, quality, channelWeight, tuneOverride)
	}

	// LDR float inputs: quantize to 8-bit and reuse the LDR encoder as a temporary implementation.
	tmp := make([]byte, len(texels))
	for i := 0; i < len(texels); i++ {
		v := texels[i]
		if !(v >= 0) {
			v = 0
		}
		if v <= 0 {
			v = 0
		} else if v >= 1 {
			v = 1
		}
		tmp[i] = uint8(flt2intRTN(v * 255.0))
	}
	return encodeBlockRGBA8LDR(profile, blockX, blockY, blockZ, tmp, quality, channelWeight, flags, rgbmScale, tuneOverride)
}

func quantizeRGBAF32ToU8(src []float32, dst []byte) {
	if len(dst) < len(src) {
		return
	}
	for i := 0; i < len(src); i++ {
		v := src[i]
		if !(v >= 0) {
			v = 0
		}
		if v <= 0 {
			v = 0
		} else if v >= 1 {
			v = 1
		}
		dst[i] = uint8(flt2intRTN(v * 255.0))
	}
}

func storeBlockRGBA8Volume(dst []byte, width, height, depth, x0, y0, z0, blockX, blockY, blockZ int, block []byte) {
	dstRowStride := width * 4
	dstSliceStride := height * dstRowStride
	srcRowBytes := blockX * 4
	for zz := 0; zz < blockZ; zz++ {
		z := z0 + zz
		if z >= depth {
			break
		}
		dstSliceBase := z * dstSliceStride
		srcSliceBase := zz * blockY * srcRowBytes
		for yy := 0; yy < blockY; yy++ {
			y := y0 + yy
			if y >= height {
				break
			}
			dstOff := dstSliceBase + y*dstRowStride + x0*4
			srcOff := srcSliceBase + yy*srcRowBytes
			rowCopyBytes := srcRowBytes
			if x0+blockX > width {
				rowCopyBytes = (width - x0) * 4
			}
			copy(dst[dstOff:dstOff+rowCopyBytes], block[srcOff:srcOff+rowCopyBytes])
		}
	}
}

func storeBlockRGBAF32Volume(dst []float32, width, height, depth, x0, y0, z0, blockX, blockY, blockZ int, block []float32) {
	dstRowStride := width * 4
	dstSliceStride := height * dstRowStride
	srcRowStride := blockX * 4
	for zz := 0; zz < blockZ; zz++ {
		z := z0 + zz
		if z >= depth {
			break
		}
		dstSliceBase := z * dstSliceStride
		srcSliceBase := zz * blockY * srcRowStride
		for yy := 0; yy < blockY; yy++ {
			y := y0 + yy
			if y >= height {
				break
			}
			dstOff := dstSliceBase + y*dstRowStride + x0*4
			srcOff := srcSliceBase + yy*srcRowStride
			rowCopy := srcRowStride
			if x0+blockX > width {
				rowCopy = (width - x0) * 4
			}
			copy(dst[dstOff:dstOff+rowCopy], block[srcOff:srcOff+rowCopy])
		}
	}
}

func storeBlockRGBAF32AsF16Volume(dst []uint16, width, height, depth, x0, y0, z0, blockX, blockY, blockZ int, block []float32) {
	dstRowStride := width * 4
	dstSliceStride := height * dstRowStride
	srcRowStride := blockX * 4
	for zz := 0; zz < blockZ; zz++ {
		z := z0 + zz
		if z >= depth {
			break
		}
		dstSliceBase := z * dstSliceStride
		srcSliceBase := zz * blockY * srcRowStride
		for yy := 0; yy < blockY; yy++ {
			y := y0 + yy
			if y >= height {
				break
			}
			dstOff := dstSliceBase + y*dstRowStride + x0*4
			srcOff := srcSliceBase + yy*srcRowStride
			rowTexels := blockX
			if x0+blockX > width {
				rowTexels = width - x0
			}
			for xx := 0; xx < rowTexels; xx++ {
				di := dstOff + xx*4
				si := srcOff + xx*4
				dst[di+0] = float32ToHalf(block[si+0])
				dst[di+1] = float32ToHalf(block[si+1])
				dst[di+2] = float32ToHalf(block[si+2])
				dst[di+3] = float32ToHalf(block[si+3])
			}
		}
	}
}
