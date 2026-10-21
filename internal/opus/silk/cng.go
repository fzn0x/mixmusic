// Port of silk/CNG.c.

package silk

// cngExc is silk_CNG_exc: generates the excitation for CNG LPC synthesis.
func cngExc(excQ14 []int32, excBufQ14 []int32, length int32, randSeed *int32) {
	var seed int32
	var i, idx, excMask int32

	excMask = cngBufMaskMax
	for excMask > length {
		excMask = excMask >> 1
	}

	seed = *randSeed
	for i = 0; i < length; i++ {
		seed = silkRAND(seed)
		idx = (seed >> 24) & excMask
		excQ14[i] = excBufQ14[idx]
	}
	*randSeed = seed
}

// cngReset is silk_CNG_Reset.
func cngReset(psDec *decoderState) {
	var i, NLSFStepQ15, NLSFAccQ15 int32

	NLSFStepQ15 = silkInt16Max / (psDec.lpcOrder + 1)
	NLSFAccQ15 = 0
	for i = 0; i < psDec.lpcOrder; i++ {
		NLSFAccQ15 += NLSFStepQ15
		psDec.sCNG.cngSmthNLSFQ15[i] = int16(NLSFAccQ15)
	}
	psDec.sCNG.cngSmthGainQ16 = 0
	psDec.sCNG.randSeed = 3176576
}

// cng is silk_CNG: updates the CNG estimate, and applies the CNG to frame,
// of length samples, when the packet was lost.
func cng(psDec *decoderState, psDecCtrl *decoderControl, frame []int16, length int32) {
	var i, subfr int32
	var LPCPredQ10, maxGainQ16, gainQ16, gainQ10 int32
	var AQ12 [maxLPCOrder]int16
	psCNG := &psDec.sCNG

	if psDec.fsKHz != psCNG.fsKHz {
		// Reset state
		cngReset(psDec)

		psCNG.fsKHz = psDec.fsKHz
	}
	if psDec.lossCnt == 0 && psDec.prevSignalType == typeNoVoiceActivity {
		// Update CNG parameters

		// Smoothing of LSF's
		for i = 0; i < psDec.lpcOrder; i++ {
			psCNG.cngSmthNLSFQ15[i] += int16(smulwb(int32(psDec.prevNLSFQ15[i])-int32(psCNG.cngSmthNLSFQ15[i]), cngNLSFSmthQ16))
		}
		// Find the subframe with the highest gain
		maxGainQ16 = 0
		subfr = 0
		for i = 0; i < psDec.nbSubfr; i++ {
			if psDecCtrl.gainsQ16[i] > maxGainQ16 {
				maxGainQ16 = psDecCtrl.gainsQ16[i]
				subfr = i
			}
		}
		// Update CNG excitation buffer with excitation from this subframe
		copy(psCNG.cngExcBufQ14[psDec.subfrLength:psDec.nbSubfr*psDec.subfrLength], psCNG.cngExcBufQ14[:])
		copy(psCNG.cngExcBufQ14[:psDec.subfrLength], psDec.excQ14[subfr*psDec.subfrLength:])

		// Smooth gains
		for i = 0; i < psDec.nbSubfr; i++ {
			psCNG.cngSmthGainQ16 += smulwb(psDecCtrl.gainsQ16[i]-psCNG.cngSmthGainQ16, cngGainSmthQ16)
			// If the smoothed gain is 3 dB greater than this subframe's gain, use this subframe's gain to adapt faster.
			if smulww(psCNG.cngSmthGainQ16, cngGainSmthThresholdQ16) > psDecCtrl.gainsQ16[i] {
				psCNG.cngSmthGainQ16 = psDecCtrl.gainsQ16[i]
			}
		}
	}

	// Add CNG when packet is lost or during DTX
	if psDec.lossCnt != 0 {
		CNGSigQ14 := psDec.scratch.cngSigQ14[:length+maxLPCOrder]

		// Generate CNG excitation
		gainQ16 = smulww(int32(psDec.sPLC.randScaleQ14), psDec.sPLC.prevGainQ16[1])
		if gainQ16 >= (1<<21) || psCNG.cngSmthGainQ16 > (1<<23) {
			gainQ16 = smultt(gainQ16, gainQ16)
			gainQ16 = smultt(psCNG.cngSmthGainQ16, psCNG.cngSmthGainQ16) - gainQ16<<5
			gainQ16 = sqrtApprox(gainQ16) << 16
		} else {
			gainQ16 = smulww(gainQ16, gainQ16)
			gainQ16 = smulww(psCNG.cngSmthGainQ16, psCNG.cngSmthGainQ16) - gainQ16<<5
			gainQ16 = sqrtApprox(gainQ16) << 8
		}
		gainQ10 = gainQ16 >> 6

		cngExc(CNGSigQ14[maxLPCOrder:], psCNG.cngExcBufQ14[:], length, &psCNG.randSeed)

		// Convert CNG NLSF to filter representation
		nlsf2A(AQ12[:], psCNG.cngSmthNLSFQ15[:], psDec.lpcOrder)

		// Generate CNG signal, by synthesis filtering
		copy(CNGSigQ14[:maxLPCOrder], psCNG.cngSynthState[:])
		for i = 0; i < length; i++ {
			// Avoids introducing a bias because silk_SMLAWB() always rounds to -inf
			LPCPredQ10 = psDec.lpcOrder >> 1
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-1], int32(AQ12[0]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-2], int32(AQ12[1]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-3], int32(AQ12[2]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-4], int32(AQ12[3]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-5], int32(AQ12[4]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-6], int32(AQ12[5]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-7], int32(AQ12[6]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-8], int32(AQ12[7]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-9], int32(AQ12[8]))
			LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-10], int32(AQ12[9]))
			if psDec.lpcOrder == 16 {
				LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-11], int32(AQ12[10]))
				LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-12], int32(AQ12[11]))
				LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-13], int32(AQ12[12]))
				LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-14], int32(AQ12[13]))
				LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-15], int32(AQ12[14]))
				LPCPredQ10 = smlawb(LPCPredQ10, CNGSigQ14[maxLPCOrder+i-16], int32(AQ12[15]))
			}

			// Update states
			CNGSigQ14[maxLPCOrder+i] = addSat32(CNGSigQ14[maxLPCOrder+i], lshiftSat32(LPCPredQ10, 4))

			// Scale with Gain and add to input signal
			frame[i] = addSat16(int32(frame[i]), sat16(rshiftRound(smulww(CNGSigQ14[maxLPCOrder+i], gainQ10), 8)))

		}
		copy(psCNG.cngSynthState[:], CNGSigQ14[length:])
	} else {
		clear(psCNG.cngSynthState[:psDec.lpcOrder])
	}
}
