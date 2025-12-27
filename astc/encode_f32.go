package astc

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

// EncodeRGBAF32 encodes an RGBA float32 pixel buffer into a .astc file using ProfileHDR and
// EncodeMedium encoder quality.
func EncodeRGBAF32(pix []float32, width, height int, blockX, blockY int) ([]byte, error) {
	return EncodeRGBAF32WithProfileAndQuality(pix, width, height, blockX, blockY, ProfileHDR, EncodeMedium)
}

// EncodeRGBAF32WithProfileAndQuality encodes an RGBA float32 pixel buffer into a .astc file.
//
// The input values are interpreted as linear floats. For HDR profiles values may be outside [0,1].
//
// Supported profiles:
//   - ProfileHDR
//   - ProfileHDRRGBLDRAlpha
func EncodeRGBAF32WithProfileAndQuality(pix []float32, width, height int, blockX, blockY int, profile Profile, quality EncodeQuality) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("astc: invalid image dimensions")
	}
	if blockX <= 0 || blockY <= 0 || blockX > 255 || blockY > 255 {
		return nil, errors.New("astc: invalid block dimensions")
	}
	if blockX*blockY > blockMaxTexels {
		return nil, errors.New("astc: invalid block dimensions")
	}
	if len(pix) != width*height*4 {
		return nil, errors.New("astc: invalid RGBAF32 buffer length")
	}
	if profile != ProfileHDR && profile != ProfileHDRRGBLDRAlpha {
		return nil, errors.New("astc: EncodeRGBAF32* only supports HDR profiles")
	}

	h := Header{
		BlockX: uint8(blockX),
		BlockY: uint8(blockY),
		BlockZ: 1,
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  1,
	}
	headerBytes, err := MarshalHeader(h)
	if err != nil {
		return nil, err
	}

	blocksX, blocksY, _, total, err := h.BlockCount()
	if err != nil {
		return nil, err
	}

	out := make([]byte, HeaderSize+total*BlockBytes)
	copy(out[:HeaderSize], headerBytes[:])
	blocksOut := out[HeaderSize:]

	totalBlocks := blocksX * blocksY
	procs := runtime.GOMAXPROCS(0)
	if procs < 1 {
		procs = 1
	}
	if procs > totalBlocks {
		procs = totalBlocks
	}

	// Small images are faster to encode sequentially.
	if procs == 1 || totalBlocks < 32 {
		blockTexels := make([]float32, blockX*blockY*4)
		for by := 0; by < blocksY; by++ {
			for bx := 0; bx < blocksX; bx++ {
				extractBlockRGBAF32(pix, width, height, bx*blockX, by*blockY, blockX, blockY, blockTexels)
				block, err := encodeBlockRGBAF32HDR(profile, blockX, blockY, 1, blockTexels, quality, [4]float32{1, 1, 1, 1}, nil)
				if err != nil {
					return nil, err
				}
				blockIdx := by*blocksX + bx
				copy(blocksOut[blockIdx*BlockBytes:(blockIdx+1)*BlockBytes], block[:])
			}
		}
		return out, nil
	}

	var next uint32
	var stop uint32
	var firstErr error
	var errOnce sync.Once

	var wg sync.WaitGroup
	wg.Add(procs)
	for w := 0; w < procs; w++ {
		go func() {
			defer wg.Done()
			blockTexels := make([]float32, blockX*blockY*4)
			for {
				if atomic.LoadUint32(&stop) != 0 {
					return
				}
				idx := int(atomic.AddUint32(&next, 1) - 1)
				if idx >= totalBlocks {
					return
				}

				bx := idx % blocksX
				by := idx / blocksX
				extractBlockRGBAF32(pix, width, height, bx*blockX, by*blockY, blockX, blockY, blockTexels)
				block, err := encodeBlockRGBAF32HDR(profile, blockX, blockY, 1, blockTexels, quality, [4]float32{1, 1, 1, 1}, nil)
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						atomic.StoreUint32(&stop, 1)
					})
					return
				}
				copy(blocksOut[idx*BlockBytes:(idx+1)*BlockBytes], block[:])
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// EncodeRGBAF32Volume encodes an RGBA float32 volume into a .astc file using ProfileHDR and
// EncodeMedium encoder quality.
func EncodeRGBAF32Volume(pix []float32, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return EncodeRGBAF32VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, ProfileHDR, EncodeMedium)
}

