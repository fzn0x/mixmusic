// Port of silk/code_signs.c (silk_decode_signs, the part the decoder uses).

package silk

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

// decMap is silk_dec_map: shifting avoids if-statement.
func decMap(a int32) int32 {
	return (a << 1) - 1
}

// decodeSigns is silk_decode_signs: decodes the signs of the excitation and
// attaches them to pulses.
func decodeSigns(psRangeDec *rangedec.Decoder, pulses []int16, length, signalType, quantOffsetType int32, sumPulses []int32) {
	var i, j, p int32
	var icdf [2]uint8

	icdf[1] = 0
	qPtr := pulses
	i = smulbb(7, quantOffsetType+signalType<<1)
	icdfPtr := signICDF[i:]
	length = (length + shellCodecFrameLength/2) >> log2ShellCodecFrameLength
	for i = 0; i < length; i++ {
		p = sumPulses[i]
		if p > 0 {
			icdf[0] = icdfPtr[min(p&0x1F, 6)]
			for j = 0; j < shellCodecFrameLength; j++ {
				if qPtr[j] > 0 {
					// attach sign
					// implementation with shift, subtraction, multiplication
					qPtr[j] = int16(int32(qPtr[j]) * decMap(int32(psRangeDec.ICDF(icdf[:], 8))))
				}
			}
		}
		qPtr = qPtr[shellCodecFrameLength:]
	}
}
