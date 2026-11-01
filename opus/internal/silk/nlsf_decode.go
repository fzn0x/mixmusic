// Port of silk/NLSF_decode.c.

package silk

// nlsfQuantLevelAdjQ10 is SILK_FIX_CONST( NLSF_QUANT_LEVEL_ADJ, 10 ),
// NLSF_QUANT_LEVEL_ADJ 0.1.
const nlsfQuantLevelAdjQ10 = 102

// nlsfResidualDequant is silk_NLSF_residual_dequant: the predictive
// dequantizer for NLSF residuals.
func nlsfResidualDequant(xQ10 []int16, indices []int8, predCoefQ8 []uint8, quantStepSizeQ16 int32, order int16) {
	var outQ10, predQ10 int32

	outQ10 = 0
	for i := int32(order) - 1; i >= 0; i-- {
		predQ10 = smulbb(outQ10, int32(int16(predCoefQ8[i]))) >> 8
		outQ10 = int32(indices[i]) << 10
		if outQ10 > 0 {
			outQ10 = outQ10 - nlsfQuantLevelAdjQ10
		} else if outQ10 < 0 {
			outQ10 = outQ10 + nlsfQuantLevelAdjQ10
		}
		outQ10 = smlawb(predQ10, outQ10, quantStepSizeQ16)
		xQ10[i] = int16(outQ10)
	}
}

// nlsfDecode is silk_NLSF_decode: the NLSF vector decoder. It writes the
// quantized NLSF vector, [ LPC_ORDER ], of the codebook path NLSFIndices,
// [ LPC_ORDER + 1 ].
func nlsfDecode(pNLSFQ15 []int16, NLSFIndices []int8, psNLSFCB *nlsfCBStruct) {
	var predQ8 [maxLPCOrder]uint8
	var ecIx [maxLPCOrder]int16
	var resQ10 [maxLPCOrder]int16
	var NLSFQ15Tmp int32

	// Unpack entropy table indices and predictor for current CB1 index
	nlsfUnpack(ecIx[:], predQ8[:], psNLSFCB, int32(NLSFIndices[0]))

	// Predictive residual dequantizer
	nlsfResidualDequant(resQ10[:], NLSFIndices[1:], predQ8[:], int32(psNLSFCB.quantStepSizeQ16), psNLSFCB.order)

	// Apply inverse square-rooted weights to first stage and add to output
	order := int32(psNLSFCB.order)
	pCBElement := psNLSFCB.cb1NLSFQ8[int32(NLSFIndices[0])*order:]
	pCBWghtQ9 := psNLSFCB.cb1WghtQ9[int32(NLSFIndices[0])*order:]
	for i := int32(0); i < order; i++ {
		NLSFQ15Tmp = (int32(resQ10[i])<<14)/int32(pCBWghtQ9[i]) + int32(int16(pCBElement[i]))<<7
		pNLSFQ15[i] = int16(limit(NLSFQ15Tmp, 0, 32767))
	}

	// NLSF stabilization
	nlsfStabilize(pNLSFQ15, psNLSFCB.deltaMinQ15, order)
}
