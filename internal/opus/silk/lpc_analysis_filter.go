// Port of silk/LPC_analysis_filter.c (the generic path; USE_CELT_FIR is 0 and FIXED_POINT undefined).

package silk

// lpcAnalysisFilter is silk_LPC_analysis_filter: a variable order MA
// prediction error filter. The filter always starts with zero state, and the
// first d output samples are set to zero.
func lpcAnalysisFilter(out, in, B []int16, length, d int32) {
	var out32Q12, out32 int32

	for ix := d; ix < length; ix++ {
		inPtr := ix - 1 // in_ptr = &in[ ix - 1 ]

		out32Q12 = smulbb(int32(in[inPtr]), int32(B[0]))
		// Allowing wrap around so that two wraps can cancel each other. The rare
		// cases where the result wraps around can only be triggered by invalid streams
		out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-1]), int32(B[1]))
		out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-2]), int32(B[2]))
		out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-3]), int32(B[3]))
		out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-4]), int32(B[4]))
		out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-5]), int32(B[5]))
		for j := int32(6); j < d; j += 2 {
			out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-j]), int32(B[j]))
			out32Q12 = smlabbOvflw(out32Q12, int32(in[inPtr-j-1]), int32(B[j+1]))
		}

		// Subtract prediction
		out32Q12 = sub32Ovflw(int32(in[inPtr+1])<<12, out32Q12)

		// Scale to Q0
		out32 = rshiftRound(out32Q12, 12)

		// Saturate output
		out[ix] = int16(sat16(out32))
	}

	// Set first d output samples to zero
	clear(out[:d])
}
