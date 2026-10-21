// Port of silk/LPC_fit.c.

package silk

// lpcFit is silk_LPC_fit: converts int32 coefficients to int16 coefs and
// makes sure there's no wrap-around.
func lpcFit(aQOUT []int16, aQIN []int32, QOUT, QIN, d int32) {
	var i, k, idx int32
	var maxabs, absval, chirpQ16 int32

	// Limit the maximum absolute value of the prediction coefficients, so that they'll fit in int16
	for i = 0; i < 10; i++ {
		// Find maximum absolute value and its index
		maxabs = 0
		for k = 0; k < d; k++ {
			absval = abs32(aQIN[k])
			if absval > maxabs {
				maxabs = absval
				idx = k
			}
		}
		maxabs = rshiftRound(maxabs, QIN-QOUT)

		if maxabs > silkInt16Max {
			// Reduce magnitude of prediction coefficients
			// ( silk_int32_MAX >> 14 ) + silk_int16_MAX = 163838
			maxabs = min(maxabs, 163838)
			// 65470 = SILK_FIX_CONST( 0.999, 16 )
			chirpQ16 = 65470 - ((maxabs-silkInt16Max)<<14)/((maxabs*(idx+1))>>2)
			bwexpander32(aQIN, d, chirpQ16)
		} else {
			break
		}
	}

	if i == 10 {
		// Reached the last iteration, clip the coefficients
		for k = 0; k < d; k++ {
			aQOUT[k] = int16(sat16(rshiftRound(aQIN[k], QIN-QOUT)))
			aQIN[k] = int32(aQOUT[k]) << (QIN - QOUT)
		}
	} else {
		for k = 0; k < d; k++ {
			aQOUT[k] = int16(rshiftRound(aQIN[k], QIN-QOUT))
		}
	}
}
