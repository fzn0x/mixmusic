// Port of silk/PLC.c (without ENABLE_DEEP_PLC).

package silk

const nbAtt = 2

var (
	harmAttQ15            = [nbAtt]int16{32440, 31130} // 0.99, 0.95
	plcRandAttenuateVQ15  = [nbAtt]int16{31130, 26214} // 0.95, 0.8
	plcRandAttenuateUVQ15 = [nbAtt]int16{32440, 29491} // 0.99, 0.9
)

// plcReset is silk_PLC_Reset.
func plcReset(psDec *decoderState) {
	psDec.sPLC.pitchLQ8 = psDec.frameLength << (8 - 1)
	psDec.sPLC.prevGainQ16[0] = 1 << 16 // SILK_FIX_CONST( 1, 16 )
	psDec.sPLC.prevGainQ16[1] = 1 << 16
	psDec.sPLC.subfrLength = 20
	psDec.sPLC.nbSubfr = 2
}

// plc is silk_PLC: conceals a lost frame into frame, or updates the PLC state
// from a decoded one.
func plc(psDec *decoderState, psDecCtrl *decoderControl, frame []int16, lost int32) {
	// PLC control function
	if psDec.fsKHz != psDec.sPLC.fsKHz {
		plcReset(psDec)
		psDec.sPLC.fsKHz = psDec.fsKHz
	}

	if lost != 0 {
		/****************************/
		/* Generate Signal          */
		/****************************/
		plcConceal(psDec, psDecCtrl, frame)

		psDec.lossCnt++
	} else {
		/****************************/
		/* Update state             */
		/****************************/
		plcUpdate(psDec, psDecCtrl)
	}
}

// plcUpdate is silk_PLC_update: updates the state of the PLC.
func plcUpdate(psDec *decoderState, psDecCtrl *decoderControl) {
	var LTPGainQ14, tempLTPGainQ14 int32
	var i, j int32
	psPLC := &psDec.sPLC

	// Update parameters used in case of packet loss
	psDec.prevSignalType = int32(psDec.indices.signalType)
	LTPGainQ14 = 0
	if psDec.indices.signalType == typeVoiced {
		// Find the parameters for the last subframe which contains a pitch pulse
		for j = 0; j*psDec.subfrLength < psDecCtrl.pitchL[psDec.nbSubfr-1]; j++ {
			if j == psDec.nbSubfr {
				break
			}
			tempLTPGainQ14 = 0
			for i = 0; i < ltpOrder; i++ {
				tempLTPGainQ14 += int32(psDecCtrl.ltpCoefQ14[(psDec.nbSubfr-1-j)*ltpOrder+i])
			}
			if tempLTPGainQ14 > LTPGainQ14 {
				LTPGainQ14 = tempLTPGainQ14
				copy(psPLC.ltpCoefQ14[:],
					psDecCtrl.ltpCoefQ14[smulbb(psDec.nbSubfr-1-j, ltpOrder):])

				psPLC.pitchLQ8 = psDecCtrl.pitchL[psDec.nbSubfr-1-j] << 8
			}
		}

		clear(psPLC.ltpCoefQ14[:])
		psPLC.ltpCoefQ14[ltpOrder/2] = int16(LTPGainQ14)

		// Limit LT coefs
		if LTPGainQ14 < vPitchGainStartMinQ14 {
			var scaleQ10, tmp int32

			tmp = vPitchGainStartMinQ14 << 10
			scaleQ10 = tmp / max(LTPGainQ14, 1)
			for i = 0; i < ltpOrder; i++ {
				psPLC.ltpCoefQ14[i] = int16(smulbb(int32(psPLC.ltpCoefQ14[i]), scaleQ10) >> 10)
			}
		} else if LTPGainQ14 > vPitchGainStartMaxQ14 {
			var scaleQ14, tmp int32

			tmp = vPitchGainStartMaxQ14 << 14
			scaleQ14 = tmp / max(LTPGainQ14, 1)
			for i = 0; i < ltpOrder; i++ {
				psPLC.ltpCoefQ14[i] = int16(smulbb(int32(psPLC.ltpCoefQ14[i]), scaleQ14) >> 14)
			}
		}
	} else {
		psPLC.pitchLQ8 = smulbb(psDec.fsKHz, 18) << 8
		clear(psPLC.ltpCoefQ14[:])
	}

	// Save LPC coefficients
	copy(psPLC.prevLPCQ12[:psDec.lpcOrder], psDecCtrl.predCoefQ12[1][:psDec.lpcOrder])
	psPLC.prevLTPScaleQ14 = int16(psDecCtrl.ltpScaleQ14)

	// Save last two gains
	copy(psPLC.prevGainQ16[:], psDecCtrl.gainsQ16[psDec.nbSubfr-2:])

	psPLC.subfrLength = psDec.subfrLength
	psPLC.nbSubfr = psDec.nbSubfr
}

