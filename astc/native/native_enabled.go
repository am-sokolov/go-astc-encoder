//go:build astcenc_native && cgo

package native

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/arm-software/astc-encoder/astc"
	nativecgo "github.com/arm-software/astc-encoder/astc/native/internal/astcenc"
)

const (
	cFlagUseDecodeUNORM8  = 1 << 1
	cFlagDecompressOnly   = 1 << 4
	defaultSmallBlockHint = 32
)

func Enabled() bool { return true }

func qualityToFloat(q astc.EncodeQuality) float32 {
	switch q {
	case astc.EncodeFastest:
		return 0
	case astc.EncodeFast:
		return 10
	case astc.EncodeMedium:
		return 60
	case astc.EncodeThorough:
		return 98
	case astc.EncodeVeryThorough:
		return 99
	case astc.EncodeExhaustive:
		return 100
	default:
		return 60
	}
}

func profileToC(p astc.Profile) (int, error) {
	switch p {
	case astc.ProfileLDR:
		return 1, nil // ASTCENC_PRF_LDR
	case astc.ProfileLDRSRGB:
		return 0, nil // ASTCENC_PRF_LDR_SRGB
	case astc.ProfileHDRRGBLDRAlpha:
		return 2, nil // ASTCENC_PRF_HDR_RGB_LDR_A
	case astc.ProfileHDR:
		return 3, nil // ASTCENC_PRF_HDR
	default:
		return 0, errors.New("astc/native: unknown profile")
	}
}

func errFromCode(code int, op string) error {
	if code == 0 {
		return nil
	}
	if msg := nativecgo.ErrorString(code); msg != "" {
		return fmt.Errorf("astc/native: %s: %s", op, msg)
	}
	return fmt.Errorf("astc/native: %s: error %d", op, code)
}

// Encoder wraps a reusable native astcenc compression context.
//
// Encoder is not safe for concurrent use.
type Encoder struct {
	ctx unsafe.Pointer
	img unsafe.Pointer

	inBuf unsafe.Pointer
	inCap int

	blockX int
	blockY int
	blockZ int

	profile     astc.Profile
	quality     astc.EncodeQuality
	threadCount int
}

func NewEncoder(blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality, threadCount int) (*Encoder, error) {
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 || blockX > 255 || blockY > 255 || blockZ > 255 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if blockX*blockY*blockZ > 216 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if threadCount <= 0 {
		threadCount = runtime.GOMAXPROCS(0)
	}
	if threadCount < 1 {
		threadCount = 1
	}

	cProf, err := profileToC(profile)
	if err != nil {
		return nil, err
	}

	ctx, code := nativecgo.ContextCreate(cProf, blockX, blockY, blockZ, qualityToFloat(quality), 0, threadCount)
	if err := errFromCode(code, "astcenc_context_alloc"); err != nil {
		return nil, err
	}

	img, code := nativecgo.ImageCreateU8()
	if err := errFromCode(code, "astcenc_image_alloc"); err != nil {
		nativecgo.ContextDestroy(ctx)
		return nil, err
	}

	return &Encoder{
		ctx:         ctx,
		img:         img,
		blockX:      blockX,
		blockY:      blockY,
		blockZ:      blockZ,
		profile:     profile,
		quality:     quality,
		threadCount: threadCount,
	}, nil
}

func (e *Encoder) Close() error {
	if e.img != nil {
		nativecgo.ImageDestroy(e.img)
		e.img = nil
	}
	if e.ctx != nil {
		nativecgo.ContextDestroy(e.ctx)
		e.ctx = nil
	}
	if e.inBuf != nil {
		nativecgo.Free(e.inBuf)
		e.inBuf = nil
		e.inCap = 0
	}
	return nil
}

func (e *Encoder) ensureInCap(n int) error {
	if n <= 0 {
		return errors.New("astc/native: invalid input buffer size")
	}
	if n <= e.inCap && e.inBuf != nil {
		return nil
	}
	p := nativecgo.Realloc(e.inBuf, n)
	if p == nil {
		return errors.New("astc/native: out of memory")
	}
	e.inBuf = p
	e.inCap = n
	return nil
}

