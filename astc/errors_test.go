package astc_test

import (
	"errors"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func TestErrorString_MatchesUpstreamNames(t *testing.T) {
	cases := []struct {
		code astc.ErrorCode
		want string
	}{
		{astc.Success, "ASTCENC_SUCCESS"},
		{astc.ErrOutOfMem, "ASTCENC_ERR_OUT_OF_MEM"},
		{astc.ErrBadCPUFloat, "ASTCENC_ERR_BAD_CPU_FLOAT"},
		{astc.ErrBadParam, "ASTCENC_ERR_BAD_PARAM"},
		{astc.ErrBadBlockSize, "ASTCENC_ERR_BAD_BLOCK_SIZE"},
		{astc.ErrBadProfile, "ASTCENC_ERR_BAD_PROFILE"},
		{astc.ErrBadQuality, "ASTCENC_ERR_BAD_QUALITY"},
		{astc.ErrBadSwizzle, "ASTCENC_ERR_BAD_SWIZZLE"},
		{astc.ErrBadFlags, "ASTCENC_ERR_BAD_FLAGS"},
		{astc.ErrBadContext, "ASTCENC_ERR_BAD_CONTEXT"},
		{astc.ErrNotImplemented, "ASTCENC_ERR_NOT_IMPLEMENTED"},
		{astc.ErrBadDecodeMode, "ASTCENC_ERR_BAD_DECODE_MODE"},
		{astc.ErrDTraceFailure, "ASTCENC_ERR_DTRACE_FAILURE"},
	}

	for _, c := range cases {
		if got := astc.ErrorString(c.code); got != c.want {
			t.Fatalf("ErrorString(%d): got %q want %q", uint32(c.code), got, c.want)
		}
	}

	if got := astc.ErrorString(astc.ErrorCode(0xDEADBEEF)); got != "" {
		t.Fatalf("ErrorString(unknown): got %q want %q", got, "")
	}
}

func TestErrorCodeOf(t *testing.T) {
	if got := astc.ErrorCodeOf(nil); got != astc.Success {
		t.Fatalf("ErrorCodeOf(nil): got %v want %v", got, astc.Success)
	}

	if _, err := astc.ConfigInit(astc.ProfileLDR, 4, 4, 1, -1, 0); err == nil {
		t.Fatalf("ConfigInit: got nil error, want error")
	} else if got := astc.ErrorCodeOf(err); got != astc.ErrBadQuality {
		t.Fatalf("ErrorCodeOf(ConfigInit bad quality): got %v want %v", got, astc.ErrBadQuality)
	}

	if got := astc.ErrorCodeOf(errors.New("some other error")); got != astc.ErrBadParam {
		t.Fatalf("ErrorCodeOf(non-astc): got %v want %v", got, astc.ErrBadParam)
	}
}
