package astc

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

// EncodeRGBA8Volume encodes an RGBA8 pixel buffer into a .astc file using ProfileLDR and
// EncodeMedium encoder quality.
func EncodeRGBA8Volume(pix []byte, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return EncodeRGBA8VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, ProfileLDR, EncodeMedium)
}

// EncodeRGBA8VolumeWithProfileAndQuality encodes an RGBA8 pixel buffer into a .astc file.
//
// The input buffer is laid out in x-major order, then y, then z:
// `((z*height+y)*width + x) * 4`.
//
// Note: ASTC files do not store a profile. The profile controls encoder optimization behavior.
func EncodeRGBA8VolumeWithProfileAndQuality(pix []byte, width, height, depth int, blockX, blockY, blockZ int, profile Profile, quality EncodeQuality) ([]byte, error) {
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
		return nil, errors.New("astc: invalid RGBA8 buffer length")
	}
	if profile != ProfileLDR && profile != ProfileLDRSRGB && profile != ProfileHDRRGBLDRAlpha && profile != ProfileHDR {
		return nil, errors.New("astc: invalid profile")
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

	// Small images are faster to encode sequentially.
	if procs == 1 || totalBlocks < 32 {
		blockTexels := make([]byte, blockX*blockY*blockZ*4)
		for bz := 0; bz < blocksZ; bz++ {
			for by := 0; by < blocksY; by++ {
				for bx := 0; bx < blocksX; bx++ {
					extractBlockRGBA8Volume(pix, width, height, depth, bx*blockX, by*blockY, bz*blockZ, blockX, blockY, blockZ, blockTexels)
					block, err := encodeBlockRGBA8LDR(profile, blockX, blockY, blockZ, blockTexels, quality)
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
			blockTexels := make([]byte, blockX*blockY*blockZ*4)
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

				extractBlockRGBA8Volume(pix, width, height, depth, bx*blockX, by*blockY, bz*blockZ, blockX, blockY, blockZ, blockTexels)
				block, err := encodeBlockRGBA8LDR(profile, blockX, blockY, blockZ, blockTexels, quality)
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

func extractBlockRGBA8Volume(pix []byte, width, height, depth, x0, y0, z0, blockX, blockY, blockZ int, dst []byte) {
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
