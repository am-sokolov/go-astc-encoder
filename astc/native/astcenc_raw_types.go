package native

import (
	"unsafe"

	"github.com/arm-software/astc-encoder/astc"
)

// Flags is a bitset of encoder/decoder options equivalent to upstream ASTCENC_FLG_*.
type Flags uint32

const (
	FlagMapNormal       Flags = 1 << 0 // ASTCENC_FLG_MAP_NORMAL
	FlagUseDecodeUNORM8 Flags = 1 << 1 // ASTCENC_FLG_USE_DECODE_UNORM8
	FlagUseAlphaWeight  Flags = 1 << 2 // ASTCENC_FLG_USE_ALPHA_WEIGHT
	FlagUsePerceptual   Flags = 1 << 3 // ASTCENC_FLG_USE_PERCEPTUAL
	FlagDecompressOnly  Flags = 1 << 4 // ASTCENC_FLG_DECOMPRESS_ONLY
	FlagSelfDecompress  Flags = 1 << 5 // ASTCENC_FLG_SELF_DECOMPRESS_ONLY
	FlagMapRGBM         Flags = 1 << 6 // ASTCENC_FLG_MAP_RGBM
	FlagAll             Flags = (1 << 7) - 1
)

// Swz is a component selector equivalent to upstream astcenc_swz.
type Swz uint8

const (
	SwzR Swz = iota
	SwzG
	SwzB
	SwzA
	Swz0
	Swz1
	SwzZ
)

// Swizzle is a component mapping equivalent to upstream astcenc_swizzle.
type Swizzle struct {
	R Swz
	G Swz
	B Swz
	A Swz
}

var SwizzleRGBA = Swizzle{R: SwzR, G: SwzG, B: SwzB, A: SwzA}

// DataType is a component storage type equivalent to upstream astcenc_type.
type DataType uint8

const (
	TypeU8 DataType = iota
	TypeF16
	TypeF32
)

// Config is the CGO/native equivalent of upstream astcenc_config.
//
// This struct intentionally mirrors upstream field names (in Go style) to make
// cross-referencing with astcenc.h straightforward.
type Config struct {
	Profile astc.Profile
	Flags   Flags

	BlockX uint32
	BlockY uint32
	BlockZ uint32

	CWRWeight float32
	CWGWeight float32
	CWBWeight float32
	CWAWeight float32

	AScaleRadius uint32
	RGBMMScale   float32

	TunePartitionCountLimit            uint32
	Tune2PartitionIndexLimit           uint32
	Tune3PartitionIndexLimit           uint32
	Tune4PartitionIndexLimit           uint32
	TuneBlockModeLimit                 uint32
	TuneRefinementLimit                uint32
	TuneCandidateLimit                 uint32
	Tune2PartitioningCandidateLimit    uint32
	Tune3PartitioningCandidateLimit    uint32
	Tune4PartitioningCandidateLimit    uint32
	TuneDBLimit                        float32
	TuneMSEOvershoot                   float32
	Tune2PartitionEarlyOutLimitFactor  float32
	Tune3PartitionEarlyOutLimitFactor  float32
	Tune2PlaneEarlyOutLimitCorrelation float32
	TuneSearchMode0Enable              float32

	// ProgressCallback is invoked with progress in [0,100] from one of the
	// worker threads executing CompressImage().
	ProgressCallback func(progress float32)
}

// Image is a tightly-packed RGBA image used for CompressImage/DecompressImage.
//
// The data slices must use x-major order, then y, then z:
// `((z*DimY+y)*DimX + x) * 4`.
type Image struct {
	DimX     int
	DimY     int
	DimZ     int
	DataType DataType

	DataU8  []byte
	DataF16 []uint16
	DataF32 []float32
}

// BlockInfo is the CGO/native equivalent of upstream astcenc_block_info.
type BlockInfo struct {
	Profile astc.Profile

	BlockX     uint32
	BlockY     uint32
	BlockZ     uint32
	TexelCount uint32

	IsErrorBlock     bool
	IsConstantBlock  bool
	IsHDRBlock       bool
	IsDualPlaneBlock bool

	PartitionCount     uint32
	PartitionIndex     uint32
	DualPlaneComponent uint32

	ColorEndpointModes [4]uint32
	ColorLevelCount    uint32
	WeightLevelCount   uint32

	WeightX uint32
	WeightY uint32
	WeightZ uint32

	ColorEndpoints      [4][2][4]float32
	WeightValuesPlane1  [216]float32
	WeightValuesPlane2  [216]float32
	PartitionAssignment [216]uint8
}

// Context is a reusable native astcenc codec context. It can be used to
// sequentially compress and decompress multiple images using the same config.
//
// It mirrors the upstream usage model: for multi-threading, the caller is
// responsible for spawning N workers and calling CompressImage/DecompressImage
// once per thread with a unique thread index.
type Context struct {
	ctx unsafe.Pointer

	cfg         Config
	threadCount int
}
