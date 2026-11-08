package audio

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

// mp3Rates are the bitrate indexes of MPEG-1 Layer III, by kbit/s.
var mp3Rates = map[int]byte{32: 1, 40: 2, 48: 3, 56: 4, 64: 5, 80: 6, 96: 7, 112: 8, 128: 9, 160: 10, 192: 11, 224: 12, 256: 13, 320: 14}

// The channel modes of a frame's fourth byte.
const (
	mp3Stereo byte = 0x00
	mp3Mono   byte = 0xC0
)

// silentFrame is a frame of MPEG-1 Layer III at 48 kHz, of kbps, in mode,
// without a CRC: 3 bytes for each kbit/s, of side information and audio
// all zero, which decodes to silence.
func silentFrame(kbps int, mode byte) []byte {
	f := make([]byte, 3*kbps)
	f[0], f[1], f[2], f[3] = 0xFF, 0xFB, mp3Rates[kbps]<<4|1<<2, mode
	return f
}

// silentMP3 is an MP3 file of silent frames in mode, one of each of kbps,
// after a first frame of 128 kbit/s, which head writes a header in, as
// encoders write theirs, or none when head is nil.
func silentMP3(mode byte, head func(frame []byte), kbps ...int) []byte {
	var b []byte
	if head != nil {
		f := silentFrame(128, mode)
		head(f)
		b = append(b, f...)
	}
	for _, k := range kbps {
		b = append(b, silentFrame(k, mode)...)
	}
	return b
}

// xingHead writes a Xing or Info header of frames, after the side
// information, with the bytes of the file too, and a LAME tag of delay and
// padding after it when lame.
func xingHead(id string, frames uint32, lame bool, delay, padding int) func([]byte) {
	return func(f []byte) {
		x := f[4+32:]
		if f[3] == mp3Mono {
			x = f[4+17:]
		}
		copy(x, id)
		binary.BigEndian.PutUint32(x[4:], 1|2) // the frames, and the bytes
		binary.BigEndian.PutUint32(x[8:], frames)
		binary.BigEndian.PutUint32(x[12:], 123456)
		if lame {
			t := x[16:]
			copy(t, "LAME3.100")
			t[21], t[22], t[23] = byte(delay>>4), byte(delay<<4|padding>>8), byte(padding)
		}
	}
}

// vbriHead writes a VBRI header of frames, of version, 32 bytes after the
// frame's header in either mode.
func vbriHead(version uint16, frames uint32) func([]byte) {
	return func(f []byte) {
		v := f[4+32:]
		copy(v, "VBRI")
		binary.BigEndian.PutUint16(v[4:], version)
		binary.BigEndian.PutUint16(v[6:], 1104) // the encoder's delay
		binary.BigEndian.PutUint16(v[8:], 75)   // the quality
		binary.BigEndian.PutUint32(v[10:], 123456)
		binary.BigEndian.PutUint32(v[14:], frames)
		binary.BigEndian.PutUint16(v[18:], 0) // no table of contents
		binary.BigEndian.PutUint16(v[20:], 1)
		binary.BigEndian.PutUint16(v[22:], 2)
		binary.BigEndian.PutUint16(v[24:], 0)
	}
}

// sameRate is n kbps.
func sameRate(kbps, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = kbps
	}
	return out
}

// TestProbeMP3Length reads the length of MP3 files of silence: of a
// variable bitrate, from the frames their Xing or VBRI header counts, less
// their LAME tag's delay and padding, where they have one, and of a
// constant bitrate without a header, from their size. 40 frames of 1152
// samples at 48 kHz play 960 ms; as of the first frame's bitrate, the
// files of a variable bitrate would play 264 ms. A deck, which reads every
// frame, plays them as long, and the header's frame as silence too, but of
// a file with a LAME tag.
func TestProbeMP3Length(t *testing.T) {
	low := sameRate(32, 40)
	id3 := append([]byte("ID3\x03\x00\x00\x00\x00\x00\x64"), make([]byte, 100)...)
	ms := time.Millisecond
	for _, c := range []struct {
		name       string
		file       []byte
		want, deck time.Duration
	}{
		{"Xing with a LAME tag", silentMP3(mp3Stereo, xingHead("Xing", 40, true, 576, 1152), low...), 924 * ms, 924 * ms},
		{"Xing without a LAME tag", silentMP3(mp3Stereo, xingHead("Xing", 40, false, 0, 0), low...), 960 * ms, 984 * ms},
		{"Xing in mono", silentMP3(mp3Mono, xingHead("Xing", 40, false, 0, 0), low...), 960 * ms, 984 * ms},
		{"VBRI", silentMP3(mp3Stereo, vbriHead(1, 40), low...), 960 * ms, 984 * ms},
		{"VBRI in mono", silentMP3(mp3Mono, vbriHead(1, 40), low...), 960 * ms, 984 * ms},
		{"constant bitrate without a header", silentMP3(mp3Stereo, nil, sameRate(128, 40)...), 960 * ms, 960 * ms},
		{"Info without a LAME tag, after ID3", append(id3, silentMP3(mp3Stereo, xingHead("Info", 40, false, 0, 0), sameRate(128, 40)...)...), 960 * ms, 984 * ms},
		// Of the first frame's bitrate then.
		{"VBRI of another version", silentMP3(mp3Stereo, vbriHead(2, 40), low...), 264 * ms, 984 * ms},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got, err := Probe(bytes.NewReader(c.file)); err != nil || got != c.want {
				t.Errorf("Probe = %v, %v; want %v", got, err, c.want)
			}
			_, tm, err := openTrack(bytes.NewReader(c.file))
			if err != nil {
				t.Fatal(err)
			}
			if got := tm.total - tm.delay; got != c.deck {
				t.Errorf("a deck plays %v, want %v", got, c.deck)
			}
		})
	}
}

// TestMP3Gap tells the gap of a file's LAME tag only: a Xing or VBRI
// header alone tells no delay or padding.
func TestMP3Gap(t *testing.T) {
	low := sameRate(32, 40)
	// Past the header's frame, the encoder's delay and the decoder's, to
	// the end of the 40 frames, before the padding.
	start := int64(1152 + 576 + mp3DecoderDelay)
	end := start + 40*1152 - 576 - 1152
	for _, c := range []struct {
		name       string
		file       []byte
		start, end int64
	}{
		{"LAME", silentMP3(mp3Stereo, xingHead("Xing", 40, true, 576, 1152), low...), start, end},
		{"LAME in mono", silentMP3(mp3Mono, xingHead("Xing", 40, true, 576, 1152), low...), start, end},
		{"Xing", silentMP3(mp3Stereo, xingHead("Xing", 40, false, 0, 0), low...), 0, 0},
		{"VBRI", silentMP3(mp3Stereo, vbriHead(1, 40), low...), 0, 0},
		{"none", silentMP3(mp3Stereo, nil, low...), 0, 0},
	} {
		if start, end := mp3Gap(bytes.NewReader(c.file)); start != c.start || end != c.end {
			t.Errorf("%s: from %d to %d, want from %d to %d", c.name, start, end, c.start, c.end)
		}
	}
}