// plcEnergy is silk_PLC_energy: the energies of the last two subframes of
// the excitation, scaled by their gains, and their shifts.
func plcEnergy(excQ14 []int32, prevGainQ10 *[2]int32, subfrLength, nbSubfr int32, excBuf []int16) (energy1, shift1, energy2, shift2 int32) {
	// Find random noise component
	// Scale previous excitation signal
	excBufPtr := excBuf
	for k := int32(0); k < 2; k++ {
		for i := int32(0); i < subfrLength; i++ {
			excBufPtr[i] = int16(sat16(
				smulww(excQ14[i+(k+nbSubfr-2)*subfrLength], prevGainQ10[k]) >> 8))
		}
		excBufPtr = excBufPtr[subfrLength:]
	}
	// Find the subframe with lowest energy of the last two and use that as random noise generator
	energy1, shift1 = sumSqrShift(excBuf, subfrLength)
	energy2, shift2 = sumSqrShift(excBuf[subfrLength:], subfrLength)
	return
}

// plcConceal is silk_PLC_conceal: generates a concealed frame, the LPC
// residual signal, into frame.
func plcConceal(psDec *decoderState, psDecCtrl *decoderControl, frame []int16) {
	var i, j, k int32
	var lag, idx, sLTPBufIdx, shift1, shift2 int32
	var randSeed, harmGainQ15, randGainQ15, invGainQ30 int32
	var energy1, energy2 int32
	var randPtr []int32
	var LPCPredQ10, LTPPredQ12 int32
	var randScaleQ14 int16
	var BQ14 []int16
	var AQ12 [maxLPCOrder]int16
	psPLC := &psDec.sPLC
	var prevGainQ10 [2]int32

	sLTPQ14 := psDec.scratch.sLTPQ15[:psDec.ltpMemLength+psDec.frameLength]
	sLTP := psDec.scratch.sLTP[:psDec.ltpMemLength]

	prevGainQ10[0] = psPLC.prevGainQ16[0] >> 6
	prevGainQ10[1] = psPLC.prevGainQ16[1] >> 6

	if psDec.firstFrameAfterReset != 0 {
		clear(psPLC.prevLPCQ12[:])
	}

	energy1, shift1, energy2, shift2 = plcEnergy(psDec.excQ14[:], &prevGainQ10, psDec.subfrLength, psDec.nbSubfr, psDec.scratch.excBuf[:])

	if energy1>>shift2 < energy2>>shift1 {
		// First sub-frame has lowest energy
		randPtr = psDec.excQ14[max(0, (psPLC.nbSubfr-1)*psPLC.subfrLength-randBufSize):]
	} else {
		// Second sub-frame has lowest energy
		randPtr = psDec.excQ14[max(0, psPLC.nbSubfr*psPLC.subfrLength-randBufSize):]
	}

	// Set up Gain to random noise component
	BQ14 = psPLC.ltpCoefQ14[:]
	randScaleQ14 = psPLC.randScaleQ14

	// Set up attenuation gains
	harmGainQ15 = int32(harmAttQ15[min(nbAtt-1, psDec.lossCnt)])
	if psDec.prevSignalType == typeVoiced {
		randGainQ15 = int32(plcRandAttenuateVQ15[min(nbAtt-1, psDec.lossCnt)])
	} else {
		randGainQ15 = int32(plcRandAttenuateUVQ15[min(nbAtt-1, psDec.lossCnt)])
	}

	// LPC concealment. Apply BWE to previous LPC
	bwexpander(psPLC.prevLPCQ12[:], psDec.lpcOrder, bweCoefQ16)

	// Preload LPC coefficients to array on stack. Gives small performance gain
	copy(AQ12[:psDec.lpcOrder], psPLC.prevLPCQ12[:psDec.lpcOrder])

	// First Lost frame
	if psDec.lossCnt == 0 {
		randScaleQ14 = 1 << 14

		// Reduce random noise Gain for voiced frames
		if psDec.prevSignalType == typeVoiced {
			for i = 0; i < ltpOrder; i++ {
				randScaleQ14 -= BQ14[i]
			}
			randScaleQ14 = max(3277, randScaleQ14) // 0.2
			randScaleQ14 = int16(smulbb(int32(randScaleQ14), int32(psPLC.prevLTPScaleQ14)) >> 14)
		} else {
			// Reduce random noise for unvoiced frames with high LPC gain
			var invGainLPCQ30, downScaleQ30 int32 // invGain_Q30, down_scale_Q30

			invGainLPCQ30 = lpcInversePredGain(psPLC.prevLPCQ12[:], psDec.lpcOrder)

			downScaleQ30 = min((int32(1)<<30)>>log2InvLPCGainHighThres, invGainLPCQ30)
			downScaleQ30 = max((int32(1)<<30)>>log2InvLPCGainLowThres, downScaleQ30)
			downScaleQ30 = downScaleQ30 << log2InvLPCGainHighThres

			randGainQ15 = smulwb(downScaleQ30, randGainQ15) >> 14
		}
	}

	randSeed = psPLC.randSeed
	lag = rshiftRound(psPLC.pitchLQ8, 8)
	sLTPBufIdx = psDec.ltpMemLength

	// Rewhiten LTP state
	idx = psDec.ltpMemLength - lag - psDec.lpcOrder - ltpOrder/2
	lpcAnalysisFilter(sLTP[idx:], psDec.outBuf[idx:], AQ12[:], psDec.ltpMemLength-idx, psDec.lpcOrder)
	// Scale LTP state
	invGainQ30 = inverse32VarQ(psPLC.prevGainQ16[1], 46)
	invGainQ30 = min(invGainQ30, silkInt32Max>>1)
	for i = idx + psDec.lpcOrder; i < psDec.ltpMemLength; i++ {
		sLTPQ14[i] = smulwb(invGainQ30, int32(sLTP[i]))
	}

	/***************************/
	/* LTP synthesis filtering */
	/***************************/
	for k = 0; k < psDec.nbSubfr; k++ {
		// Set up pointer
		predLagPtr := sLTPBufIdx - lag + ltpOrder/2 // pred_lag_ptr = &sLTP_Q14[ ... ]
		for i = 0; i < psDec.subfrLength; i++ {
			// Unrolled loop
			// Avoids introducing a bias because silk_SMLAWB() always rounds to -inf
			LTPPredQ12 = 2
			LTPPredQ12 = smlawb(LTPPredQ12, sLTPQ14[predLagPtr], int32(BQ14[0]))
			LTPPredQ12 = smlawb(LTPPredQ12, sLTPQ14[predLagPtr-1], int32(BQ14[1]))
			LTPPredQ12 = smlawb(LTPPredQ12, sLTPQ14[predLagPtr-2], int32(BQ14[2]))
			LTPPredQ12 = smlawb(LTPPredQ12, sLTPQ14[predLagPtr-3], int32(BQ14[3]))
			LTPPredQ12 = smlawb(LTPPredQ12, sLTPQ14[predLagPtr-4], int32(BQ14[4]))
			predLagPtr++

			// Generate LPC excitation
			randSeed = silkRAND(randSeed)
			idx = (randSeed >> 25) & randBufMask
			sLTPQ14[sLTPBufIdx] = smlawb(LTPPredQ12, randPtr[idx], int32(randScaleQ14)) << 2
			sLTPBufIdx++
		}

		// Gradually reduce LTP gain
		for j = 0; j < ltpOrder; j++ {
			BQ14[j] = int16(smulbb(harmGainQ15, int32(BQ14[j])) >> 15)
		}
		// Gradually reduce excitation gain
		randScaleQ14 = int16(smulbb(int32(randScaleQ14), randGainQ15) >> 15)

		// Slowly increase pitch lag
		psPLC.pitchLQ8 = smlawb(psPLC.pitchLQ8, psPLC.pitchLQ8, pitchDriftFacQ16)
		psPLC.pitchLQ8 = min(psPLC.pitchLQ8, smulbb(maxPitchLagMs, psDec.fsKHz)<<8)
		lag = rshiftRound(psPLC.pitchLQ8, 8)
	}

	/***************************/
	/* LPC synthesis filtering */
	/***************************/
	sLPCQ14Ptr := sLTPQ14[psDec.ltpMemLength-maxLPCOrder:]

	// Copy LPC state
	copy(sLPCQ14Ptr[:maxLPCOrder], psDec.sLPCQ14Buf[:])

	for i = 0; i < psDec.frameLength; i++ {
		// partly unrolled
		// Avoids introducing a bias because silk_SMLAWB() always rounds to -inf
		LPCPredQ10 = psDec.lpcOrder >> 1
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-1], int32(AQ12[0]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-2], int32(AQ12[1]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-3], int32(AQ12[2]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-4], int32(AQ12[3]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-5], int32(AQ12[4]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-6], int32(AQ12[5]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-7], int32(AQ12[6]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-8], int32(AQ12[7]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-9], int32(AQ12[8]))
		LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-10], int32(AQ12[9]))
		for j = 10; j < psDec.lpcOrder; j++ {
			LPCPredQ10 = smlawb(LPCPredQ10, sLPCQ14Ptr[maxLPCOrder+i-j-1], int32(AQ12[j]))
		}

		// Add prediction to LPC excitation
		sLPCQ14Ptr[maxLPCOrder+i] = addSat32(sLPCQ14Ptr[maxLPCOrder+i],
			lshiftSat32(LPCPredQ10, 4))

		// Scale with Gain
		frame[i] = int16(sat16(sat16(rshiftRound(smulww(sLPCQ14Ptr[maxLPCOrder+i], prevGainQ10[1]), 8))))
	}

	// Save LPC state
	copy(psDec.sLPCQ14Buf[:], sLPCQ14Ptr[psDec.frameLength:])

	/**************************************/
	/* Update states                      */
	/**************************************/
	psPLC.randSeed = randSeed
	psPLC.randScaleQ14 = randScaleQ14
	for i = 0; i < maxNbSubfr; i++ {
		psDecCtrl.pitchL[i] = lag
	}
}

