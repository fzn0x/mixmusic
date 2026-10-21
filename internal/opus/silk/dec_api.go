// Port of silk/dec_API.c (without ENABLE_OSCE, ENABLE_OSCE_BWE or ENABLE_DEEP_PLC).

package silk

import (
	"errors"

	"github.com/fzn0x/mixmusic/internal/opus/rangedec"
)

// Errors of misuse, where the C code has undefined behaviour.
var (
	errChannels    = errors.New("silk: number of channels not 1 or 2")
	errFrames      = errors.New("silk: more frames than a packet holds")
	errShortBuffer = errors.New("silk: output buffer too small")
)

// Decoder is silk_decoder, the decoder super struct: the state of a SILK
// decoder of up to two channels. Init must be called before use.
type Decoder struct {
	channelState         [decoderNumChannels]decoderState
	sStereo              stereoDecState
	nChannelsAPI         int32
	nChannelsInternal    int32
	prevDecodeOnlyMiddle int32

	// Scratch of Decode (stack arrays in C).
	samplesOut1Tmp [2][maxFrameLength + 2]int16
	samplesOut2Tmp [maxAPIFsKHz * maxFrameLengthMs]int16
}

// errorOf converts a SILK error code, or a sum of them, to an error.
func errorOf(ret int32) error {
	if ret == 0 {
		return nil
	}
	return Error(ret)
}

// Reset is silk_ResetDecoder: it resets the decoder state, as an Opus
// decoder's OPUS_RESET_STATE and a switch from CELT do. Like the C code, it
// keeps the channel counts of the last call.
func (d *Decoder) Reset() error {
	var ret int32
	channelState := &d.channelState

	for n := range channelState {
		ret = resetDecoder(&channelState[n])
	}
	d.sStereo = stereoDecState{}
	// Not strictly needed, but it's cleaner that way
	d.prevDecodeOnlyMiddle = 0

	return errorOf(ret)
}

// Init is silk_InitDecoder, on a decoder cleared as opus_decoder_init clears
// its memory first: it initializes d.
func (d *Decoder) Init() error {
	var ret int32
	*d = Decoder{}
	channelState := &d.channelState

	for n := range channelState {
		ret = initDecoder(&channelState[n])
	}
	d.sStereo = stereoDecState{}
	// Not strictly needed, but it's cleaner that way
	d.prevDecodeOnlyMiddle = 0

	return errorOf(ret)
}

