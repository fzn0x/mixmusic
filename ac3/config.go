package ac3

import (
	"errors"
	"fmt"
)

// Config is what an MP4 file's sample entry tells of an AC-3 stream: its
// dac3 box, the AC3SpecificBox of ETSI TS 102 366, Annex F. A Decoder
// needs none of it, as each syncframe says it all again; it describes the
// stream before its first frame is read.
type Config struct {
	// SampleRate is the sampling frequency, in Hz.
	SampleRate int
	// BSID is the version of the stream's syntax: up to 8 for AC-3, 6
	// for the alternate syntax of Annex D.
	BSID int
	// Mode is the audio coding mode, of the Mode constants.
	Mode int
	// LFE is whether the stream has a low frequency effects channel.
	LFE bool
	// Bitrate is the stream's rate, in bits per second.
	Bitrate int
}

// Channels returns the channels of the stream, its LFE among them: 6 for
// 5.1.
func (c Config) Channels() int { return Info{Mode: c.Mode, LFE: c.LFE}.Channels() }

// sampleRates are the frequencies of fscod.
var sampleRates = [3]int{48000, 44100, 32000}

// bitrates are the nominal rates of frmsizecod / 2, and of a dac3's
// bit_rate_code, in kbit/s.
var bitrates = [19]int{32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 384, 448, 512, 576, 640}

// ParseDAC3 reads the payload of a dac3 box, the box header left out. It
// returns errors.ErrUnsupported for the streams a Decoder doesn't decode:
// those of a bsid above 8.
func ParseDAC3(b []byte) (Config, error) {
	if len(b) < 3 {
		return Config{}, fmt.Errorf("%w: dac3 of %d bytes", ErrFormat, len(b))
	}
	v := uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2])
	fscod := int(v >> 22)
	c := Config{
		BSID: int(v>>17) & 31,
		Mode: int(v>>11) & 7,
		LFE:  v>>10&1 != 0,
	}
	if fscod == 3 {
		return Config{}, fmt.Errorf("%w: dac3: fscod 3", ErrFormat)
	}
	c.SampleRate = sampleRates[fscod]
	rate := int(v>>5) & 31
	if rate >= len(bitrates) {
		return Config{}, fmt.Errorf("%w: dac3: bit_rate_code %d", ErrFormat, rate)
	}
	c.Bitrate = bitrates[rate] * 1000
	if c.BSID > 8 {
		return Config{}, fmt.Errorf("ac3: dac3: bsid %d: %w", c.BSID, errors.ErrUnsupported)
	}
	return c, nil
}
