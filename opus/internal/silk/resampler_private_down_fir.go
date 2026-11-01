// Port of silk/resampler_private_down_FIR.c.

package silk

// resamplerPrivateDownFIRInterpol is
// silk_resampler_private_down_FIR_INTERPOL; it returns out advanced past the
// samples it wrote.
func resamplerPrivateDownFIRInterpol(out []int16, buf []int32, FIRCoefs []int16, FIROrder, FIRFracs, maxIndexQ16, indexIncrementQ16 int32) []int16 {
	var indexQ16, resQ6, interpolInd int32

	switch FIROrder {
	case resamplerDownOrderFIR0:
		for indexQ16 = 0; indexQ16 < maxIndexQ16; indexQ16 += indexIncrementQ16 {
			// Integer part gives pointer to buffered input
			bufPtr := buf[indexQ16>>16:]

			// Fractional part gives interpolation coefficients
			interpolInd = smulwb(indexQ16&0xFFFF, FIRFracs)

			// Inner product
			interpolPtr := FIRCoefs[resamplerDownOrderFIR0/2*interpolInd:]
			resQ6 = smulwb(bufPtr[0], int32(interpolPtr[0]))
			resQ6 = smlawb(resQ6, bufPtr[1], int32(interpolPtr[1]))
			resQ6 = smlawb(resQ6, bufPtr[2], int32(interpolPtr[2]))
			resQ6 = smlawb(resQ6, bufPtr[3], int32(interpolPtr[3]))
			resQ6 = smlawb(resQ6, bufPtr[4], int32(interpolPtr[4]))
			resQ6 = smlawb(resQ6, bufPtr[5], int32(interpolPtr[5]))
			resQ6 = smlawb(resQ6, bufPtr[6], int32(interpolPtr[6]))
			resQ6 = smlawb(resQ6, bufPtr[7], int32(interpolPtr[7]))
			resQ6 = smlawb(resQ6, bufPtr[8], int32(interpolPtr[8]))
			interpolPtr = FIRCoefs[resamplerDownOrderFIR0/2*(FIRFracs-1-interpolInd):]
			resQ6 = smlawb(resQ6, bufPtr[17], int32(interpolPtr[0]))
			resQ6 = smlawb(resQ6, bufPtr[16], int32(interpolPtr[1]))
			resQ6 = smlawb(resQ6, bufPtr[15], int32(interpolPtr[2]))
			resQ6 = smlawb(resQ6, bufPtr[14], int32(interpolPtr[3]))
			resQ6 = smlawb(resQ6, bufPtr[13], int32(interpolPtr[4]))
			resQ6 = smlawb(resQ6, bufPtr[12], int32(interpolPtr[5]))
			resQ6 = smlawb(resQ6, bufPtr[11], int32(interpolPtr[6]))
			resQ6 = smlawb(resQ6, bufPtr[10], int32(interpolPtr[7]))
			resQ6 = smlawb(resQ6, bufPtr[9], int32(interpolPtr[8]))

			// Scale down, saturate and store in output array
			out[0] = int16(sat16(rshiftRound(resQ6, 6)))
			out = out[1:]
		}
	case resamplerDownOrderFIR1:
		for indexQ16 = 0; indexQ16 < maxIndexQ16; indexQ16 += indexIncrementQ16 {
			// Integer part gives pointer to buffered input
			bufPtr := buf[indexQ16>>16:]

			// Inner product
			resQ6 = smulwb(bufPtr[0]+bufPtr[23], int32(FIRCoefs[0]))
			resQ6 = smlawb(resQ6, bufPtr[1]+bufPtr[22], int32(FIRCoefs[1]))
			resQ6 = smlawb(resQ6, bufPtr[2]+bufPtr[21], int32(FIRCoefs[2]))
			resQ6 = smlawb(resQ6, bufPtr[3]+bufPtr[20], int32(FIRCoefs[3]))
			resQ6 = smlawb(resQ6, bufPtr[4]+bufPtr[19], int32(FIRCoefs[4]))
			resQ6 = smlawb(resQ6, bufPtr[5]+bufPtr[18], int32(FIRCoefs[5]))
			resQ6 = smlawb(resQ6, bufPtr[6]+bufPtr[17], int32(FIRCoefs[6]))
			resQ6 = smlawb(resQ6, bufPtr[7]+bufPtr[16], int32(FIRCoefs[7]))
			resQ6 = smlawb(resQ6, bufPtr[8]+bufPtr[15], int32(FIRCoefs[8]))
			resQ6 = smlawb(resQ6, bufPtr[9]+bufPtr[14], int32(FIRCoefs[9]))
			resQ6 = smlawb(resQ6, bufPtr[10]+bufPtr[13], int32(FIRCoefs[10]))
			resQ6 = smlawb(resQ6, bufPtr[11]+bufPtr[12], int32(FIRCoefs[11]))

			// Scale down, saturate and store in output array
			out[0] = int16(sat16(rshiftRound(resQ6, 6)))
			out = out[1:]
		}
	case resamplerDownOrderFIR2:
		for indexQ16 = 0; indexQ16 < maxIndexQ16; indexQ16 += indexIncrementQ16 {
			// Integer part gives pointer to buffered input
			bufPtr := buf[indexQ16>>16:]

			// Inner product
			resQ6 = smulwb(bufPtr[0]+bufPtr[35], int32(FIRCoefs[0]))
			resQ6 = smlawb(resQ6, bufPtr[1]+bufPtr[34], int32(FIRCoefs[1]))
			resQ6 = smlawb(resQ6, bufPtr[2]+bufPtr[33], int32(FIRCoefs[2]))
			resQ6 = smlawb(resQ6, bufPtr[3]+bufPtr[32], int32(FIRCoefs[3]))
			resQ6 = smlawb(resQ6, bufPtr[4]+bufPtr[31], int32(FIRCoefs[4]))
			resQ6 = smlawb(resQ6, bufPtr[5]+bufPtr[30], int32(FIRCoefs[5]))
			resQ6 = smlawb(resQ6, bufPtr[6]+bufPtr[29], int32(FIRCoefs[6]))
			resQ6 = smlawb(resQ6, bufPtr[7]+bufPtr[28], int32(FIRCoefs[7]))
			resQ6 = smlawb(resQ6, bufPtr[8]+bufPtr[27], int32(FIRCoefs[8]))
			resQ6 = smlawb(resQ6, bufPtr[9]+bufPtr[26], int32(FIRCoefs[9]))
			resQ6 = smlawb(resQ6, bufPtr[10]+bufPtr[25], int32(FIRCoefs[10]))
			resQ6 = smlawb(resQ6, bufPtr[11]+bufPtr[24], int32(FIRCoefs[11]))
			resQ6 = smlawb(resQ6, bufPtr[12]+bufPtr[23], int32(FIRCoefs[12]))
			resQ6 = smlawb(resQ6, bufPtr[13]+bufPtr[22], int32(FIRCoefs[13]))
			resQ6 = smlawb(resQ6, bufPtr[14]+bufPtr[21], int32(FIRCoefs[14]))
			resQ6 = smlawb(resQ6, bufPtr[15]+bufPtr[20], int32(FIRCoefs[15]))
			resQ6 = smlawb(resQ6, bufPtr[16]+bufPtr[19], int32(FIRCoefs[16]))
			resQ6 = smlawb(resQ6, bufPtr[17]+bufPtr[18], int32(FIRCoefs[17]))

			// Scale down, saturate and store in output array
			out[0] = int16(sat16(rshiftRound(resQ6, 6)))
			out = out[1:]
		}
	}
	return out
}

