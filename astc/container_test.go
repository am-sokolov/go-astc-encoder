package astc_test

import (
	"bytes"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestHeaderRoundTrip(t *testing.T) {
	h := astc.Header{
		BlockX: 4,
		BlockY: 4,
		BlockZ: 1,
		SizeX:  1024,
		SizeY:  768,
		SizeZ:  1,
	}

	enc, err := astc.MarshalHeader(h)
	if err != nil {
		t.Fatalf("MarshalHeader: %v", err)
	}
	got, err := astc.ParseHeader(enc[:])
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	if got != h {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, h)
	}

	// Sanity check magic.
	if !bytes.Equal(enc[0:4], []byte{0x13, 0xAB, 0xA1, 0x5C}) {
		t.Fatalf("unexpected magic: %x", enc[0:4])
	}
}
