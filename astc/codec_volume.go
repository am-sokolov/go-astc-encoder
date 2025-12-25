package astc

import "errors"

// DecodeRGBA8VolumeWithProfileInto decodes a .astc file into a caller-provided RGBA8 pixel buffer.
//
// The dst slice must have length at least `width*height*depth*4`. Pixels are laid out in x-major
// order, then y, then z: `((z*height+y)*width + x) * 4`.
//
// Limitations:
//   - Only LDR profiles (ProfileLDR, ProfileLDRSRGB).
func DecodeRGBA8VolumeWithProfileInto(astcData []byte, profile Profile, dst []byte) (width, height, depth int, err error) {
	h, blocks, err := ParseFile(astcData)
	if err != nil {
		return 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return 0, 0, 0, errors.New("astc: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return 0, 0, 0, errors.New("astc: output buffer too small")
	}

	if err := decodeRGBA8VolumeFromParsed(profile, h, blocks, dst[:width*height*depth*4]); err != nil {
		return 0, 0, 0, err
	}
	return width, height, depth, nil
}

// DecodeRGBA8VolumeFromParsedWithProfileInto decodes ASTC blocks returned by ParseFile into a
// caller-provided RGBA8 buffer.
//
// This avoids parsing overhead when decoding the same payload multiple times (e.g. in benchmarks).
func DecodeRGBA8VolumeFromParsedWithProfileInto(profile Profile, h Header, blocks []byte, dst []byte) error {
	width := int(h.SizeX)
	height := int(h.SizeY)
	depth := int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return errors.New("astc: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return errors.New("astc: output buffer too small")
	}
	return decodeRGBA8VolumeFromParsed(profile, h, blocks, dst[:width*height*depth*4])
}

func decodeRGBA8VolumeFromParsed(profile Profile, h Header, blocks []byte, dst []byte) error {
	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return err
	}
	if len(blocks) < total*BlockBytes {
		return ioErrUnexpectedEOF("astc blocks", total*BlockBytes, len(blocks))
	}

	width := int(h.SizeX)
	height := int(h.SizeY)
	depth := int(h.SizeZ)

	blockStrideX := BlockBytes
	blockStrideY := blocksX * blockStrideX
	blockStrideZ := blocksY * blockStrideY

	blockX := int(h.BlockX)
	blockY := int(h.BlockY)
	blockZ := int(h.BlockZ)
	texelCount := blockX * blockY * blockZ
	if texelCount <= 0 || texelCount > blockMaxTexels {
		return errors.New("astc: invalid block dimensions")
	}
	if profile != ProfileLDR && profile != ProfileLDRSRGB {
		return errUnsupportedProfileRGBA8
	}

	ctx := getDecodeContext(blockX, blockY, blockZ)

	var decodedBlock [blockMaxTexels * 4]byte
	decoded := decodedBlock[:texelCount*4]

	dstRowStride := width * 4
	dstSliceStride := height * dstRowStride
	srcRowBytes := blockX * 4
	for bz := 0; bz < blocksZ; bz++ {
		for by := 0; by < blocksY; by++ {
			for bx := 0; bx < blocksX; bx++ {
				blockOff := bz*blockStrideZ + by*blockStrideY + bx*blockStrideX
				block := blocks[blockOff : blockOff+BlockBytes]

				decodeBlockToRGBA8(profile, ctx, block, decoded)

				x0 := bx * blockX
				y0 := by * blockY
				z0 := bz * blockZ

				x1 := x0 + blockX
				if x1 > width {
					x1 = width
				}
				y1 := y0 + blockY
				if y1 > height {
					y1 = height
				}
				z1 := z0 + blockZ
				if z1 > depth {
					z1 = depth
				}

				rowCopyBytes := (x1 - x0) * 4

				for zz := 0; zz < blockZ; zz++ {
					z := z0 + zz
					if z >= z1 {
						break
					}
					dstSliceBase := z * dstSliceStride
					srcSliceBase := zz * blockY * srcRowBytes
					for yy := 0; yy < blockY; yy++ {
						y := y0 + yy
						if y >= y1 {
							break
						}
						dstOff := dstSliceBase + y*dstRowStride + x0*4
						srcOff := srcSliceBase + yy*srcRowBytes
						copy(dst[dstOff:dstOff+rowCopyBytes], decoded[srcOff:srcOff+rowCopyBytes])
					}
				}
			}
		}
	}

	return nil
}

// DecodeRGBA8VolumeWithProfile decodes a .astc file into an RGBA8 pixel buffer.
//
// The returned pixel buffer is laid out in x-major order, then y, then z:
// `((z*height+y)*width + x) * 4`.
//
// Limitations:
//   - Only LDR profiles (ProfileLDR, ProfileLDRSRGB).
func DecodeRGBA8VolumeWithProfile(astcData []byte, profile Profile) (pix []byte, width, height, depth int, err error) {
	h, blocks, err := ParseFile(astcData)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, 0, 0, 0, errors.New("astc: invalid image dimensions")
	}

	pix = make([]byte, width*height*depth*4)
	if err := decodeRGBA8VolumeFromParsed(profile, h, blocks, pix); err != nil {
		return nil, 0, 0, 0, err
	}
	return pix, width, height, depth, nil
}

