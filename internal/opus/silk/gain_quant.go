// Port of silk/gain_quant.c (silk_gains_dequant, the part the decoder uses).

package silk

const (
	gainOffset      = (minQGainDB*128)/6 + 16*128                                            // OFFSET
	gainInvScaleQ16 = (65536 * (((maxQGainDB - minQGainDB) * 128) / 6)) / (nLevelsQGain - 1) // INV_SCALE_Q16
)

// gainsDequant is silk_gains_dequant: gain scalar dequantization, uniform on
// log scale. conditional is whether the first gain is delta coded.
func gainsDequant(gainQ16 *[maxNbSubfr]int32, ind *[maxNbSubfr]int8, prevInd *int8, conditional bool, nbSubfr int32) {
	var indTmp, doubleStepSizeThreshold int32

	for k := int32(0); k < nbSubfr; k++ {
		if k == 0 && !conditional {
			// Gain index is not allowed to go down more than 16 steps (~21.8 dB)
			*prevInd = int8(max(int32(ind[k]), int32(*prevInd)-16))
		} else {
			// Delta index
			indTmp = int32(ind[k]) + minDeltaGainQuant

			// Accumulate deltas
			doubleStepSizeThreshold = 2*maxDeltaGainQuant - nLevelsQGain + int32(*prevInd)
			if indTmp > doubleStepSizeThreshold {
				*prevInd = int8(int32(*prevInd) + indTmp<<1 - doubleStepSizeThreshold)
			} else {
				*prevInd = int8(int32(*prevInd) + indTmp)
			}
		}
		*prevInd = int8(limit(int32(*prevInd), 0, nLevelsQGain-1))

		// Scale and convert to linear scale
		gainQ16[k] = log2lin(min(smulwb(gainInvScaleQ16, int32(*prevInd))+gainOffset, 3967)) // 3967 = 31 in Q7
	}
}
