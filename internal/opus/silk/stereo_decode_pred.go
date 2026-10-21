// Port of silk/stereo_decode_pred.c.

package silk

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

// stereoDecodePred is silk_stereo_decode_pred: decodes the mid/side
// predictors.
func stereoDecodePred(psRangeDec *rangedec.Decoder, predQ13 []int32) {
	var n int32
	var ix [2][3]int32
	var lowQ13, stepQ13 int32

	// Entropy decoding
	n = int32(psRangeDec.ICDF(stereoPredJointICDF[:], 8))
	ix[0][2] = n / 5
	ix[1][2] = n - 5*ix[0][2]
	for n = 0; n < 2; n++ {
		ix[n][0] = int32(psRangeDec.ICDF(uniform3ICDF[:], 8))
		ix[n][1] = int32(psRangeDec.ICDF(uniform5ICDF[:], 8))
	}

	// Dequantize
	for n = 0; n < 2; n++ {
		ix[n][0] += 3 * ix[n][2]
		lowQ13 = int32(stereoPredQuantQ13[ix[n][0]])
		stepQ13 = smulwb(int32(stereoPredQuantQ13[ix[n][0]+1])-lowQ13,
			6554) // SILK_FIX_CONST( 0.5 / STEREO_QUANT_SUB_STEPS, 16 )
		predQ13[n] = smlabb(lowQ13, stepQ13, 2*ix[n][1]+1)
	}

	// Subtract second from first predictor (helps when actually applying these)
	predQ13[0] -= predQ13[1]
}

// stereoDecodeMidOnly is silk_stereo_decode_mid_only: decodes the flag that
// only the mid channel has been coded.
func stereoDecodeMidOnly(psRangeDec *rangedec.Decoder, decodeOnlyMid *int32) {
	// Decode flag that only mid channel is coded
	*decodeOnlyMid = int32(psRangeDec.ICDF(stereoOnlyCodeMidICDF[:], 8))
}
