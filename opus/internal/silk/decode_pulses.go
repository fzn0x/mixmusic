// Port of silk/decode_pulses.c.

package silk

import "github.com/fzn0x/mixmusic/opus/internal/rangedec"

// decodePulses is silk_decode_pulses: decodes the quantization indices of the
// excitation; pulses must hold frameLength samples rounded up to a multiple
// of 16.
func decodePulses(psRangeDec *rangedec.Decoder, pulses []int16, signalType, quantOffsetType, frameLength int32) {
	var i, j, k, iter, absQ, nLS, RateLevelIndex int32
	var sumPulses, nLshifts [maxNbShellBlocks]int32

	/*********************/
	/* Decode rate level */
	/*********************/
	RateLevelIndex = int32(psRangeDec.ICDF(rateLevelsICDF[signalType>>1][:], 8))

	// Calculate number of shell blocks
	iter = frameLength >> log2ShellCodecFrameLength
	if iter*shellCodecFrameLength < frameLength {
		// Make sure only happens for 10 ms @ 12 kHz
		iter++
	}

	/***************************************************/
	/* Sum-Weighted-Pulses Decoding                    */
	/***************************************************/
	cdfPtr := pulsesPerBlockICDF[RateLevelIndex][:]
	for i = 0; i < iter; i++ {
		nLshifts[i] = 0
		sumPulses[i] = int32(psRangeDec.ICDF(cdfPtr, 8))

		// LSB indication
		for sumPulses[i] == silkMaxPulses+1 {
			nLshifts[i]++
			// When we've already got 10 LSBs, we shift the table to not allow (SILK_MAX_PULSES + 1)
			sumPulses[i] = int32(psRangeDec.ICDF(
				pulsesPerBlockICDF[nRateLevels-1][b2i(nLshifts[i] == 10):], 8))
		}
	}

	/***************************************************/
	/* Shell decoding                                  */
	/***************************************************/
	for i = 0; i < iter; i++ {
		if sumPulses[i] > 0 {
			shellDecoder(pulses[smulbb(i, shellCodecFrameLength):], psRangeDec, sumPulses[i])
		} else {
			clear(pulses[smulbb(i, shellCodecFrameLength):][:shellCodecFrameLength])
		}
	}

	/***************************************************/
	/* LSB Decoding                                    */
	/***************************************************/
	for i = 0; i < iter; i++ {
		if nLshifts[i] > 0 {
			nLS = nLshifts[i]
			pulsesPtr := pulses[smulbb(i, shellCodecFrameLength):]
			for k = 0; k < shellCodecFrameLength; k++ {
				absQ = int32(pulsesPtr[k])
				for j = 0; j < nLS; j++ {
					absQ = absQ << 1
					absQ += int32(psRangeDec.ICDF(lsbICDF[:], 8))
				}
				pulsesPtr[k] = int16(absQ)
			}
			// Mark the number of pulses non-zero for sign decoding.
			sumPulses[i] |= nLS << 5
		}
	}

	/****************************************/
	/* Decode and add signs to pulse signal */
	/****************************************/
	decodeSigns(psRangeDec, pulses, frameLength, signalType, quantOffsetType, sumPulses[:])
}