// resamplerPrivateDownFIR is silk_resampler_private_down_FIR: resamples with a
// 2nd order AR filter followed by FIR interpolation.
func resamplerPrivateDownFIR(S *resamplerStateStruct, out, in []int16, inLen int32) {
	var nSamplesIn, maxIndexQ16, indexIncrementQ16 int32

	buf := S.buf32[:S.batchSize+S.firOrder]

	// Copy buffered samples to start of buffer
	copy(buf, S.sFIRI32[:S.firOrder])

	FIRCoefs := S.coefs[2:]

	// Iterate over blocks of frameSizeIn input samples
	indexIncrementQ16 = S.invRatioQ16
	for {
		nSamplesIn = min(inLen, S.batchSize)

		// Second-order AR filter (output in Q8)
		resamplerPrivateAR2(S.sIIR[:], buf[S.firOrder:], in, S.coefs, nSamplesIn)

		maxIndexQ16 = nSamplesIn << 16

		// Interpolate filtered signal
		out = resamplerPrivateDownFIRInterpol(out, buf, FIRCoefs, S.firOrder,
			S.firFracs, maxIndexQ16, indexIncrementQ16)

		in = in[nSamplesIn:]
		inLen -= nSamplesIn

		if inLen > 1 {
			// More iterations to do; copy last part of filtered signal to beginning of buffer
			copy(buf[:S.firOrder], buf[nSamplesIn:])
		} else {
			break
		}
	}

	// Copy last part of filtered signal to the state for the next call
	copy(S.sFIRI32[:S.firOrder], buf[nSamplesIn:])
}