func (e *Encoder) EncodeRGBA8(pix []byte, width, height int) ([]byte, error) {
	if e.blockZ != 1 {
		return nil, errors.New("astc/native: EncodeRGBA8 requires a 2D encoder (blockZ==1)")
	}
	return e.EncodeRGBA8Volume(pix, width, height, 1)
}

func (e *Encoder) EncodeRGBA8Volume(pix []byte, width, height, depth int) ([]byte, error) {
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, errors.New("astc/native: invalid image dimensions")
	}
	if len(pix) != width*height*depth*4 {
		return nil, errors.New("astc/native: invalid RGBA8 buffer length")
	}

	h := astc.Header{
		BlockX: uint8(e.blockX),
		BlockY: uint8(e.blockY),
		BlockZ: uint8(e.blockZ),
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  uint32(depth),
	}
	headerBytes, err := astc.MarshalHeader(h)
	if err != nil {
		return nil, err
	}

	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return nil, err
	}

	out := make([]byte, astc.HeaderSize+total*astc.BlockBytes)
	copy(out[:astc.HeaderSize], headerBytes[:])
	blocksOut := out[astc.HeaderSize:]

	if err := e.ensureInCap(len(pix)); err != nil {
		return nil, err
	}
	copy(unsafe.Slice((*byte)(e.inBuf), len(pix)), pix)

	code := nativecgo.ImageInitU8(e.img, width, height, depth, e.inBuf)
	if err := errFromCode(code, "astcenc_image_init"); err != nil {
		return nil, err
	}

	totalBlocks := blocksX * blocksY * blocksZ
	workers := e.threadCount
	if workers < 1 {
		workers = 1
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	outPtr := unsafe.Pointer(&blocksOut[0])
	outLen := len(blocksOut)

	if workers == 1 || totalBlocks < defaultSmallBlockHint {
		code := nativecgo.CompressImage(e.ctx, e.img, outPtr, outLen, 0)
		resetCode := nativecgo.CompressReset(e.ctx)
		if err := errFromCode(code, "astcenc_compress_image"); err != nil {
			_ = errFromCode(resetCode, "astcenc_compress_reset")
			return nil, err
		}
		if err := errFromCode(resetCode, "astcenc_compress_reset"); err != nil {
			return nil, err
		}
		return out, nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	var firstErr error
	var once sync.Once
	for i := 0; i < workers; i++ {
		threadIndex := i
		go func() {
			defer wg.Done()
			code := nativecgo.CompressImage(e.ctx, e.img, outPtr, outLen, threadIndex)
			if code != 0 {
				once.Do(func() {
					firstErr = errFromCode(code, "astcenc_compress_image")
				})
			}
		}()
	}
	wg.Wait()

	resetCode := nativecgo.CompressReset(e.ctx)
	if firstErr != nil {
		_ = errFromCode(resetCode, "astcenc_compress_reset")
		return nil, firstErr
	}
	if err := errFromCode(resetCode, "astcenc_compress_reset"); err != nil {
		return nil, err
	}
	return out, nil
}

// EncoderF16 wraps a reusable native astcenc compression context for RGBA float16 (IEEE binary16)
// input.
//
// EncoderF16 is not safe for concurrent use.
type EncoderF16 struct {
	ctx unsafe.Pointer
	img unsafe.Pointer

	inBuf unsafe.Pointer
	inCap int // bytes

	blockX int
	blockY int
	blockZ int

	profile     astc.Profile
	quality     astc.EncodeQuality
	threadCount int
}

func NewEncoderF16(blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality, threadCount int) (*EncoderF16, error) {
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 || blockX > 255 || blockY > 255 || blockZ > 255 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if blockX*blockY*blockZ > 216 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if threadCount <= 0 {
		threadCount = runtime.GOMAXPROCS(0)
	}
	if threadCount < 1 {
		threadCount = 1
	}

	cProf, err := profileToC(profile)
	if err != nil {
		return nil, err
	}

	ctx, code := nativecgo.ContextCreate(cProf, blockX, blockY, blockZ, qualityToFloat(quality), 0, threadCount)
	if err := errFromCode(code, "astcenc_context_alloc"); err != nil {
		return nil, err
	}

	img, code := nativecgo.ImageCreateF16()
	if err := errFromCode(code, "astcenc_image_alloc"); err != nil {
		nativecgo.ContextDestroy(ctx)
		return nil, err
	}

	return &EncoderF16{
		ctx:         ctx,
		img:         img,
		blockX:      blockX,
		blockY:      blockY,
		blockZ:      blockZ,
		profile:     profile,
		quality:     quality,
		threadCount: threadCount,
	}, nil
}

func (e *EncoderF16) Close() error {
	if e.img != nil {
		nativecgo.ImageDestroy(e.img)
		e.img = nil
	}
	if e.ctx != nil {
		nativecgo.ContextDestroy(e.ctx)
		e.ctx = nil
	}
	if e.inBuf != nil {
		nativecgo.Free(e.inBuf)
		e.inBuf = nil
		e.inCap = 0
	}
	return nil
}

func (e *EncoderF16) ensureInCap(n int) error {
	if n <= 0 {
		return errors.New("astc/native: invalid input buffer size")
	}
	if n <= e.inCap && e.inBuf != nil {
		return nil
	}
	p := nativecgo.Realloc(e.inBuf, n)
	if p == nil {
		return errors.New("astc/native: out of memory")
	}
	e.inBuf = p
	e.inCap = n
	return nil
}

func (e *EncoderF16) EncodeRGBAF16(pix []uint16, width, height int) ([]byte, error) {
	if e.blockZ != 1 {
		return nil, errors.New("astc/native: EncodeRGBAF16 requires a 2D encoder (blockZ==1)")
	}
	return e.EncodeRGBAF16Volume(pix, width, height, 1)
}

func (e *EncoderF16) EncodeRGBAF16Volume(pix []uint16, width, height, depth int) ([]byte, error) {
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, errors.New("astc/native: invalid image dimensions")
	}
	if len(pix) != width*height*depth*4 {
		return nil, errors.New("astc/native: invalid RGBAF16 buffer length")
	}

	h := astc.Header{
		BlockX: uint8(e.blockX),
		BlockY: uint8(e.blockY),
		BlockZ: uint8(e.blockZ),
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  uint32(depth),
	}
	headerBytes, err := astc.MarshalHeader(h)
	if err != nil {
		return nil, err
	}

	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return nil, err
	}

	out := make([]byte, astc.HeaderSize+total*astc.BlockBytes)
	copy(out[:astc.HeaderSize], headerBytes[:])
	blocksOut := out[astc.HeaderSize:]

	inBytes := len(pix) * 2
	if err := e.ensureInCap(inBytes); err != nil {
		return nil, err
	}
	copy(unsafe.Slice((*uint16)(e.inBuf), len(pix)), pix)

	code := nativecgo.ImageInitF16(e.img, width, height, depth, e.inBuf)
	if err := errFromCode(code, "astcenc_image_init"); err != nil {
		return nil, err
	}

	totalBlocks := blocksX * blocksY * blocksZ
	workers := e.threadCount
	if workers < 1 {
		workers = 1
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	outPtr := unsafe.Pointer(&blocksOut[0])
	outLen := len(blocksOut)

	if workers == 1 || totalBlocks < defaultSmallBlockHint {
		code := nativecgo.CompressImage(e.ctx, e.img, outPtr, outLen, 0)
		resetCode := nativecgo.CompressReset(e.ctx)
		if err := errFromCode(code, "astcenc_compress_image"); err != nil {
			_ = errFromCode(resetCode, "astcenc_compress_reset")
			return nil, err
		}
		if err := errFromCode(resetCode, "astcenc_compress_reset"); err != nil {
			return nil, err
		}
		return out, nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	var firstErr error
	var once sync.Once
	for i := 0; i < workers; i++ {
		threadIndex := i
		go func() {
			defer wg.Done()
			code := nativecgo.CompressImage(e.ctx, e.img, outPtr, outLen, threadIndex)
			if code != 0 {
				once.Do(func() {
					firstErr = errFromCode(code, "astcenc_compress_image")
				})
			}
		}()
	}
	wg.Wait()

	resetCode := nativecgo.CompressReset(e.ctx)
	if firstErr != nil {
		_ = errFromCode(resetCode, "astcenc_compress_reset")
		return nil, firstErr
	}
	if err := errFromCode(resetCode, "astcenc_compress_reset"); err != nil {
		return nil, err
	}
	return out, nil
}

// EncoderF32 wraps a reusable native astcenc compression context for RGBA float32 input.
//
// EncoderF32 is not safe for concurrent use.
type EncoderF32 struct {
	ctx unsafe.Pointer
	img unsafe.Pointer

	inBuf unsafe.Pointer
	inCap int // bytes

	blockX int
	blockY int
	blockZ int

	profile     astc.Profile
	quality     astc.EncodeQuality
	threadCount int
}

func NewEncoderF32(blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality, threadCount int) (*EncoderF32, error) {
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 || blockX > 255 || blockY > 255 || blockZ > 255 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if blockX*blockY*blockZ > 216 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if threadCount <= 0 {
		threadCount = runtime.GOMAXPROCS(0)
	}
	if threadCount < 1 {
		threadCount = 1
	}

	cProf, err := profileToC(profile)
	if err != nil {
		return nil, err
	}

	ctx, code := nativecgo.ContextCreate(cProf, blockX, blockY, blockZ, qualityToFloat(quality), 0, threadCount)
	if err := errFromCode(code, "astcenc_context_alloc"); err != nil {
		return nil, err
	}

	img, code := nativecgo.ImageCreateF32()
	if err := errFromCode(code, "astcenc_image_alloc"); err != nil {
		nativecgo.ContextDestroy(ctx)
		return nil, err
	}

	return &EncoderF32{
		ctx:         ctx,
		img:         img,
		blockX:      blockX,
		blockY:      blockY,
		blockZ:      blockZ,
		profile:     profile,
		quality:     quality,
		threadCount: threadCount,
	}, nil
}

func (e *EncoderF32) Close() error {
	if e.img != nil {
		nativecgo.ImageDestroy(e.img)
		e.img = nil
	}
	if e.ctx != nil {
		nativecgo.ContextDestroy(e.ctx)
		e.ctx = nil
	}
	if e.inBuf != nil {
		nativecgo.Free(e.inBuf)
		e.inBuf = nil
		e.inCap = 0
	}
	return nil
}

func (e *EncoderF32) ensureInCap(n int) error {
	if n <= 0 {
		return errors.New("astc/native: invalid input buffer size")
	}
	if n <= e.inCap && e.inBuf != nil {
		return nil
	}
	p := nativecgo.Realloc(e.inBuf, n)
	if p == nil {
		return errors.New("astc/native: out of memory")
	}
	e.inBuf = p
	e.inCap = n
	return nil
}

func (e *EncoderF32) EncodeRGBAF32(pix []float32, width, height int) ([]byte, error) {
	if e.blockZ != 1 {
		return nil, errors.New("astc/native: EncodeRGBAF32 requires a 2D encoder (blockZ==1)")
	}
	return e.EncodeRGBAF32Volume(pix, width, height, 1)
}

func (e *EncoderF32) EncodeRGBAF32Volume(pix []float32, width, height, depth int) ([]byte, error) {
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, errors.New("astc/native: invalid image dimensions")
	}
	if len(pix) != width*height*depth*4 {
		return nil, errors.New("astc/native: invalid RGBAF32 buffer length")
	}

	h := astc.Header{
		BlockX: uint8(e.blockX),
		BlockY: uint8(e.blockY),
		BlockZ: uint8(e.blockZ),
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  uint32(depth),
	}
	headerBytes, err := astc.MarshalHeader(h)
	if err != nil {
		return nil, err
	}

	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return nil, err
	}

	out := make([]byte, astc.HeaderSize+total*astc.BlockBytes)
	copy(out[:astc.HeaderSize], headerBytes[:])
	blocksOut := out[astc.HeaderSize:]

	inBytes := len(pix) * 4
	if err := e.ensureInCap(inBytes); err != nil {
		return nil, err
	}
	copy(unsafe.Slice((*float32)(e.inBuf), len(pix)), pix)

	code := nativecgo.ImageInitF32(e.img, width, height, depth, e.inBuf)
	if err := errFromCode(code, "astcenc_image_init"); err != nil {
		return nil, err
	}

	totalBlocks := blocksX * blocksY * blocksZ
	workers := e.threadCount
	if workers < 1 {
		workers = 1
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	outPtr := unsafe.Pointer(&blocksOut[0])
	outLen := len(blocksOut)

	if workers == 1 || totalBlocks < defaultSmallBlockHint {
		code := nativecgo.CompressImage(e.ctx, e.img, outPtr, outLen, 0)
		resetCode := nativecgo.CompressReset(e.ctx)
		if err := errFromCode(code, "astcenc_compress_image"); err != nil {
			_ = errFromCode(resetCode, "astcenc_compress_reset")
			return nil, err
		}
		if err := errFromCode(resetCode, "astcenc_compress_reset"); err != nil {
			return nil, err
		}
		return out, nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	var firstErr error
	var once sync.Once
	for i := 0; i < workers; i++ {
		threadIndex := i
		go func() {
			defer wg.Done()
			code := nativecgo.CompressImage(e.ctx, e.img, outPtr, outLen, threadIndex)
			if code != 0 {
				once.Do(func() {
					firstErr = errFromCode(code, "astcenc_compress_image")
				})
			}
		}()
	}
	wg.Wait()

	resetCode := nativecgo.CompressReset(e.ctx)
	if firstErr != nil {
		_ = errFromCode(resetCode, "astcenc_compress_reset")
		return nil, firstErr
	}
	if err := errFromCode(resetCode, "astcenc_compress_reset"); err != nil {
		return nil, err
	}
	return out, nil
}

// Decoder wraps a reusable native astcenc decompression context.
//
// Decoder is not safe for concurrent use.
type Decoder struct {
	ctx unsafe.Pointer

	blockX int
	blockY int
	blockZ int

	profile     astc.Profile
	threadCount int
}

func NewDecoder(blockX, blockY, blockZ int, profile astc.Profile, threadCount int) (*Decoder, error) {
	if blockX <= 0 || blockY <= 0 || blockZ <= 0 || blockX > 255 || blockY > 255 || blockZ > 255 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if blockX*blockY*blockZ > 216 {
		return nil, errors.New("astc/native: invalid block dimensions")
	}
	if threadCount <= 0 {
		threadCount = runtime.GOMAXPROCS(0)
	}
	if threadCount < 1 {
		threadCount = 1
	}

	cProf, err := profileToC(profile)
	if err != nil {
		return nil, err
	}

	ctx, code := nativecgo.ContextCreate(cProf, blockX, blockY, blockZ, 0, cFlagDecompressOnly, threadCount)
	if err := errFromCode(code, "astcenc_context_alloc"); err != nil {
		return nil, err
	}

	return &Decoder{
		ctx:         ctx,
		blockX:      blockX,
		blockY:      blockY,
		blockZ:      blockZ,
		profile:     profile,
		threadCount: threadCount,
	}, nil
}

func (d *Decoder) Close() error {
	if d.ctx != nil {
		nativecgo.ContextDestroy(d.ctx)
		d.ctx = nil
	}
	return nil
}

func (d *Decoder) DecodeRGBA8VolumeInto(width, height, depth int, blocks []byte, dst []byte) error {
	if width <= 0 || height <= 0 || depth <= 0 {
		return errors.New("astc/native: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return errors.New("astc/native: output buffer too small")
	}

	totalBlocks := ((width + d.blockX - 1) / d.blockX) * ((height + d.blockY - 1) / d.blockY) * ((depth + d.blockZ - 1) / d.blockZ)
	needBlocks := totalBlocks * astc.BlockBytes
	if len(blocks) < needBlocks {
		return errors.New("astc/native: block buffer too small")
	}

	workers := d.threadCount
	if workers < 1 {
		workers = 1
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	dataPtr := unsafe.Pointer(&blocks[0])
	dataLen := needBlocks
	outPtr := unsafe.Pointer(&dst[0])
	outLen := width * height * depth * 4

	if workers == 1 || totalBlocks < defaultSmallBlockHint {
		code := nativecgo.DecompressImageRGBA8(d.ctx, dataPtr, dataLen, width, height, depth, outPtr, outLen, 0)
		resetCode := nativecgo.DecompressReset(d.ctx)
		if err := errFromCode(code, "astcenc_decompress_image"); err != nil {
			_ = errFromCode(resetCode, "astcenc_decompress_reset")
			return err
		}
		if err := errFromCode(resetCode, "astcenc_decompress_reset"); err != nil {
			return err
		}
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	var firstErr error
	var once sync.Once
	for i := 0; i < workers; i++ {
		threadIndex := i
		go func() {
			defer wg.Done()
			code := nativecgo.DecompressImageRGBA8(d.ctx, dataPtr, dataLen, width, height, depth, outPtr, outLen, threadIndex)
			if code != 0 {
				once.Do(func() {
					firstErr = errFromCode(code, "astcenc_decompress_image")
				})
			}
		}()
	}
	wg.Wait()

	resetCode := nativecgo.DecompressReset(d.ctx)
	if firstErr != nil {
		_ = errFromCode(resetCode, "astcenc_decompress_reset")
		return firstErr
	}
	if err := errFromCode(resetCode, "astcenc_decompress_reset"); err != nil {
		return err
	}
	return nil
}

func (d *Decoder) DecodeRGBAF32VolumeInto(width, height, depth int, blocks []byte, dst []float32) error {
	if width <= 0 || height <= 0 || depth <= 0 {
		return errors.New("astc/native: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return errors.New("astc/native: output buffer too small")
	}

	totalBlocks := ((width + d.blockX - 1) / d.blockX) * ((height + d.blockY - 1) / d.blockY) * ((depth + d.blockZ - 1) / d.blockZ)
	needBlocks := totalBlocks * astc.BlockBytes
	if len(blocks) < needBlocks {
		return errors.New("astc/native: block buffer too small")
	}

	workers := d.threadCount
	if workers < 1 {
		workers = 1
	}
	if workers > totalBlocks {
		workers = totalBlocks
	}

	dataPtr := unsafe.Pointer(&blocks[0])
	dataLen := needBlocks
	outPtr := unsafe.Pointer(&dst[0])
	outLen := width * height * depth * 4 * 4 // float32 bytes

	if workers == 1 || totalBlocks < defaultSmallBlockHint {
		code := nativecgo.DecompressImageRGBAF32(d.ctx, dataPtr, dataLen, width, height, depth, outPtr, outLen, 0)
		resetCode := nativecgo.DecompressReset(d.ctx)
		if err := errFromCode(code, "astcenc_decompress_image"); err != nil {
			_ = errFromCode(resetCode, "astcenc_decompress_reset")
			return err
		}
		if err := errFromCode(resetCode, "astcenc_decompress_reset"); err != nil {
			return err
		}
		return nil
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	var firstErr error
	var once sync.Once
	for i := 0; i < workers; i++ {
		threadIndex := i
		go func() {
			defer wg.Done()
			code := nativecgo.DecompressImageRGBAF32(d.ctx, dataPtr, dataLen, width, height, depth, outPtr, outLen, threadIndex)
			if code != 0 {
				once.Do(func() {
					firstErr = errFromCode(code, "astcenc_decompress_image")
				})
			}
		}()
	}
	wg.Wait()

	resetCode := nativecgo.DecompressReset(d.ctx)
	if firstErr != nil {
		_ = errFromCode(resetCode, "astcenc_decompress_reset")
		return firstErr
	}
	if err := errFromCode(resetCode, "astcenc_decompress_reset"); err != nil {
		return err
	}
	return nil
}

func EncodeRGBA8(pix []byte, width, height int, blockX, blockY int) ([]byte, error) {
	return EncodeRGBA8WithProfileAndQuality(pix, width, height, blockX, blockY, astc.ProfileLDR, astc.EncodeMedium)
}

func EncodeRGBA8WithProfileAndQuality(pix []byte, width, height int, blockX, blockY int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	enc, err := NewEncoder(blockX, blockY, 1, profile, quality, 0)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeRGBA8(pix, width, height)
}

func EncodeRGBA8Volume(pix []byte, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return EncodeRGBA8VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, astc.ProfileLDR, astc.EncodeMedium)
}

func EncodeRGBA8VolumeWithProfileAndQuality(pix []byte, width, height, depth int, blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	enc, err := NewEncoder(blockX, blockY, blockZ, profile, quality, 0)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeRGBA8Volume(pix, width, height, depth)
}

func EncodeRGBAF16(pix []uint16, width, height int, blockX, blockY int) ([]byte, error) {
	return EncodeRGBAF16WithProfileAndQuality(pix, width, height, blockX, blockY, astc.ProfileLDR, astc.EncodeMedium)
}

func EncodeRGBAF16WithProfileAndQuality(pix []uint16, width, height int, blockX, blockY int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	enc, err := NewEncoderF16(blockX, blockY, 1, profile, quality, 0)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeRGBAF16(pix, width, height)
}

func EncodeRGBAF16Volume(pix []uint16, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return EncodeRGBAF16VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, astc.ProfileLDR, astc.EncodeMedium)
}

func EncodeRGBAF16VolumeWithProfileAndQuality(pix []uint16, width, height, depth int, blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	enc, err := NewEncoderF16(blockX, blockY, blockZ, profile, quality, 0)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeRGBAF16Volume(pix, width, height, depth)
}

func EncodeRGBAF32(pix []float32, width, height int, blockX, blockY int) ([]byte, error) {
	return EncodeRGBAF32WithProfileAndQuality(pix, width, height, blockX, blockY, astc.ProfileLDR, astc.EncodeMedium)
}

func EncodeRGBAF32WithProfileAndQuality(pix []float32, width, height int, blockX, blockY int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	enc, err := NewEncoderF32(blockX, blockY, 1, profile, quality, 0)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeRGBAF32(pix, width, height)
}

func EncodeRGBAF32Volume(pix []float32, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return EncodeRGBAF32VolumeWithProfileAndQuality(pix, width, height, depth, blockX, blockY, blockZ, astc.ProfileLDR, astc.EncodeMedium)
}

func EncodeRGBAF32VolumeWithProfileAndQuality(pix []float32, width, height, depth int, blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	enc, err := NewEncoderF32(blockX, blockY, blockZ, profile, quality, 0)
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeRGBAF32Volume(pix, width, height, depth)
}

func DecodeRGBA8(astcData []byte) (pix []byte, width, height int, err error) {
	return DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
}

func DecodeRGBA8WithProfile(astcData []byte, profile astc.Profile) (pix []byte, width, height int, err error) {
	pix, width, height, depth, err := DecodeRGBA8VolumeWithProfile(astcData, profile)
	if err != nil {
		return nil, 0, 0, err
	}
	if depth != 1 {
		return nil, 0, 0, errors.New("astc/native: DecodeRGBA8WithProfile only supports 2D images (z==1); use DecodeRGBA8VolumeWithProfile")
	}
	return pix, width, height, nil
}

func DecodeRGBA8VolumeWithProfile(astcData []byte, profile astc.Profile) (pix []byte, width, height, depth int, err error) {
	h, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, 0, 0, 0, errors.New("astc/native: invalid image dimensions")
	}

	pix = make([]byte, width*height*depth*4)
	if err := DecodeRGBA8VolumeFromParsedWithProfileInto(profile, h, blocks, pix); err != nil {
		return nil, 0, 0, 0, err
	}
	return pix, width, height, depth, nil
}

func DecodeRGBA8VolumeWithProfileInto(astcData []byte, profile astc.Profile, dst []byte) (width, height, depth int, err error) {
	h, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		return 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return 0, 0, 0, errors.New("astc/native: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return 0, 0, 0, errors.New("astc/native: output buffer too small")
	}
	if err := DecodeRGBA8VolumeFromParsedWithProfileInto(profile, h, blocks, dst[:width*height*depth*4]); err != nil {
		return 0, 0, 0, err
	}
	return width, height, depth, nil
}

func DecodeRGBA8VolumeFromParsedWithProfileInto(profile astc.Profile, h astc.Header, blocks []byte, dst []byte) error {
	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return err
	}
	if len(blocks) < total*astc.BlockBytes {
		return errors.New("astc/native: block buffer too small")
	}

	width := int(h.SizeX)
	height := int(h.SizeY)
	depth := int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return errors.New("astc/native: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return errors.New("astc/native: output buffer too small")
	}

	blockX := int(h.BlockX)
	blockY := int(h.BlockY)
	blockZ := int(h.BlockZ)

	dec, err := NewDecoder(blockX, blockY, blockZ, profile, 0)
	if err != nil {
		return err
	}
	defer dec.Close()

	// Keep a similar small-image heuristic as the pure-Go implementation.
	_ = blocksX
	_ = blocksY
	_ = blocksZ
	return dec.DecodeRGBA8VolumeInto(width, height, depth, blocks[:total*astc.BlockBytes], dst)
}

func DecodeRGBAF32VolumeWithProfile(astcData []byte, profile astc.Profile) (pix []float32, width, height, depth int, err error) {
	h, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		return nil, 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return nil, 0, 0, 0, errors.New("astc/native: invalid image dimensions")
	}

	pix = make([]float32, width*height*depth*4)
	if err := DecodeRGBAF32VolumeFromParsedWithProfileInto(profile, h, blocks, pix); err != nil {
		return nil, 0, 0, 0, err
	}
	return pix, width, height, depth, nil
}

func DecodeRGBAF32VolumeWithProfileInto(astcData []byte, profile astc.Profile, dst []float32) (width, height, depth int, err error) {
	h, blocks, err := astc.ParseFile(astcData)
	if err != nil {
		return 0, 0, 0, err
	}

	width = int(h.SizeX)
	height = int(h.SizeY)
	depth = int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return 0, 0, 0, errors.New("astc/native: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return 0, 0, 0, errors.New("astc/native: output buffer too small")
	}
	if err := DecodeRGBAF32VolumeFromParsedWithProfileInto(profile, h, blocks, dst[:width*height*depth*4]); err != nil {
		return 0, 0, 0, err
	}
	return width, height, depth, nil
}

func DecodeRGBAF32VolumeFromParsedWithProfileInto(profile astc.Profile, h astc.Header, blocks []byte, dst []float32) error {
	blocksX, blocksY, blocksZ, total, err := h.BlockCount()
	if err != nil {
		return err
	}
	if len(blocks) < total*astc.BlockBytes {
		return errors.New("astc/native: block buffer too small")
	}

	width := int(h.SizeX)
	height := int(h.SizeY)
	depth := int(h.SizeZ)
	if width <= 0 || height <= 0 || depth <= 0 {
		return errors.New("astc/native: invalid image dimensions")
	}
	if len(dst) < width*height*depth*4 {
		return errors.New("astc/native: output buffer too small")
	}

	blockX := int(h.BlockX)
	blockY := int(h.BlockY)
	blockZ := int(h.BlockZ)

	dec, err := NewDecoder(blockX, blockY, blockZ, profile, 0)
	if err != nil {
		return err
	}
	defer dec.Close()

	_ = blocksX
	_ = blocksY
	_ = blocksZ
	return dec.DecodeRGBAF32VolumeInto(width, height, depth, blocks[:total*astc.BlockBytes], dst)
}
