// Port of silk/decode_core.c.

package silk

// decodeCore is silk_decode_core: the core decoder, which performs the
// inverse NSQ operation LTP + LPC, writing the decoded speech to xq.
func decodeCore(psDec *decoderState, psDecCtrl *decoderControl, xq []int16, pulses []int16) {
	var i, k, lag, startIdx, sLTPBufIdx, NLSFInterpolationFlag, signalType int32
	var AQ12, BQ14, pxq []int16
	var AQ12Tmp [maxLPCOrder]int16
	var LTPPredQ13, LPCPredQ10, GainQ10, invGainQ31, gainAdjQ16, randSeed, offsetQ10 int32
	var pexcQ14, presQ14 []int32

	sLTP := psDec.scratch.sLTP[:psDec.ltpMemLength]
	sLTPQ15 := psDec.scratch.sLTPQ15[:psDec.ltpMemLength+psDec.frameLength]
	resQ14 := psDec.scratch.resQ14[:psDec.subfrLength]
	sLPCQ14 := psDec.scratch.sLPCQ14[:psDec.subfrLength+maxLPCOrder]

	offsetQ10 = int32(quantizationOffsetsQ10[psDec.indices.signalType>>1][psDec.indices.quantOffsetType])

	if psDec.indices.nlsfInterpCoefQ2 < 1<<2 {
		NLSFInterpolationFlag = 1
	} else {
		NLSFInterpolationFlag = 0
	}

	// Decode excitation
	randSeed = int32(psDec.indices.seed)
	for i = 0; i < psDec.frameLength; i++ {
		randSeed = silkRAND(randSeed)
		psDec.excQ14[i] = int32(pulses[i]) << 14
		if psDec.excQ14[i] > 0 {
			psDec.excQ14[i] -= quantLevelAdjustQ10 << 4
		} else if psDec.excQ14[i] < 0 {
			psDec.excQ14[i] += quantLevelAdjustQ10 << 4
		}
		psDec.excQ14[i] += offsetQ10 << 4
		if randSeed < 0 {
			psDec.excQ14[i] = -psDec.excQ14[i]
		}

		randSeed = add32Ovflw(randSeed, int32(pulses[i]))
	}

	// Copy LPC state
	copy(sLPCQ14[:maxLPCOrder], psDec.sLPCQ14Buf[:])

	pexcQ14 = psDec.excQ14[:]
	pxq = xq
	sLTPBufIdx = psDec.ltpMemLength
	// Loop over subframes
	for k = 0; k < psDec.nbSubfr; k++ {
		presQ14 = resQ14
		AQ12 = psDecCtrl.predCoefQ12[k>>1][:]

		// Preload LPC coefficients to array on stack. Gives small performance gain
		copy(AQ12Tmp[:psDec.lpcOrder], AQ12[:psDec.lpcOrder])
		BQ14 = psDecCtrl.ltpCoefQ14[k*ltpOrder:]
		signalType = int32(psDec.indices.signalType)

		GainQ10 = psDecCtrl.gainsQ16[k] >> 6
		invGainQ31 = inverse32VarQ(psDecCtrl.gainsQ16[k], 47)

		// Calculate gain adjustment factor
		if psDecCtrl.gainsQ16[k] != psDec.prevGainQ16 {
			gainAdjQ16 = div32VarQ(psDec.prevGainQ16, psDecCtrl.gainsQ16[k], 16)

			// Scale short term state
			for i = 0; i < maxLPCOrder; i++ {
				sLPCQ14[i] = smulww(gainAdjQ16, sLPCQ14[i])
			}
		} else {
			gainAdjQ16 = 1 << 16
		}

		// Save inv_gain
		psDec.prevGainQ16 = psDecCtrl.gainsQ16[k]

		// Avoid abrupt transition from voiced PLC to unvoiced normal decoding
		if psDec.lossCnt != 0 && psDec.prevSignalType == typeVoiced &&
			psDec.indices.signalType != typeVoiced && k < maxNbSubfr/2 {

			clear(BQ14[:ltpOrder])
			BQ14[ltpOrder/2] = 4096 // SILK_FIX_CONST( 0.25, 14 )

			signalType = typeVoiced
			psDecCtrl.pitchL[k] = psDec.lagPrev
		}

		if signalType == typeVoiced {
			// Voiced
			lag = psDecCtrl.pitchL[k]

			// Re-whitening
			if k == 0 || (k == 2 && NLSFInterpolationFlag != 0) {
				// Rewhiten with new A coefs
				startIdx = psDec.ltpMemLength - lag - psDec.lpcOrder - ltpOrder/2

				if k == 2 {
					copy(psDec.outBuf[psDec.ltpMemLength:psDec.ltpMemLength+2*psDec.subfrLength], xq[:2*psDec.subfrLength])
				}

				lpcAnalysisFilter(sLTP[startIdx:], psDec.outBuf[startIdx+k*psDec.subfrLength:],
					AQ12, psDec.ltpMemLength-startIdx, psDec.lpcOrder)

				// After rewhitening the LTP state is unscaled
				if k == 0 {
					// Do LTP downscaling to reduce inter-packet dependency
					invGainQ31 = smulwb(invGainQ31, psDecCtrl.ltpScaleQ14) << 2
				}
				for i = 0; i < lag+ltpOrder/2; i++ {
					sLTPQ15[sLTPBufIdx-i-1] = smulwb(invGainQ31, int32(sLTP[psDec.ltpMemLength-i-1]))
				}
			} else {
				// Update LTP state when Gain changes
				if gainAdjQ16 != 1<<16 {
					for i = 0; i < lag+ltpOrder/2; i++ {
						sLTPQ15[sLTPBufIdx-i-1] = smulww(gainAdjQ16, sLTPQ15[sLTPBufIdx-i-1])
					}
				}
			}
		}

		// Long-term prediction
		if signalType == typeVoiced {
			// Set up pointer
			predLagPtr := sLTPBufIdx - lag + ltpOrder/2 // pred_lag_ptr = &sLTP_Q15[ ... ]
			for i = 0; i < psDec.subfrLength; i++ {
				// Unrolled loop
				// Avoids introducing a bias because silk_SMLAWB() always rounds to -inf
				LTPPredQ13 = 2
				LTPPredQ13 = smlawb(LTPPredQ13, sLTPQ15[predLagPtr], int32(BQ14[0]))
				LTPPredQ13 = smlawb(LTPPredQ13, sLTPQ15[predLagPtr-1], int32(BQ14[1]))
				LTPPredQ13 = smlawb(LTPPredQ13, sLTPQ15[predLagPtr-2], int32(BQ14[2]))
				LTPPredQ13 = smlawb(LTPPredQ13, sLTPQ15[predLagPtr-3], int32(BQ14[3]))
				LTPPredQ13 = smlawb(LTPPredQ13, sLTPQ15[predLagPtr-4], int32(BQ14[4]))
				predLagPtr++

				// Generate LPC excitation
				presQ14[i] = pexcQ14[i] + LTPPredQ13<<1

				// Update states
				sLTPQ15[sLTPBufIdx] = presQ14[i] << 1
				sLTPBufIdx++
			}
		} else {
			presQ14 = pexcQ14
		}

		for i = 0; i < psDec.subfrLength; i++ {
			// Short-term prediction
			// Avoids introducing a bias because silk_SMLAWB() always rounds to -inf
			LPCPredQ10 = psDec.lpcOrder >> 1
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-1], int32(AQ12Tmp[0]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-2], int32(AQ12Tmp[1]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-3], int32(AQ12Tmp[2]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-4], int32(AQ12Tmp[3]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-5], int32(AQ12Tmp[4]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-6], int32(AQ12Tmp[5]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-7], int32(AQ12Tmp[6]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-8], int32(AQ12Tmp[7]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-9], int32(AQ12Tmp[8]))
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-10], int32(AQ12Tmp[9]))
			if psDec.lpcOrder == 16 {
				LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-11], int32(AQ12Tmp[10]))
				LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-12], int32(AQ12Tmp[11]))
				LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-13], int32(AQ12Tmp[12]))
				LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-14], int32(AQ12Tmp[13]))
				LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-15], int32(AQ12Tmp[14]))
				LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14[maxLPCOrder+i-16], int32(AQ12Tmp[15]))
			}

			// Add prediction to LPC excitation
			sLPCQ14[maxLPCOrder+i] = addSat32(presQ14[i], lshiftSat32(LPCPredQ10, 4))

			// Scale with gain
			pxq[i] = int16(sat16(rshiftRound(smulww(sLPCQ14[maxLPCOrder+i], GainQ10), 8)))
		}

		// Update LPC filter state
		copy(sLPCQ14[:maxLPCOrder], sLPCQ14[psDec.subfrLength:])
		pexcQ14 = pexcQ14[psDec.subfrLength:]
		pxq = pxq[psDec.subfrLength:]
	}

	// Save LPC state
	copy(psDec.sLPCQ14Buf[:], sLPCQ14[:maxLPCOrder])
}
