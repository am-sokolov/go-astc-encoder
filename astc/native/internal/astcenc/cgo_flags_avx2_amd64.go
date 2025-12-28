//go:build astcenc_native && cgo && astcenc_avx2 && amd64

package astcenc

/*
#cgo CXXFLAGS: -mavx2 -mfma -msse4.1 -msse4.2 -mpopcnt -mf16c
*/
import "C"
