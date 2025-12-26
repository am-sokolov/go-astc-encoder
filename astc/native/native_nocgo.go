//go:build astcenc_native && !cgo

package native

import (
	"errors"

	"github.com/arm-software/astc-encoder/astc"
)

var errNoCGO = errors.New("astc/native: astcenc_native set but CGO is disabled (set CGO_ENABLED=1)")

func Enabled() bool { return false }

type Encoder struct{}

func NewEncoder(blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality, threadCount int) (*Encoder, error) {
	return nil, errNoCGO
}

func (e *Encoder) Close() error { return errNoCGO }

func (e *Encoder) EncodeRGBA8(pix []byte, width, height int) ([]byte, error) { return nil, errNoCGO }

func (e *Encoder) EncodeRGBA8Volume(pix []byte, width, height, depth int) ([]byte, error) {
	return nil, errNoCGO
}

type EncoderF16 struct{}

func NewEncoderF16(blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality, threadCount int) (*EncoderF16, error) {
	return nil, errNoCGO
}

func (e *EncoderF16) Close() error { return errNoCGO }

func (e *EncoderF16) EncodeRGBAF16(pix []uint16, width, height int) ([]byte, error) {
	return nil, errNoCGO
}

func (e *EncoderF16) EncodeRGBAF16Volume(pix []uint16, width, height, depth int) ([]byte, error) {
	return nil, errNoCGO
}

type EncoderF32 struct{}

func NewEncoderF32(blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality, threadCount int) (*EncoderF32, error) {
	return nil, errNoCGO
}

func (e *EncoderF32) Close() error { return errNoCGO }

func (e *EncoderF32) EncodeRGBAF32(pix []float32, width, height int) ([]byte, error) {
	return nil, errNoCGO
}

func (e *EncoderF32) EncodeRGBAF32Volume(pix []float32, width, height, depth int) ([]byte, error) {
	return nil, errNoCGO
}

type Decoder struct{}

func NewDecoder(blockX, blockY, blockZ int, profile astc.Profile, threadCount int) (*Decoder, error) {
	return nil, errNoCGO
}

func (d *Decoder) Close() error { return errNoCGO }

func (d *Decoder) DecodeRGBA8VolumeInto(width, height, depth int, blocks []byte, dst []byte) error {
	return errNoCGO
}

func (d *Decoder) DecodeRGBAF32VolumeInto(width, height, depth int, blocks []byte, dst []float32) error {
	return errNoCGO
}

func EncodeRGBA8(pix []byte, width, height int, blockX, blockY int) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBA8WithProfileAndQuality(pix []byte, width, height int, blockX, blockY int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBA8Volume(pix []byte, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBA8VolumeWithProfileAndQuality(pix []byte, width, height, depth int, blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF16(pix []uint16, width, height int, blockX, blockY int) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF16WithProfileAndQuality(pix []uint16, width, height int, blockX, blockY int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF16Volume(pix []uint16, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF16VolumeWithProfileAndQuality(pix []uint16, width, height, depth int, blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF32(pix []float32, width, height int, blockX, blockY int) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF32WithProfileAndQuality(pix []float32, width, height int, blockX, blockY int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF32Volume(pix []float32, width, height, depth int, blockX, blockY, blockZ int) ([]byte, error) {
	return nil, errNoCGO
}

func EncodeRGBAF32VolumeWithProfileAndQuality(pix []float32, width, height, depth int, blockX, blockY, blockZ int, profile astc.Profile, quality astc.EncodeQuality) ([]byte, error) {
	return nil, errNoCGO
}

func DecodeRGBA8(astcData []byte) (pix []byte, width, height int, err error) {
	return nil, 0, 0, errNoCGO
}

func DecodeRGBA8WithProfile(astcData []byte, profile astc.Profile) (pix []byte, width, height int, err error) {
	return nil, 0, 0, errNoCGO
}

func DecodeRGBA8VolumeWithProfile(astcData []byte, profile astc.Profile) (pix []byte, width, height, depth int, err error) {
	return nil, 0, 0, 0, errNoCGO
}

func DecodeRGBA8VolumeWithProfileInto(astcData []byte, profile astc.Profile, dst []byte) (width, height, depth int, err error) {
	return 0, 0, 0, errNoCGO
}

func DecodeRGBA8VolumeFromParsedWithProfileInto(profile astc.Profile, h astc.Header, blocks []byte, dst []byte) error {
	return errNoCGO
}

func DecodeRGBAF32VolumeWithProfile(astcData []byte, profile astc.Profile) (pix []float32, width, height, depth int, err error) {
	return nil, 0, 0, 0, errNoCGO
}

func DecodeRGBAF32VolumeWithProfileInto(astcData []byte, profile astc.Profile, dst []float32) (width, height, depth int, err error) {
	return 0, 0, 0, errNoCGO
}

func DecodeRGBAF32VolumeFromParsedWithProfileInto(profile astc.Profile, h astc.Header, blocks []byte, dst []float32) error {
	return errNoCGO
}
