package astc

import (
	"sync"
	"sync/atomic"
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

// Config is a Go equivalent of upstream astcenc_config.
type Config struct {
	Profile Profile
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

	ProgressCallback func(progress float32)
}

// Image is a tightly-packed RGBA image used for CompressImage/DecompressImage.
type Image struct {
	DimX     int
	DimY     int
	DimZ     int
	DataType DataType

	DataU8  []byte
	DataF16 []uint16
	DataF32 []float32
}

// BlockInfo is a Go equivalent of upstream astcenc_block_info.
type BlockInfo struct {
	Profile Profile

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

type contextState uint32

const (
	ctxIdle contextState = iota
	ctxCompressActive
	ctxDecompressActive
)

// Context is a reusable pure-Go codec context modeled after upstream astcenc_context.
//
// It can compress or decompress only one image at a time (mirroring upstream). For multi-threaded
// use, callers should create N goroutines and call CompressImage/DecompressImage once per worker
// with a unique thread index.
type Context struct {
	cfg         Config
	threadCount int

	blockX int
	blockY int
	blockZ int

	decodeCtx *decodeContext

	// Derived from cfg.TuneDBLimit during context allocation (mirrors upstream context behavior).
	// For LDR profiles this becomes a linear MSE threshold, not a dB value.
	tuneDBLimitInternal float32

	// One active operation at a time.
	state atomic.Uint32

	compress   opState
	decompress opState
}

type opState struct {
	needsReset atomic.Uint32

	// 0 idle, 1 initializing, 2 active
	initState atomic.Uint32
	workers   atomic.Int32

	cancel atomic.Uint32

	// Task scheduling.
	totalBlocks atomic.Uint32
	nextBlock   atomic.Uint32
	doneBlocks  atomic.Uint32

	// Progress callback throttling (mirrors upstream ParallelManager behavior).
	progressMu            sync.Mutex
	progressMinDiffBits   atomic.Uint32 // float32 bits
	progressLastValueBits atomic.Uint32 // float32 bits

	// Alpha-scale RDO precompute (mirrors upstream input_alpha_averages).
	inputAlphaAverages []float32
}
