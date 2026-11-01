// Package ac3 decodes AC-3 audio, of ATSC A/52, such as the 5.1 surround
// YouTube serves in MP4 (format 380): a pure Go decoder written from the
// published standard.
//
// A Decoder decodes the syncframes of a stream into six channels of float32
// samples, interleaved in the order of Left, Right, Center, LFE,
// LeftSurround and RightSurround, those the stream lacks silent, and tells
// the stereo downmix the stream's own levels ask for.
//
// The decoder checks what it reads against the bounds of the standard, so
// that malformed frames make errors, never panics.
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
	// allows.
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
