// Package aac decodes AAC-LC audio (ISO/IEC 14496-3), such as the AAC
// YouTube serves in MP4 (itag 140): a pure Go decoder derived from
// SharpJaad, the C# port of JAAD, under the MIT license of
// LICENSE.sharpjaad. JAAD itself is in the public domain.
//
// It decodes the raw data blocks of the Low Complexity object type, with
// frames of 1024 samples, in mono and in stereo, with all of its tools:
// section data, scalefactors, pulse data, temporal noise shaping, M/S and
// intensity stereo, perceptual noise substitution, and the long, start,
// eight short and stop windows, of sine or KBD shape. It skips data
// stream and fill elements, the SBR and PS data of HE-AAC among them, so
// that a stream which signals SBR only in its fill elements decodes as
// its AAC-LC core, at the core's rate. The other object types, among them
// the explicit HE-AAC ones (5 and 29), Main's prediction and LTP, and
// streams of more than two channels, are errors.ErrUnsupported. ParseADTS
// reads the headers of ADTS frames, as the packed audio of YouTube's live
// streams holds them, which give the config of their stream.
//
// SharpJaad, like JAAD, leaves temporal noise shaping undone and the
// pulse data unapplied, and gives the channels of a pair noise of their
// own where the standard has them share it: this package does all three
// as the decoding process of the standard describes them.
//
// The decoder checks what it reads against the bounds of the standard,
// so that malformed frames make errors, never panics.
package aac

import (
	_ "embed"
	"errors"
	"fmt"
)

// License is the license of SharpJaad, whose binary copies must reproduce
// it.
//
//go:embed LICENSE.sharpjaad
var License string

var (
	// ErrFormat is wrapped by the errors of data that is not AAC, or is
	// corrupt.
	ErrFormat = errors.New("aac: invalid data")
	// ErrBufferTooSmall is returned by Decode for room for less than a
	// frame.
	ErrBufferTooSmall = errors.New("aac: buffer too small")
)

// FrameLength is the samples per channel of a frame.
const FrameLength = 1024

// Audio object types of an AudioSpecificConfig.
const (
	ObjectTypeMain = 1  // AAC Main
	ObjectTypeLC   = 2  // AAC LC, the one Decoder decodes
	ObjectTypeSSR  = 3  // AAC SSR
	ObjectTypeLTP  = 4  // AAC LTP
	ObjectTypeSBR  = 5  // SBR, of HE-AAC
	ObjectTypePS   = 29 // PS, of HE-AAC v2
)

// Config is what a decoder needs of a stream: what its
// AudioSpecificConfig says, or the header of an ADTS frame.
type Config struct {
	// ObjectType is the audio object type: ObjectTypeLC for the streams
	// a Decoder decodes.
	ObjectType int
	// SampleRate is the sampling frequency of the AAC core, in Hz, which
	// a Decoder outputs.
	SampleRate int
	// Channels is the number of channels, 1 or 2 for the streams a
	// Decoder decodes: those of the channel configuration, or of its
	// program config element when the configuration is 0.
	Channels int
	// SBR is whether the config signals SBR data in a sync extension, as
	// an HE-AAC stream compatible with AAC-LC does. A Decoder skips it.
	SBR bool
}

// sampleRates are the frequencies of the sampling frequency indexes.
var sampleRates = [13]int{
	96000, 88200, 64000, 48000, 44100, 32000, 24000,
	22050, 16000, 12000, 11025, 8000, 7350,
}

// ParseConfig reads an AudioSpecificConfig, as the DecoderSpecificInfo of
// an MP4 file's esds holds it. It returns errors.ErrUnsupported for the
// streams a Decoder doesn't decode, among them those of 960-sample frames.
func ParseConfig(asc []byte) (Config, error) {
	var r bitReader
	r.init(asc)
	var c Config
	c.ObjectType = objectType(&r)
	rate, err := sampleRate(&r)
	if err != nil {
		return Config{}, err
	}
	c.SampleRate = rate
	channelConfig := r.read(4)
	if r.over() {
		return Config{}, fmt.Errorf("%w: AudioSpecificConfig too short", ErrFormat)
	}
	switch c.ObjectType {
	case ObjectTypeLC:
	case ObjectTypeSBR, ObjectTypePS:
		return Config{}, fmt.Errorf("aac: HE-AAC (audio object type %d): %w", c.ObjectType, errors.ErrUnsupported)
	default:
		return Config{}, fmt.Errorf("aac: audio object type %d: %w", c.ObjectType, errors.ErrUnsupported)
	}

	// GASpecificConfig.
	if r.flag() {
		return Config{}, fmt.Errorf("aac: frames of 960 samples: %w", errors.ErrUnsupported)
	}
	if r.flag() { // dependsOnCoreCoder
		r.skip(14) // coreCoderDelay
	}
	extension := r.flag()
	switch channelConfig {
	case 1, 2:
		c.Channels = channelConfig
	case 0:
		// A program config element follows, which must be of a single
		// front element: an SCE or a CPE.
		var pce programConfig
		if err := pce.parse(&r); err != nil {
			return Config{}, err
		}
		if pce.front != 1 || pce.side+pce.back+pce.lfe > 0 {
			return Config{}, fmt.Errorf("aac: a program of %d channels and %d LFE: %w", pce.channels, pce.lfe, errors.ErrUnsupported)
		}
		c.Channels = pce.channels
	default:
		return Config{}, fmt.Errorf("aac: channel configuration %d: %w", channelConfig, errors.ErrUnsupported)
	}
	if extension {
		r.skip(1) // extensionFlag3
	}
	if r.over() {
		return Config{}, fmt.Errorf("%w: AudioSpecificConfig too short", ErrFormat)
	}
	if c.Channels != 1 && c.Channels != 2 {
		return Config{}, fmt.Errorf("aac: %d channels: %w", c.Channels, errors.ErrUnsupported)
	}

	// A sync extension may signal SBR after the config of the core, which
	// decodes without it.
	if r.left() >= 16 && r.read(11) == 0x2B7 && objectType(&r) == ObjectTypeSBR {
		c.SBR = r.flag() && !r.over()
	}
	return c, nil
}

// objectType reads an audio object type, of 5 bits or, escaped, 11.
func objectType(r *bitReader) int {
	t := r.read(5)
	if t == 31 {
		t = 32 + r.read(6)
	}
	return t
}

// sampleRate reads a sampling frequency index, or the frequency that
// follows the escape index 15.
func sampleRate(r *bitReader) (int, error) {
	i := r.read(4)
	switch {
	case i < len(sampleRates):
		return sampleRates[i], nil
	case i == 15:
		if rate := r.read(24); rate > 0 {
			return rate, nil
		}
	}
	return 0, fmt.Errorf("%w: sampling frequency index %d", ErrFormat, i)
}

// rateIndex is the sampling frequency index whose tables a stream at
// rate uses, after the standard's sampling frequency mapping, which maps
// a frequency the indexes don't name to the nearest one's tables.
func rateIndex(rate int) int {
	for i, low := range [...]int{92017, 75132, 55426, 46009, 37566, 27713, 23004, 18783, 13856, 11502, 9391} {
		if rate >= low {
			return i
		}
	}
	return 11
}
