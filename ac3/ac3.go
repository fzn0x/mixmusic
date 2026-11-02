// Package ac3 decodes AC-3 and E-AC-3 audio, of ATSC A/52 and its Annex
// E, such as the 5.1 surround YouTube serves in MP4 (formats 380 and 328):
// a pure Go decoder written from the published standards, ATSC A/52:2012
// and ETSI TS 102 366 V1.4.1.
//
// A Decoder decodes the syncframes of a stream into six channels of float32
// samples, interleaved in the order of Left, Right, Center, LFE,
// LeftSurround and RightSurround, those the stream lacks silent, and tells
// the stereo downmix the stream's own mix levels ask for.
//
// Of AC-3, it decodes every audio coding mode, from 1/0 to 3/2, with or
// without the LFE, at 48, 44.1 and 32 kHz, of bsid 8 and below, with all of
// the tools of the standard: the exponent strategies, the parametric bit
// allocation and its delta bit allocation, coupling and its phase flags,
// rematrixing, block switching, and dither. It reads the Lo/Ro mix levels
// of the alternate syntax of Annex D (bsid 6) for the downmix.
//
// Of E-AC-3 (bsid 11 to 16), it decodes frames of 1, 2, 3 or 6 blocks, and
// the tools of Annex E: the frame's exponent strategies and SNR offsets,
// spectral extension and its notch filter, and the adaptive hybrid
// transform, of vector and gain adaptive quantization. It decodes
// independent substream 0, and skips the frames of the others: the
// channels past 5.1 of a stream of more, and the objects of joint object
// coding, whose 5.1 it plays. Enhanced coupling, which encoders don't use,
// and the reduced sample rates are errors.ErrUnsupported, and transient
// pre-noise processing, which a decoder may leave out, it leaves out.
//
// It applies the dynamic range gains of the blocks, as the standard asks a
// decoder to by default, and leaves the dialogue level, dialnorm, to the
// player. A 1+1 stream's Ch1 is Left and its Ch2 Right. It checks the CRCs
// of each syncframe.
//
// The bit allocation and the unpacking follow the standards to the bit;
// the transform is computed in float32, with an FFT, to about 140 dB of
// the standard's equations. The decoder checks what it reads against the
// bounds of the standards, so that malformed frames make errors, never
// panics.
package ac3

import (
	"errors"
)

var (
	// ErrFormat is wrapped by the errors of data that is not AC-3, or is
	// corrupt.
	ErrFormat = errors.New("ac3: invalid data")
	// ErrBufferTooSmall is returned by Decode for room for less than the
	// frames of the data.
	ErrBufferTooSmall = errors.New("ac3: buffer too small")
)

const (
	// BlockLength is the samples per channel of an audio block.
	BlockLength = 256
	// FrameLength is the samples per channel of an AC-3 syncframe, its six
	// audio blocks, and so of an MP4 sample of AC-3.
	FrameLength = 6 * BlockLength
	// MaxChannels is the channels of the samples Decode writes: 5.1.
	MaxChannels = 6
)

// The channels of the samples Decode writes, in their order: that of
// WAVE files, and of the 5.1 layouts of most outputs.
const (
	Left = iota
	Right
	Center
	LFE
	LeftSurround
	RightSurround
)

// Audio coding modes, the acmod of a stream: its full-bandwidth channels,
// as the front ones / the surround ones.
const (
	ModeDualMono = 0 // 1+1: two independent mono channels, Ch1 and Ch2
	ModeMono     = 1 // 1/0: C
	ModeStereo   = 2 // 2/0: L, R
	Mode30       = 3 // 3/0: L, C, R
	Mode21       = 4 // 2/1: L, R, S
	Mode31       = 5 // 3/1: L, C, R, S
	Mode22       = 6 // 2/2: L, R, Ls, Rs
	Mode32       = 7 // 3/2: L, C, R, Ls, Rs
)

// Info is what the syncframes a Decoder decoded last tell of their stream.
type Info struct {
	// SampleRate is the sampling frequency, in Hz: 48000, 44100 or 32000.
	SampleRate int
	// Mode is the audio coding mode, of the Mode constants.
	Mode int
	// LFE is whether the stream has a low frequency effects channel.
	LFE bool
	// Bitrate is the stream's rate, in bits per second.
	Bitrate int
	// Downmix is the stereo downmix of the six channels Decode writes, as
	// the stream's center and surround mix levels ask for it: the LoRo
	// downmix of the standard. The left output is the sum, over the
	// channels c, of Downmix[0][c] times channel c, and the right output
	// is that of Downmix[1][c]. Its gains are scaled down so that neither
	// output can overload, and it leaves the LFE out, as the standard
	// allows, but of an E-AC-3 stream that gives the LFE's mix level.
	Downmix [2][MaxChannels]float32
}

// Channels returns the channels of the stream, its LFE among them: 6 for
// 5.1.
func (i Info) Channels() int {
	n := modeChannels[i.Mode&7]
	if i.LFE {
		n++
	}
	return n
}

// modeChannels are the full-bandwidth channels of each audio coding mode.
var modeChannels = [8]int{2, 1, 2, 3, 3, 4, 4, 5}
