package astc

func avgBlockRGBA8Scalar(pix []byte, width, height, x0, y0, blockX, blockY int) (r, g, b, a uint8) {
	var sumR, sumG, sumB, sumA uint32
	var count uint32

	for yy := 0; yy < blockY; yy++ {
		y := y0 + yy
		if y >= height {
			break
		}
		row := y * width * 4
		for xx := 0; xx < blockX; xx++ {
			x := x0 + xx
			if x >= width {
				break
			}
			p := row + x*4
			sumR += uint32(pix[p+0])
			sumG += uint32(pix[p+1])
			sumB += uint32(pix[p+2])
			sumA += uint32(pix[p+3])
			count++
		}
	}

	if count == 0 {
		return 0, 0, 0, 0
	}

	half := count / 2
	return uint8((sumR + half) / count),
		uint8((sumG + half) / count),
		uint8((sumB + half) / count),
		uint8((sumA + half) / count)
}
