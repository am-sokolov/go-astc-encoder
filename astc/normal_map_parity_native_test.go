//go:build astcenc_native && cgo

package astc_test

import (
	"math"
	"math/rand"
	"testing"

	"github.com/arm-software/astc-encoder/astc"
	"github.com/arm-software/astc-encoder/astc/native"
)

func TestCompress_MapNormal_AngularError_CloseToNative(t *testing.T) {
	if !native.Enabled() {
		t.Fatalf("native.Enabled() = false; want true")
	}

	const (
		blockX  = 6
		blockY  = 6
		blockZ  = 1
		width   = 24
		height  = 24
		depth   = 1
		quality = float32(60)
	)

	// Generate a deterministic normal map in RG (snorm mapped to unorm8), with Z reconstructed.
	// The MAP_NORMAL mode expects the caller to use an input swizzle such as rrrg.
	rnd := rand.New(rand.NewSource(1))
	src := make([]byte, width*height*depth*4)
	ref := make([][3]float64, width*height*depth)
	for i := 0; i < width*height*depth; i++ {
		var x, y float64
		for {
			x = rnd.Float64()*2 - 1
			y = rnd.Float64()*2 - 1
			if x*x+y*y <= 1 {
				break
			}
		}
		r := snormToUNorm8(x)
		g := snormToUNorm8(y)

		src[i*4+0] = r
		src[i*4+1] = g
		src[i*4+2] = 0
		src[i*4+3] = 255

		ref[i] = normalXYZFromRG(r, g)
	}

	swzGo := astc.Swizzle{R: astc.SwzR, G: astc.SwzR, B: astc.SwzR, A: astc.SwzG} // rrrg
	swzNative := native.Swizzle{R: native.SwzR, G: native.SwzR, B: native.SwzR, A: native.SwzG}

	cfgGo, err := astc.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, quality, astc.FlagMapNormal)
	if err != nil {
		t.Fatalf("astc.ConfigInit: %v", err)
	}
	ctxGo, err := astc.ContextAlloc(&cfgGo, 1)
	if err != nil {
		t.Fatalf("astc.ContextAlloc: %v", err)
	}
	defer ctxGo.Close()

	cfgN, err := native.ConfigInit(astc.ProfileLDR, blockX, blockY, blockZ, quality, native.FlagMapNormal)
	if err != nil {
		t.Fatalf("native.ConfigInit: %v", err)
	}
	ctxN, err := native.ContextAlloc(&cfgN, 1)
	if err != nil {
		t.Fatalf("native.ContextAlloc: %v", err)
	}
	defer ctxN.Close()

	imgGo := &astc.Image{DimX: width, DimY: height, DimZ: depth, DataType: astc.TypeU8, DataU8: src}
	imgN := &native.Image{DimX: width, DimY: height, DimZ: depth, DataType: native.TypeU8, DataU8: src}

	blocksX := (width + blockX - 1) / blockX
	blocksY := (height + blockY - 1) / blockY
	outLen := blocksX * blocksY * astc.BlockBytes

	outGo := make([]byte, outLen)
	outN := make([]byte, outLen)

	if err := ctxGo.CompressImage(imgGo, swzGo, outGo, 0); err != nil {
		t.Fatalf("astc compress: %v", err)
	}
	if err := ctxN.CompressImage(imgN, swzNative, outN, 0); err != nil {
		t.Fatalf("native compress: %v", err)
	}

	// Sanity: both encoders should prefer L+A endpoint modes for MAP_NORMAL.
	{
		var b [astc.BlockBytes]byte
		copy(b[:], outGo[:astc.BlockBytes])
		infoGo, err := ctxGo.GetBlockInfo(b)
		if err != nil {
			t.Fatalf("astc.GetBlockInfo: %v", err)
		}
		for i := 0; i < int(infoGo.PartitionCount); i++ {
			if infoGo.ColorEndpointModes[i] != 4 {
				t.Fatalf("go endpoint mode[%d] = %d; want 4 (L+A)", i, infoGo.ColorEndpointModes[i])
			}
		}

		copy(b[:], outN[:astc.BlockBytes])
		infoN, err := ctxN.GetBlockInfo(b)
		if err != nil {
			t.Fatalf("native.GetBlockInfo: %v", err)
		}
		for i := 0; i < int(infoN.PartitionCount); i++ {
			if infoN.ColorEndpointModes[i] != 4 {
				t.Fatalf("native endpoint mode[%d] = %d; want 4 (L+A)", i, infoN.ColorEndpointModes[i])
			}
		}
	}

	hdr := astc.Header{
		BlockX: uint8(blockX),
		BlockY: uint8(blockY),
		BlockZ: uint8(blockZ),
		SizeX:  uint32(width),
		SizeY:  uint32(height),
		SizeZ:  uint32(depth),
	}

	decGo := make([]byte, width*height*depth*4)
	decN := make([]byte, width*height*depth*4)
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, hdr, outGo, decGo); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto(go): %v", err)
	}
	if err := astc.DecodeRGBA8VolumeFromParsedWithProfileInto(astc.ProfileLDR, hdr, outN, decN); err != nil {
		t.Fatalf("DecodeRGBA8VolumeFromParsedWithProfileInto(native): %v", err)
	}

	meanGo := meanNormalAngularError1MinusDot(ref, decGo)
	meanN := meanNormalAngularError1MinusDot(ref, decN)

	// Pure-Go doesn't need to be bit-exact, but should be in the same ballpark.
	// Use a small relative tolerance to avoid false positives from noisy inputs.
	if meanGo > meanN*1.25+1e-6 {
		t.Fatalf("mean angular error too high: go=%g native=%g", meanGo, meanN)
	}
}

func snormToUNorm8(v float64) byte {
	if v < -1 {
		v = -1
	} else if v > 1 {
		v = 1
	}
	u := int(math.Round((v*0.5 + 0.5) * 255.0))
	if u < 0 {
		u = 0
	} else if u > 255 {
		u = 255
	}
	return byte(u)
}

func normalXYZFromRG(r, g byte) [3]float64 {
	x := float64(r)*(2.0/255.0) - 1.0
	y := float64(g)*(2.0/255.0) - 1.0
	z2 := 1.0 - x*x - y*y
	if z2 < 0 {
		z2 = 0
	}
	z := math.Sqrt(z2)
	n := math.Sqrt(x*x + y*y + z*z)
	if n > 0 {
		x /= n
		y /= n
		z /= n
	}
	return [3]float64{x, y, z}
}

func meanNormalAngularError1MinusDot(ref [][3]float64, decodedRGBA8 []byte) float64 {
	if len(decodedRGBA8) != len(ref)*4 {
		panic("meanNormalAngularError1MinusDot: length mismatch")
	}
	var sum float64
	for i := range ref {
		r := decodedRGBA8[i*4+0]
		a := decodedRGBA8[i*4+3]

		dec := normalXYZFromRG(r, a)
		dot := ref[i][0]*dec[0] + ref[i][1]*dec[1] + ref[i][2]*dec[2]
		if dot < -1 {
			dot = -1
		} else if dot > 1 {
			dot = 1
		}
		sum += 1.0 - dot
	}
	return sum / float64(len(ref))
}
