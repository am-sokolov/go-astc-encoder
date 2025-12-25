package astc

import "sort"

// selectBestPartitionIndices picks a small set of promising partition seeds to try.
//
// It ranks seeds by their total within-partition SSE in RGB (and A if includeAlpha is true),
// and returns a deterministic list sorted by partition index.
//
// The dst slice is used as output storage; the returned value is the number of entries written.
func selectBestPartitionIndices(dst []int, texels []byte, pt *partitionTable, partitionCount int, searchLimit int, includeAlpha bool) int {
	if pt == nil || len(dst) == 0 || searchLimit <= 0 || partitionCount < 2 || partitionCount > 4 {
		return 0
	}
	texelCount := pt.texelCount
	if texelCount <= 0 || len(texels) < texelCount*4 {
		return 0
	}

	limit := searchLimit
	if limit > (1 << partitionIndexBits) {
		limit = 1 << partitionIndexBits
	}

	// Keep the best N candidates in-place in dst, tracking their scores separately.
	// N is small (<=128), and limit is at most 1024, so O(N*limit) selection is fine.
	var scoresArr [128]uint64
	scores := scoresArr[:len(dst)]

	bestCount := 0
	for pidx := 0; pidx < limit; pidx++ {
		assign := pt.partitionsForIndex(pidx)

		var score uint64
		switch partitionCount {
		case 2:
			var count0, count1 uint32
			var sum0r, sum0g, sum0b, sum0a uint32
			var sum1r, sum1g, sum1b, sum1a uint32
			var sq0r, sq0g, sq0b, sq0a uint64
			var sq1r, sq1g, sq1b, sq1a uint64

			for t := 0; t < texelCount; t++ {
				part := int(assign[t])
				off := t * 4
				r := uint32(texels[off+0])
				g := uint32(texels[off+1])
				b := uint32(texels[off+2])
				a := uint32(texels[off+3])

				if part == 0 {
					count0++
					sum0r += r
					sum0g += g
					sum0b += b
					sum0a += a
					sq0r += uint64(r * r)
					sq0g += uint64(g * g)
					sq0b += uint64(b * b)
					sq0a += uint64(a * a)
				} else {
					count1++
					sum1r += r
					sum1g += g
					sum1b += b
					sum1a += a
					sq1r += uint64(r * r)
					sq1g += uint64(g * g)
					sq1b += uint64(b * b)
					sq1a += uint64(a * a)
				}
			}

			if count0 == 0 || count1 == 0 {
				continue
			}
			n0 := uint64(count0)
			n1 := uint64(count1)

			s := uint64(sum0r)
			score += sq0r - (s*s)/n0
			s = uint64(sum0g)
			score += sq0g - (s*s)/n0
			s = uint64(sum0b)
			score += sq0b - (s*s)/n0
			if includeAlpha {
				s = uint64(sum0a)
				score += sq0a - (s*s)/n0
			}

			s = uint64(sum1r)
			score += sq1r - (s*s)/n1
			s = uint64(sum1g)
			score += sq1g - (s*s)/n1
			s = uint64(sum1b)
			score += sq1b - (s*s)/n1
			if includeAlpha {
				s = uint64(sum1a)
				score += sq1a - (s*s)/n1
			}

		case 3:
			var count0, count1, count2 uint32
			var sum0r, sum0g, sum0b, sum0a uint32
			var sum1r, sum1g, sum1b, sum1a uint32
			var sum2r, sum2g, sum2b, sum2a uint32
			var sq0r, sq0g, sq0b, sq0a uint64
			var sq1r, sq1g, sq1b, sq1a uint64
			var sq2r, sq2g, sq2b, sq2a uint64

			for t := 0; t < texelCount; t++ {
				part := int(assign[t])
				off := t * 4
				r := uint32(texels[off+0])
				g := uint32(texels[off+1])
				b := uint32(texels[off+2])
				a := uint32(texels[off+3])

				switch part {
				case 0:
					count0++
					sum0r += r
					sum0g += g
					sum0b += b
					sum0a += a
					sq0r += uint64(r * r)
					sq0g += uint64(g * g)
					sq0b += uint64(b * b)
					sq0a += uint64(a * a)
				case 1:
					count1++
					sum1r += r
					sum1g += g
					sum1b += b
					sum1a += a
					sq1r += uint64(r * r)
					sq1g += uint64(g * g)
					sq1b += uint64(b * b)
					sq1a += uint64(a * a)
				default:
					count2++
					sum2r += r
					sum2g += g
					sum2b += b
					sum2a += a
					sq2r += uint64(r * r)
					sq2g += uint64(g * g)
					sq2b += uint64(b * b)
					sq2a += uint64(a * a)
				}
			}

			if count0 == 0 || count1 == 0 || count2 == 0 {
				continue
			}
			n0 := uint64(count0)
			n1 := uint64(count1)
			n2 := uint64(count2)

			s := uint64(sum0r)
			score += sq0r - (s*s)/n0
			s = uint64(sum0g)
			score += sq0g - (s*s)/n0
			s = uint64(sum0b)
			score += sq0b - (s*s)/n0
			if includeAlpha {
				s = uint64(sum0a)
				score += sq0a - (s*s)/n0
			}

			s = uint64(sum1r)
			score += sq1r - (s*s)/n1
			s = uint64(sum1g)
			score += sq1g - (s*s)/n1
			s = uint64(sum1b)
			score += sq1b - (s*s)/n1
			if includeAlpha {
				s = uint64(sum1a)
				score += sq1a - (s*s)/n1
			}

			s = uint64(sum2r)
			score += sq2r - (s*s)/n2
			s = uint64(sum2g)
			score += sq2g - (s*s)/n2
			s = uint64(sum2b)
			score += sq2b - (s*s)/n2
			if includeAlpha {
				s = uint64(sum2a)
				score += sq2a - (s*s)/n2
			}

		case 4:
			var count0, count1, count2, count3 uint32
			var sum0r, sum0g, sum0b, sum0a uint32
			var sum1r, sum1g, sum1b, sum1a uint32
			var sum2r, sum2g, sum2b, sum2a uint32
			var sum3r, sum3g, sum3b, sum3a uint32
			var sq0r, sq0g, sq0b, sq0a uint64
			var sq1r, sq1g, sq1b, sq1a uint64
			var sq2r, sq2g, sq2b, sq2a uint64
			var sq3r, sq3g, sq3b, sq3a uint64

			for t := 0; t < texelCount; t++ {
				part := int(assign[t])
				off := t * 4
				r := uint32(texels[off+0])
				g := uint32(texels[off+1])
				b := uint32(texels[off+2])
				a := uint32(texels[off+3])

				switch part {
				case 0:
					count0++
					sum0r += r
					sum0g += g
					sum0b += b
					sum0a += a
					sq0r += uint64(r * r)
					sq0g += uint64(g * g)
					sq0b += uint64(b * b)
					sq0a += uint64(a * a)
				case 1:
					count1++
					sum1r += r
					sum1g += g
					sum1b += b
					sum1a += a
					sq1r += uint64(r * r)
					sq1g += uint64(g * g)
					sq1b += uint64(b * b)
					sq1a += uint64(a * a)
				case 2:
					count2++
					sum2r += r
					sum2g += g
					sum2b += b
					sum2a += a
					sq2r += uint64(r * r)
					sq2g += uint64(g * g)
					sq2b += uint64(b * b)
					sq2a += uint64(a * a)
				default:
					count3++
					sum3r += r
					sum3g += g
					sum3b += b
					sum3a += a
					sq3r += uint64(r * r)
					sq3g += uint64(g * g)
					sq3b += uint64(b * b)
					sq3a += uint64(a * a)
				}
			}

			if count0 == 0 || count1 == 0 || count2 == 0 || count3 == 0 {
				continue
			}
			n0 := uint64(count0)
			n1 := uint64(count1)
			n2 := uint64(count2)
			n3 := uint64(count3)

			s := uint64(sum0r)
			score += sq0r - (s*s)/n0
			s = uint64(sum0g)
			score += sq0g - (s*s)/n0
			s = uint64(sum0b)
			score += sq0b - (s*s)/n0
			if includeAlpha {
				s = uint64(sum0a)
				score += sq0a - (s*s)/n0
			}

			s = uint64(sum1r)
			score += sq1r - (s*s)/n1
			s = uint64(sum1g)
			score += sq1g - (s*s)/n1
			s = uint64(sum1b)
			score += sq1b - (s*s)/n1
			if includeAlpha {
				s = uint64(sum1a)
				score += sq1a - (s*s)/n1
			}

			s = uint64(sum2r)
			score += sq2r - (s*s)/n2
			s = uint64(sum2g)
			score += sq2g - (s*s)/n2
			s = uint64(sum2b)
			score += sq2b - (s*s)/n2
			if includeAlpha {
				s = uint64(sum2a)
				score += sq2a - (s*s)/n2
			}

			s = uint64(sum3r)
			score += sq3r - (s*s)/n3
			s = uint64(sum3g)
			score += sq3g - (s*s)/n3
			s = uint64(sum3b)
			score += sq3b - (s*s)/n3
			if includeAlpha {
				s = uint64(sum3a)
				score += sq3a - (s*s)/n3
			}
		}

		if bestCount < len(dst) {
			dst[bestCount] = pidx
			scores[bestCount] = score
			bestCount++
			continue
		}

		// Replace the current worst candidate if this one is better.
		worst := 0
		worstScore := scores[0]
		worstIdx := dst[0]
		for i := 1; i < bestCount; i++ {
			s := scores[i]
			pi := dst[i]
			if s > worstScore || (s == worstScore && pi > worstIdx) {
				worst = i
				worstScore = s
				worstIdx = pi
			}
		}
		if score < worstScore || (score == worstScore && pidx < worstIdx) {
			dst[worst] = pidx
			scores[worst] = score
		}
	}

	if bestCount == 0 {
		return 0
	}

	// Ensure deterministic evaluation order (ascending index).
	sort.Ints(dst[:bestCount])
	return bestCount
}

// selectBestPartitionIndices2 is a specialized wrapper for the most common encoder case.
func selectBestPartitionIndices2(dst []int, texels []byte, pt *partitionTable, searchLimit int, includeAlpha bool) int {
	return selectBestPartitionIndices(dst, texels, pt, 2, searchLimit, includeAlpha)
}
