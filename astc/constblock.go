package astc

import (
	"encoding/binary"
	"errors"
	"math"
)

const (
	// BlockBytes is the size in bytes of a single ASTC block payload.
	BlockBytes = 16
)

var (
	constBlockU16Prefix = [8]byte{0xFC, 0xFD, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	constBlockF16Prefix = [8]byte{0xFC, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
)

// EncodeConstBlockUNorm16 encodes an ASTC constant-color block storing UNORM16 RGBA values.
func EncodeConstBlockUNorm16(r, g, b, a uint16) [BlockBytes]byte {
	var out [BlockBytes]byte
	copy(out[:8], constBlockU16Prefix[:])
	binary.LittleEndian.PutUint16(out[8:10], r)
	binary.LittleEndian.PutUint16(out[10:12], g)
	binary.LittleEndian.PutUint16(out[12:14], b)
	binary.LittleEndian.PutUint16(out[14:16], a)
	return out
}

// EncodeConstBlockRGBA8 encodes an ASTC constant-color block for an RGBA8 pixel value.
//
// The pixel is stored as UNORM16 values using 8->16 bit replication (v*257).
func EncodeConstBlockRGBA8(r, g, b, a uint8) [BlockBytes]byte {
	return EncodeConstBlockUNorm16(
		uint16(r)*257,
		uint16(g)*257,
		uint16(b)*257,
		uint16(a)*257,
	)
}

// EncodeConstBlockF16 encodes an ASTC constant-color block storing FP16 RGBA values.
//
// This block type is only valid in HDR profiles.
func EncodeConstBlockF16(r, g, b, a uint16) [BlockBytes]byte {
	var out [BlockBytes]byte
	copy(out[:8], constBlockF16Prefix[:])
	binary.LittleEndian.PutUint16(out[8:10], r)
	binary.LittleEndian.PutUint16(out[10:12], g)
	binary.LittleEndian.PutUint16(out[12:14], b)
	binary.LittleEndian.PutUint16(out[14:16], a)
	return out
}

// DecodeConstBlockRGBA8 decodes an ASTC constant-color block into an RGBA8 value.
//
// This only supports UNORM16 constant blocks.
func DecodeConstBlockRGBA8(block []byte) (r, g, b, a uint8, err error) {
	if len(block) < BlockBytes {
		return 0, 0, 0, 0, ioErrUnexpectedEOF("astc block", BlockBytes, len(block))
	}

	if isU16ConstBlock(block) {
		ru := binary.LittleEndian.Uint16(block[8:10])
		gu := binary.LittleEndian.Uint16(block[10:12])
		bu := binary.LittleEndian.Uint16(block[12:14])
		au := binary.LittleEndian.Uint16(block[14:16])
		return unorm16ToUnorm8(ru), unorm16ToUnorm8(gu), unorm16ToUnorm8(bu), unorm16ToUnorm8(au), nil
	}

	if isF16ConstBlock(block) {
		rf := halfToFloat32(binary.LittleEndian.Uint16(block[8:10]))
		gf := halfToFloat32(binary.LittleEndian.Uint16(block[10:12]))
		bf := halfToFloat32(binary.LittleEndian.Uint16(block[12:14]))
		af := halfToFloat32(binary.LittleEndian.Uint16(block[14:16]))
		return float01ToUnorm8(rf), float01ToUnorm8(gf), float01ToUnorm8(bf), float01ToUnorm8(af), nil
	}

	return 0, 0, 0, 0, errors.New("astc: not a constant-color block")
}

func isU16ConstBlock(block []byte) bool {
	if len(block) < BlockBytes {
		return false
	}
	return block[0] == constBlockU16Prefix[0] &&
		block[1] == constBlockU16Prefix[1] &&
		block[2] == constBlockU16Prefix[2] &&
		block[3] == constBlockU16Prefix[3] &&
		block[4] == constBlockU16Prefix[4] &&
		block[5] == constBlockU16Prefix[5] &&
		block[6] == constBlockU16Prefix[6] &&
		block[7] == constBlockU16Prefix[7]
}

func isF16ConstBlock(block []byte) bool {
	if len(block) < BlockBytes {
		return false
	}
	return block[0] == constBlockF16Prefix[0] &&
		block[1] == constBlockF16Prefix[1] &&
		block[2] == constBlockF16Prefix[2] &&
		block[3] == constBlockF16Prefix[3] &&
		block[4] == constBlockF16Prefix[4] &&
		block[5] == constBlockF16Prefix[5] &&
		block[6] == constBlockF16Prefix[6] &&
		block[7] == constBlockF16Prefix[7]
}

func unorm16ToUnorm8(v uint16) uint8 {
	// Round to nearest while mapping [0,65535] -> [0,255].
	//
	// For values written via 8->16 replication (x*257), this is exactly x.
	return uint8((uint32(v) + 128) / 257)
}

func float01ToUnorm8(v float32) uint8 {
	// Handle NaNs.
	if !(v >= 0) {
		return 0
	}
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// halfToFloat32 converts an IEEE 754 binary16 float to float32.
//
// This is a small helper for future FP16 constant-block support.
func halfToFloat32(h uint16) float32 {
	sign := uint32(h>>15) & 0x1
	exp := uint32(h>>10) & 0x1F
	mant := uint32(h) & 0x3FF

	switch exp {
	case 0:
		if mant == 0 {
			return math.Float32frombits(sign << 31)
		}
		// Subnormal -> normalized float32.
		// Scale mantissa into float32 and adjust exponent.
		e := int32(-14)
		for (mant & 0x400) == 0 {
			mant <<= 1
			e--
		}
		mant &= 0x3FF
		exp32 := uint32(e + 127)
		mant32 := mant << 13
		return math.Float32frombits((sign << 31) | (exp32 << 23) | mant32)
	case 0x1F:
		// Inf/NaN
		return math.Float32frombits((sign << 31) | 0x7F800000 | (mant << 13))
	default:
		// Normal number.
		exp32 := exp + (127 - 15)
		mant32 := mant << 13
		return math.Float32frombits((sign << 31) | (exp32 << 23) | mant32)
	}
}

func float32ToHalf(f float32) uint16 {
	bits := math.Float32bits(f)
	sign := uint16((bits >> 16) & 0x8000)
	exp := int32((bits >> 23) & 0xFF)
	mant := bits & 0x7FFFFF

	// Inf/NaN.
	if exp == 0xFF {
		if mant == 0 {
			return sign | 0x7C00
		}
		// Quiet NaN; preserve payload bits where possible.
		payload := uint16(mant>>13) & 0x03FF
		if payload == 0 {
			payload = 1
		}
		return sign | 0x7C00 | payload
	}

	// Convert exponent bias from 127 to 15.
	exp = exp - 127 + 15

	// Subnormals/underflow.
	if exp <= 0 {
		if exp < -10 {
			// Too small -> signed zero.
			return sign
		}

		// Convert to a subnormal half.
		mant |= 0x800000 // implicit leading 1
		shift := uint32(1 - exp)

		// Round to nearest, ties to even.
		roundBit := uint32(0x1000) << shift
		mant = mant + roundBit
		halfMant := uint16(mant >> (13 + shift))
		return sign | halfMant
	}

	// Overflow -> inf.
	if exp >= 0x1F {
		return sign | 0x7C00
	}

	// Normalized number.
	// Round to nearest, ties to even.
	mant = mant + 0x1000
	if mant&0x800000 != 0 {
		mant = 0
		exp++
		if exp >= 0x1F {
			return sign | 0x7C00
		}
	}
	return sign | (uint16(exp) << 10) | uint16(mant>>13)
}
