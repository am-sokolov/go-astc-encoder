//go:build !goexperiment.simd || !amd64

package astc

func avgBlockRGBA8(pix []byte, width, height, x0, y0, blockX, blockY int) (r, g, b, a uint8) {
	return avgBlockRGBA8Scalar(pix, width, height, x0, y0, blockX, blockY)
}
