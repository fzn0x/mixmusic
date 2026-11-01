// Port of silk/init_decoder.c (without ENABLE_OSCE).

package silk

// resetDecoder is silk_reset_decoder. Without OSCE,
// SILK_DECODER_STATE_RESET_START is the first field, so it clears the whole
// state.
func resetDecoder(psDec *decoderState) int32 {
	// Clear the entire encoder state, except anything copied
	*psDec = decoderState{}

	// Used to deactivate LSF interpolation
	psDec.firstFrameAfterReset = 1
	psDec.prevGainQ16 = 65536

	// Reset CNG state
	cngReset(psDec)

	// Reset PLC state
	plcReset(psDec)

	return 0
}

// initDecoder is silk_init_decoder.
func initDecoder(psDec *decoderState) int32 {
	// Clear the entire encoder state, except anything copied
	*psDec = decoderState{}

	resetDecoder(psDec)

	return 0
}
