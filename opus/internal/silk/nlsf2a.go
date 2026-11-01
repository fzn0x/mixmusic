// Port of silk/NLSF2A.c.

package silk

// Conversion between prediction filter coefficients and LSFs. The order
// should be even. A piecewise linear approximation maps LSF <-> cos(LSF),
// therefore the result is not accurate LSFs, but the two functions are
// accurate inverses of each other.

const nlsf2AQA = 16 // QA

// This ordering was found to maximize quality. It improves numerical accuracy
// of silk_NLSF2A_find_poly() compared to "standard" ordering.
var (
	nlsf2AOrdering16 = [16]uint8{
		0, 15, 8, 7, 4, 11, 12, 3, 2, 13, 10, 5, 6, 9, 14, 1,
	}
	nlsf2AOrdering10 = [10]uint8{
		0, 9, 6, 3, 4, 5, 8, 1, 2, 7,
	}
)

// nlsf2AFindPoly is silk_NLSF2A_find_poly: the intermediate polynomial, QA
// [dd+1], of the interleaved 2*cos(LSFs) cLSF, QA [d].
func nlsf2AFindPoly(out []int32, cLSF []int32, dd int32) {
	var k, n int32
	var ftmp int32

	out[0] = 1 << nlsf2AQA
	out[1] = -cLSF[0]
	for k = 1; k < dd; k++ {
		ftmp = cLSF[2*k] // QA
		out[k+1] = (out[k-1] << 1) - int32(rshiftRound64(smull(ftmp, out[k]), nlsf2AQA))
		for n = k; n > 1; n-- {
			out[n] += out[n-2] - int32(rshiftRound64(smull(ftmp, out[n-1]), nlsf2AQA))
		}
		out[1] -= ftmp
	}
}

// nlsf2A is silk_NLSF2A: the monic whitening filter coefficients in Q12, [d],
// of the normalized line spectral frequencies NLSF in Q15, [d].
func nlsf2A(aQ12 []int16, NLSF []int16, d int32) {
	var ordering []uint8
	var k, i, dd int32
	var cosLSFQA [silkMaxOrderLPC]int32
	var P, Q [silkMaxOrderLPC/2 + 1]int32
	var Ptmp, Qtmp, fInt, fFrac, cosVal, delta int32
	var a32QA1 [silkMaxOrderLPC]int32

	// convert LSFs to 2*cos(LSF), using piecewise linear curve from table
	if d == 16 {
		ordering = nlsf2AOrdering16[:]
	} else {
		ordering = nlsf2AOrdering10[:]
	}
	for k = 0; k < d; k++ {
		// f_int on a scale 0-127 (rounded down)
		fInt = int32(NLSF[k]) >> (15 - 7)

		// f_frac, range: 0..255
		fFrac = int32(NLSF[k]) - fInt<<(15-7)

		// Read start and end value from table
		cosVal = int32(lsfCosTabFIXQ12[fInt])           // Q12
		delta = int32(lsfCosTabFIXQ12[fInt+1]) - cosVal // Q12, with a range of 0..200

		// Linear interpolation
		cosLSFQA[ordering[k]] = rshiftRound(cosVal<<8+delta*fFrac, 20-nlsf2AQA) // QA
	}

	dd = d >> 1

	// generate even and odd polynomials using convolution
	nlsf2AFindPoly(P[:], cosLSFQA[0:], dd)
	nlsf2AFindPoly(Q[:], cosLSFQA[1:], dd)

	// convert even and odd polynomials to opus_int32 Q12 filter coefs
	for k = 0; k < dd; k++ {
		Ptmp = P[k+1] + P[k]
		Qtmp = Q[k+1] - Q[k]

		// the Ptmp and Qtmp values at this stage need to fit in int32
		a32QA1[k] = -Qtmp - Ptmp    // QA+1
		a32QA1[d-k-1] = Qtmp - Ptmp // QA+1
	}

	// Convert int32 coefficients to Q12 int16 coefs
	lpcFit(aQ12, a32QA1[:], 12, nlsf2AQA+1, d)

	for i = 0; lpcInversePredGain(aQ12, d) == 0 && i < maxLPCStabilizeIterations; i++ {
		// Prediction coefficients are (too close to) unstable; apply bandwidth expansion
		// on the unscaled coefficients, convert to Q12 and measure again
		bwexpander32(a32QA1[:], d, 65536-int32(2)<<i)
		for k = 0; k < d; k++ {
			aQ12[k] = int16(rshiftRound(a32QA1[k], nlsf2AQA+1-12)) // QA+1 -> Q12
		}
	}
}
