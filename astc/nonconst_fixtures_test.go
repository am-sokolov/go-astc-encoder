package astc_test

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/draw"
	"image/png"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
)

func mustDecodeBase64(t *testing.T, b64 string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	return data
}

func mustDecodePNGToRGBA8(t *testing.T, pngData []byte) (pix []byte, width, height int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}

	b := img.Bounds()
	width, height = b.Dx(), b.Dy()
	// PNG pixels are stored as straight alpha. Use NRGBA to avoid premultiplication during conversion.
	rgba := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba.Pix, width, height
}

func TestDecodeRGBA8_NonConst_LDR_4x4_Medium(t *testing.T) {
	astcData := mustDecodeBase64(t, fixtureLDRComplex4x4MediumASTCBase64)
	wantPNG := mustDecodeBase64(t, fixtureLDRComplex4x4MediumPNGBase64)
	wantPix, wantW, wantH := mustDecodePNGToRGBA8(t, wantPNG)

	gotPix, gotW, gotH, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
	if err != nil {
		t.Fatalf("DecodeRGBA8WithProfile: %v", err)
	}
	if gotW != wantW || gotH != wantH {
		t.Fatalf("unexpected dimensions: got %dx%d want %dx%d", gotW, gotH, wantW, wantH)
	}
	if !bytes.Equal(gotPix, wantPix) {
		i := firstMismatchByte(gotPix, wantPix)
		x, y := (i/4)%wantW, (i/4)/wantW
		off := (y*wantW + x) * 4
		t.Fatalf("decoded pixels mismatch at (%d,%d): got (%d,%d,%d,%d) want (%d,%d,%d,%d)",
			x, y,
			gotPix[off+0], gotPix[off+1], gotPix[off+2], gotPix[off+3],
			wantPix[off+0], wantPix[off+1], wantPix[off+2], wantPix[off+3])
	}
}

func TestDecodeRGBA8_NonConst_LDR_6x6_Medium(t *testing.T) {
	astcData := mustDecodeBase64(t, fixtureLDRComplex6x6MediumASTCBase64)
	wantPNG := mustDecodeBase64(t, fixtureLDRComplex6x6MediumPNGBase64)
	wantPix, wantW, wantH := mustDecodePNGToRGBA8(t, wantPNG)

	gotPix, gotW, gotH, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDR)
	if err != nil {
		t.Fatalf("DecodeRGBA8WithProfile: %v", err)
	}
	if gotW != wantW || gotH != wantH {
		t.Fatalf("unexpected dimensions: got %dx%d want %dx%d", gotW, gotH, wantW, wantH)
	}
	if !bytes.Equal(gotPix, wantPix) {
		i := firstMismatchByte(gotPix, wantPix)
		x, y := (i/4)%wantW, (i/4)/wantW
		off := (y*wantW + x) * 4
		t.Fatalf("decoded pixels mismatch at (%d,%d): got (%d,%d,%d,%d) want (%d,%d,%d,%d)",
			x, y,
			gotPix[off+0], gotPix[off+1], gotPix[off+2], gotPix[off+3],
			wantPix[off+0], wantPix[off+1], wantPix[off+2], wantPix[off+3])
	}
}

func TestDecodeRGBA8_NonConst_LDR_SRGB_4x4_Medium(t *testing.T) {
	astcData := mustDecodeBase64(t, fixtureLDRComplexSRGB4x4MediumASTCBase64)
	wantPNG := mustDecodeBase64(t, fixtureLDRComplexSRGB4x4MediumPNGBase64)
	wantPix, wantW, wantH := mustDecodePNGToRGBA8(t, wantPNG)

	gotPix, gotW, gotH, err := astc.DecodeRGBA8WithProfile(astcData, astc.ProfileLDRSRGB)
	if err != nil {
		t.Fatalf("DecodeRGBA8WithProfile: %v", err)
	}
	if gotW != wantW || gotH != wantH {
		t.Fatalf("unexpected dimensions: got %dx%d want %dx%d", gotW, gotH, wantW, wantH)
	}
	if !bytes.Equal(gotPix, wantPix) {
		i := firstMismatchByte(gotPix, wantPix)
		x, y := (i/4)%wantW, (i/4)/wantW
		off := (y*wantW + x) * 4
		t.Fatalf("decoded pixels mismatch at (%d,%d): got (%d,%d,%d,%d) want (%d,%d,%d,%d)",
			x, y,
			gotPix[off+0], gotPix[off+1], gotPix[off+2], gotPix[off+3],
			wantPix[off+0], wantPix[off+1], wantPix[off+2], wantPix[off+3])
	}
}

