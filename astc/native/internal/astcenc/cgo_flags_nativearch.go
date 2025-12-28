//go:build astcenc_native && cgo && astcenc_nativearch

package astcenc

/*
#cgo CXXFLAGS: -march=native
*/
import "C"
