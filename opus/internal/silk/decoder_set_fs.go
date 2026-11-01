// Port of silk/decoder_set_fs.c.

package silk

// decoderSetFs is silk_decoder_set_fs: sets the decoder sampling rate, fsKHz
// 8, 12 or 16, and the API sampling rate in Hz.
func decoderSetFs(psDec *decoderState, fsKHz, fsAPIHz int32) int32 {
	var frameLength, ret int32

	// New (sub)frame length
	psDec.subfrLength = smulbb(subFrameLengthMs, fsKHz)
	frameLength = smulbb(psDec.nbSubfr, psDec.subfrLength)

	// Initialize resampler when switching internal or external sampling frequency
	if psDec.fsKHz != fsKHz || psDec.fsAPIHz != fsAPIHz {
		// Initialize the resampler for dec_API.c preparing resampling from fs_kHz to API_fs_Hz
		ret += resamplerInit(&psDec.resamplerState, smulbb(fsKHz, 1000), fsAPIHz)

		psDec.fsAPIHz = fsAPIHz
	}

	if psDec.fsKHz != fsKHz || frameLength != psDec.frameLength {
		if fsKHz == 8 {
			if psDec.nbSubfr == maxNbSubfr {
				psDec.pitchContourICDF = pitchContourNBICDF[:]
			} else {
				psDec.pitchContourICDF = pitchContour10MsNBICDF[:]
			}
		} else {
			if psDec.nbSubfr == maxNbSubfr {
				psDec.pitchContourICDF = pitchContourICDF[:]
			} else {
				psDec.pitchContourICDF = pitchContour10MsICDF[:]
			}
		}
		if psDec.fsKHz != fsKHz {
			psDec.ltpMemLength = smulbb(ltpMemLengthMs, fsKHz)
			if fsKHz == 8 || fsKHz == 12 {
				psDec.lpcOrder = minLPCOrder
				psDec.psNLSFCB = &nlsfCBNBMB
			} else {
				psDec.lpcOrder = maxLPCOrder
				psDec.psNLSFCB = &nlsfCBWB
			}
			if fsKHz == 16 {
				psDec.pitchLagLowBitsICDF = uniform8ICDF[:]
			} else if fsKHz == 12 {
				psDec.pitchLagLowBitsICDF = uniform6ICDF[:]
			} else if fsKHz == 8 {
				psDec.pitchLagLowBitsICDF = uniform4ICDF[:]
			}
			psDec.firstFrameAfterReset = 1
			psDec.lagPrev = 100
			psDec.lastGainIndex = 10
			psDec.prevSignalType = typeNoVoiceActivity
			clear(psDec.outBuf[:])
			clear(psDec.sLPCQ14Buf[:])
		}

		psDec.fsKHz = fsKHz
		psDec.frameLength = frameLength
	}

	return ret
}
