package output

import (
	"encoding/binary"
	"testing"
)

// TestSurroundMask checks the channel masks of WASAPI streams of 5.1:
// those of the device's surround speakers, side ones before back ones, and
// none on a device without 5.1, which a stream plays stereo on.
func TestSurroundMask(t *testing.T) {
	const speaker71 = speaker51 | speakerSideLeft | speakerSideRight
	const speaker71Wide = speaker51 | 0x40 | 0x80 // front left and right of center
	for _, c := range []struct {
		name     string
		channels int
		mask     uint32
		want     uint32
		ok       bool
	}{
		{"stereo", 2, speakerStereo, 0, false},
		{"quad", 4, speakerStereo | speakerBackLeft | speakerBackRight, 0, false},
		{"5.1 of back speakers", 6, speaker51, speaker51, true},
		{"5.1 of side speakers", 6, speaker51Side, speaker51Side, true},
		{"7.1", 8, speaker71, speaker51Side, true},
		{"7.1 wide", 8, speaker71Wide, speaker51, true},
		{"six channels of no speakers", 6, 0, speaker51Side, true},
		{"six channels without an LFE", 6, speakerStereo | speakerFrontCenter | speakerBackLeft | speakerBackRight | 0x100, 0, false},
		{"five channels of 5.1's mask", 5, speaker51, 0, false},
	} {
		got, ok := surroundMask(c.channels, c.mask)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: mask %#x, %v; want %#x, %v", c.name, got, ok, c.want, c.ok)
		}
	}
	// The masks a Stream opens, by the order of its channels.
	if speaker51 != 0x3F || speaker51Side != 0x60F {
		t.Errorf("KSAUDIO_SPEAKER_5POINT1 %#x, _SURROUND %#x", speaker51, speaker51Side)
	}
}

// TestPulseSurroundMap checks the channel maps of PulseAudio streams of
// 5.1 on sinks of their positions.
func TestPulseSurroundMap(t *testing.T) {
	side := [Surround]int32{paFrontLeft, paFrontRight, paFrontCenter, paLFE, paSideLeft, paSideRight}
	rear := [Surround]int32{paFrontLeft, paFrontRight, paFrontCenter, paLFE, paRearLeft, paRearRight}
	for _, c := range []struct {
		name string
		sink []int32
		want [Surround]int32
		ok   bool
	}{
		{"stereo", []int32{paFrontLeft, paFrontRight}, [Surround]int32{}, false},
		{"5.1 of rear speakers", []int32{paFrontLeft, paFrontRight, paRearLeft, paRearRight, paFrontCenter, paLFE}, rear, true},
		{"5.1 of side speakers", []int32{paFrontLeft, paFrontRight, paFrontCenter, paLFE, paSideLeft, paSideRight}, side, true},
		{"7.1", []int32{paFrontLeft, paFrontRight, paFrontCenter, paLFE, paRearLeft, paRearRight, paSideLeft, paSideRight}, side, true},
		{"5.0", []int32{paFrontLeft, paFrontRight, paFrontCenter, paRearLeft, paRearRight}, [Surround]int32{}, false},
		{"six aux channels", []int32{12, 13, 14, 15, 16, 17}, [Surround]int32{}, false},
		{"unknown positions", []int32{-1, 99, paFrontLeft, paFrontRight, paFrontCenter, paLFE}, [Surround]int32{}, false},
	} {
		got, ok := pulseSurroundMap(c.sink)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// TestCoreAudioLayout checks the channels of Audio Queues, and the layout
// of 5.1 they tell Core Audio: kAudioChannelLayoutTag_MPEG_5_1_A, of L, R,
// C, LFE, Ls and Rs.
func TestCoreAudioLayout(t *testing.T) {
	if kAudioChannelLayoutTagMPEG51A != 0x790006 {
		t.Errorf("layout tag %#x", kAudioChannelLayoutTagMPEG51A)
	}
	for _, c := range []struct{ asked, device, want int }{
		{2, 2, 2}, {2, 6, 2}, {Surround, 2, 2}, {Surround, 6, Surround}, {Surround, 8, Surround}, {Surround, 0, 2},
	} {
		if got := coreAudioChannels(c.asked, c.device); got != c.want {
			t.Errorf("asking for %d on %d channels: %d", c.asked, c.device, got)
		}
	}
	// An AudioBufferList of a stream of 2 channels and one of 6, and one
	// that claims more buffers than it holds.
	list := func(n uint32, channels ...uint32) []byte {
		b := binary.LittleEndian.AppendUint32(nil, n)
		b = append(b, 0, 0, 0, 0)
		for _, c := range channels {
			b = binary.LittleEndian.AppendUint32(b, c)
			b = append(b, make([]byte, 12)...)
		}
		return b
	}
	for _, c := range []struct {
		b    []byte
		want int
	}{
		{list(1, 2), 2}, {list(2, 2, 6), 8}, {list(5, 6), 6}, {list(0), 0}, {[]byte{1, 0}, 0},
	} {
		if got := bufferListChannels(c.b); got != c.want {
			t.Errorf("% x: %d channels, want %d", c.b, got, c.want)
		}
	}
}
