// Package native provides an optional CGO-backed wrapper around the upstream C++ astcenc
// implementation (including its native SIMD vecmathlib).
//
// By default this package builds in "disabled" mode (pure Go, no CGO), returning an error from all
// operations. To enable it, build with:
//
//	-tags astcenc_native
//
// and ensure CGO is enabled (e.g. `CGO_ENABLED=1`).
//
// Optional build tags for x86-64 performance tuning:
//   - `astcenc_avx2`: compile the native library with AVX2/FMA/SSE4.1 enabled (portable only to AVX2 CPUs).
//   - `astcenc_nativearch`: compile with `-march=native` (not portable).
package native
