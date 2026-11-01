// Port of silk/define.h, silk/structs.h, silk/control.h, silk/errors.h, silk/PLC.h, silk/pitch_est_defines.h and silk/resampler_structs.h.

// Package silk is the SILK decoder of Opus (RFC 6716, section 4.2): a port
// of the SILK decoder of libopus 1.6.1 as its default float build compiles it
// (no deep PLC, OSCE, DRED or QEXT), under the BSD license of
// opus/LICENSE.libopus. SILK is fixed-point code even in that build, and the port
// is bit-exact with it.
//
// Copyright (c) 2006-2011, Skype Limited. All rights reserved.
package silk

import "fmt"

// Decoder API flags (control.h).
const (
	flagDecodeNormal = 0
	flagPacketLost   = 1
	flagDecodeLBRR   = 2
)

// define.h
const (
	decoderNumChannels = 2

	maxFramesPerPacket = 3

	// Maximum sampling frequency.
	maxFsKHz    = 16
	maxAPIFsKHz = 48

	// Signal types.
	typeNoVoiceActivity = 0
	typeUnvoiced        = 1
	typeVoiced          = 2

	// Conditional coding types.
	codeIndependently             = 0
	codeIndependentlyNoLTPScaling = 1
	codeConditionally             = 2

	// Settings for stereo processing.
	stereoQuantTabSize  = 16
	stereoQuantSubSteps = 5
	stereoInterpLenMs   = 8

	// Range of pitch lag estimates.
	pitchEstMinLagMs = 2
	pitchEstMaxLagMs = 18

	maxNbSubfr = 4

	// Number of samples per frame.
	ltpMemLengthMs    = 20
	subFrameLengthMs  = 5
	maxSubFrameLength = subFrameLengthMs * maxFsKHz
	maxFrameLengthMs  = subFrameLengthMs * maxNbSubfr
	maxFrameLength    = maxFrameLengthMs * maxFsKHz

	// Gain quantization.
	minQGainDB        = 2
	maxQGainDB        = 88
	nLevelsQGain      = 64
	maxDeltaGainQuant = 36
	minDeltaGainQuant = -4

	// Quantization offsets (multiples of 4).
	offsetVLQ10  = 32
	offsetVHQ10  = 100
	offsetUVLQ10 = 100
	offsetUVHQ10 = 240

	quantLevelAdjustQ10 = 80

	maxLPCStabilizeIterations = 16

	maxLPCOrder = 16
	minLPCOrder = 10

	ltpOrder  = 5
	nbLTPCbks = 3

	// Excitation entropy coding.
	shellCodecFrameLength     = 16
	log2ShellCodecFrameLength = 4
	maxNbShellBlocks          = maxFrameLength / shellCodecFrameLength
	nRateLevels               = 10
	silkMaxPulses             = 16

	lsfCosTabSzFIX = 128

	nlsfQuantMaxAmplitude = 4

	// BWE factor to apply after packet loss.
	bweAfterLossQ16 = 63570

	// CN generation.
	cngBufMaskMax           = 255
	cngGainSmthQ16          = 4634
	cngGainSmthThresholdQ16 = 46396
	cngNLSFSmthQ16          = 16348
)

// PLC.h
const (
	bweCoefQ16              = 64881 // SILK_FIX_CONST( BWE_COEF, 16 ), BWE_COEF 0.99
	vPitchGainStartMinQ14   = 11469
	vPitchGainStartMaxQ14   = 15565
	maxPitchLagMs           = 18
	randBufSize             = 128
	randBufMask             = randBufSize - 1
	log2InvLPCGainHighThres = 3
	log2InvLPCGainLowThres  = 8
	pitchDriftFacQ16        = 655
)

// pitch_est_defines.h
const (
	peMaxNbSubfr       = 4
	peMaxLagMs         = 18
	peMinLagMs         = 2
	peNbCbksStage2Ext  = 11
	peNbCbksStage3Max  = 34
	peNbCbksStage310Ms = 12
	peNbCbksStage210Ms = 3
)

// silkMaxOrderLPC is SILK_MAX_ORDER_LPC (SigProc_FIX.h).
const silkMaxOrderLPC = 24

// Error is a SILK error code (errors.h). Decode can also return the sum of
// several codes, as silk_Decode does.
type Error int32

