package astc_test

import (
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestEncodeDecodeConstBlockRGBA8(t *testing.T) {
	const (
		r = 10
		g = 20
		b = 30
		a = 40
	)

	blk := astc.EncodeConstBlockRGBA8(r, g, b, a)

	gotR, gotG, gotB, gotA, err := astc.DecodeConstBlockRGBA8(blk[:])
	if err != nil {
		t.Fatalf("DecodeConstBlockRGBA8: %v", err)
	}

	if gotR != r || gotG != g || gotB != b || gotA != a {
		t.Fatalf("decoded mismatch: got (%d,%d,%d,%d) want (%d,%d,%d,%d)", gotR, gotG, gotB, gotA, r, g, b, a)
	}
}