func fillConstRGBAF32(dst []float32, r, g, b, a float32) {
	for i := 0; i < len(dst); i += 4 {
		dst[i+0] = r
		dst[i+1] = g
		dst[i+2] = b
		dst[i+3] = a
	}
}

func fillErrorRGBAF32(dst []float32) {
	fillConstRGBAF32(dst, 1, 0, 1, 1)
}

// DecodeRGBAF32VolumeWithProfileInto decodes a .astc file into a caller-provided RGBA float32
// pixel buffer.
//
// The dst slice must have length at least `width*height*depth*4`. Pixels are laid out in x-major
// order, then y, then z: `((z*height+y)*width + x) * 4`.
func DecodeRGBAF32VolumeWithProfileInto(astcData []byte, profile Profile, dst []float32) (width, height, depth int, err error) {
	h, blocks, err := ParseFile(astcData)
	if err != nil {
		return 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return 0, 0, 0, errors.New("astc: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return 0, 0, 0, errors.New("astc: output buffer too small")
	}

	if err := decodeRGBAF32VolumeFromParsed(profile, h, blocks, dst[:width*height*depth*4]); err != nil {
		return 0, 0, 0, err
	}
	return width, height, depth, nil
}

// DecodeRGBAF32VolumeFromParsedWithProfileInto decodes ASTC blocks returned by ParseFile into a
// caller-provided RGBA float32 buffer.
//
// This avoids parsing overhead when decoding the same payload multiple times (e.g. in benchmarks).
func DecodeRGBAF32VolumeFromParsedWithProfileInto(profile Profile, h Header, blocks []byte, dst []float32) error {
	width := int(h.SizeX)
	height := int(h.SizeY)
	depth := int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return errors.New("astc: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return errors.New("astc: output buffer too small")
	}
	return decodeRGBAF32VolumeFromParsed(profile, h, blocks, dst[:width*height*depth*4])
}

func decodeRGBAF32VolumeFromParsed(profile Profile, h Header, blocks []byte, dst []float32) error {
	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return err
	}
	if len(blocks) < total*BlockBytes {
		return ioErrUnexpectedEOF("astc blocks", total*BlockBytes, len(blocks))
	}

	width := int(h.SizeX)
	height := int(h.SizeY)
	depth := int(h.SizeZ)

	blockStrideX := BlockBytes
	blockStrideY := blocksX * blockStrideX
	blockStrideZ := blocksY * blockStrideY

	blockX := int(h.BlockX)
	blockY := int(h.BlockY)
	blockZ := int(h.BlockZ)
	texelCount := blockX * blockY * blockZ
	if texelCount <= 0 || texelCount > blockMaxTexels {
		return errors.New("astc: invalid block dimensions")
	}
	ctx := getDecodeContext(blockX, blockY, blockZ)

	var decodedBlockArr [blockMaxTexels * 4]float32
	decodedBlock := decodedBlockArr[:texelCount*4]

	dstRowStride := width * 4
	dstSliceStride := height * dstRowStride
	srcRowElems := blockX * 4
	for bz := 0; bz < blocksZ; bz++ {
		for by := 0; by < blocksY; by++ {
			for bx := 0; bx < blocksX; bx++ {
				blockOff := bz*blockStrideZ + by*blockStrideY + bx*blockStrideX
				block := blocks[blockOff : blockOff+BlockBytes]

				decodeBlockToRGBAF32(profile, ctx, block, decodedBlock)

				x0 := bx * blockX
				y0 := by * blockY
				z0 := bz * blockZ

				x1 := x0 + blockX
				if x1 > width {
					x1 = width
				}
				y1 := y0 + blockY
				if y1 > height {
					y1 = height
				}
				z1 := z0 + blockZ
				if z1 > depth {
					z1 = depth
				}

				rowCopyElems := (x1 - x0) * 4

				for zz := 0; zz < blockZ; zz++ {
					z := z0 + zz
					if z >= z1 {
						break
					}
					dstSliceBase := z * dstSliceStride
					srcSliceBase := zz * blockY * srcRowElems
					for yy := 0; yy < blockY; yy++ {
						y := y0 + yy
						if y >= y1 {
							break
						}
						dstOff := dstSliceBase + y*dstRowStride + x0*4
						srcOff := srcSliceBase + yy*srcRowElems
						copy(dst[dstOff:dstOff+rowCopyElems], decodedBlock[srcOff:srcOff+rowCopyElems])
					}
				}
			}
		}
	}

	return nil
}

// DecodeRGBAF32VolumeWithProfile decodes a .astc file into an RGBA float32 pixel buffer.
//
// The returned pixel buffer is laid out in x-major order, then y, then z:
// `((z*height+y)*width + x) * 4`.
func DecodeRGBAF32VolumeWithProfile(astcData []byte, profile Profile) (pix []float32, width, height, depth int, err error) {
	h, blocks, err := ParseFile(astcData)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, 0, 0, 0, errors.New("astc: invalid image dimensions")
	}

	pix = make([]float32, width*height*depth*4)

	if err := decodeRGBAF32VolumeFromParsed(profile, h, blocks, pix); err != nil {
		return nil, 0, 0, 0, err
	}

	return pix, width, height, depth, nil
}
