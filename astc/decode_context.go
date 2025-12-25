package astc

import "sync"

type blockModeInfo struct {
	ok            bool
	xWeights      uint8
	yWeights      uint8
	zWeights      uint8
	isDualPlane   bool
	weightQuant   quantMethod
	weightBits    uint8
	weightCount   uint8 // Number of weights per plane.
	realWeightCnt uint8 // Total number of ISE symbols (includes dual-plane).
	noDecimation  bool
	decimation    []decimationEntry
}

type decodeContext struct {
	blockX     int
	blockY     int
	blockZ     int
	texelCount int

	blockModes [1 << 11]blockModeInfo

	partitionTables [blockMaxPartitions + 1]*partitionTable
}

type decodeContextKey struct {
	bx uint8
	by uint8
	bz uint8
}

var decodeContexts struct {
	mu sync.RWMutex
	m  map[decodeContextKey]*decodeContext
}

func getDecodeContext(blockX, blockY, blockZ int) *decodeContext {
	key := decodeContextKey{bx: uint8(blockX), by: uint8(blockY), bz: uint8(blockZ)}

	decodeContexts.mu.RLock()
	if decodeContexts.m != nil {
		if ctx := decodeContexts.m[key]; ctx != nil {
			decodeContexts.mu.RUnlock()
			return ctx
		}
	}
	decodeContexts.mu.RUnlock()

	decodeContexts.mu.Lock()
	defer decodeContexts.mu.Unlock()

	if decodeContexts.m == nil {
		decodeContexts.m = make(map[decodeContextKey]*decodeContext)
	} else if ctx := decodeContexts.m[key]; ctx != nil {
		return ctx
	}

	ctx := newDecodeContext(blockX, blockY, blockZ)
	decodeContexts.m[key] = ctx
	return ctx
}

func newDecodeContext(blockX, blockY, blockZ int) *decodeContext {
	ctx := &decodeContext{
		blockX:     blockX,
		blockY:     blockY,
		blockZ:     blockZ,
		texelCount: blockX * blockY * blockZ,
	}

	for pc := 2; pc <= blockMaxPartitions; pc++ {
		ctx.partitionTables[pc] = getPartitionTable(blockX, blockY, blockZ, pc)
	}

	for bm := 0; bm < (1 << 11); bm++ {
		var (
			xWeights, yWeights, zWeights int
			isDualPlane                  bool
			weightQuant                  quantMethod
			weightBits                   int
			ok                           bool
		)

		if blockZ == 1 {
			xWeights, yWeights, isDualPlane, weightQuant, weightBits, ok = decodeBlockMode2D(bm)
			zWeights = 1
			if ok && (xWeights > blockX || yWeights > blockY) {
				ok = false
			}
		} else {
			xWeights, yWeights, zWeights, isDualPlane, weightQuant, weightBits, ok = decodeBlockMode3D(bm)
			if ok && (xWeights > blockX || yWeights > blockY || zWeights > blockZ) {
				ok = false
			}
		}

		if !ok {
			continue
		}

		weightCountPerPlane := xWeights * yWeights * zWeights
		realWeightCount := weightCountPerPlane
		if isDualPlane {
			realWeightCount *= 2
		}

		ctx.blockModes[bm] = blockModeInfo{
			ok:            true,
			xWeights:      uint8(xWeights),
			yWeights:      uint8(yWeights),
			zWeights:      uint8(zWeights),
			isDualPlane:   isDualPlane,
			weightQuant:   weightQuant,
			weightBits:    uint8(weightBits),
			weightCount:   uint8(weightCountPerPlane),
			realWeightCnt: uint8(realWeightCount),
			noDecimation:  xWeights == blockX && yWeights == blockY && zWeights == blockZ,
			decimation:    getDecimationTable(blockX, blockY, blockZ, xWeights, yWeights, zWeights),
		}
	}

	return ctx
}
