package output

import "encoding/binary"

// The 5.1 layouts the outputs open: the channels of a Stream of six
// channels are, in order, left, right, center, LFE, left surround and
// right surround, as WAVE files and ac3's decoder order them.

// Surround is the channels of a Stream of 5.1 surround.
const Surround = 6

// The speaker positions of WAVEFORMATEXTENSIBLE's dwChannelMask.
const (
	speakerFrontLeft    = 0x1
	speakerFrontRight   = 0x2
	speakerFrontCenter  = 0x4
	speakerLowFrequency = 0x8
	speakerBackLeft     = 0x10
	speakerBackRight    = 0x20
	speakerSideLeft     = 0x200
	speakerSideRight    = 0x400

	// speakerStereo is KSAUDIO_SPEAKER_STEREO.
	speakerStereo = speakerFrontLeft | speakerFrontRight
	// speaker51 is KSAUDIO_SPEAKER_5POINT1, of back speakers, and
	// speaker51Side KSAUDIO_SPEAKER_5POINT1_SURROUND, of side ones.
	speaker51     = speakerStereo | speakerFrontCenter | speakerLowFrequency | speakerBackLeft | speakerBackRight
	speaker51Side = speakerStereo | speakerFrontCenter | speakerLowFrequency | speakerSideLeft | speakerSideRight
)

// surroundMask returns the channel mask of a WASAPI stream of 5.1 on a
// device of channels whose mix format has mask, 0 when unknown: the 5.1
// of the device's surround speakers, side ones where it has them, as 5.1
// and 7.1 systems have, else back ones. It reports false for a device of
// fewer than six channels, or without a center, an LFE and a pair of
// surrounds, which a Stream plays stereo on.
func surroundMask(channels int, mask uint32) (uint32, bool) {
	if channels < Surround {
		return 0, false
	}
	if mask == 0 {
		// A device that names no speakers: six channels or more are taken
		// in the order of the masks, the side speakers' for 5.1.
		return speaker51Side, true
	}
	if mask&(speakerStereo|speakerFrontCenter|speakerLowFrequency) != speakerStereo|speakerFrontCenter|speakerLowFrequency {
		return 0, false
	}
	switch {
	case mask&(speakerSideLeft|speakerSideRight) == speakerSideLeft|speakerSideRight:
		return speaker51Side, true
	case mask&(speakerBackLeft|speakerBackRight) == speakerBackLeft|speakerBackRight:
		return speaker51, true
	}
	return 0, false
}

// The channel positions of PulseAudio's pa_channel_position_t.
const (
	paFrontLeft   = 1
	paFrontRight  = 2
	paFrontCenter = 3
	paRearLeft    = 5
	paRearRight   = 6
	paLFE         = 7
	paSideLeft    = 10
	paSideRight   = 11
)

// pulseSurroundMap returns the positions of a PulseAudio stream of 5.1 on
// a sink of the channel map positions, in a Stream's order: the sink's
// side speakers for the surrounds where it has them, else its rear ones.
// It reports false for a sink of fewer than six channels, or without a
// center, an LFE and a pair of surrounds, which a Stream plays stereo on.
func pulseSurroundMap(positions []int32) ([Surround]int32, bool) {
	var has [paSideRight + 1]bool
	for _, p := range positions {
		if p >= 0 && int(p) < len(has) {
			has[p] = true
		}
	}
	if len(positions) < Surround || !has[paFrontLeft] || !has[paFrontRight] || !has[paFrontCenter] || !has[paLFE] {
		return [Surround]int32{}, false
	}
	switch {
	case has[paSideLeft] && has[paSideRight]:
		return [Surround]int32{paFrontLeft, paFrontRight, paFrontCenter, paLFE, paSideLeft, paSideRight}, true
	case has[paRearLeft] && has[paRearRight]:
		return [Surround]int32{paFrontLeft, paFrontRight, paFrontCenter, paLFE, paRearLeft, paRearRight}, true
	}
	return [Surround]int32{}, false
}

// kAudioChannelLayoutTag_MPEG_5_1_A is Core Audio's layout of L, R, C,
// LFE, Ls and Rs, a Stream's order of 5.1.
const kAudioChannelLayoutTagMPEG51A = 121<<16 | 6

// coreAudioChannels returns the channels an Audio Queue of a Stream that
// asks for asked plays on a device of deviceChannels output channels: six
// where the device has them, else two.
func coreAudioChannels(asked, deviceChannels int) int {
	if asked == Surround && deviceChannels >= Surround {
		return Surround
	}
	return 2
}

// bufferListChannels returns the channels of the buffers of an
// AudioBufferList, as 64-bit systems lay it out.
func bufferListChannels(b []byte) int {
	if len(b) < 8 {
		return 0
	}
	n := int(binary.LittleEndian.Uint32(b))
	channels := 0
	for i := range n {
		at := 8 + 16*i
		if at+4 > len(b) {
			break
		}
		channels += int(binary.LittleEndian.Uint32(b[at:]))
	}
	return channels
}
