package opus

// Port of src/opus_decoder.c: the decoder of a stream, which runs SILK,
// CELT or both on each frame, and smooths the switches between them.

import (
	"errors"
	"math"

	"github.com/fzn0x/mixmusic/internal/opus/celt"
	"github.com/fzn0x/mixmusic/internal/opus/rangedec"
	"github.com/fzn0x/mixmusic/internal/opus/silk"
)

var (
	// ErrBadArg is OPUS_BAD_ARG.
	ErrBadArg = errors.New("opus: invalid argument")
	// ErrBufferTooSmall is OPUS_BUFFER_TOO_SMALL.
	ErrBufferTooSmall = errors.New("opus: buffer too small")
	// ErrInternal is OPUS_INTERNAL_ERROR.
	ErrInternal = errors.New("opus: internal error")
)

// Decoder decodes an Opus stream, OpusDecoder. It is not safe for
// concurrent use.
type Decoder struct {
	celt     *celt.Decoder
	silk     silk.Decoder
	channels int
	fs       int
	ctl      silk.DecControl
	gain     int

	// The state Reset clears.
	streamChannels     int
	bandwidth          int
	mode               int
	prevMode           int
	frameSize          int
	prevRedundancy     bool
	lastPacketDuration int
	rangeFinal         uint32

	// Stack arrays of the C code. The recursive calls of decodeFrame, which
	// conceal, never use one their caller still needs: the caller fills
	// transition through them, and only reads silkPCM before them and
	// redundant after them.
	transition []float32
	redundant  []float32
	silkPCM    []float32
}

// NewDecoder returns a decoder of channels channels, 1 or 2, at
// sampleRate: 8000, 12000, 16000, 24000 or 48000 Hz.
func NewDecoder(sampleRate, channels int) (*Decoder, error) {
	switch sampleRate {
	case 48000, 24000, 16000, 12000, 8000:
	default:
		return nil, ErrBadArg
	}
	if channels != 1 && channels != 2 {
		return nil, ErrBadArg
	}
	st := &Decoder{channels: channels, fs: sampleRate, streamChannels: channels}
	st.ctl.APISampleRate = int32(sampleRate)
	st.ctl.ChannelsAPI = int32(channels)
	if err := st.silk.Init(); err != nil {
		return nil, ErrInternal
	}
	c, err := celt.NewDecoder(sampleRate, channels)
	if err != nil {
		return nil, ErrInternal
	}
	st.celt = c
	st.frameSize = sampleRate / 400
	f10 := sampleRate / 100
	st.transition = make([]float32, f10/2*channels)
	st.redundant = make([]float32, f10/2*channels)
	st.silkPCM = make([]float32, f10*channels)
	return st, nil
}

// Reset is OPUS_RESET_STATE: the decoder forgets the stream, as for a new
// one or after a seek.
func (st *Decoder) Reset() {
	st.bandwidth = 0
	st.mode = 0
	st.prevMode = 0
	st.prevRedundancy = false
	st.lastPacketDuration = 0
	st.rangeFinal = 0
	st.celt.Reset()
	st.silk.Reset()
	st.streamChannels = st.channels
	st.frameSize = st.fs / 400
}

// SetGain is OPUS_SET_GAIN: a gain on the output, in 1/256 dB, from
// -32768 to 32767, as the output gain of an Opus header.
func (st *Decoder) SetGain(gain int) error {
	if gain < -32768 || gain > 32767 {
		return ErrBadArg
	}
	st.gain = gain
	return nil
}

// FinalRange is OPUS_GET_FINAL_RANGE: the state of the range decoder at
// the end of the last frame, for checks against the encoder's.
func (st *Decoder) FinalRange() uint32 { return st.rangeFinal }

// LastPacketDuration is OPUS_GET_LAST_PACKET_DURATION: the samples per
// channel of the last packet decoded or concealed.
func (st *Decoder) LastPacketDuration() int { return st.lastPacketDuration }

// Decode decodes packet into pcm, interleaved, as opus_decode_float does,
// and returns the number of samples per channel. pcm holds len(pcm)/channels
// samples per channel: at least the packet's duration, up to 120 ms. Samples
// are about [-1, 1], and not clipped.
//
// A nil packet conceals a lost one, over all of pcm, which must then be a
// multiple of 2.5 ms; LastPacketDuration is a good length. With fec, the
// decoder conceals the lost packet before this one, over all of pcm, with
// the redundancy this one carries, if any; the caller then decodes this
// packet again, without fec.
func (st *Decoder) Decode(packet []byte, pcm []float32, fec bool) (int, error) {
	frameSize := len(pcm) / st.channels
	if frameSize <= 0 {
		return 0, ErrBadArg
	}
	return st.decodeNative(packet, pcm, frameSize, fec)
}

