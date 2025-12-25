package astc

import "sync"

type decimationKey struct {
	bx uint8
	by uint8
	bz uint8
	wx uint8
	wy uint8
	wz uint8
}

type decimationEntry struct {
	idx [4]uint8
	w   [4]uint8
}

var decimationTables struct {
	mu sync.RWMutex
	m  map[decimationKey][]decimationEntry
}

func getDecimationTable(blockX, blockY, blockZ, xWeights, yWeights, zWeights int) []decimationEntry {
	key := decimationKey{
		bx: uint8(blockX),
		by: uint8(blockY),
		bz: uint8(blockZ),
		wx: uint8(xWeights),
		wy: uint8(yWeights),
		wz: uint8(zWeights),
	}

	decimationTables.mu.RLock()
	if decimationTables.m != nil {
		if t, ok := decimationTables.m[key]; ok {
			decimationTables.mu.RUnlock()
			return t
		}
	}
	decimationTables.mu.RUnlock()

	decimationTables.mu.Lock()
	defer decimationTables.mu.Unlock()
	if decimationTables.m == nil {
		decimationTables.m = make(map[decimationKey][]decimationEntry)
	} else if t, ok := decimationTables.m[key]; ok {
		return t
	}

	texelCount := blockX * blockY * blockZ
	weightsPerPlane := xWeights * yWeights * zWeights

	table := make([]decimationEntry, texelCount)
	if texelCount == 0 || weightsPerPlane <= 0 {
		decimationTables.m[key] = table
		return table
	}

	// These scale factors and interpolation formulas are direct ports of unpackWeights2D/3D.
	if blockZ == 1 {
		if blockX <= 1 || blockY <= 1 || xWeights <= 0 || yWeights <= 0 {
			decimationTables.m[key] = table
			return table
		}
		xScale := (1024 + blockX/2) / (blockX - 1)
		yScale := (1024 + blockY/2) / (blockY - 1)

		for y := 0; y < blockY; y++ {
			for x := 0; x < blockX; x++ {
				tix := y*blockX + x

				xWeight := (xScale*x*(xWeights-1) + 32) >> 6
				yWeight := (yScale*y*(yWeights-1) + 32) >> 6

				xFrac := xWeight & 0xF
				yFrac := yWeight & 0xF
				xInt := xWeight >> 4
				yInt := yWeight >> 4

				q0 := xInt + yInt*xWeights
				q1 := q0 + 1
				q2 := q0 + xWeights
				q3 := q2 + 1

				prod := xFrac * yFrac
				w3 := (prod + 8) >> 4
				w1 := xFrac - w3
				w2 := yFrac - w3
				w0 := 16 - xFrac - yFrac + w3

				e := decimationEntry{}
				idx := [4]int{q0, q1, q2, q3}
				w := [4]int{w0, w1, w2, w3}
				for i := 0; i < 4; i++ {
					if w[i] == 0 || idx[i] < 0 || idx[i] >= weightsPerPlane {
						continue
					}
					e.idx[i] = uint8(idx[i])
					e.w[i] = uint8(w[i])
				}
				table[tix] = e
			}
		}
	} else {
		if blockX <= 1 || blockY <= 1 || blockZ <= 1 || xWeights <= 0 || yWeights <= 0 || zWeights <= 0 {
			decimationTables.m[key] = table
			return table
		}
		xScale := (1024 + blockX/2) / (blockX - 1)
		yScale := (1024 + blockY/2) / (blockY - 1)
		zScale := (1024 + blockZ/2) / (blockZ - 1)

		N := xWeights
		NM := xWeights * yWeights

		tix := 0
		for z := 0; z < blockZ; z++ {
			for y := 0; y < blockY; y++ {
				for x := 0; x < blockX; x++ {
					xWeight := (xScale*x*(xWeights-1) + 32) >> 6
					yWeight := (yScale*y*(yWeights-1) + 32) >> 6
					zWeight := (zScale*z*(zWeights-1) + 32) >> 6

					fs := xWeight & 0xF
					ft := yWeight & 0xF
					fp := zWeight & 0xF
					xInt := xWeight >> 4
					yInt := yWeight >> 4
					zInt := zWeight >> 4

					q0 := (zInt*yWeights+yInt)*xWeights + xInt
					q3 := ((zInt+1)*yWeights+(yInt+1))*xWeights + (xInt + 1)

					cas := 0
					if fs > ft {
						cas |= 4
					}
					if ft > fp {
						cas |= 2
					}
					if fs > fp {
						cas |= 1
					}

					s1, s2, w0, w1, w2, w3 := 0, 0, 0, 0, 0, 0
					switch cas {
					case 7:
						s1 = 1
						s2 = N
						w0 = 16 - fs
						w1 = fs - ft
						w2 = ft - fp
						w3 = fp
					case 3:
						s1 = N
						s2 = 1
						w0 = 16 - ft
						w1 = ft - fs
						w2 = fs - fp
						w3 = fp
					case 5:
						s1 = 1
						s2 = NM
						w0 = 16 - fs
						w1 = fs - fp
						w2 = fp - ft
						w3 = ft
					case 4:
						s1 = NM
						s2 = 1
						w0 = 16 - fp
						w1 = fp - fs
						w2 = fs - ft
						w3 = ft
					case 2:
						s1 = N
						s2 = NM
						w0 = 16 - ft
						w1 = ft - fp
						w2 = fp - fs
						w3 = fs
					case 0:
						fallthrough
					default:
						s1 = NM
						s2 = N
						w0 = 16 - fp
						w1 = fp - ft
						w2 = ft - fs
						w3 = fs
					}

					q1 := q0 + s1
					q2 := q1 + s2

					e := decimationEntry{}
					idx := [4]int{q0, q1, q2, q3}
					w := [4]int{w0, w1, w2, w3}
					for i := 0; i < 4; i++ {
						if w[i] == 0 || idx[i] < 0 || idx[i] >= weightsPerPlane {
							continue
						}
						e.idx[i] = uint8(idx[i])
						e.w[i] = uint8(w[i])
					}
					table[tix] = e
					tix++
				}
			}
		}
	}

	decimationTables.m[key] = table
	return table
}
