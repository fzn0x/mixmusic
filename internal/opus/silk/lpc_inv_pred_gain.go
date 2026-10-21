// Port of silk/LPC_inv_pred_gain.c (the generic C functions).

package silk

const (
	invPredGainQA = 24
	aLimit        = 16773022 // SILK_FIX_CONST( 0.99975, QA )

	// invMaxPredictionPowerGainQ30 is
	// SILK_FIX_CONST( 1.0f / MAX_PREDICTION_POWER_GAIN, 30 ), MAX_PREDICTION_POWER_GAIN 1e4f.
	invMaxPredictionPowerGainQ30 = 107374
)

// mul32FracQ is MUL32_FRAC_Q.
func mul32FracQ(a32, b32, Q int32) int32 {
	return int32(rshiftRound64(smull(a32, b32), Q))
}

// lpcInversePredGainQA is LPC_inverse_pred_gain_QA_c: the inverse of the LPC
// prediction gain in energy domain, Q30, and 0 if the LPC coefficients are
// not stable (all poles within unit circle).
func lpcInversePredGainQA(AQA *[silkMaxOrderLPC]int32, order int32) int32 {
	var k, n, mult2Q int32
	var invGainQ30, rcQ31, rcMult1Q30, rcMult2, tmp1, tmp2 int32

	invGainQ30 = 1 << 30
	for k = order - 1; k > 0; k-- {
		// Check for stability
		if (AQA[k] > aLimit) || (AQA[k] < -aLimit) {
			return 0
		}

		// Set RC equal to negated AR coef
		rcQ31 = -(AQA[k] << (31 - invPredGainQA))

		// rc_mult1_Q30 range: [ 1 : 2^30 ]
		rcMult1Q30 = (1 << 30) - smmul(rcQ31, rcQ31)

		// Update inverse gain
		// invGain_Q30 range: [ 0 : 2^30 ]
		invGainQ30 = smmul(invGainQ30, rcMult1Q30) << 2
		if invGainQ30 < invMaxPredictionPowerGainQ30 {
			return 0
		}

		// rc_mult2 range: [ 2^30 : silk_int32_MAX ]
		mult2Q = 32 - clz32(abs32(rcMult1Q30))
		rcMult2 = inverse32VarQ(rcMult1Q30, mult2Q+30)

		// Update AR coefficient
		for n = 0; n < (k+1)>>1; n++ {
			var tmp64 int64
			tmp1 = AQA[n]
			tmp2 = AQA[k-n-1]
			tmp64 = rshiftRound64(smull(subSat32(tmp1,
				mul32FracQ(tmp2, rcQ31, 31)), rcMult2), mult2Q)
			if tmp64 > int64(silkInt32Max) || tmp64 < int64(silkInt32Min) {
				return 0
			}
			AQA[n] = int32(tmp64)
			tmp64 = rshiftRound64(smull(subSat32(tmp2,
				mul32FracQ(tmp1, rcQ31, 31)), rcMult2), mult2Q)
			if tmp64 > int64(silkInt32Max) || tmp64 < int64(silkInt32Min) {
				return 0
			}
			AQA[k-n-1] = int32(tmp64)
		}
	}

	// Check for stability
	if (AQA[k] > aLimit) || (AQA[k] < -aLimit) {
		return 0
	}

	// Set RC equal to negated AR coef
	rcQ31 = -(AQA[0] << (31 - invPredGainQA))

	// Range: [ 1 : 2^30 ]
	rcMult1Q30 = (1 << 30) - smmul(rcQ31, rcQ31)

	// Update inverse gain
	// Range: [ 0 : 2^30 ]
	invGainQ30 = smmul(invGainQ30, rcMult1Q30) << 2
	if invGainQ30 < invMaxPredictionPowerGainQ30 {
		return 0
	}

	return invGainQ30
}

// lpcInversePredGain is silk_LPC_inverse_pred_gain_c, for input in Q12: the
// inverse of the LPC prediction gain in energy domain, Q30.
func lpcInversePredGain(AQ12 []int16, order int32) int32 {
	var AtmpQA [silkMaxOrderLPC]int32
	var DCResp int32

	// Increase Q domain of the AR coefficients
	for k := int32(0); k < order; k++ {
		DCResp += int32(AQ12[k])
		AtmpQA[k] = int32(AQ12[k]) << (invPredGainQA - 12)
	}
	// If the DC is unstable, we don't even need to do the full calculations
	if DCResp >= 4096 {
		return 0
	}
	return lpcInversePredGainQA(&AtmpQA, order)
}
