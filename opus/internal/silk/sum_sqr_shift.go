// Port of silk/sum_sqr_shift.c.

package silk

// sumSqrShift is silk_sum_sqr_shift: the energy of x, of length samples, and
// the number of bits it was shifted right to fit in an int32.
func sumSqrShift(x []int16, length int32) (energy, shift int32) {
	var i, shft int32
	var nrgTmp uint32
	var nrg int32

	// Do a first run with the maximum shift we could have.
	shft = 31 - clz32(length)
	// Let's be conservative with rounding and start with nrg=len.
	nrg = length
	for i = 0; i < length-1; i += 2 {
		nrgTmp = uint32(smulbb(int32(x[i]), int32(x[i])))
		nrgTmp = uint32(smlabbOvflw(int32(nrgTmp), int32(x[i+1]), int32(x[i+1])))
		nrg = int32(uint32(nrg) + nrgTmp>>shft)
	}
	if i < length {
		// One sample left to process
		nrgTmp = uint32(smulbb(int32(x[i]), int32(x[i])))
		nrg = int32(uint32(nrg) + nrgTmp>>shft)
	}
	// Make sure the result will fit in a 32-bit signed integer with two bits
	// of headroom.
	shft = max(0, shft+3-clz32(nrg))
	nrg = 0
	for i = 0; i < length-1; i += 2 {
		nrgTmp = uint32(smulbb(int32(x[i]), int32(x[i])))
		nrgTmp = uint32(smlabbOvflw(int32(nrgTmp), int32(x[i+1]), int32(x[i+1])))
		nrg = int32(uint32(nrg) + nrgTmp>>shft)
	}
	if i < length {
		// One sample left to process
		nrgTmp = uint32(smulbb(int32(x[i]), int32(x[i])))
		nrg = int32(uint32(nrg) + nrgTmp>>shft)
	}

	// Output arguments
	return nrg, shft
}
