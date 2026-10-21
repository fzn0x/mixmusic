// Port of silk/bwexpander.c and silk/bwexpander_32.c.

package silk

// bwexpander is silk_bwexpander: chirp (bandwidth expand) LP AR filter.
func bwexpander(ar []int16, d, chirpQ16 int32) {
	chirpMinusOneQ16 := chirpQ16 - 65536

	// NB: Dont use silk_SMULWB, instead of silk_RSHIFT_ROUND( silk_MUL(), 16 ), below.
	// Bias in silk_SMULWB can lead to unstable filters
	for i := int32(0); i < d-1; i++ {
		ar[i] = int16(rshiftRound(chirpQ16*int32(ar[i]), 16))
		chirpQ16 += rshiftRound(chirpQ16*chirpMinusOneQ16, 16)
	}
	ar[d-1] = int16(rshiftRound(chirpQ16*int32(ar[d-1]), 16))
}

// bwexpander32 is silk_bwexpander_32: chirp (bandwidth expand) LP AR filter.
func bwexpander32(ar []int32, d, chirpQ16 int32) {
	chirpMinusOneQ16 := chirpQ16 - 65536

	for i := int32(0); i < d-1; i++ {
		ar[i] = smulww(chirpQ16, ar[i])
		chirpQ16 += rshiftRound(chirpQ16*chirpMinusOneQ16, 16)
	}
	ar[d-1] = smulww(chirpQ16, ar[d-1])
}
