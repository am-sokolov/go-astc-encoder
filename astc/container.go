package astc

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var astcMagic = [4]byte{0x13, 0xAB, 0xA1, 0x5C}

// Header is the 16-byte ASTC file header, as stored in .astc files.
//
// It describes the compressed block footprint and the uncompressed image size.
type Header struct {
	BlockX uint8
	BlockY uint8
	BlockZ uint8

	SizeX uint32
	SizeY uint32
	SizeZ uint32
}

func (h Header) String() string {
	return fmt.Sprintf("ASTC %dx%dx%d blocks, %dx%dx%d texels",
		h.BlockX, h.BlockY, h.BlockZ,
		h.SizeX, h.SizeY, h.SizeZ)
}

func (h Header) validate() error {
	if h.BlockX == 0 || h.BlockY == 0 || h.BlockZ == 0 {
		return errors.New("astc: invalid header: zero block dimension")
	}
	if h.SizeX == 0 || h.SizeY == 0 || h.SizeZ == 0 {
		return errors.New("astc: invalid header: zero image dimension")
	}
	return nil
}

// BlockCount returns the number of compressed blocks for this image.
func (h Header) BlockCount() (blocksX, blocksY, blocksZ, total int, err error) {
	if err := h.validate(); err != nil {
		return 0, 0, 0, 0, err
	}

	blocksX = int((h.SizeX + uint32(h.BlockX) - 1) / uint32(h.BlockX))
	blocksY = int((h.SizeY + uint32(h.BlockY) - 1) / uint32(h.BlockY))
	blocksZ = int((h.SizeZ + uint32(h.BlockZ) - 1) / uint32(h.BlockZ))
	if blocksX <= 0 || blocksY <= 0 || blocksZ <= 0 {
		return 0, 0, 0, 0, errors.New("astc: invalid header: computed non-positive block count")
	}

	total = blocksX * blocksY * blocksZ
	if total/blocksX/blocksY != blocksZ { // overflow check
		return 0, 0, 0, 0, errors.New("astc: invalid header: block count overflow")
	}
	return blocksX, blocksY, blocksZ, total, nil
}

// HeaderSize is the size in bytes of an ASTC file header.
const HeaderSize = 16

// ParseHeader parses the 16-byte ASTC file header.
func ParseHeader(data []byte) (Header, error) {
	if len(data) < HeaderSize {
		return Header{}, ioErrUnexpectedEOF("astc header", HeaderSize, len(data))
	}
	if data[0] != astcMagic[0] || data[1] != astcMagic[1] || data[2] != astcMagic[2] || data[3] != astcMagic[3] {
		return Header{}, errors.New("astc: invalid magic")
	}

	h := Header{
		BlockX: data[4],
		BlockY: data[5],
		BlockZ: data[6],
		SizeX:  decodeU24LE(data[7:10]),
		SizeY:  decodeU24LE(data[10:13]),
		SizeZ:  decodeU24LE(data[13:16]),
	}
	if err := h.validate(); err != nil {
		return Header{}, err
	}
	return h, nil
}

// MarshalHeader returns the 16-byte ASTC header encoding for h.
func MarshalHeader(h Header) ([HeaderSize]byte, error) {
	if err := h.validate(); err != nil {
		return [HeaderSize]byte{}, err
	}

	var out [HeaderSize]byte
	copy(out[0:4], astcMagic[:])
	out[4] = h.BlockX
	out[5] = h.BlockY
	out[6] = h.BlockZ
	encodeU24LE(out[7:10], h.SizeX)
	encodeU24LE(out[10:13], h.SizeY)
	encodeU24LE(out[13:16], h.SizeZ)
	return out, nil
}

// ParseFile parses a full .astc file.
//
// It returns the header and a slice of 16-byte blocks (the slice aliases data).
func ParseFile(data []byte) (Header, []byte, error) {
	h, err := ParseHeader(data)
	if err != nil {
		return Header{}, nil, err
	}

	_, _, _, total, err := h.BlockCount()
	if err != nil {
		return Header{}, nil, err
	}

	need := HeaderSize + total*16
	if len(data) < need {
		return Header{}, nil, ioErrUnexpectedEOF("astc file", need, len(data))
	}
	if len(data) > need {
		// Allow trailing data but reject non-zero padding to catch accidental concatenation.
		tail := data[need:]
		for _, b := range tail {
			if b != 0 {
				return Header{}, nil, errors.New("astc: trailing non-zero data")
			}
		}
	}

	return h, data[HeaderSize:need], nil
}

func decodeU24LE(b []byte) uint32 {
	// b must be at least 3 bytes.
	_ = b[2]
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16
}

func encodeU24LE(dst []byte, v uint32) {
	// dst must be at least 3 bytes.
	_ = dst[2]
	if v > 0xFFFFFF {
		// Clamp rather than error; the caller's validate() should have caught this already.
		v = 0xFFFFFF
	}
	dst[0] = byte(v)
	dst[1] = byte(v >> 8)
	dst[2] = byte(v >> 16)
}

func ioErrUnexpectedEOF(what string, want, got int) error {
	return fmt.Errorf("astc: %s: unexpected EOF: want %d bytes, got %d", what, want, got)
}

// littleEndianU16 is a tiny helper to make it obvious we are using LE in block payloads.
func littleEndianU16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
