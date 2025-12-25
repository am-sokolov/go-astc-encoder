package astc

import "sync"

type partitionTableKey struct {
	bx uint8
	by uint8
	bz uint8
	pc uint8
}

type partitionTable struct {
	texelCount int
	// data is indexed as [partitionIndex][texelIndex] where partitionIndex is 0..1023.
	data []uint8
}

var partitionTables struct {
	mu sync.RWMutex
	m  map[partitionTableKey]*partitionTable
}

func getPartitionTable(blockX, blockY, blockZ, partitionCount int) *partitionTable {
	if partitionCount <= 1 {
		return nil
	}

	key := partitionTableKey{
		bx: uint8(blockX),
		by: uint8(blockY),
		bz: uint8(blockZ),
		pc: uint8(partitionCount),
	}

	partitionTables.mu.RLock()
	if partitionTables.m != nil {
		if t := partitionTables.m[key]; t != nil {
			partitionTables.mu.RUnlock()
			return t
		}
	}
	partitionTables.mu.RUnlock()

	partitionTables.mu.Lock()
	defer partitionTables.mu.Unlock()
	if partitionTables.m == nil {
		partitionTables.m = make(map[partitionTableKey]*partitionTable)
	} else if t := partitionTables.m[key]; t != nil {
		return t
	}

	texelCount := blockX * blockY * blockZ
	smallBlock := texelCount < 32
	data := make([]uint8, (1<<partitionIndexBits)*texelCount)

	for pidx := 0; pidx < (1 << partitionIndexBits); pidx++ {
		base := pidx * texelCount
		tix := 0
		for z := 0; z < blockZ; z++ {
			for y := 0; y < blockY; y++ {
				for x := 0; x < blockX; x++ {
					data[base+tix] = selectPartition(pidx, x, y, z, partitionCount, smallBlock)
					tix++
				}
			}
		}
	}

	t := &partitionTable{texelCount: texelCount, data: data}
	partitionTables.m[key] = t
	return t
}

func (t *partitionTable) partitionsForIndex(partitionIndex int) []uint8 {
	if t == nil {
		return nil
	}
	// The ASTC format encodes 10 bits for the partition index.
	partitionIndex &= (1 << partitionIndexBits) - 1
	base := partitionIndex * t.texelCount
	return t.data[base : base+t.texelCount]
}
