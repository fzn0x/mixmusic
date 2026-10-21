// Port of silk/resampler_private_up2_HQ.c.

package silk

// resamplerPrivateUp2HQ is silk_resampler_private_up2_HQ: upsamples by a
// factor 2, high quality. Uses 2nd order allpass filters for the 2x
// upsampling, followed by a notch filter just above Nyquist.
func resamplerPrivateUp2HQ(S []int32, out, in []int16, length int32) {
	var in32, out321, out322, Y, X int32

	// Internal variables and state are in Q10 format
	for k := int32(0); k < length; k++ {
		// Convert to Q10
		in32 = int32(in[k]) << 10

		// First all-pass section for even output sample
		Y = in32 - S[0]
		X = smulwb(Y, int32(resamplerUp2HQ0[0]))
		out321 = S[0] + X
		S[0] = in32 + X

		// Second all-pass section for even output sample
		Y = out321 - S[1]
		X = smulwb(Y, int32(resamplerUp2HQ0[1]))
		out322 = S[1] + X
		S[1] = out321 + X

		// Third all-pass section for even output sample
		Y = out322 - S[2]
		X = smlawb(Y, Y, int32(resamplerUp2HQ0[2]))
		out321 = S[2] + X
		S[2] = out322 + X

		// Apply gain in Q15, convert back to int16 and store to output
		out[2*k] = int16(sat16(rshiftRound(out321, 10)))

		// First all-pass section for odd output sample
		Y = in32 - S[3]
		X = smulwb(Y, int32(resamplerUp2HQ1[0]))
		out321 = S[3] + X
		S[3] = in32 + X

		// Second all-pass section for odd output sample
		Y = out321 - S[4]
		X = smulwb(Y, int32(resamplerUp2HQ1[1]))
		out322 = S[4] + X
		S[4] = out321 + X

		// Third all-pass section for odd output sample
		Y = out322 - S[5]
		X = smlawb(Y, Y, int32(resamplerUp2HQ1[2]))
		out321 = S[5] + X
		S[5] = out322 + X

		// Apply gain in Q15, convert back to int16 and store to output
		out[2*k+1] = int16(sat16(rshiftRound(out321, 10)))
	}
}

// resamplerPrivateUp2HQWrapper is silk_resampler_private_up2_HQ_wrapper.
func resamplerPrivateUp2HQWrapper(S *resamplerStateStruct, out, in []int16, length int32) {
	resamplerPrivateUp2HQ(S.sIIR[:], out, in, length)
}
