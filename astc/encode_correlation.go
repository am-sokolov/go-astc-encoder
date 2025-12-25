package astc

import "math"

// alphaRGBAbsCorrelation returns |corr(alpha, luma)| for a block's RGBA8 texels, where
// luma is the simple sum r+g+b.
func alphaRGBAbsCorrelation(texels []byte) float64 {
	n := len(texels) / 4
	if n <= 1 {
		return 1
	}

	var sumL, sumA int64
	var sumLL, sumAA int64
	var sumLA int64

	for i := 0; i < n; i++ {
		off := i * 4
		l := int64(texels[off+0]) + int64(texels[off+1]) + int64(texels[off+2])
		a := int64(texels[off+3])
		sumL += l
		sumA += a
		sumLL += l * l
		sumAA += a * a
		sumLA += l * a
	}

	nn := int64(n)
	varL := sumLL*nn - sumL*sumL
	varA := sumAA*nn - sumA*sumA
	if varL <= 0 || varA <= 0 {
		// No variance -> a single weight plane is sufficient.
		return 1
	}

	cov := sumLA*nn - sumL*sumA
	corr := float64(cov) / math.Sqrt(float64(varL)*float64(varA))
	if corr < 0 {
		corr = -corr
	}
	if corr > 1 {
		corr = 1
	}
	return corr
}
