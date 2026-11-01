// Port of silk/decode_frame.c.

package silk

import "github.com/fzn0x/mixmusic/opus/internal/rangedec"

// decodeFrame is silk_decode_frame: decodes one frame into pOut, or conceals
// it, and returns its length (*pN in C) and an error code.
func decodeFrame(psDec *decoderState, psRangeDec *rangedec.Decoder, pOut []int16, lostFlag, condCoding int32) (pN, ret int32) {
	var psDecCtrl decoderControl
	var L, mvLen int32

	L = psDec.frameLength
	psDecCtrl.ltpScaleQ14 = 0

	if lostFlag == flagDecodeNormal ||
		(lostFlag == flagDecodeLBRR && psDec.lbrrFlags[psDec.nFramesDecoded] == 1) {
		pulses := psDec.scratch.pulses[:(L+shellCodecFrameLength-1)&^(shellCodecFrameLength-1)]
		/*********************************************/
		/* Decode quantization indices of side info  */
		/*********************************************/
		decodeIndices(psDec, psRangeDec, psDec.nFramesDecoded, lostFlag, condCoding)

		/*********************************************/
		/* Decode quantization indices of excitation */
		/*********************************************/
		decodePulses(psRangeDec, pulses, int32(psDec.indices.signalType),
			int32(psDec.indices.quantOffsetType), psDec.frameLength)

		/********************************************/
		/* Decode parameters and pulse signal       */
		/********************************************/
		decodeParameters(psDec, &psDecCtrl, condCoding)

		/********************************************************/
		/* Run inverse NSQ                                      */
		/********************************************************/
		decodeCore(psDec, &psDecCtrl, pOut, pulses)

		/*************************/
		/* Update output buffer. */
		/*************************/
		mvLen = psDec.ltpMemLength - psDec.frameLength
		copy(psDec.outBuf[:mvLen], psDec.outBuf[psDec.frameLength:])
		copy(psDec.outBuf[mvLen:mvLen+psDec.frameLength], pOut[:psDec.frameLength])

		/********************************************************/
		/* Update PLC state                                     */
		/********************************************************/
		plc(psDec, &psDecCtrl, pOut, 0)

		psDec.lossCnt = 0
		psDec.prevSignalType = int32(psDec.indices.signalType)

		// A frame has been decoded without errors
		psDec.firstFrameAfterReset = 0
	} else {
		// Handle packet loss by extrapolation
		plc(psDec, &psDecCtrl, pOut, 1)

		/*************************/
		/* Update output buffer. */
		/*************************/
		mvLen = psDec.ltpMemLength - psDec.frameLength
		copy(psDec.outBuf[:mvLen], psDec.outBuf[psDec.frameLength:])
		copy(psDec.outBuf[mvLen:mvLen+psDec.frameLength], pOut[:psDec.frameLength])
	}

	/************************************************/
	/* Comfort noise generation / estimation        */
	/************************************************/
	cng(psDec, &psDecCtrl, pOut, L)

	/****************************************************************/
	/* Ensure smooth connection of extrapolated and good frames     */
	/****************************************************************/
	plcGlueFrames(psDec, pOut, L)

	// Update some decoder state variables
	psDec.lagPrev = psDecCtrl.pitchL[psDec.nbSubfr-1]

	// Set output frame length
	return L, ret
}
