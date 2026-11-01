// Port of silk/NLSF_stabilize.c.

package silk

// NLSF stabilizer:
//
//   - Moves NLSFs further apart if they are too close
//   - Moves NLSFs away from borders if they are too close
//   - High effort to achieve a modification with minimum
//     Euclidean distance to input vector
//   - Output are sorted NLSF coefficients

const nlsfStabilizeMaxLoops = 20 // MAX_LOOPS

// nlsfStabilize is silk_NLSF_stabilize, for a single input data vector of L
// values; NDeltaMinQ15 holds L+1 minimum distances, NDeltaMinQ15[L] >= 1.
func nlsfStabilize(NLSFQ15 []int16, NDeltaMinQ15 []int16, L int32) {
	var i, I, k, loops int32
	var centerFreqQ15 int16
	var diffQ15, minDiffQ15, minCenterQ15, maxCenterQ15 int32

	for loops = 0; loops < nlsfStabilizeMaxLoops; loops++ {
		/**************************/
		/* Find smallest distance */
		/**************************/
		// First element
		minDiffQ15 = int32(NLSFQ15[0]) - int32(NDeltaMinQ15[0])
		I = 0
		// Middle elements
		for i = 1; i <= L-1; i++ {
			diffQ15 = int32(NLSFQ15[i]) - (int32(NLSFQ15[i-1]) + int32(NDeltaMinQ15[i]))
			if diffQ15 < minDiffQ15 {
				minDiffQ15 = diffQ15
				I = i
			}
		}
		// Last element
		diffQ15 = (1 << 15) - (int32(NLSFQ15[L-1]) + int32(NDeltaMinQ15[L]))
		if diffQ15 < minDiffQ15 {
			minDiffQ15 = diffQ15
			I = L
		}

		/***************************************************/
		/* Now check if the smallest distance non-negative */
		/***************************************************/
		if minDiffQ15 >= 0 {
			return
		}

		if I == 0 {
			// Move away from lower limit
			NLSFQ15[0] = NDeltaMinQ15[0]

		} else if I == L {
			// Move away from higher limit
			NLSFQ15[L-1] = int16((1 << 15) - int32(NDeltaMinQ15[L]))

		} else {
			// Find the lower extreme for the location of the current center frequency
			minCenterQ15 = 0
			for k = 0; k < I; k++ {
				minCenterQ15 += int32(NDeltaMinQ15[k])
			}
			minCenterQ15 += int32(NDeltaMinQ15[I]) >> 1

			// Find the upper extreme for the location of the current center frequency
			maxCenterQ15 = 1 << 15
			for k = L; k > I; k-- {
				maxCenterQ15 -= int32(NDeltaMinQ15[k])
			}
			maxCenterQ15 -= int32(NDeltaMinQ15[I]) >> 1

			// Move apart, sorted by value, keeping the same center frequency
			centerFreqQ15 = int16(limit(rshiftRound(int32(NLSFQ15[I-1])+int32(NLSFQ15[I]), 1),
				minCenterQ15, maxCenterQ15))
			NLSFQ15[I-1] = int16(int32(centerFreqQ15) - int32(NDeltaMinQ15[I])>>1)
			NLSFQ15[I] = int16(int32(NLSFQ15[I-1]) + int32(NDeltaMinQ15[I]))
		}
	}

	// Safe and simple fall back method, which is less ideal than the above
	if loops == nlsfStabilizeMaxLoops {
		// Insertion sort (fast for already almost sorted arrays):
		// Best case:  O(n)   for an already sorted array
		// Worst case: O(n^2) for an inversely sorted array
		insertionSortIncreasingAllValuesInt16(NLSFQ15, L)

		// First NLSF should be no less than NDeltaMin[0]
		NLSFQ15[0] = int16(max(int32(NLSFQ15[0]), int32(NDeltaMinQ15[0])))

		// Keep delta_min distance between the NLSFs
		for i = 1; i < L; i++ {
			NLSFQ15[i] = int16(max(int32(NLSFQ15[i]), int32(addSat16(int32(NLSFQ15[i-1]), int32(NDeltaMinQ15[i])))))
		}

		// Last NLSF should be no higher than 1 - NDeltaMin[L]
		NLSFQ15[L-1] = int16(min(int32(NLSFQ15[L-1]), (1<<15)-int32(NDeltaMinQ15[L])))

		// Keep NDeltaMin distance between the NLSFs
		for i = L - 2; i >= 0; i-- {
			NLSFQ15[i] = int16(min(int32(NLSFQ15[i]), int32(NLSFQ15[i+1])-int32(NDeltaMinQ15[i+1])))
		}
	}
}
