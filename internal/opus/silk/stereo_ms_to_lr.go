// Port of silk/stereo_MS_to_LR.c.

package silk

// stereoMSToLR is silk_stereo_MS_to_LR: converts the adaptive Mid/Side
// representation in x1 and x2 to a Left/Right stereo signal. Both hold
// frameLength + 2 samples; the first two are buffered from the last call.
func stereoMSToLR(state *stereoDecState, x1, x2 []int16, predQ13 []int32, fsKHz, frameLength int32) {
	var n, denomQ16, delta0Q13, delta1Q13 int32
	var sum, diff, pred0Q13, pred1Q13 int32

	// Buffering
	copy(x1[:2], state.sMid[:])
	copy(x2[:2], state.sSide[:])
	copy(state.sMid[:], x1[frameLength:])
	copy(state.sSide[:], x2[frameLength:])

	// Interpolate predictors and add prediction to side channel
	pred0Q13 = int32(state.predPrevQ13[0])
	pred1Q13 = int32(state.predPrevQ13[1])
	denomQ16 = (int32(1) << 16) / (stereoInterpLenMs * fsKHz)
	delta0Q13 = rshiftRound(smulbb(predQ13[0]-int32(state.predPrevQ13[0]), denomQ16), 16)
	delta1Q13 = rshiftRound(smulbb(predQ13[1]-int32(state.predPrevQ13[1]), denomQ16), 16)
	for n = 0; n < stereoInterpLenMs*fsKHz; n++ {
		pred0Q13 += delta0Q13
		pred1Q13 += delta1Q13
		sum = (int32(x1[n]) + int32(x1[n+2]) + int32(x1[n+1])<<1) << 9 // Q11
		sum = smlawb(int32(x2[n+1])<<8, sum, pred0Q13)                 // Q8
		sum = smlawb(sum, int32(x1[n+1])<<11, pred1Q13)                // Q8
		x2[n+1] = int16(sat16(rshiftRound(sum, 8)))
	}
	pred0Q13 = predQ13[0]
	pred1Q13 = predQ13[1]
	for n = stereoInterpLenMs * fsKHz; n < frameLength; n++ {
		sum = (int32(x1[n]) + int32(x1[n+2]) + int32(x1[n+1])<<1) << 9 // Q11
		sum = smlawb(int32(x2[n+1])<<8, sum, pred0Q13)                 // Q8
		sum = smlawb(sum, int32(x1[n+1])<<11, pred1Q13)                // Q8
		x2[n+1] = int16(sat16(rshiftRound(sum, 8)))
	}
	state.predPrevQ13[0] = int16(predQ13[0])
	state.predPrevQ13[1] = int16(predQ13[1])

	// Convert to left/right signals
	for n = 0; n < frameLength; n++ {
		sum = int32(x1[n+1]) + int32(x2[n+1])
		diff = int32(x1[n+1]) - int32(x2[n+1])
		x1[n+1] = int16(sat16(sum))
		x2[n+1] = int16(sat16(diff))
	}
}
