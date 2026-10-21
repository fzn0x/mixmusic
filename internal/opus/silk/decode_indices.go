// Port of silk/decode_indices.c.

package silk

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

// decodeIndices is silk_decode_indices: decodes the side-information
// parameters of frame FrameIndex from the payload.
func decodeIndices(psDec *decoderState, psRangeDec *rangedec.Decoder, FrameIndex, decodeLBRR, condCoding int32) {
	var i, k, Ix int32
	var decodeAbsoluteLagIndex, deltaLagIndex int32
	var ecIx [maxLPCOrder]int16
	var predQ8 [maxLPCOrder]uint8

	/*******************************************/
	/* Decode signal type and quantizer offset */
	/*******************************************/
	if decodeLBRR != 0 || psDec.vadFlags[FrameIndex] != 0 {
		Ix = int32(psRangeDec.ICDF(typeOffsetVADICDF[:], 8)) + 2
	} else {
		Ix = int32(psRangeDec.ICDF(typeOffsetNoVADICDF[:], 8))
	}
	psDec.indices.signalType = int8(Ix >> 1)
	psDec.indices.quantOffsetType = int8(Ix & 1)

	/****************/
	/* Decode gains */
	/****************/
	// First subframe
	if condCoding == codeConditionally {
		// Conditional coding
		psDec.indices.gainsIndices[0] = int8(psRangeDec.ICDF(deltaGainICDF[:], 8))
	} else {
		// Independent coding, in two stages: MSB bits followed by 3 LSBs
		psDec.indices.gainsIndices[0] = int8(int32(psRangeDec.ICDF(gainICDF[psDec.indices.signalType][:], 8)) << 3)
		psDec.indices.gainsIndices[0] += int8(psRangeDec.ICDF(uniform8ICDF[:], 8))
	}

	// Remaining subframes
	for i = 1; i < psDec.nbSubfr; i++ {
		psDec.indices.gainsIndices[i] = int8(psRangeDec.ICDF(deltaGainICDF[:], 8))
	}

	/**********************/
	/* Decode LSF Indices */
	/**********************/
	psDec.indices.nlsfIndices[0] = int8(psRangeDec.ICDF(psDec.psNLSFCB.cb1ICDF[(int32(psDec.indices.signalType)>>1)*int32(psDec.psNLSFCB.nVectors):], 8))
	nlsfUnpack(ecIx[:], predQ8[:], psDec.psNLSFCB, int32(psDec.indices.nlsfIndices[0]))
	for i = 0; i < int32(psDec.psNLSFCB.order); i++ {
		Ix = int32(psRangeDec.ICDF(psDec.psNLSFCB.ecICDF[ecIx[i]:], 8))
		if Ix == 0 {
			Ix -= int32(psRangeDec.ICDF(nlsfExtICDF[:], 8))
		} else if Ix == 2*nlsfQuantMaxAmplitude {
			Ix += int32(psRangeDec.ICDF(nlsfExtICDF[:], 8))
		}
		psDec.indices.nlsfIndices[i+1] = int8(Ix - nlsfQuantMaxAmplitude)
	}

	// Decode LSF interpolation factor
	if psDec.nbSubfr == maxNbSubfr {
		psDec.indices.nlsfInterpCoefQ2 = int8(psRangeDec.ICDF(nlsfInterpolationFactorICDF[:], 8))
	} else {
		psDec.indices.nlsfInterpCoefQ2 = 4
	}

	if psDec.indices.signalType == typeVoiced {
		/*********************/
		/* Decode pitch lags */
		/*********************/
		// Get lag index
		decodeAbsoluteLagIndex = 1
		if condCoding == codeConditionally && psDec.ecPrevSignalType == typeVoiced {
			// Decode Delta index
			deltaLagIndex = int32(int16(psRangeDec.ICDF(pitchDeltaICDF[:], 8)))
			if deltaLagIndex > 0 {
				deltaLagIndex = deltaLagIndex - 9
				psDec.indices.lagIndex = int16(int32(psDec.ecPrevLagIndex) + deltaLagIndex)
				decodeAbsoluteLagIndex = 0
			}
		}
		if decodeAbsoluteLagIndex != 0 {
			// Absolute decoding
			psDec.indices.lagIndex = int16(int32(int16(psRangeDec.ICDF(pitchLagICDF[:], 8))) * (psDec.fsKHz >> 1))
			psDec.indices.lagIndex += int16(psRangeDec.ICDF(psDec.pitchLagLowBitsICDF, 8))
		}
		psDec.ecPrevLagIndex = psDec.indices.lagIndex

		// Get contour index
		psDec.indices.contourIndex = int8(psRangeDec.ICDF(psDec.pitchContourICDF, 8))

		/********************/
		/* Decode LTP gains */
		/********************/
		// Decode PERIndex value
		psDec.indices.perIndex = int8(psRangeDec.ICDF(ltpPerIndexICDF[:], 8))

		for k = 0; k < psDec.nbSubfr; k++ {
			psDec.indices.ltpIndex[k] = int8(psRangeDec.ICDF(ltpGainICDFPtrs[psDec.indices.perIndex], 8))
		}

		/**********************/
		/* Decode LTP scaling */
		/**********************/
		if condCoding == codeIndependently {
			psDec.indices.ltpScaleIndex = int8(psRangeDec.ICDF(ltpScaleICDF[:], 8))
		} else {
			psDec.indices.ltpScaleIndex = 0
		}
	}
	psDec.ecPrevSignalType = int32(psDec.indices.signalType)

	/***************/
	/* Decode seed */
	/***************/
	psDec.indices.seed = int8(psRangeDec.ICDF(uniform4ICDF[:], 8))
}
