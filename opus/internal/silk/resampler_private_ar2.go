// Port of silk/resampler_private_AR2.c.

package silk

// resamplerPrivateAR2 is silk_resampler_private_AR2: a second order AR filter
// with single delay elements.
func resamplerPrivateAR2(S []int32, outQ8 []int32, in []int16, AQ14 []int16, length int32) {
	var out32 int32

	for k := int32(0); k < length; k++ {
		out32 = S[0] + int32(in[k])<<8
		outQ8[k] = out32
		out32 = out32 << 2
		S[0] = smlawb(S[1], out32, int32(AQ14[0]))
		S[1] = smulwb(out32, int32(AQ14[1]))
	}
}
