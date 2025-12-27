package astc

import "sort"

// selectBestPartitionIndicesU16 picks a small set of promising partition seeds to try for 16-bit
// per-channel texel data.
//
// The semantics match selectBestPartitionIndices(), but operate on code values in the 0..65535
// range (e.g. UNORM16 or LNS codes).
func selectBestPartitionIndicesU16(dst []int, texels [][4]uint16, pt *partitionTable, partitionCount int, searchLimit int, includeAlpha bool) int {
	if pt == nil || len(dst) == 0 || searchLimit <= 0 || partitionCount < 2 || partitionCount > 4 {
		return 0
	}
	texelCount := pt.texelCount
	if texelCount <= 0 || len(texels) < texelCount {
		return 0
	}

	limit := searchLimit
	if limit > (1 << partitionIndexBits) {
		limit = 1 << partitionIndexBits
	}

	var scoresArr [128]uint64
	scores := scoresArr[:len(dst)]

	bestCount := 0
	for pidx := 0; pidx < limit; pidx++ {
		assign := pt.partitionsForIndex(pidx)

		var score uint64
		switch partitionCount {
		case 2:
			var count0, count1 uint32
			var sum0r, sum0g, sum0b, sum0a uint64
			var sum1r, sum1g, sum1b, sum1a uint64
			var sq0r, sq0g, sq0b, sq0a uint64
			var sq1r, sq1g, sq1b, sq1a uint64

			for t := 0; t < texelCount; t++ {
				part := int(assign[t])
				r := uint64(texels[t][0])
				g := uint64(texels[t][1])
				b := uint64(texels[t][2])
				a := uint64(texels[t][3])

				if part == 0 {
					count0++
					sum0r += r
					sum0g += g
					sum0b += b
					sum0a += a
					sq0r += r * r
					sq0g += g * g
					sq0b += b * b
					sq0a += a * a
				} else {
					count1++
					sum1r += r
					sum1g += g
					sum1b += b
					sum1a += a
					sq1r += r * r
					sq1g += g * g
					sq1b += b * b
					sq1a += a * a
				}
			}

			if count0 == 0 || count1 == 0 {
				continue
			}
			n0 := uint64(count0)
			n1 := uint64(count1)

			s := sum0r
			score += sq0r - (s*s)/n0
			s = sum0g
			score += sq0g - (s*s)/n0
			s = sum0b
			score += sq0b - (s*s)/n0
			if includeAlpha {
				s = sum0a
				score += sq0a - (s*s)/n0
			}

			s = sum1r
			score += sq1r - (s*s)/n1
			s = sum1g
			score += sq1g - (s*s)/n1
			s = sum1b
			score += sq1b - (s*s)/n1
			if includeAlpha {
				s = sum1a
				score += sq1a - (s*s)/n1
			}
		case 3:
			var count0, count1, count2 uint32
			var sum0r, sum0g, sum0b, sum0a uint64
			var sum1r, sum1g, sum1b, sum1a uint64
			var sum2r, sum2g, sum2b, sum2a uint64
			var sq0r, sq0g, sq0b, sq0a uint64
			var sq1r, sq1g, sq1b, sq1a uint64
			var sq2r, sq2g, sq2b, sq2a uint64

			for t := 0; t < texelCount; t++ {
				part := int(assign[t])
				r := uint64(texels[t][0])
				g := uint64(texels[t][1])
				b := uint64(texels[t][2])
				a := uint64(texels[t][3])

				switch part {
				case 0:
					count0++
					sum0r += r
					sum0g += g
					sum0b += b
					sum0a += a
					sq0r += r * r
					sq0g += g * g
					sq0b += b * b
					sq0a += a * a
				case 1:
					count1++
					sum1r += r
					sum1g += g
					sum1b += b
					sum1a += a
					sq1r += r * r
					sq1g += g * g
					sq1b += b * b
					sq1a += a * a
				default:
					count2++
					sum2r += r
					sum2g += g
					sum2b += b
					sum2a += a
					sq2r += r * r
					sq2g += g * g
					sq2b += b * b
					sq2a += a * a
				}
			}

			if count0 == 0 || count1 == 0 || count2 == 0 {
				continue
			}
			n0 := uint64(count0)
			n1 := uint64(count1)
			n2 := uint64(count2)

			s := sum0r
			score += sq0r - (s*s)/n0
			s = sum0g
			score += sq0g - (s*s)/n0
			s = sum0b
			score += sq0b - (s*s)/n0
			if includeAlpha {
				s = sum0a
				score += sq0a - (s*s)/n0
			}

			s = sum1r
			score += sq1r - (s*s)/n1
			s = sum1g
			score += sq1g - (s*s)/n1
			s = sum1b
			score += sq1b - (s*s)/n1
			if includeAlpha {
				s = sum1a
				score += sq1a - (s*s)/n1
			}

			s = sum2r
			score += sq2r - (s*s)/n2
			s = sum2g
			score += sq2g - (s*s)/n2
			s = sum2b
			score += sq2b - (s*s)/n2
			if includeAlpha {
				s = sum2a
				score += sq2a - (s*s)/n2
			}
		case 4:
			var count0, count1, count2, count3 uint32
			var sum0r, sum0g, sum0b, sum0a uint64
			var sum1r, sum1g, sum1b, sum1a uint64
			var sum2r, sum2g, sum2b, sum2a uint64
			var sum3r, sum3g, sum3b, sum3a uint64
			var sq0r, sq0g, sq0b, sq0a uint64
			var sq1r, sq1g, sq1b, sq1a uint64
			var sq2r, sq2g, sq2b, sq2a uint64
			var sq3r, sq3g, sq3b, sq3a uint64

			for t := 0; t < texelCount; t++ {
				part := int(assign[t])
				r := uint64(texels[t][0])
				g := uint64(texels[t][1])
				b := uint64(texels[t][2])
				a := uint64(texels[t][3])

				switch part {
				case 0:
					count0++
					sum0r += r
					sum0g += g
					sum0b += b
					sum0a += a
					sq0r += r * r
					sq0g += g * g
					sq0b += b * b
					sq0a += a * a
				case 1:
					count1++
					sum1r += r
					sum1g += g
					sum1b += b
					sum1a += a
					sq1r += r * r
					sq1g += g * g
					sq1b += b * b
					sq1a += a * a
				case 2:
					count2++
					sum2r += r
					sum2g += g
					sum2b += b
					sum2a += a
					sq2r += r * r
					sq2g += g * g
					sq2b += b * b
					sq2a += a * a
				default:
					count3++
					sum3r += r
					sum3g += g
					sum3b += b
					sum3a += a
					sq3r += r * r
					sq3g += g * g
					sq3b += b * b
					sq3a += a * a
				}
			}

			if count0 == 0 || count1 == 0 || count2 == 0 || count3 == 0 {
				continue
			}
			n0 := uint64(count0)
			n1 := uint64(count1)
			n2 := uint64(count2)
			n3 := uint64(count3)

			s := sum0r
			score += sq0r - (s*s)/n0
			s = sum0g
			score += sq0g - (s*s)/n0
			s = sum0b
			score += sq0b - (s*s)/n0
			if includeAlpha {
				s = sum0a
				score += sq0a - (s*s)/n0
			}

			s = sum1r
			score += sq1r - (s*s)/n1
			s = sum1g
			score += sq1g - (s*s)/n1
			s = sum1b
			score += sq1b - (s*s)/n1
			if includeAlpha {
				s = sum1a
				score += sq1a - (s*s)/n1
			}

			s = sum2r
			score += sq2r - (s*s)/n2
			s = sum2g
			score += sq2g - (s*s)/n2
			s = sum2b
			score += sq2b - (s*s)/n2
			if includeAlpha {
				s = sum2a
				score += sq2a - (s*s)/n2
			}

			s = sum3r
			score += sq3r - (s*s)/n3
			s = sum3g
			score += sq3g - (s*s)/n3
			s = sum3b
			score += sq3b - (s*s)/n3
			if includeAlpha {
				s = sum3a
				score += sq3a - (s*s)/n3
			}
		}

		if bestCount < len(dst) {
			dst[bestCount] = pidx
			scores[bestCount] = score
			bestCount++
			continue
		}

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

	sort.Ints(dst[:bestCount])
	return bestCount
}