// Decoder error codes.
const (
	// ErrInvalidSamplingFrequency: output sampling frequency lower than
	// internal decoded sampling frequency.
	ErrInvalidSamplingFrequency Error = -200
	// ErrPayloadTooLarge: payload size exceeded the maximum allowed 1024 bytes.
	ErrPayloadTooLarge Error = -201
	// ErrPayloadError: payload has bit errors.
	ErrPayloadError Error = -202
	// ErrInvalidFrameSize: payload has bit errors.
	ErrInvalidFrameSize Error = -203
)

func (e Error) Error() string {
	switch e {
	case ErrInvalidSamplingFrequency:
		return "silk: invalid sampling frequency"
	case ErrPayloadTooLarge:
		return "silk: payload too large"
	case ErrPayloadError:
		return "silk: payload error"
	case ErrInvalidFrameSize:
		return "silk: invalid frame size"
	}
	return fmt.Sprintf("silk: error %d", int32(e))
}

// DecControl is silk_DecControlStruct: how to decode, and the decoder's
// status.
type DecControl struct {
	// ChannelsAPI is the number of output channels, 1 or 2.
	ChannelsAPI int32
	// ChannelsInternal is the number of coded channels, 1 or 2.
	ChannelsInternal int32
	// APISampleRate is the output sampling rate in Hz: 8000, 12000, 16000,
	// 24000 or 48000.
	APISampleRate int32
	// InternalSampleRate is the coded sampling rate in Hz: 8000, 12000 or
	// 16000.
	InternalSampleRate int32
	// PayloadSizeMs is the duration of the packet in ms: 10, 20, 40 or 60
	// (0 for a lost packet, as 10).
	PayloadSizeMs int
	// PrevPitchLag is output: the pitch lag of the previous frame (0 if
	// unvoiced), in samples at 48 kHz.
	PrevPitchLag int
}

// nlsfCBStruct is silk_NLSF_CB_struct, without the encoder's ec_Rates_Q5.
type nlsfCBStruct struct {
	nVectors           int16
	order              int16
	quantStepSizeQ16   int16
	invQuantStepSizeQ6 int16
	cb1NLSFQ8          []uint8
	cb1WghtQ9          []int16
	cb1ICDF            []uint8
	predQ8             []uint8
	ecSel              []uint8
	ecICDF             []uint8
	deltaMinQ15        []int16
}

// stereoDecState is stereo_dec_state.
type stereoDecState struct {
	predPrevQ13 [2]int16
	sMid        [2]int16
	sSide       [2]int16
}

// sideInfoIndices is SideInfoIndices.
type sideInfoIndices struct {
	gainsIndices     [maxNbSubfr]int8
	ltpIndex         [maxNbSubfr]int8
	nlsfIndices      [maxLPCOrder + 1]int8
	lagIndex         int16
	contourIndex     int8
	signalType       int8
	quantOffsetType  int8
	nlsfInterpCoefQ2 int8
	perIndex         int8
	ltpScaleIndex    int8
	seed             int8
}

// plcStruct is silk_PLC_struct, without enable_deep_plc.
type plcStruct struct {
	pitchLQ8        int32           // Pitch lag to use for voiced concealment
	ltpCoefQ14      [ltpOrder]int16 // LTP coefficients to use for voiced concealment
	prevLPCQ12      [maxLPCOrder]int16
	lastFrameLost   int32 // Was previous frame lost
	randSeed        int32 // Seed for unvoiced signal generation
	randScaleQ14    int16 // Scaling of unvoiced random signal
	concEnergy      int32
	concEnergyShift int32
	prevLTPScaleQ14 int16
	prevGainQ16     [2]int32
	fsKHz           int32
	nbSubfr         int32
	subfrLength     int32
}

// cngStruct is silk_CNG_struct.
type cngStruct struct {
	cngExcBufQ14   [maxFrameLength]int32
	cngSmthNLSFQ15 [maxLPCOrder]int16
	cngSynthState  [maxLPCOrder]int32
	cngSmthGainQ16 int32
	randSeed       int32
	fsKHz          int32
}