func firstMismatchByte(a, b []byte) int {
	if len(a) < len(b) {
		b = b[:len(a)]
	}
	for i := range b {
		if a[i] != b[i] {
			return i
		}
	}
	return 0
}

const fixtureLDRComplex4x4MediumASTCBase64 = "E6uhXAQEAQgAAAgAAAEAAEOC4ZYIc61cRJMGkPvibfxDglXhgXNtpoe401e2oMBhUoKt8pEHrYGgYRECn5B1UkOCmwg6YG0AAzfs5V6xGo0="
const fixtureLDRComplex4x4MediumPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAYAAADED76LAAABEklEQVR4XmMsnZZ12c8vguHEiYMM165dZGBk/Mfw588vhps3zzKoqxszML/6dSvr27dvDEpKagxXr15guHTpAMPPn98ZNDRMGbS1LRiYWUR/Zt25c53B0dGDwdnZh0FERJ7h+fMHDCoqegyammYMTCoqGgyCgsIM379/Z+Dk5GQwM3NnkJRUZLhw4RDD168fGZh0tQyYNdXUmbm5JJi/fmFj/svIymBhHcrAw63E8PLVBwamd+9eMNy/f4nhwvm9DGLi8gx8/IIM79++ZpCVU2bQ0DZkYPn54yuDlKQyw7kzuxneAxWbWAUxgMCaFbMZxCVkGBhtImWvSUopM4AU3r9/meHjR86/GlqGDKeO7wObBgDt7V4rzrsREQAAAABJRU5ErkJggg=="

const fixtureLDRComplex6x6MediumASTCBase64 = "E6uhXAYGAQgAAAgAAAEAAPOAL4RMaC0f1E3WDxA1LN9dqAb0URZztvvirHOgpQMtfoCp2S0CrQVTKIMxmELhVGHJh/W1J7u0VdemL33f930="
const fixtureLDRComplex6x6MediumPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAYAAADED76LAAABCUlEQVR4XmNM70i4HhqWzXD50nGGCxcOM3z5+olBRFiS4c6la98FxfkYmL+xPcz58f0bAy+vAMO3b18YTp/aw/AVqEheReaPlW0QEzO7+I+c9+9fMejoWDCYmjoxCAtLMLx48YhB00j5j6KiESuLjIwyw/cf3xiYmVkYpKQUGdjYOBhAGk5sPc8gK2b7l0Vb24zh5cvHDCDw6dM7MA0S+/TpKcPvX7/+M71585zhxo1zYF2srOwMfHxCDCAxGVVxBiVNZTZmQzu1bMb/jAx3bl1ieHj/IgMn+z8GLg4WhjkN6xhVdBX+M1qEKl+TkpJh+PTpI9BxTxl+/PjBYGJiybB980EGNk7OvwD0UGXc1+hLsQAAAABJRU5ErkJggg=="

const fixtureLDRComplexSRGB4x4MediumASTCBase64 = "E6uhXAQEAQgAAAgAAAEAAEOC4ZYIc61cRJMGkPvibfxDglXhgXNtpoe401e2oMBhQ4IbzZ9oLSgSZZRCJ2hvKkOCmwg6YG0AAzfs5V6xGo0="
const fixtureLDRComplexSRGB4x4MediumPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAYAAADED76LAAABEUlEQVR4XmMsnZZ12dc3guHkyYMM165dZGBk/Mfw588vhps3zzGoqxsxML/6dTvr27evDEpKagxXr15guHTpAMPPnz8YNDRMGLS1LRiYWUR/ZN29e53BwcGDwdnZm0FERIHh+fP7DCoqegyamqYMTCoqmgwCAsIM379/Z+Dg4GIwM3NnkJRUZLhw4RDD16+fGJh0tfQYVJXkGXi4pRh+fOdm+MvIymBhHQrkKzG8fPWBgenVq4cMT5/cYrhy+RCDsIgMAx+/IMP7t68ZZOWUGTS0DRhYfv/+ySAmLs9w6uRmhjdvnjAYmfkzgMCaFbMZxCVkGBhto2UuS0urM3z58g5s0tu3bAya2oYMJ4/tA5sGANI9XP1ksX97AAAAAElFTkSuQmCC"