// silence is a CELT frame of silence, which fades CELT out when a stream
// goes from hybrid to SILK.
var silence = [2]byte{0xFF, 0xFF}

// smoothFade is smooth_fade: the crossfade of overlap samples from in1 to
// in2, into out, which may be either.
func smoothFade(in1, in2, out []float32, overlap, channels int, window []float32, fs int) {
	inc := 48000 / fs
	for c := 0; c < channels; c++ {
		for i := 0; i < overlap; i++ {
			w := window[i*inc] * window[i*inc]
			out[i*channels+c] = w*in2[i*channels+c] + (1-w)*in1[i*channels+c]
		}
	}
}

// decodeFrame is opus_decode_frame: it decodes a frame, or conceals a lost
// one when data is nil, into pcm, of frameSize samples per channel at
// most, and returns the samples per channel.
func (st *Decoder) decodeFrame(data []byte, pcm []float32, frameSize int, fec bool) (int, error) {
	f20 := st.fs / 50
	f10 := f20 >> 1
	f5 := f10 >> 1
	f2_5 := f5 >> 1
	if frameSize < f2_5 {
		return 0, ErrBufferTooSmall
	}
	// Limit frameSize to avoid excessive stack allocations.
	frameSize = min(frameSize, st.fs/25*3)
	n := len(data)
	// Payloads of 1 (2 including ToC) or 0 trigger the PLC/DTX.
	if n <= 1 {
		data = nil
		// In that case, don't conceal more than what the ToC says.
		frameSize = min(frameSize, st.frameSize)
	}
	var dec rangedec.Decoder
	var audiosize, mode, bandwidth int
	if data != nil {
		audiosize = st.frameSize
		mode = st.mode
		bandwidth = st.bandwidth
		dec.Init(data)
	} else {
		audiosize = frameSize
		// Run the PLC using the last mode (CELT if we ended with CELT
		// redundancy).
		mode = st.prevMode
		if st.prevRedundancy {
			mode = ModeCELTOnly
		}
		if mode == 0 {
			// If we haven't got any packet yet, all we can do is return
			// zeros.
			clear(pcm[:audiosize*st.channels])
			return audiosize, nil
		}
		// Avoids trying to run the PLC on sizes other than 2.5 (CELT), 5
		// (CELT), 10, or 20 (e.g. 12.5 or 30 ms).
		if audiosize > f20 {
			for audiosize > 0 {
				ret, err := st.decodeFrame(nil, pcm, min(audiosize, f20), false)
				if err != nil {
					return 0, err
				}
				pcm = pcm[ret*st.channels:]
				audiosize -= ret
			}
			return frameSize, nil
		} else if audiosize < f20 {
			if audiosize > f10 {
				audiosize = f10
			} else if mode != ModeSILKOnly && audiosize > f5 && audiosize < f10 {
				audiosize = f5
			}
		}
	}

	// In fixed-point, we can tell CELT to do the accumulation on top of the
	// SILK PCM buffer. This saves some stack space.
	celtAccum := mode != ModeCELTOnly

	transition := data != nil && st.prevMode > 0 &&
		((mode == ModeCELTOnly && st.prevMode != ModeCELTOnly && !st.prevRedundancy) ||
			(mode != ModeCELTOnly && st.prevMode == ModeCELTOnly))
	var pcmTransition []float32
	if transition && mode == ModeCELTOnly {
		pcmTransition = st.transition
		st.decodeFrame(nil, pcmTransition, min(f5, audiosize), false)
	}
	if audiosize > frameSize {
		return 0, ErrBadArg
	}
	frameSize = audiosize

	// SILK processing.
	if mode != ModeCELTOnly {
		pcmTooSmall := frameSize < f10
		pcmPtr := pcm
		if pcmTooSmall {
			pcmPtr = st.silkPCM
		}
		if st.prevMode == ModeCELTOnly {
			st.silk.Reset()
		}
		// The SILK PLC cannot produce frames of less than 10 ms.
		st.ctl.PayloadSizeMs = max(10, 1000*audiosize/st.fs)
		if data != nil {
			st.ctl.ChannelsInternal = int32(st.streamChannels)
			if mode == ModeSILKOnly {
				switch bandwidth {
				case BandwidthNarrowband:
					st.ctl.InternalSampleRate = 8000
				case BandwidthMediumband:
					st.ctl.InternalSampleRate = 12000
				default:
					st.ctl.InternalSampleRate = 16000
				}
			} else {
				// Hybrid mode.
				st.ctl.InternalSampleRate = 16000
			}
		}
		lostFlag := 0
		if data == nil {
			lostFlag = 1
		} else if fec {
			lostFlag = 2
		}
		decoded := 0
		for {
			k, err := st.silk.Decode(&st.ctl, lostFlag, decoded == 0, &dec, pcmPtr)
			if err != nil {
				if lostFlag == 0 {
					return 0, ErrInternal
				}
				// PLC failure should not be fatal: silence for the frame, which
				// ends the loop. (C clears a whole frame from pcmPtr, past the
				// buffer when it fails on a second SILK frame.)
				clear(pcmPtr[:min(len(pcmPtr), frameSize*st.channels)])
				break
			}
			pcmPtr = pcmPtr[k*st.channels:]
			decoded += k
			if decoded >= frameSize {
				break
			}
		}
		if pcmTooSmall {
			copy(pcm[:frameSize*st.channels], st.silkPCM)
		}
	}

	startBand := 0
	redundancy := false
	celtToSilk := false
	redundancyBytes := 0
	if !fec && mode != ModeCELTOnly && data != nil &&
		dec.Tell()+17+20*b2i(mode == ModeHybrid) <= 8*n {
		// Check if we have a redundant 0-8 kHz band.
		if mode == ModeHybrid {
			redundancy = dec.BitLogp(12)
		} else {
			redundancy = true
		}
		if redundancy {
			celtToSilk = dec.BitLogp(1)
			// redundancyBytes will be at least two, in the non-hybrid case
			// due to the Tell check above.
			if mode == ModeHybrid {
				redundancyBytes = int(dec.Uint(256)) + 2
			} else {
				redundancyBytes = n - ((dec.Tell() + 7) >> 3)
			}
			n -= redundancyBytes
			// This is a sanity check. It should never happen for a valid
			// packet, so the exact behaviour is not normative.
			if n*8 < dec.Tell() {
				n = 0
				redundancyBytes = 0
				redundancy = false
			}
			// Shrink the decoder because of the raw bits.
			dec.Shrink(uint32(redundancyBytes))
		}
	}
	if mode != ModeCELTOnly {
		startBand = 17
	}
	if redundancy {
		transition = false
	}
	if transition && mode != ModeCELTOnly {
		pcmTransition = st.transition
		st.decodeFrame(nil, pcmTransition, min(f5, audiosize), false)
	}

	if bandwidth != 0 {
		endBand := 21
		switch bandwidth {
		case BandwidthNarrowband:
			endBand = 13
		case BandwidthMediumband, BandwidthWideband:
			endBand = 17
		case BandwidthSuperwideband:
			endBand = 19
		}
		st.celt.SetEndBand(endBand)
	}
	st.celt.SetStreamChannels(st.streamChannels)

	var redundantRng uint32
	redundantAudio := st.redundant
	// 5 ms redundant frame for CELT->SILK.
	if redundancy && celtToSilk {
		// If the previous frame did not use CELT (the first redundancy frame
		// in a transition from SILK may have been lost) then the CELT
		// decoder is stale at this point and the redundancy audio is not
		// useful, however the final range is still needed (for testing), so
		// the redundancy is always decoded but the decoded audio may not be
		// used.
		st.celt.SetStartBand(0)
		st.celt.Decode(data[n:n+redundancyBytes], redundantAudio, f5, nil, false)
		redundantRng = st.celt.FinalRange()
	}

	// MUST be after PLC.
	st.celt.SetStartBand(startBand)

	var celtErr error
	if mode != ModeSILKOnly {
		celtFrameSize := min(f20, frameSize)
		// Make sure to discard any previous CELT state.
		if mode != st.prevMode && st.prevMode > 0 && !st.prevRedundancy {
			st.celt.Reset()
		}
		var celtData []byte
		if !fec && data != nil {
			celtData = data[:n]
		}
		_, celtErr = st.celt.Decode(celtData, pcm, celtFrameSize, &dec, celtAccum)
		st.rangeFinal = st.celt.FinalRange()
	} else {
		if !celtAccum {
			clear(pcm[:frameSize*st.channels])
		}
		// For hybrid -> SILK transitions, we let the CELT MDCT do a fade-out
		// by decoding a silence frame.
		if st.prevMode == ModeHybrid && !(redundancy && celtToSilk && st.prevRedundancy) {
			st.celt.SetStartBand(0)
			st.celt.Decode(silence[:], pcm, f2_5, nil, celtAccum)
		}
		st.rangeFinal = dec.Range()
	}

	window := st.celt.Window()
	ch := st.channels

	// 5 ms redundant frame for SILK->CELT.
	if redundancy && !celtToSilk {
		st.celt.Reset()
		st.celt.SetStartBand(0)
		st.celt.Decode(data[n:n+redundancyBytes], redundantAudio, f5, nil, false)
		redundantRng = st.celt.FinalRange()
		smoothFade(pcm[ch*(frameSize-f2_5):], redundantAudio[ch*f2_5:],
			pcm[ch*(frameSize-f2_5):], f2_5, ch, window, st.fs)
	}
	// 5 ms redundant frame for CELT->SILK; ignore if the previous frame did
	// not use CELT (the first redundancy frame in a transition from SILK may
	// have been lost).
	if redundancy && celtToSilk && (st.prevMode != ModeSILKOnly || st.prevRedundancy) {
		copy(pcm[:ch*f2_5], redundantAudio)
		smoothFade(redundantAudio[ch*f2_5:], pcm[ch*f2_5:], pcm[ch*f2_5:], f2_5, ch, window, st.fs)
	}
	if transition {
		if audiosize >= f5 {
			copy(pcm[:ch*f2_5], pcmTransition)
			smoothFade(pcmTransition[ch*f2_5:], pcm[ch*f2_5:], pcm[ch*f2_5:], f2_5, ch, window, st.fs)
		} else {
			// Not enough time to do a clean transition, but we do it anyway.
			// This will not preserve amplitude perfectly and may introduce a
			// bit of temporal aliasing, but it shouldn't be too bad and
			// that's pretty much the best we can do. In any case, generating
			// this transition is pretty silly in the first place.
			smoothFade(pcmTransition, pcm, pcm, f2_5, ch, window, st.fs)
		}
	}

	if st.gain != 0 {
		gain := float32(math.Exp(0.6931471805599453094 * float64(float32(6.48814081e-4)*float32(st.gain))))
		for i := range pcm[:frameSize*ch] {
			pcm[i] *= gain
		}
	}

	if n <= 1 {
		st.rangeFinal = 0
	} else {
		st.rangeFinal ^= redundantRng
	}
	st.prevMode = mode
	st.prevRedundancy = redundancy && !celtToSilk
	if errors.Is(celtErr, celt.ErrBadArg) {
		return 0, ErrBadArg
	} else if celtErr != nil {
		return 0, ErrInternal
	}
	return audiosize, nil
}

