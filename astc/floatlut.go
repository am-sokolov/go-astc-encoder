package astc

// Precomputed conversion tables for float32 output decoding.
//
// These are used heavily by DecodeRGBAF32* hot paths. Computing these on the fly is significantly
// slower than a table lookup.

var (
	unorm16ToFloat32Table [1 << 16]float32
	lnsToFloat32Table     [1 << 16]float32
)

func init() {
	for i := 0; i < (1 << 16); i++ {
		u := uint16(i)
		unorm16ToFloat32Table[u] = halfToFloat32(unorm16ToSF16(u))
		lnsToFloat32Table[u] = halfToFloat32(lnsToSF16(u))
	}
}
