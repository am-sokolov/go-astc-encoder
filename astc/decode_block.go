package astc

import "errors"

var errUnsupportedProfileRGBA8 = errors.New("astc: DecodeRGBA8 only supports LDR profiles")

func decodeBlockToRGBA8(profile Profile, ctx *decodeContext, block []byte, out []byte) {
	texelCount := ctx.texelCount
	dst := out[:texelCount*4]

	scb := physicalToSymbolicWithCtx(block, ctx)
	switch scb.blockType {
	case symBlockError:
		fillErrorRGBA8(dst)
		return
	case symBlockConstU16:
		r := uint8(scb.constantColor[0] >> 8)
		g := uint8(scb.constantColor[1] >> 8)
		b := uint8(scb.constantColor[2] >> 8)
		a := uint8(scb.constantColor[3] >> 8)
		fillConstRGBA8(dst, r, g, b, a)
		return
	case symBlockConstF16:
		// FP16 constant blocks are only valid in HDR profiles.
		fillErrorRGBA8(dst)
		return
	}

	bmi := ctx.blockModes[scb.blockMode]
	if !bmi.ok {
		fillErrorRGBA8(dst)
		return
	}

	partitionCount := int(scb.partitionCount)

	// Pre-decode endpoints for each partition.
	var ep0r [blockMaxPartitions]int
	var ep0g [blockMaxPartitions]int
	var ep0b [blockMaxPartitions]int
	var ep0a [blockMaxPartitions]int

	var epdr [blockMaxPartitions]int
	var epdg [blockMaxPartitions]int
	var epdb [blockMaxPartitions]int
	var epda [blockMaxPartitions]int

	for p := 0; p < partitionCount; p++ {
		_, _, e0, e1 := unpackColorEndpoints(profile, scb.colorFormats[p], scb.colorValues[p][:])
		ep0r[p] = e0[0]
		ep0g[p] = e0[1]
		ep0b[p] = e0[2]
		ep0a[p] = e0[3]

		epdr[p] = e1[0] - e0[0]
		epdg[p] = e1[1] - e0[1]
		epdb[p] = e1[2] - e0[2]
		epda[p] = e1[3] - e0[3]
	}

	// Common case: 1 partition.
	if partitionCount == 1 {
		e0r0 := ep0r[0]
		e0g0 := ep0g[0]
		e0b0 := ep0b[0]
		e0a0 := ep0a[0]

		dr0 := epdr[0]
		dg0 := epdg[0]
		db0 := epdb[0]
		da0 := epda[0]

		// Fast path: no decimation (weight grid matches texel grid).
		if bmi.noDecimation {
			wTex1 := scb.weights[:texelCount]
			if !bmi.isDualPlane {
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w := int(wTex1[tix])
					dst[off+0] = uint8((e0r0 + ((dr0*w + 32) >> 6)) >> 8)
					dst[off+1] = uint8((e0g0 + ((dg0*w + 32) >> 6)) >> 8)
					dst[off+2] = uint8((e0b0 + ((db0*w + 32) >> 6)) >> 8)
					dst[off+3] = uint8((e0a0 + ((da0*w + 32) >> 6)) >> 8)
					off += 4
				}
				return
			}

			wTex2 := scb.weights[weightsPlane2Offset : weightsPlane2Offset+texelCount]
			plane2Component := int(scb.plane2Component)
			switch plane2Component {
			case 0:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])
					dst[off+0] = uint8((e0r0 + ((dr0*w2 + 32) >> 6)) >> 8)
					dst[off+1] = uint8((e0g0 + ((dg0*w1 + 32) >> 6)) >> 8)
					dst[off+2] = uint8((e0b0 + ((db0*w1 + 32) >> 6)) >> 8)
					dst[off+3] = uint8((e0a0 + ((da0*w1 + 32) >> 6)) >> 8)
					off += 4
				}
			case 1:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])
					dst[off+0] = uint8((e0r0 + ((dr0*w1 + 32) >> 6)) >> 8)
					dst[off+1] = uint8((e0g0 + ((dg0*w2 + 32) >> 6)) >> 8)
					dst[off+2] = uint8((e0b0 + ((db0*w1 + 32) >> 6)) >> 8)
					dst[off+3] = uint8((e0a0 + ((da0*w1 + 32) >> 6)) >> 8)
					off += 4
				}
			case 2:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])
					dst[off+0] = uint8((e0r0 + ((dr0*w1 + 32) >> 6)) >> 8)
					dst[off+1] = uint8((e0g0 + ((dg0*w1 + 32) >> 6)) >> 8)
					dst[off+2] = uint8((e0b0 + ((db0*w2 + 32) >> 6)) >> 8)
					dst[off+3] = uint8((e0a0 + ((da0*w1 + 32) >> 6)) >> 8)
					off += 4
				}
			case 3:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])
					dst[off+0] = uint8((e0r0 + ((dr0*w1 + 32) >> 6)) >> 8)
					dst[off+1] = uint8((e0g0 + ((dg0*w1 + 32) >> 6)) >> 8)
					dst[off+2] = uint8((e0b0 + ((db0*w1 + 32) >> 6)) >> 8)
					dst[off+3] = uint8((e0a0 + ((da0*w2 + 32) >> 6)) >> 8)
					off += 4
				}
			default:
				fillErrorRGBA8(dst)
			}
			return
		}

		dec := bmi.decimation
		wvals := scb.weights[:]
		if !bmi.isDualPlane {
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				e := dec[tix]
				sum := uint32(8)
				sum += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
				sum += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
				sum += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
				sum += uint32(wvals[e.idx[3]]) * uint32(e.w[3])
				w := int(sum >> 4)

				dst[off+0] = uint8((e0r0 + ((dr0*w + 32) >> 6)) >> 8)
				dst[off+1] = uint8((e0g0 + ((dg0*w + 32) >> 6)) >> 8)
				dst[off+2] = uint8((e0b0 + ((db0*w + 32) >> 6)) >> 8)
				dst[off+3] = uint8((e0a0 + ((da0*w + 32) >> 6)) >> 8)
				off += 4
			}
			return
		}

		plane2Component := int(scb.plane2Component)
		switch plane2Component {
		case 0:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				e := dec[tix]
				sum1 := uint32(8)
				sum2 := uint32(8)
				sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
				sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
				sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
				sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

				sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
				sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
				sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
				sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

				w1 := int(sum1 >> 4)
				w2 := int(sum2 >> 4)
				dst[off+0] = uint8((e0r0 + ((dr0*w2 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((e0g0 + ((dg0*w1 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((e0b0 + ((db0*w1 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((e0a0 + ((da0*w1 + 32) >> 6)) >> 8)
				off += 4
			}
		case 1:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				e := dec[tix]
				sum1 := uint32(8)
				sum2 := uint32(8)
				sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
				sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
				sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
				sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

				sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
				sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
				sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
				sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

				w1 := int(sum1 >> 4)
				w2 := int(sum2 >> 4)
				dst[off+0] = uint8((e0r0 + ((dr0*w1 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((e0g0 + ((dg0*w2 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((e0b0 + ((db0*w1 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((e0a0 + ((da0*w1 + 32) >> 6)) >> 8)
				off += 4
			}
		case 2:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				e := dec[tix]
				sum1 := uint32(8)
				sum2 := uint32(8)
				sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
				sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
				sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
				sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

				sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
				sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
				sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
				sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

				w1 := int(sum1 >> 4)
				w2 := int(sum2 >> 4)
				dst[off+0] = uint8((e0r0 + ((dr0*w1 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((e0g0 + ((dg0*w1 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((e0b0 + ((db0*w2 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((e0a0 + ((da0*w1 + 32) >> 6)) >> 8)
				off += 4
			}
		case 3:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				e := dec[tix]
				sum1 := uint32(8)
				sum2 := uint32(8)
				sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
				sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
				sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
				sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

				sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
				sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
				sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
				sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

				w1 := int(sum1 >> 4)
				w2 := int(sum2 >> 4)
				dst[off+0] = uint8((e0r0 + ((dr0*w1 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((e0g0 + ((dg0*w1 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((e0b0 + ((db0*w1 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((e0a0 + ((da0*w2 + 32) >> 6)) >> 8)
				off += 4
			}
		default:
			fillErrorRGBA8(dst)
		}
		return
	}

	// Partitioned block.
	pt := ctx.partitionTables[partitionCount]
	if pt == nil {
		fillErrorRGBA8(dst)
		return
	}
	pidx := int(scb.partitionIndex) & ((1 << partitionIndexBits) - 1)
	partByTexel := pt.data[pidx*texelCount : pidx*texelCount+texelCount]

	if bmi.noDecimation {
		wTex1 := scb.weights[:texelCount]
		if !bmi.isDualPlane {
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w := int(wTex1[tix])
				dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w + 32) >> 6)) >> 8)
				dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w + 32) >> 6)) >> 8)
				dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w + 32) >> 6)) >> 8)
				dst[off+3] = uint8((ep0a[part] + ((epda[part]*w + 32) >> 6)) >> 8)
				off += 4
			}
			return
		}

		wTex2 := scb.weights[weightsPlane2Offset : weightsPlane2Offset+texelCount]
		plane2Component := int(scb.plane2Component)
		switch plane2Component {
		case 0:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])
				dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w2 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w1 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w1 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((ep0a[part] + ((epda[part]*w1 + 32) >> 6)) >> 8)
				off += 4
			}
		case 1:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])
				dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w1 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w2 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w1 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((ep0a[part] + ((epda[part]*w1 + 32) >> 6)) >> 8)
				off += 4
			}
		case 2:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])
				dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w1 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w1 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w2 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((ep0a[part] + ((epda[part]*w1 + 32) >> 6)) >> 8)
				off += 4
			}
		case 3:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])
				dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w1 + 32) >> 6)) >> 8)
				dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w1 + 32) >> 6)) >> 8)
				dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w1 + 32) >> 6)) >> 8)
				dst[off+3] = uint8((ep0a[part] + ((epda[part]*w2 + 32) >> 6)) >> 8)
				off += 4
			}
		default:
			fillErrorRGBA8(dst)
		}
		return
	}

	dec := bmi.decimation
	wvals := scb.weights[:]
	if !bmi.isDualPlane {
		off := 0
		for tix := 0; tix < texelCount; tix++ {
			e := dec[tix]
			sum := uint32(8)
			sum += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
			sum += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
			sum += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
			sum += uint32(wvals[e.idx[3]]) * uint32(e.w[3])
			w := int(sum >> 4)

			part := int(partByTexel[tix])
			dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w + 32) >> 6)) >> 8)
			dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w + 32) >> 6)) >> 8)
			dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w + 32) >> 6)) >> 8)
			dst[off+3] = uint8((ep0a[part] + ((epda[part]*w + 32) >> 6)) >> 8)
			off += 4
		}
		return
	}

	plane2Component := int(scb.plane2Component)
	switch plane2Component {
	case 0:
		off := 0
		for tix := 0; tix < texelCount; tix++ {
			e := dec[tix]
			sum1 := uint32(8)
			sum2 := uint32(8)
			sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
			sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
			sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
			sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

			sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
			sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
			sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
			sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

			w1 := int(sum1 >> 4)
			w2 := int(sum2 >> 4)
			part := int(partByTexel[tix])
			dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w2 + 32) >> 6)) >> 8)
			dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w1 + 32) >> 6)) >> 8)
			dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w1 + 32) >> 6)) >> 8)
			dst[off+3] = uint8((ep0a[part] + ((epda[part]*w1 + 32) >> 6)) >> 8)
			off += 4
		}
	case 1:
		off := 0
		for tix := 0; tix < texelCount; tix++ {
			e := dec[tix]
			sum1 := uint32(8)
			sum2 := uint32(8)
			sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
			sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
			sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
			sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

			sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
			sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
			sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
			sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

			w1 := int(sum1 >> 4)
			w2 := int(sum2 >> 4)
			part := int(partByTexel[tix])
			dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w1 + 32) >> 6)) >> 8)
			dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w2 + 32) >> 6)) >> 8)
			dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w1 + 32) >> 6)) >> 8)
			dst[off+3] = uint8((ep0a[part] + ((epda[part]*w1 + 32) >> 6)) >> 8)
			off += 4
		}
	case 2:
		off := 0
		for tix := 0; tix < texelCount; tix++ {
			e := dec[tix]
			sum1 := uint32(8)
			sum2 := uint32(8)
			sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
			sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
			sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
			sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

			sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
			sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
			sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
			sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

			w1 := int(sum1 >> 4)
			w2 := int(sum2 >> 4)
			part := int(partByTexel[tix])
			dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w1 + 32) >> 6)) >> 8)
			dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w1 + 32) >> 6)) >> 8)
			dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w2 + 32) >> 6)) >> 8)
			dst[off+3] = uint8((ep0a[part] + ((epda[part]*w1 + 32) >> 6)) >> 8)
			off += 4
		}
	case 3:
		off := 0
		for tix := 0; tix < texelCount; tix++ {
			e := dec[tix]
			sum1 := uint32(8)
			sum2 := uint32(8)
			sum1 += uint32(wvals[e.idx[0]]) * uint32(e.w[0])
			sum1 += uint32(wvals[e.idx[1]]) * uint32(e.w[1])
			sum1 += uint32(wvals[e.idx[2]]) * uint32(e.w[2])
			sum1 += uint32(wvals[e.idx[3]]) * uint32(e.w[3])

			sum2 += uint32(wvals[int(e.idx[0])+weightsPlane2Offset]) * uint32(e.w[0])
			sum2 += uint32(wvals[int(e.idx[1])+weightsPlane2Offset]) * uint32(e.w[1])
			sum2 += uint32(wvals[int(e.idx[2])+weightsPlane2Offset]) * uint32(e.w[2])
			sum2 += uint32(wvals[int(e.idx[3])+weightsPlane2Offset]) * uint32(e.w[3])

			w1 := int(sum1 >> 4)
			w2 := int(sum2 >> 4)
			part := int(partByTexel[tix])
			dst[off+0] = uint8((ep0r[part] + ((epdr[part]*w1 + 32) >> 6)) >> 8)
			dst[off+1] = uint8((ep0g[part] + ((epdg[part]*w1 + 32) >> 6)) >> 8)
			dst[off+2] = uint8((ep0b[part] + ((epdb[part]*w1 + 32) >> 6)) >> 8)
			dst[off+3] = uint8((ep0a[part] + ((epda[part]*w2 + 32) >> 6)) >> 8)
			off += 4
		}
	default:
		fillErrorRGBA8(dst)
	}
}

func fillConstRGBA8(dst []byte, r, g, b, a uint8) {
	for i := 0; i < len(dst); i += 4 {
		dst[i+0] = r
		dst[i+1] = g
		dst[i+2] = b
		dst[i+3] = a
	}
}

func fillErrorRGBA8(dst []byte) {
	fillConstRGBA8(dst, 0xFF, 0x00, 0xFF, 0xFF)
}