// EncodeRGBAF32VolumeWithProfileAndQuality encodes an RGBA float32 volume into a .astc file.
//
// The input buffer is laid out in x-major order, then y, then z:
// `((z*height+y)*width + x) * 4`.
//
// Supported profiles:
//   - ProfileHDR
//   - ProfileHDRRGBLDRAlpha
func EncodeRGBAF32VolumeWithProfileAndQuality(pix []float32, width, height, depth int, blockX, blockY, blockZ int, profile Profile, quality EncodeQuality) ([]byte, error) {
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, errors.New("astc: invalid image dimensions")
	}
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 || blockX > 255 || blockY > 255 || blockZ > 255 {
		return nil, errors.New("astc: invalid block dimensions")
	}
	if blockX*blockY*blockZ > blockMaxTexels {
		return nil, errors.New("astc: invalid block dimensions")
	}
	if len(pix) != width*height*depth*4 {
		return nil, errors.New("astc: invalid RGBAF32 buffer length")
	}
	if profile != ProfileHDR && profile != ProfileHDRRGBLDRAlpha {
		return nil, errors.New("astc: EncodeRGBAF32* only supports HDR profiles")
	}

	h := Header{
		BlockX: uint8(blockX),
		BlockY: uint8(blockY),
		BlockZ: uint8(blockZ),
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  uint32(depth),
	}
	headerBytes, err := MarshalHeader(h)
	if err != nil {
		return nil, err
	}

	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return nil, err
	}

	out := make([]byte, HeaderSize+total*BlockBytes)
	copy(out[:HeaderSize], headerBytes[:])
	blocksOut := out[HeaderSize:]

	totalBlocks := blocksX * blocksY * blocksZ
	procs := runtime.GOMAXPROCS(0)
	if procs < 1 {
		procs = 1
	}
	if procs > totalBlocks {
		procs = totalBlocks
	}

	// Small volumes are faster to encode sequentially.
	if procs == 1 || totalBlocks < 32 {
		blockTexels := make([]float32, blockX*blockY*blockZ*4)
		for bz := 0; bz < blocksZ; bz++ {
			for by := 0; by < blocksY; by++ {
				for bx := 0; bx < blocksX; bx++ {
					extractBlockRGBAF32Volume(pix, width, height, depth, bx*blockX, by*blockY, bz*blockZ, blockX, blockY, blockZ, blockTexels)
					block, err := encodeBlockRGBAF32HDR(profile, blockX, blockY, blockZ, blockTexels, quality, [4]float32{1, 1, 1, 1}, nil)
					if err != nil {
						return nil, err
					}
					blockIdx := (bz*blocksY+by)*blocksX + bx
					copy(blocksOut[blockIdx*BlockBytes:(blockIdx+1)*BlockBytes], block[:])
				}
			}
		}
		return out, nil
	}

	var next uint32
	var stop uint32
	var firstErr error
	var errOnce sync.Once

	xy := blocksX * blocksY
	var wg sync.WaitGroup
	wg.Add(procs)
	for w := 0; w < procs; w++ {
		go func() {
			defer wg.Done()
			blockTexels := make([]float32, blockX*blockY*blockZ*4)
			for {
				if atomic.LoadUint32(&stop) != 0 {
					return
				}
				idx := int(atomic.AddUint32(&next, 1) - 1)
				if idx >= totalBlocks {
					return
				}

				bx := idx % blocksX
				by := (idx / blocksX) % blocksY
				bz := idx / xy

				extractBlockRGBAF32Volume(pix, width, height, depth, bx*blockX, by*blockY, bz*blockZ, blockX, blockY, blockZ, blockTexels)
				block, err := encodeBlockRGBAF32HDR(profile, blockX, blockY, blockZ, blockTexels, quality, [4]float32{1, 1, 1, 1}, nil)
				if err != nil {
					errOnce.Do(func() {
						firstErr = err
						atomic.StoreUint32(&stop, 1)
					})
					return
				}
				copy(blocksOut[idx*BlockBytes:(idx+1)*BlockBytes], block[:])
			}
		}()
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

func extractBlockRGBAF32(pix []float32, width, height, x0, y0, blockX, blockY int, dst []float32) {
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

func extractBlockRGBAF32Volume(pix []float32, width, height, depth, x0, y0, z0, blockX, blockY, blockZ int, dst []float32) {
	xyStride := width * height * 4
	yStride := width * 4

	for bz := 0; bz < blockZ; bz++ {
		z := z0 + bz
		if z >= depth {
			z = depth - 1
		}
		zBase := z * xyStride
		for by := 0; by < blockY; by++ {
			y := y0 + by
			if y >= height {
				y = height - 1
			}
			yBase := zBase + y*yStride
			for bx := 0; bx < blockX; bx++ {
				x := x0 + bx
				if x >= width {
					x = width - 1
				}

				src := yBase + x*4
				dstOff := ((bz*blockY+by)*blockX + bx) * 4
				dst[dstOff+0] = pix[src+0]
				dst[dstOff+1] = pix[src+1]
				dst[dstOff+2] = pix[src+2]
				dst[dstOff+3] = pix[src+3]
			}
		}
	}
}