// plcGlueFrames is silk_PLC_glue_frames: glues concealed frames with new good
// received frames.
func plcGlueFrames(psDec *decoderState, frame []int16, length int32) {
	var i, energyShift int32
	var energy int32
	psPLC := &psDec.sPLC

	if psDec.lossCnt != 0 {
		// Calculate energy in concealed residual
		psPLC.concEnergy, psPLC.concEnergyShift = sumSqrShift(frame, length)

		psPLC.lastFrameLost = 1
	} else {
		if psDec.sPLC.lastFrameLost != 0 {
			// Calculate residual in decoded signal if last frame was lost
			energy, energyShift = sumSqrShift(frame, length)

			// Normalize energies
			if energyShift > psPLC.concEnergyShift {
				psPLC.concEnergy = psPLC.concEnergy >> (energyShift - psPLC.concEnergyShift)
			} else if energyShift < psPLC.concEnergyShift {
				energy = energy >> (psPLC.concEnergyShift - energyShift)
			}

			// Fade in the energy difference
			if energy > psPLC.concEnergy {
				var fracQ24, LZ int32
				var gainQ16, slopeQ16 int32

				LZ = clz32(psPLC.concEnergy)
				LZ = LZ - 1
				psPLC.concEnergy = psPLC.concEnergy << LZ
				energy = energy >> max(24-LZ, 0)

				fracQ24 = psPLC.concEnergy / max(energy, 1)

				gainQ16 = sqrtApprox(fracQ24) << 4
				slopeQ16 = ((int32(1) << 16) - gainQ16) / length
				// Make slope 4x steeper to avoid missing onsets after DTX
				slopeQ16 = slopeQ16 << 2

				for i = 0; i < length; i++ {
					frame[i] = int16(smulwb(gainQ16, int32(frame[i])))
					gainQ16 += slopeQ16
					if gainQ16 > int32(1)<<16 {
						break
					}
				}
			}
		}
		psPLC.lastFrameLost = 0
	}
}
