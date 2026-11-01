// Port of silk/log2lin.c.

package silk

// log2lin is silk_log2lin: an approximation of 2^() (very close inverse of
// silk_lin2log()), converting the input to a linear scale.
func log2lin(inLogQ7 int32) int32 {
	var out, fracQ7 int32

	if inLogQ7 < 0 {
		return 0
	} else if inLogQ7 >= 3967 {
		return silkInt32Max
	}

	out = int32(1) << (inLogQ7 >> 7)
	fracQ7 = inLogQ7 & 0x7F
	if inLogQ7 < 2048 {
		// Piece-wise parabolic approximation
		out = out + (out*smlawb(fracQ7, smulbb(fracQ7, 128-fracQ7), -174))>>7
	} else {
		// Piece-wise parabolic approximation
		out = out + (out>>7)*smlawb(fracQ7, smulbb(fracQ7, 128-fracQ7), -174)
	}
	return out
}