// Decode is silk_Decode: it decodes a frame of the packet that dec reads.
// lostFlag is 0 to decode, 1 to conceal a lost frame (dec is not read), or 2
// to decode the frame's in-band FEC (LBRR) data; newPacket is true for the
// first call of a packet. It writes nSamplesOut samples per channel to out,
// interleaved when ctl.ChannelsAPI is 2: out must hold a frame of 20 ms at
// ctl.APISampleRate for each channel. It returns the SILK error codes of the
// C code as Error values, with the samples when the C code wrote them too.
func (d *Decoder) Decode(ctl *DecControl, lostFlag int, newPacket bool, dec *rangedec.Decoder, out []float32) (nSamplesOut int, err error) {
	var i, n, decodeOnlyMiddle, ret int32
	var nSamplesOutDec, LBRRSymbol int32
	var samplesOut1Tmp [2][]int16
	var MSPredQ13 [2]int32
	var resampleOutPtr []int16
	psDec := d
	channelState := &psDec.channelState
	var hasSide, stereoToMono bool
	decControl := ctl
	flag := int32(lostFlag)

	// celt_assert( decControl->nChannelsInternal == 1 || decControl->nChannelsInternal == 2 )
	if decControl.ChannelsInternal != 1 && decControl.ChannelsInternal != 2 ||
		decControl.ChannelsAPI != 1 && decControl.ChannelsAPI != 2 {
		return 0, errChannels
	}

	/**********************************/
	/* Test if first frame in payload */
	/**********************************/
	if newPacket {
		for n = 0; n < decControl.ChannelsInternal; n++ {
			channelState[n].nFramesDecoded = 0 // Used to count frames in packet
		}
	}
	// Not in C: more calls than frames in a packet would read past the flags.
	if channelState[0].nFramesDecoded >= maxFramesPerPacket {
		return 0, errFrames
	}

	// If Mono -> Stereo transition in bitstream: init state of second channel
	if decControl.ChannelsInternal > psDec.nChannelsInternal {
		ret += initDecoder(&channelState[1])
	}

	stereoToMono = decControl.ChannelsInternal == 1 && psDec.nChannelsInternal == 2 &&
		(decControl.InternalSampleRate == 1000*channelState[0].fsKHz)

	if channelState[0].nFramesDecoded == 0 {
		for n = 0; n < decControl.ChannelsInternal; n++ {
			var fsKHzDec int32
			if decControl.PayloadSizeMs == 0 {
				// Assuming packet loss, use 10 ms
				channelState[n].nFramesPerPacket = 1
				channelState[n].nbSubfr = 2
			} else if decControl.PayloadSizeMs == 10 {
				channelState[n].nFramesPerPacket = 1
				channelState[n].nbSubfr = 2
			} else if decControl.PayloadSizeMs == 20 {
				channelState[n].nFramesPerPacket = 1
				channelState[n].nbSubfr = 4
			} else if decControl.PayloadSizeMs == 40 {
				channelState[n].nFramesPerPacket = 2
				channelState[n].nbSubfr = 4
			} else if decControl.PayloadSizeMs == 60 {
				channelState[n].nFramesPerPacket = 3
				channelState[n].nbSubfr = 4
			} else {
				return 0, ErrInvalidFrameSize
			}
			fsKHzDec = (decControl.InternalSampleRate >> 10) + 1
			if fsKHzDec != 8 && fsKHzDec != 12 && fsKHzDec != 16 {
				return 0, ErrInvalidSamplingFrequency
			}
			ret += decoderSetFs(&channelState[n], fsKHzDec, decControl.APISampleRate)
		}
	}

	if decControl.ChannelsAPI == 2 && decControl.ChannelsInternal == 2 && (psDec.nChannelsAPI == 1 || psDec.nChannelsInternal == 1) {
		psDec.sStereo.predPrevQ13 = [2]int16{}
		psDec.sStereo.sSide = [2]int16{}
		channelState[1].resamplerState = channelState[0].resamplerState
	}
	psDec.nChannelsAPI = decControl.ChannelsAPI
	psDec.nChannelsInternal = decControl.ChannelsInternal

	if decControl.APISampleRate > maxAPIFsKHz*1000 || decControl.APISampleRate < 8000 {
		ret = int32(ErrInvalidSamplingFrequency)
		return 0, errorOf(ret)
	}

	// Not in C: check the output fits before decoding anything.
	if int64(len(out)) < int64(channelState[0].frameLength)*int64(decControl.APISampleRate)/int64(smulbb(channelState[0].fsKHz, 1000))*int64(decControl.ChannelsAPI) {
		return 0, errShortBuffer
	}

	if flag != flagPacketLost && channelState[0].nFramesDecoded == 0 {
		// First decoder call for this payload
		// Decode VAD flags and LBRR flag
		for n = 0; n < decControl.ChannelsInternal; n++ {
			for i = 0; i < channelState[n].nFramesPerPacket; i++ {
				channelState[n].vadFlags[i] = b2i(dec.BitLogp(1))
			}
			channelState[n].lbrrFlag = b2i(dec.BitLogp(1))
		}
		// Decode LBRR flags
		for n = 0; n < decControl.ChannelsInternal; n++ {
			channelState[n].lbrrFlags = [maxFramesPerPacket]int32{}
			if channelState[n].lbrrFlag != 0 {
				if channelState[n].nFramesPerPacket == 1 {
					channelState[n].lbrrFlags[0] = 1
				} else {
					LBRRSymbol = int32(dec.ICDF(lbrrFlagsICDFPtr[channelState[n].nFramesPerPacket-2], 8)) + 1
					for i = 0; i < channelState[n].nFramesPerPacket; i++ {
						channelState[n].lbrrFlags[i] = (LBRRSymbol >> i) & 1
					}
				}
			}
		}

		if flag == flagDecodeNormal {
			// Regular decoding: skip all LBRR data
			for i = 0; i < channelState[0].nFramesPerPacket; i++ {
				for n = 0; n < decControl.ChannelsInternal; n++ {
					if channelState[n].lbrrFlags[i] != 0 {
						var pulses [maxFrameLength]int16
						var condCoding int32

						if decControl.ChannelsInternal == 2 && n == 0 {
							stereoDecodePred(dec, MSPredQ13[:])
							if channelState[1].lbrrFlags[i] == 0 {
								stereoDecodeMidOnly(dec, &decodeOnlyMiddle)
							}
						}
						// Use conditional coding if previous frame available
						if i > 0 && channelState[n].lbrrFlags[i-1] != 0 {
							condCoding = codeConditionally
						} else {
							condCoding = codeIndependently
						}
						decodeIndices(&channelState[n], dec, i, 1, condCoding)
						decodePulses(dec, pulses[:], int32(channelState[n].indices.signalType),
							int32(channelState[n].indices.quantOffsetType), channelState[n].frameLength)
					}
				}
			}
		}
	}

	// Get MS predictor index
	if decControl.ChannelsInternal == 2 {
		if flag == flagDecodeNormal ||
			(flag == flagDecodeLBRR && channelState[0].lbrrFlags[channelState[0].nFramesDecoded] == 1) {
			stereoDecodePred(dec, MSPredQ13[:])
			// For LBRR data, decode mid-only flag only if side-channel's LBRR flag is false
			if (flag == flagDecodeNormal && channelState[1].vadFlags[channelState[0].nFramesDecoded] == 0) ||
				(flag == flagDecodeLBRR && channelState[1].lbrrFlags[channelState[0].nFramesDecoded] == 0) {
				stereoDecodeMidOnly(dec, &decodeOnlyMiddle)
			} else {
				decodeOnlyMiddle = 0
			}
		} else {
			for n = 0; n < 2; n++ {
				MSPredQ13[n] = int32(psDec.sStereo.predPrevQ13[n])
			}
		}
	}

	// Reset side channel decoder prediction memory for first frame with side coding
	if decControl.ChannelsInternal == 2 && decodeOnlyMiddle == 0 && psDec.prevDecodeOnlyMiddle == 1 {
		clear(psDec.channelState[1].outBuf[:])
		clear(psDec.channelState[1].sLPCQ14Buf[:])
		psDec.channelState[1].lagPrev = 100
		psDec.channelState[1].lastGainIndex = 10
		psDec.channelState[1].prevSignalType = typeNoVoiceActivity
		psDec.channelState[1].firstFrameAfterReset = 1
	}

	// samplesOut1_tmp: frame_length + 2 samples per channel
	samplesOut1Tmp[0] = psDec.samplesOut1Tmp[0][:]
	samplesOut1Tmp[1] = psDec.samplesOut1Tmp[1][:]

	if flag == flagDecodeNormal {
		hasSide = decodeOnlyMiddle == 0
	} else {
		hasSide = psDec.prevDecodeOnlyMiddle == 0 ||
			(decControl.ChannelsInternal == 2 && flag == flagDecodeLBRR && channelState[1].lbrrFlags[channelState[1].nFramesDecoded] == 1)
	}
	// channel_state[ 0 ].sPLC.enable_deep_plc only matters with ENABLE_DEEP_PLC.

	// Call decoder for one frame
	for n = 0; n < decControl.ChannelsInternal; n++ {
		if n == 0 || hasSide {
			var FrameIndex, condCoding, r int32

			FrameIndex = channelState[0].nFramesDecoded - n
			// Use independent coding if no previous frame available
			if FrameIndex <= 0 {
				condCoding = codeIndependently
			} else if flag == flagDecodeLBRR {
				if channelState[n].lbrrFlags[FrameIndex-1] != 0 {
					condCoding = codeConditionally
				} else {
					condCoding = codeIndependently
				}
			} else if n > 0 && psDec.prevDecodeOnlyMiddle != 0 {
				// If we skipped a side frame in this packet, we don't
				// need LTP scaling; the LTP state is well-defined.
				condCoding = codeIndependentlyNoLTPScaling
			} else {
				condCoding = codeConditionally
			}
			nSamplesOutDec, r = decodeFrame(&channelState[n], dec, samplesOut1Tmp[n][2:], flag, condCoding)
			ret += r
		} else {
			clear(samplesOut1Tmp[n][2 : 2+nSamplesOutDec])
		}
		channelState[n].nFramesDecoded++
	}

	if decControl.ChannelsAPI == 2 && decControl.ChannelsInternal == 2 {
		// Convert Mid/Side to Left/Right
		stereoMSToLR(&psDec.sStereo, samplesOut1Tmp[0], samplesOut1Tmp[1], MSPredQ13[:], channelState[0].fsKHz, nSamplesOutDec)
	} else {
		// Buffering
		copy(samplesOut1Tmp[0][:2], psDec.sStereo.sMid[:])
		copy(psDec.sStereo.sMid[:], samplesOut1Tmp[0][nSamplesOutDec:])
	}

	// Number of output samples
	nSamplesOut32 := (nSamplesOutDec * decControl.APISampleRate) / smulbb(channelState[0].fsKHz, 1000)

	// Set up pointers to temp buffers
	resampleOutPtr = psDec.samplesOut2Tmp[:nSamplesOut32]

	for n = 0; n < min(decControl.ChannelsAPI, decControl.ChannelsInternal); n++ {
		// Resample decoded signal to API_sampleRate
		ret += resampler(&channelState[n].resamplerState, resampleOutPtr, samplesOut1Tmp[n][1:], nSamplesOutDec)

		// Interleave if stereo output and stereo stream
		if decControl.ChannelsAPI == 2 {
			for i = 0; i < nSamplesOut32; i++ {
				out[n+2*i] = int16ToRes(resampleOutPtr[i])
			}
		} else {
			for i = 0; i < nSamplesOut32; i++ {
				out[i] = int16ToRes(resampleOutPtr[i])
			}
		}
	}

	// Create two channel output from mono stream
	if decControl.ChannelsAPI == 2 && decControl.ChannelsInternal == 1 {
		if stereoToMono {
			// Resample right channel for newly collapsed stereo just in case
			// we weren't doing collapsing when switching to mono
			ret += resampler(&channelState[1].resamplerState, resampleOutPtr, samplesOut1Tmp[0][1:], nSamplesOutDec)

			for i = 0; i < nSamplesOut32; i++ {
				out[1+2*i] = int16ToRes(resampleOutPtr[i])
			}
		} else {
			for i = 0; i < nSamplesOut32; i++ {
				out[1+2*i] = out[0+2*i]
			}
		}
	}

	// Export pitch lag, measured at 48 kHz sampling rate
	if channelState[0].prevSignalType == typeVoiced {
		multTab := [3]int32{6, 4, 3}
		decControl.PrevPitchLag = int(channelState[0].lagPrev * multTab[(channelState[0].fsKHz-8)>>2])
	} else {
		decControl.PrevPitchLag = 0
	}

	if flag == flagPacketLost {
		// On packet loss, remove the gain clamping to prevent having the energy "bounce back"
		// if we lose packets when the energy is going down
		for i = 0; i < psDec.nChannelsInternal; i++ {
			psDec.channelState[i].lastGainIndex = 10
		}
	} else {
		psDec.prevDecodeOnlyMiddle = decodeOnlyMiddle
	}
	return int(nSamplesOut32), errorOf(ret)
}

// int16ToRes is INT16TORES of the float build: (a)*(1/CELT_SIG_SCALE).
func int16ToRes(a int16) float32 {
	return float32(a) * (1 / float32(32768))
}
