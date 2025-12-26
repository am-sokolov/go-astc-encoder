package astc

func decodeBlockToRGBAF32(profile Profile, ctx *decodeContext, block []byte, out []float32) {
	texelCount := ctx.texelCount
	dst := out[:texelCount*4]

	scb := physicalToSymbolicWithCtx(block, ctx)
	switch scb.blockType {
	case symBlockError:
		fillErrorRGBAF32(dst)
		return
	case symBlockConstU16:
		r := unorm16ToFloat32Table[scb.constantColor[0]]
		g := unorm16ToFloat32Table[scb.constantColor[1]]
		b := unorm16ToFloat32Table[scb.constantColor[2]]
		a := unorm16ToFloat32Table[scb.constantColor[3]]
		fillConstRGBAF32(dst, r, g, b, a)
		return
	case symBlockConstF16:
		// FP16 constant blocks are only valid in HDR profiles.
		if profile == ProfileLDR || profile == ProfileLDRSRGB {
			fillErrorRGBAF32(dst)
			return
		}
		r := halfToFloat32(scb.constantColor[0])
		g := halfToFloat32(scb.constantColor[1])
		b := halfToFloat32(scb.constantColor[2])
		a := halfToFloat32(scb.constantColor[3])
		fillConstRGBAF32(dst, r, g, b, a)
		return
	}

	bmi := ctx.blockModes[scb.blockMode]
	if !bmi.ok {
		fillErrorRGBAF32(dst)
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

	var rgbLNS [blockMaxPartitions]bool
	var alphaLNS [blockMaxPartitions]bool
	for p := 0; p < partitionCount; p++ {
		rgb, a, e0, e1 := unpackColorEndpoints(profile, scb.colorFormats[p], scb.colorValues[p][:])
		rgbLNS[p] = rgb
		alphaLNS[p] = a

		ep0r[p] = e0[0]
		ep0g[p] = e0[1]
		ep0b[p] = e0[2]
		ep0a[p] = e0[3]

		epdr[p] = e1[0] - e0[0]
		epdg[p] = e1[1] - e0[1]
		epdb[p] = e1[2] - e0[2]
		epda[p] = e1[3] - e0[3]
	}

	var rgbTableByPart [blockMaxPartitions]*[1 << 16]float32
	var alphaTableByPart [blockMaxPartitions]*[1 << 16]float32
	for p := 0; p < partitionCount; p++ {
		if rgbLNS[p] {
			rgbTableByPart[p] = &lnsToFloat32Table
		} else {
			rgbTableByPart[p] = &unorm16ToFloat32Table
		}
		if alphaLNS[p] {
			alphaTableByPart[p] = &lnsToFloat32Table
		} else {
			alphaTableByPart[p] = &unorm16ToFloat32Table
		}
	}

	// Decode texels.
	if partitionCount == 1 {
		rgbTable := rgbTableByPart[0]
		alphaTable := alphaTableByPart[0]

		e0r0 := ep0r[0]
		e0g0 := ep0g[0]
		e0b0 := ep0b[0]
		e0a0 := ep0a[0]

		dr0 := epdr[0]
		dg0 := epdg[0]
		db0 := epdb[0]
		da0 := epda[0]

		if bmi.noDecimation {
			wTex1 := scb.weights[:texelCount]

			if !bmi.isDualPlane {
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w := int(wTex1[tix])

					rv := e0r0 + ((dr0*w + 32) >> 6)
					gv := e0g0 + ((dg0*w + 32) >> 6)
					bv := e0b0 + ((db0*w + 32) >> 6)
					av := e0a0 + ((da0*w + 32) >> 6)

					if (uint32(rv)|uint32(gv)|uint32(bv)|uint32(av))&^0xFFFF == 0 {
						dst[off+0] = rgbTable[uint16(rv)]
						dst[off+1] = rgbTable[uint16(gv)]
						dst[off+2] = rgbTable[uint16(bv)]
						dst[off+3] = alphaTable[uint16(av)]
					} else {
						if rv < 0 {
							rv = 0
						} else if rv > 0xFFFF {
							rv = 0xFFFF
						}
						if gv < 0 {
							gv = 0
						} else if gv > 0xFFFF {
							gv = 0xFFFF
						}
						if bv < 0 {
							bv = 0
						} else if bv > 0xFFFF {
							bv = 0xFFFF
						}
						if av < 0 {
							av = 0
						} else if av > 0xFFFF {
							av = 0xFFFF
						}

						dst[off+0] = rgbTable[uint16(rv)]
						dst[off+1] = rgbTable[uint16(gv)]
						dst[off+2] = rgbTable[uint16(bv)]
						dst[off+3] = alphaTable[uint16(av)]
					}

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

					rv := e0r0 + ((dr0*w2 + 32) >> 6)
					gv := e0g0 + ((dg0*w1 + 32) >> 6)
					bv := e0b0 + ((db0*w1 + 32) >> 6)
					av := e0a0 + ((da0*w1 + 32) >> 6)

					if rv < 0 {
						rv = 0
					} else if rv > 0xFFFF {
						rv = 0xFFFF
					}
					if gv < 0 {
						gv = 0
					} else if gv > 0xFFFF {
						gv = 0xFFFF
					}
					if bv < 0 {
						bv = 0
					} else if bv > 0xFFFF {
						bv = 0xFFFF
					}
					if av < 0 {
						av = 0
					} else if av > 0xFFFF {
						av = 0xFFFF
					}

					dst[off+0] = rgbTable[uint16(rv)]
					dst[off+1] = rgbTable[uint16(gv)]
					dst[off+2] = rgbTable[uint16(bv)]
					dst[off+3] = alphaTable[uint16(av)]

					off += 4
				}
			case 1:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])

					rv := e0r0 + ((dr0*w1 + 32) >> 6)
					gv := e0g0 + ((dg0*w2 + 32) >> 6)
					bv := e0b0 + ((db0*w1 + 32) >> 6)
					av := e0a0 + ((da0*w1 + 32) >> 6)

					if rv < 0 {
						rv = 0
					} else if rv > 0xFFFF {
						rv = 0xFFFF
					}
					if gv < 0 {
						gv = 0
					} else if gv > 0xFFFF {
						gv = 0xFFFF
					}
					if bv < 0 {
						bv = 0
					} else if bv > 0xFFFF {
						bv = 0xFFFF
					}
					if av < 0 {
						av = 0
					} else if av > 0xFFFF {
						av = 0xFFFF
					}

					dst[off+0] = rgbTable[uint16(rv)]
					dst[off+1] = rgbTable[uint16(gv)]
					dst[off+2] = rgbTable[uint16(bv)]
					dst[off+3] = alphaTable[uint16(av)]

					off += 4
				}
			case 2:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])

					rv := e0r0 + ((dr0*w1 + 32) >> 6)
					gv := e0g0 + ((dg0*w1 + 32) >> 6)
					bv := e0b0 + ((db0*w2 + 32) >> 6)
					av := e0a0 + ((da0*w1 + 32) >> 6)

					if rv < 0 {
						rv = 0
					} else if rv > 0xFFFF {
						rv = 0xFFFF
					}
					if gv < 0 {
						gv = 0
					} else if gv > 0xFFFF {
						gv = 0xFFFF
					}
					if bv < 0 {
						bv = 0
					} else if bv > 0xFFFF {
						bv = 0xFFFF
					}
					if av < 0 {
						av = 0
					} else if av > 0xFFFF {
						av = 0xFFFF
					}

					dst[off+0] = rgbTable[uint16(rv)]
					dst[off+1] = rgbTable[uint16(gv)]
					dst[off+2] = rgbTable[uint16(bv)]
					dst[off+3] = alphaTable[uint16(av)]

					off += 4
				}
			case 3:
				off := 0
				for tix := 0; tix < texelCount; tix++ {
					w1 := int(wTex1[tix])
					w2 := int(wTex2[tix])

					rv := e0r0 + ((dr0*w1 + 32) >> 6)
					gv := e0g0 + ((dg0*w1 + 32) >> 6)
					bv := e0b0 + ((db0*w1 + 32) >> 6)
					av := e0a0 + ((da0*w2 + 32) >> 6)

					if rv < 0 {
						rv = 0
					} else if rv > 0xFFFF {
						rv = 0xFFFF
					}
					if gv < 0 {
						gv = 0
					} else if gv > 0xFFFF {
						gv = 0xFFFF
					}
					if bv < 0 {
						bv = 0
					} else if bv > 0xFFFF {
						bv = 0xFFFF
					}
					if av < 0 {
						av = 0
					} else if av > 0xFFFF {
						av = 0xFFFF
					}

					dst[off+0] = rgbTable[uint16(rv)]
					dst[off+1] = rgbTable[uint16(gv)]
					dst[off+2] = rgbTable[uint16(bv)]
					dst[off+3] = alphaTable[uint16(av)]

					off += 4
				}
			default:
				fillErrorRGBAF32(dst)
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

				rv := e0r0 + ((dr0*w + 32) >> 6)
				gv := e0g0 + ((dg0*w + 32) >> 6)
				bv := e0b0 + ((db0*w + 32) >> 6)
				av := e0a0 + ((da0*w + 32) >> 6)

				if (uint32(rv)|uint32(gv)|uint32(bv)|uint32(av))&^0xFFFF == 0 {
					dst[off+0] = rgbTable[uint16(rv)]
					dst[off+1] = rgbTable[uint16(gv)]
					dst[off+2] = rgbTable[uint16(bv)]
					dst[off+3] = alphaTable[uint16(av)]
				} else {
					if rv < 0 {
						rv = 0
					} else if rv > 0xFFFF {
						rv = 0xFFFF
					}
					if gv < 0 {
						gv = 0
					} else if gv > 0xFFFF {
						gv = 0xFFFF
					}
					if bv < 0 {
						bv = 0
					} else if bv > 0xFFFF {
						bv = 0xFFFF
					}
					if av < 0 {
						av = 0
					} else if av > 0xFFFF {
						av = 0xFFFF
					}

					dst[off+0] = rgbTable[uint16(rv)]
					dst[off+1] = rgbTable[uint16(gv)]
					dst[off+2] = rgbTable[uint16(bv)]
					dst[off+3] = alphaTable[uint16(av)]
				}

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

				rv := e0r0 + ((dr0*w2 + 32) >> 6)
				gv := e0g0 + ((dg0*w1 + 32) >> 6)
				bv := e0b0 + ((db0*w1 + 32) >> 6)
				av := e0a0 + ((da0*w1 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

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

				rv := e0r0 + ((dr0*w1 + 32) >> 6)
				gv := e0g0 + ((dg0*w2 + 32) >> 6)
				bv := e0b0 + ((db0*w1 + 32) >> 6)
				av := e0a0 + ((da0*w1 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

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

				rv := e0r0 + ((dr0*w1 + 32) >> 6)
				gv := e0g0 + ((dg0*w1 + 32) >> 6)
				bv := e0b0 + ((db0*w2 + 32) >> 6)
				av := e0a0 + ((da0*w1 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

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

				rv := e0r0 + ((dr0*w1 + 32) >> 6)
				gv := e0g0 + ((dg0*w1 + 32) >> 6)
				bv := e0b0 + ((db0*w1 + 32) >> 6)
				av := e0a0 + ((da0*w2 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

				off += 4
			}
		default:
			fillErrorRGBAF32(dst)
		}

		return
	}

	// Partitioned block.
	pt := ctx.partitionTables[partitionCount]
	if pt == nil {
		fillErrorRGBAF32(dst)
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

				rv := ep0r[part] + ((epdr[part]*w + 32) >> 6)
				gv := ep0g[part] + ((epdg[part]*w + 32) >> 6)
				bv := ep0b[part] + ((epdb[part]*w + 32) >> 6)
				av := ep0a[part] + ((epda[part]*w + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				rgbTable := rgbTableByPart[part]
				alphaTable := alphaTableByPart[part]

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

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

				rv := ep0r[part] + ((epdr[part]*w2 + 32) >> 6)
				gv := ep0g[part] + ((epdg[part]*w1 + 32) >> 6)
				bv := ep0b[part] + ((epdb[part]*w1 + 32) >> 6)
				av := ep0a[part] + ((epda[part]*w1 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				rgbTable := rgbTableByPart[part]
				alphaTable := alphaTableByPart[part]

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

				off += 4
			}
		case 1:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])

				rv := ep0r[part] + ((epdr[part]*w1 + 32) >> 6)
				gv := ep0g[part] + ((epdg[part]*w2 + 32) >> 6)
				bv := ep0b[part] + ((epdb[part]*w1 + 32) >> 6)
				av := ep0a[part] + ((epda[part]*w1 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				rgbTable := rgbTableByPart[part]
				alphaTable := alphaTableByPart[part]

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

				off += 4
			}
		case 2:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])

				rv := ep0r[part] + ((epdr[part]*w1 + 32) >> 6)
				gv := ep0g[part] + ((epdg[part]*w1 + 32) >> 6)
				bv := ep0b[part] + ((epdb[part]*w2 + 32) >> 6)
				av := ep0a[part] + ((epda[part]*w1 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				rgbTable := rgbTableByPart[part]
				alphaTable := alphaTableByPart[part]

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

				off += 4
			}
		case 3:
			off := 0
			for tix := 0; tix < texelCount; tix++ {
				part := int(partByTexel[tix])
				w1 := int(wTex1[tix])
				w2 := int(wTex2[tix])

				rv := ep0r[part] + ((epdr[part]*w1 + 32) >> 6)
				gv := ep0g[part] + ((epdg[part]*w1 + 32) >> 6)
				bv := ep0b[part] + ((epdb[part]*w1 + 32) >> 6)
				av := ep0a[part] + ((epda[part]*w2 + 32) >> 6)

				if rv < 0 {
					rv = 0
				} else if rv > 0xFFFF {
					rv = 0xFFFF
				}
				if gv < 0 {
					gv = 0
				} else if gv > 0xFFFF {
					gv = 0xFFFF
				}
				if bv < 0 {
					bv = 0
				} else if bv > 0xFFFF {
					bv = 0xFFFF
				}
				if av < 0 {
					av = 0
				} else if av > 0xFFFF {
					av = 0xFFFF
				}

				rgbTable := rgbTableByPart[part]
				alphaTable := alphaTableByPart[part]

				dst[off+0] = rgbTable[uint16(rv)]
				dst[off+1] = rgbTable[uint16(gv)]
				dst[off+2] = rgbTable[uint16(bv)]
				dst[off+3] = alphaTable[uint16(av)]

				off += 4
			}
		default:
			fillErrorRGBAF32(dst)
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

			rv := ep0r[part] + ((epdr[part]*w + 32) >> 6)
			gv := ep0g[part] + ((epdg[part]*w + 32) >> 6)
			bv := ep0b[part] + ((epdb[part]*w + 32) >> 6)
			av := ep0a[part] + ((epda[part]*w + 32) >> 6)

			if rv < 0 {
				rv = 0
			} else if rv > 0xFFFF {
				rv = 0xFFFF
			}
			if gv < 0 {
				gv = 0
			} else if gv > 0xFFFF {
				gv = 0xFFFF
			}
			if bv < 0 {
				bv = 0
			} else if bv > 0xFFFF {
				bv = 0xFFFF
			}
			if av < 0 {
				av = 0
			} else if av > 0xFFFF {
				av = 0xFFFF
			}

			rgbTable := rgbTableByPart[part]
			alphaTable := alphaTableByPart[part]

			dst[off+0] = rgbTable[uint16(rv)]
			dst[off+1] = rgbTable[uint16(gv)]
			dst[off+2] = rgbTable[uint16(bv)]
			dst[off+3] = alphaTable[uint16(av)]

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

			rv := ep0r[part] + ((epdr[part]*w2 + 32) >> 6)
			gv := ep0g[part] + ((epdg[part]*w1 + 32) >> 6)
			bv := ep0b[part] + ((epdb[part]*w1 + 32) >> 6)
			av := ep0a[part] + ((epda[part]*w1 + 32) >> 6)

			if rv < 0 {
				rv = 0
			} else if rv > 0xFFFF {
				rv = 0xFFFF
			}
			if gv < 0 {
				gv = 0
			} else if gv > 0xFFFF {
				gv = 0xFFFF
			}
			if bv < 0 {
				bv = 0
			} else if bv > 0xFFFF {
				bv = 0xFFFF
			}
			if av < 0 {
				av = 0
			} else if av > 0xFFFF {
				av = 0xFFFF
			}

			rgbTable := rgbTableByPart[part]
			alphaTable := alphaTableByPart[part]

			dst[off+0] = rgbTable[uint16(rv)]
			dst[off+1] = rgbTable[uint16(gv)]
			dst[off+2] = rgbTable[uint16(bv)]
			dst[off+3] = alphaTable[uint16(av)]

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

			rv := ep0r[part] + ((epdr[part]*w1 + 32) >> 6)
			gv := ep0g[part] + ((epdg[part]*w2 + 32) >> 6)
			bv := ep0b[part] + ((epdb[part]*w1 + 32) >> 6)
			av := ep0a[part] + ((epda[part]*w1 + 32) >> 6)

			if rv < 0 {
				rv = 0
			} else if rv > 0xFFFF {
				rv = 0xFFFF
			}
			if gv < 0 {
				gv = 0
			} else if gv > 0xFFFF {
				gv = 0xFFFF
			}
			if bv < 0 {
				bv = 0
			} else if bv > 0xFFFF {
				bv = 0xFFFF
			}
			if av < 0 {
				av = 0
			} else if av > 0xFFFF {
				av = 0xFFFF
			}

			rgbTable := rgbTableByPart[part]
			alphaTable := alphaTableByPart[part]

			dst[off+0] = rgbTable[uint16(rv)]
			dst[off+1] = rgbTable[uint16(gv)]
			dst[off+2] = rgbTable[uint16(bv)]
			dst[off+3] = alphaTable[uint16(av)]

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

			rv := ep0r[part] + ((epdr[part]*w1 + 32) >> 6)
			gv := ep0g[part] + ((epdg[part]*w1 + 32) >> 6)
			bv := ep0b[part] + ((epdb[part]*w2 + 32) >> 6)
			av := ep0a[part] + ((epda[part]*w1 + 32) >> 6)

			if rv < 0 {
				rv = 0
			} else if rv > 0xFFFF {
				rv = 0xFFFF
			}
			if gv < 0 {
				gv = 0
			} else if gv > 0xFFFF {
				gv = 0xFFFF
			}
			if bv < 0 {
				bv = 0
			} else if bv > 0xFFFF {
				bv = 0xFFFF
			}
			if av < 0 {
				av = 0
			} else if av > 0xFFFF {
				av = 0xFFFF
			}

			rgbTable := rgbTableByPart[part]
			alphaTable := alphaTableByPart[part]

			dst[off+0] = rgbTable[uint16(rv)]
			dst[off+1] = rgbTable[uint16(gv)]
			dst[off+2] = rgbTable[uint16(bv)]
			dst[off+3] = alphaTable[uint16(av)]

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

			rv := ep0r[part] + ((epdr[part]*w1 + 32) >> 6)
			gv := ep0g[part] + ((epdg[part]*w1 + 32) >> 6)
			bv := ep0b[part] + ((epdb[part]*w1 + 32) >> 6)
			av := ep0a[part] + ((epda[part]*w2 + 32) >> 6)

			if rv < 0 {
				rv = 0
			} else if rv > 0xFFFF {
				rv = 0xFFFF
			}
			if gv < 0 {
				gv = 0
			} else if gv > 0xFFFF {
				gv = 0xFFFF
			}
			if bv < 0 {
				bv = 0
			} else if bv > 0xFFFF {
				bv = 0xFFFF
			}
			if av < 0 {
				av = 0
			} else if av > 0xFFFF {
				av = 0xFFFF
			}

			rgbTable := rgbTableByPart[part]
			alphaTable := alphaTableByPart[part]

			dst[off+0] = rgbTable[uint16(rv)]
			dst[off+1] = rgbTable[uint16(gv)]
			dst[off+2] = rgbTable[uint16(bv)]
			dst[off+3] = alphaTable[uint16(av)]

			off += 4
		}
	default:
		fillErrorRGBAF32(dst)
	}
}
