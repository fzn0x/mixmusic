// Port of silk/resampler.c and silk/resampler_private.h, for the decoder (forEnc = 0).

package silk

/*
 * Matrix of resampling methods used:
 *                                 Fs_out (kHz)
 *                        8      12     16     24     48
 *
 *               8        C      UF     U      UF     UF
 *              12        AF     C      UF     U      UF
 * Fs_in (kHz)  16        D      AF     C      UF     UF
 *              24        AF     D      AF     C      U
 *              48        AF     AF     AF     D      C
 *
 * C   -> Copy (no resampling)
 * D   -> Allpass-based 2x downsampling
 * U   -> Allpass-based 2x upsampling
 * UF  -> Allpass-based 2x upsampling followed by FIR interpolation
 * AF  -> AR2 filter followed by FIR interpolation
 */

// Number of input samples to process in the inner loop.
const (
	resamplerMaxBatchSizeMs = 10
	resamplerMaxFsKHz       = 48
	resamplerMaxBatchSizeIn = resamplerMaxBatchSizeMs * resamplerMaxFsKHz
)

// delayMatrixDec is delay_matrix_dec: delay compensation values to equalize
// total delay for different modes.
var delayMatrixDec = [3][6]int8{
	/* in  \ out  8  12  16  24  48  96*/
	/*  8 */ {4, 0, 2, 0, 0, 0},
	/* 12 */ {0, 9, 4, 7, 4, 4},
	/* 16 */ {0, 3, 12, 7, 7, 7},
}

// rateID maps [8000, 12000, 16000, 24000, 48000] to [0, 1, 2, 3, 4].
func rateID(R int32) int32 {
	return min(5, (((R>>12)-b2i(R > 16000))>>b2i(R > 24000))-1)
}

// b2i is a C comparison's value.
func b2i(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

const (
	useSilkResamplerCopy                = 0
	useSilkResamplerPrivateUp2HQWrapper = 1
	useSilkResamplerPrivateIIRFIR       = 2
	useSilkResamplerPrivateDownFIR      = 3
)

// resamplerInit is silk_resampler_init with forEnc = 0: initializes or resets
// the resampler state for a given pair of input/output sampling rates.
func resamplerInit(S *resamplerStateStruct, FsHzIn, FsHzOut int32) int32 {
	var up2x int32

	// Clear state
	*S = resamplerStateStruct{}

	// Input checking
	if (FsHzIn != 8000 && FsHzIn != 12000 && FsHzIn != 16000) ||
		(FsHzOut != 8000 && FsHzOut != 12000 && FsHzOut != 16000 && FsHzOut != 24000 && FsHzOut != 48000) {
		return -1
	}
	S.inputDelay = int32(delayMatrixDec[rateID(FsHzIn)][rateID(FsHzOut)])

	S.fsInKHz = FsHzIn / 1000
	S.fsOutKHz = FsHzOut / 1000

	// Number of samples processed per batch
	S.batchSize = S.fsInKHz * resamplerMaxBatchSizeMs

	// Find resampler with the right sampling ratio
	up2x = 0
	if FsHzOut > FsHzIn {
		// Upsample
		if FsHzOut == FsHzIn*2 { // Fs_out : Fs_in = 2 : 1
			// Special case: directly use 2x upsampler
			S.resamplerFunction = useSilkResamplerPrivateUp2HQWrapper
		} else {
			// Default resampler
			S.resamplerFunction = useSilkResamplerPrivateIIRFIR
			up2x = 1
		}
	} else if FsHzOut < FsHzIn {
		// Downsample
		S.resamplerFunction = useSilkResamplerPrivateDownFIR
		if FsHzOut*4 == FsHzIn*3 { // Fs_out : Fs_in = 3 : 4
			S.firFracs = 3
			S.firOrder = resamplerDownOrderFIR0
			S.coefs = resampler34Coefs[:]
		} else if FsHzOut*3 == FsHzIn*2 { // Fs_out : Fs_in = 2 : 3
			S.firFracs = 2
			S.firOrder = resamplerDownOrderFIR0
			S.coefs = resampler23Coefs[:]
		} else if FsHzOut*2 == FsHzIn { // Fs_out : Fs_in = 1 : 2
			S.firFracs = 1
			S.firOrder = resamplerDownOrderFIR1
			S.coefs = resampler12Coefs[:]
		} else if FsHzOut*3 == FsHzIn { // Fs_out : Fs_in = 1 : 3
			S.firFracs = 1
			S.firOrder = resamplerDownOrderFIR2
			S.coefs = resampler13Coefs[:]
		} else if FsHzOut*4 == FsHzIn { // Fs_out : Fs_in = 1 : 4
			S.firFracs = 1
			S.firOrder = resamplerDownOrderFIR2
			S.coefs = resampler14Coefs[:]
		} else if FsHzOut*6 == FsHzIn { // Fs_out : Fs_in = 1 : 6
			S.firFracs = 1
			S.firOrder = resamplerDownOrderFIR2
			S.coefs = resampler16Coefs[:]
		} else {
			// None available
			return -1
		}
	} else {
		// Input and output sampling rates are equal: copy
		S.resamplerFunction = useSilkResamplerCopy
	}

	// Ratio of input/output samples
	S.invRatioQ16 = ((FsHzIn << (14 + up2x)) / FsHzOut) << 2
	// Make sure the ratio is rounded up
	for smulww(S.invRatioQ16, FsHzOut) < FsHzIn<<up2x {
		S.invRatioQ16++
	}

	return 0
}

// resampler is silk_resampler: converts in, of inLen samples, from one
// sampling rate to another. Input and output sampling rate are at most 48000
// Hz.
func resampler(S *resamplerStateStruct, out, in []int16, inLen int32) int32 {
	// Need at least 1 ms of input data, and the delay can't exceed the 1 ms
	// of buffering (celt_assert).
	nSamples := S.fsInKHz - S.inputDelay

	// Copy to delay buffer
	copy(S.delayBuf[S.inputDelay:S.inputDelay+nSamples], in[:nSamples])

	switch S.resamplerFunction {
	case useSilkResamplerPrivateUp2HQWrapper:
		resamplerPrivateUp2HQWrapper(S, out, S.delayBuf[:], S.fsInKHz)
		resamplerPrivateUp2HQWrapper(S, out[S.fsOutKHz:], in[nSamples:], inLen-S.fsInKHz)
	case useSilkResamplerPrivateIIRFIR:
		resamplerPrivateIIRFIR(S, out, S.delayBuf[:], S.fsInKHz)
		resamplerPrivateIIRFIR(S, out[S.fsOutKHz:], in[nSamples:], inLen-S.fsInKHz)
	case useSilkResamplerPrivateDownFIR:
		resamplerPrivateDownFIR(S, out, S.delayBuf[:], S.fsInKHz)
		resamplerPrivateDownFIR(S, out[S.fsOutKHz:], in[nSamples:], inLen-S.fsInKHz)
	default:
		copy(out[:S.fsInKHz], S.delayBuf[:S.fsInKHz])
		copy(out[S.fsOutKHz:S.fsOutKHz+inLen-S.fsInKHz], in[nSamples:nSamples+inLen-S.fsInKHz])
	}

	// Copy to delay buffer
	copy(S.delayBuf[:S.inputDelay], in[inLen-S.inputDelay:inLen])

	return 0
}