// decodeNative is opus_decode_native, without soft clipping.
func (st *Decoder) decodeNative(data []byte, pcm []float32, frameSize int, fec bool) (int, error) {
	// For FEC/PLC, frameSize has to be a multiple of 2.5 ms.
	if (fec || len(data) == 0) && frameSize%(st.fs/400) != 0 {
		return 0, ErrBadArg
	}
	if len(data) == 0 {
		count := 0
		for count < frameSize {
			ret, err := st.decodeFrame(nil, pcm[count*st.channels:], frameSize-count, false)
			if err != nil {
				return 0, err
			}
			count += ret
		}
		st.lastPacketDuration = count
		return count, nil
	}

	packetMode := PacketMode(data[0])
	packetBandwidth := PacketBandwidth(data[0])
	packetFrameSize := SamplesPerFrame(data[0], st.fs)
	packetStreamChannels := PacketChannels(data[0])
	var all [48][]byte
	_, count, err := parsePacket(data, &all)
	if err != nil {
		return 0, err
	}
	frames := all[:count]

	if fec {
		// If no FEC can be present, run the PLC (recursive call).
		if frameSize < packetFrameSize || packetMode == ModeCELTOnly || st.mode == ModeCELTOnly {
			return st.decodeNative(nil, pcm, frameSize, false)
		}
		// Otherwise, run the PLC on everything except the size for which we
		// might have FEC.
		durationCopy := st.lastPacketDuration
		if frameSize-packetFrameSize != 0 {
			if _, err := st.decodeNative(nil, pcm, frameSize-packetFrameSize, false); err != nil {
				st.lastPacketDuration = durationCopy
				return 0, err
			}
		}
		// Complete with FEC.
		st.mode = packetMode
		st.bandwidth = packetBandwidth
		st.frameSize = packetFrameSize
		st.streamChannels = packetStreamChannels
		if _, err := st.decodeFrame(frames[0], pcm[st.channels*(frameSize-packetFrameSize):], packetFrameSize, true); err != nil {
			return 0, err
		}
		st.lastPacketDuration = frameSize
		return frameSize, nil
	}

	if len(frames)*packetFrameSize > frameSize {
		return 0, ErrBufferTooSmall
	}

	// Update the state as the last step to avoid updating it on an invalid
	// packet.
	st.mode = packetMode
	st.bandwidth = packetBandwidth
	st.frameSize = packetFrameSize
	st.streamChannels = packetStreamChannels

	nbSamples := 0
	for _, f := range frames {
		ret, err := st.decodeFrame(f, pcm[nbSamples*st.channels:], frameSize-nbSamples, false)
		if err != nil {
			return 0, err
		}
		nbSamples += ret
	}
	st.lastPacketDuration = nbSamples
	return nbSamples, nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
