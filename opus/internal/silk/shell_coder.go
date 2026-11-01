// Port of silk/shell_coder.c (the decoder; the pulse-subframe length is hardcoded).

package silk

import "github.com/fzn0x/mixmusic/opus/internal/rangedec"

// decodeSplit is decode_split: splits the p pulses of a subframe between its
// two child subframes.
func decodeSplit(pChild1, pChild2 *int16, psRangeDec *rangedec.Decoder, p int32, shellTable []uint8) {
	if p > 0 {
		*pChild1 = int16(psRangeDec.ICDF(shellTable[shellCodeTableOffsets[p]:], 8))
		*pChild2 = int16(p - int32(*pChild1))
	} else {
		*pChild1 = 0
		*pChild2 = 0
	}
}

// shellDecoder is silk_shell_decoder: decodes the nonnegative pulse
// amplitudes of one shell code frame of 16 pulses, pulses4 in all.
func shellDecoder(pulses0 []int16, psRangeDec *rangedec.Decoder, pulses4 int32) {
	var pulses3 [2]int16
	var pulses2 [4]int16
	var pulses1 [8]int16

	// this function operates on one shell code frame of 16 pulses
	decodeSplit(&pulses3[0], &pulses3[1], psRangeDec, pulses4, shellCodeTable3[:])

	decodeSplit(&pulses2[0], &pulses2[1], psRangeDec, int32(pulses3[0]), shellCodeTable2[:])

	decodeSplit(&pulses1[0], &pulses1[1], psRangeDec, int32(pulses2[0]), shellCodeTable1[:])
	decodeSplit(&pulses0[0], &pulses0[1], psRangeDec, int32(pulses1[0]), shellCodeTable0[:])
	decodeSplit(&pulses0[2], &pulses0[3], psRangeDec, int32(pulses1[1]), shellCodeTable0[:])

	decodeSplit(&pulses1[2], &pulses1[3], psRangeDec, int32(pulses2[1]), shellCodeTable1[:])
	decodeSplit(&pulses0[4], &pulses0[5], psRangeDec, int32(pulses1[2]), shellCodeTable0[:])
	decodeSplit(&pulses0[6], &pulses0[7], psRangeDec, int32(pulses1[3]), shellCodeTable0[:])

	decodeSplit(&pulses2[2], &pulses2[3], psRangeDec, int32(pulses3[1]), shellCodeTable2[:])

	decodeSplit(&pulses1[4], &pulses1[5], psRangeDec, int32(pulses2[2]), shellCodeTable1[:])
	decodeSplit(&pulses0[8], &pulses0[9], psRangeDec, int32(pulses1[4]), shellCodeTable0[:])
	decodeSplit(&pulses0[10], &pulses0[11], psRangeDec, int32(pulses1[5]), shellCodeTable0[:])

	decodeSplit(&pulses1[6], &pulses1[7], psRangeDec, int32(pulses2[3]), shellCodeTable1[:])
	decodeSplit(&pulses0[12], &pulses0[13], psRangeDec, int32(pulses1[6]), shellCodeTable0[:])
	decodeSplit(&pulses0[14], &pulses0[15], psRangeDec, int32(pulses1[7]), shellCodeTable0[:])
}
