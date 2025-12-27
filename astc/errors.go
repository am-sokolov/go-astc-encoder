package astc

import "errors"

// ErrorCode is a codec API error code equivalent to upstream astcenc_error.
type ErrorCode uint32

const (
	// Success is equivalent to ASTCENC_SUCCESS.
	Success ErrorCode = 0

	// ErrOutOfMem is equivalent to ASTCENC_ERR_OUT_OF_MEM.
	ErrOutOfMem ErrorCode = 1

	// ErrBadCPUFloat is equivalent to ASTCENC_ERR_BAD_CPU_FLOAT.
	//
	// This is not expected to be returned by the pure-Go implementation.
	ErrBadCPUFloat ErrorCode = 2

	// ErrBadParam is equivalent to ASTCENC_ERR_BAD_PARAM.
	ErrBadParam ErrorCode = 3

	// ErrBadBlockSize is equivalent to ASTCENC_ERR_BAD_BLOCK_SIZE.
	ErrBadBlockSize ErrorCode = 4

	// ErrBadProfile is equivalent to ASTCENC_ERR_BAD_PROFILE.
	ErrBadProfile ErrorCode = 5

	// ErrBadQuality is equivalent to ASTCENC_ERR_BAD_QUALITY.
	ErrBadQuality ErrorCode = 6

	// ErrBadSwizzle is equivalent to ASTCENC_ERR_BAD_SWIZZLE.
	ErrBadSwizzle ErrorCode = 7

	// ErrBadFlags is equivalent to ASTCENC_ERR_BAD_FLAGS.
	ErrBadFlags ErrorCode = 8

	// ErrBadContext is equivalent to ASTCENC_ERR_BAD_CONTEXT.
	ErrBadContext ErrorCode = 9

	// ErrNotImplemented is equivalent to ASTCENC_ERR_NOT_IMPLEMENTED.
	ErrNotImplemented ErrorCode = 10

	// ErrBadDecodeMode is equivalent to ASTCENC_ERR_BAD_DECODE_MODE.
	ErrBadDecodeMode ErrorCode = 11

	// ErrDTraceFailure is equivalent to ASTCENC_ERR_DTRACE_FAILURE (diagnostic builds only).
	ErrDTraceFailure ErrorCode = 12
)

// ErrorString returns the upstream error string name for a code (equivalent to astcenc_get_error_string).
//
// For unknown codes, it returns "" (mirrors upstream behavior returning nullptr).
func ErrorString(code ErrorCode) string {
	switch code {
	case Success:
		return "ASTCENC_SUCCESS"
	case ErrOutOfMem:
		return "ASTCENC_ERR_OUT_OF_MEM"
	case ErrBadCPUFloat:
		return "ASTCENC_ERR_BAD_CPU_FLOAT"
	case ErrBadParam:
		return "ASTCENC_ERR_BAD_PARAM"
	case ErrBadBlockSize:
		return "ASTCENC_ERR_BAD_BLOCK_SIZE"
	case ErrBadProfile:
		return "ASTCENC_ERR_BAD_PROFILE"
	case ErrBadQuality:
		return "ASTCENC_ERR_BAD_QUALITY"
	case ErrBadFlags:
		return "ASTCENC_ERR_BAD_FLAGS"
	case ErrBadSwizzle:
		return "ASTCENC_ERR_BAD_SWIZZLE"
	case ErrBadContext:
		return "ASTCENC_ERR_BAD_CONTEXT"
	case ErrNotImplemented:
		return "ASTCENC_ERR_NOT_IMPLEMENTED"
	case ErrBadDecodeMode:
		return "ASTCENC_ERR_BAD_DECODE_MODE"
	case ErrDTraceFailure:
		return "ASTCENC_ERR_DTRACE_FAILURE"
	default:
		return ""
	}
}

// Error is a typed error that carries an upstream-equivalent error code.
type Error struct {
	Code ErrorCode
	Msg  string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Msg != "" {
		return e.Msg
	}
	if s := ErrorString(e.Code); s != "" {
		return "astc: " + s
	}
	return "astc: error"
}

// ErrorCodeOf returns the astcenc-equivalent error code for err, or Success for nil.
//
// For non-*Error errors it returns ErrBadParam as a conservative fallback.
func ErrorCodeOf(err error) ErrorCode {
	if err == nil {
		return Success
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ErrBadParam
}

func newError(code ErrorCode, msg string) error {
	return &Error{Code: code, Msg: msg}
}
