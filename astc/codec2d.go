package astc

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

// DecodeRGBA8 decodes a .astc file into an RGBA8 pixel buffer.
func DecodeRGBA8(astcData []byte) (pix []byte, width, height int, err error) {
	return DecodeRGBA8WithProfile(astcData, ProfileLDR)
}

// DecodeRGBA8WithProfile decodes a .astc file into an RGBA8 pixel buffer.
//
// Limitations:
//   - Only 2D images (SizeZ==1, BlockZ==1).
//   - Only LDR profiles (ProfileLDR, ProfileLDRSRGB).
func DecodeRGBA8WithProfile(astcData []byte, profile Profile) (pix []byte, width, height int, err error) {
	pix, width, height, depth, err := DecodeRGBA8VolumeWithProfile(astcData, profile)
	if err != nil {
		return nil, 0, 0, err
	}
	if depth != 1 {
		return nil, 0, 0, errors.New("astc: DecodeRGBA8WithProfile only supports 2D images (z==1); use DecodeRGBA8VolumeWithProfile")
	}
	return pix, width, height, nil
}

// DecodeRGBAF32WithProfile decodes a .astc file into an RGBA float32 pixel buffer.
//
// The float values match the reference decoder behavior: LDR endpoints are returned as unorm16
// values converted to float, and HDR endpoints are returned as LNS values converted to float.
//
// Limitations:
//   - Only 2D images (SizeZ==1, BlockZ==1).
func DecodeRGBAF32WithProfile(astcData []byte, profile Profile) (pix []float32, width, height int, err error) {
	pix, width, height, depth, err := DecodeRGBAF32VolumeWithProfile(astcData, profile)
	if err != nil {
		return nil, 0, 0, err
	}
	if depth != 1 {
		return nil, 0, 0, errors.New("astc: DecodeRGBAF32WithProfile only supports 2D images (z==1); use DecodeRGBAF32VolumeWithProfile")
	}
	return pix, width, height, nil
}

// EncodeRGBA8 encodes an RGBA8 pixel buffer into a .astc file.
func EncodeRGBA8(pix []byte, width, height int, blockX, blockY int) ([]byte, error) {
	return EncodeRGBA8WithProfileAndQuality(pix, width, height, blockX, blockY, ProfileLDR, EncodeMedium)
}

// EncodeRGBA8WithProfileAndQuality encodes an RGBA8 pixel buffer into a .astc file.
//
// Note: ASTC files do not store a profile. The profile controls encoder optimization behavior
// (it matches the profile the caller intends to use when decoding).
func EncodeRGBA8WithProfileAndQuality(pix []byte, width, height int, blockX, blockY int, profile Profile, quality EncodeQuality) ([]byte, error) {
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
		return nil, errors.New("astc: invalid RGBA8 buffer length")
	}
	if profile != ProfileLDR && profile != ProfileLDRSRGB && profile != ProfileHDRRGBLDRAlpha && profile != ProfileHDR {
		return nil, errors.New("astc: invalid profile")
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
		blockTexels := make([]byte, blockX*blockY*4)
		for by := 0; by < blocksY; by++ {
			for bx := 0; bx < blocksX; bx++ {
				extractBlockRGBA8(pix, width, height, bx*blockX, by*blockY, blockX, blockY, blockTexels)
				block, err := encodeBlockRGBA8LDR(profile, blockX, blockY, 1, blockTexels, quality, [4]float32{1, 1, 1, 1}, 0, 1, nil)
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
			blockTexels := make([]byte, blockX*blockY*4)
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
				extractBlockRGBA8(pix, width, height, bx*blockX, by*blockY, blockX, blockY, blockTexels)
				block, err := encodeBlockRGBA8LDR(profile, blockX, blockY, 1, blockTexels, quality, [4]float32{1, 1, 1, 1}, 0, 1, nil)
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