// decoderState is silk_decoder_state, the state of one channel.
type decoderState struct {
	prevGainQ16          int32
	excQ14               [maxFrameLength]int32
	sLPCQ14Buf           [maxLPCOrder]int32
	outBuf               [maxFrameLength + 2*maxSubFrameLength]int16 // Buffer for output signal
	lagPrev              int32                                       // Previous Lag
	lastGainIndex        int8                                        // Previous gain index
	fsKHz                int32                                       // Sampling frequency in kHz
	fsAPIHz              int32                                       // API sample frequency (Hz)
	nbSubfr              int32                                       // Number of 5 ms subframes in a frame
	frameLength          int32                                       // Frame length (samples)
	subfrLength          int32                                       // Subframe length (samples)
	ltpMemLength         int32                                       // Length of LTP memory
	lpcOrder             int32                                       // LPC order
	prevNLSFQ15          [maxLPCOrder]int16                          // Used to interpolate LSFs
	firstFrameAfterReset int32                                       // Flag for deactivating NLSF interpolation
	pitchLagLowBitsICDF  []uint8                                     // iCDF table for low bits of pitch lag index
	pitchContourICDF     []uint8                                     // iCDF table for pitch contour index

	// For buffering payload in case of more frames per packet.
	nFramesDecoded   int32
	nFramesPerPacket int32

	// Specifically for entropy coding.
	ecPrevSignalType int32
	ecPrevLagIndex   int16

	vadFlags  [maxFramesPerPacket]int32
	lbrrFlag  int32
	lbrrFlags [maxFramesPerPacket]int32

	resamplerState resamplerStateStruct

	psNLSFCB *nlsfCBStruct // NLSF codebook

	// Quantization indices.
	indices sideInfoIndices

	// CNG state.
	sCNG cngStruct

	// Stuff used for PLC.
	lossCnt        int32
	prevSignalType int32

	sPLC plcStruct

	// Scratch of the functions that run on this state (stack arrays in C).
	scratch decoderScratch
}

// decoderScratch holds the temporary arrays that the C code allocates on the
// stack; every function writes the parts it reads first.
type decoderScratch struct {
	pulses    [maxFrameLength]int16                  // silk_decode_frame
	sLTP      [maxFrameLength]int16                  // silk_decode_core, silk_PLC_conceal
	sLTPQ15   [2 * maxFrameLength]int32              // silk_decode_core; sLTP_Q14 of silk_PLC_conceal
	resQ14    [maxSubFrameLength]int32               // silk_decode_core
	sLPCQ14   [maxSubFrameLength + maxLPCOrder]int32 // silk_decode_core
	excBuf    [2 * maxSubFrameLength]int16           // silk_PLC_energy
	cngSigQ14 [maxFrameLength + maxLPCOrder]int32    // silk_CNG
}

// decoderControl is silk_decoder_control.
type decoderControl struct {
	// Prediction and coding parameters.
	pitchL   [maxNbSubfr]int32
	gainsQ16 [maxNbSubfr]int32
	// Holds interpolated and final coefficients.
	predCoefQ12 [2][maxLPCOrder]int16
	ltpCoefQ14  [ltpOrder * maxNbSubfr]int16
	ltpScaleQ14 int32
}

// resampler_structs.h
const (
	resamplerMaxFIROrder = 36
	resamplerMaxIIROrder = 6
)

// resamplerStateStruct is silk_resampler_state_struct. The C union sFIR is
// two arrays here: a state only ever uses one of them.
type resamplerStateStruct struct {
	sIIR              [resamplerMaxIIROrder]int32 // this must be the first element of this struct
	sFIRI32           [resamplerMaxFIROrder]int32 // sFIR.i32
	sFIRI16           [resamplerMaxFIROrder]int16 // sFIR.i16
	delayBuf          [96]int16
	resamplerFunction int32
	batchSize         int32
	invRatioQ16       int32
	firOrder          int32
	firFracs          int32
	fsInKHz           int32
	fsOutKHz          int32
	inputDelay        int32
	coefs             []int16

	// Scratch of the resampling functions (stack arrays in C).
	buf16 [2*resamplerMaxBatchSizeMs*maxFsKHz + resamplerOrderFIR12]int16
	buf32 [resamplerMaxBatchSizeMs*maxFsKHz + resamplerDownOrderFIR2]int32
}
