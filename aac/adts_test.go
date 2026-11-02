package aac

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// readADTS splits an ADTS stream, such as ffmpeg -f adts writes, into the
// raw data blocks of its frames, and returns the config its first header
// gives. A stream of channel configuration 0 holds the program config
// element in its first frame: the config then has no channels.
func readADTS(b []byte) (Config, [][]byte, error) {
	var cfg Config
	var frames [][]byte
	for len(b) > 0 {
		h, err := ParseADTS(b)
		if err != nil {
			return cfg, nil, err
		}
		if h.Blocks != 1 {
			return cfg, nil, errors.New("adts: frames of more than one raw data block")
		}
		if h.Length > len(b) {
			return cfg, nil, fmt.Errorf("adts: frame of %d bytes", h.Length)
		}
		if frames == nil {
			cfg = h.Config
		}
		frames = append(frames, b[h.HeaderLength:h.Length])
		b = b[h.Length:]
	}
	return cfg, frames, nil
}

// TestParseADTS reads the headers of the fixtures, as ffmpeg wrote them,
// and headers made up for what ffmpeg doesn't write: CRCs, MPEG-2's ID,
// several raw data blocks, and the bytes that are no header.
func TestParseADTS(t *testing.T) {
	for _, c := range []struct {
		name     string
		rate     int
		channels int
	}{
		{"stereo-44k-clicks", 44100, 2},
		{"mono-48k-bursts", 48000, 1},
		{"mono-22k", 22050, 1},
		{"stereo-32k", 32000, 2},
		{"stereo-44k-pce", 44100, 0},
	} {
		data, err := os.ReadFile(filepath.Join("testdata", c.name+".aac"))
		if err != nil {
			t.Fatal(err)
		}
		h, err := ParseADTS(data)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		want := Config{ObjectType: ObjectTypeLC, SampleRate: c.rate, Channels: c.channels}
		if h.Config != want || h.HeaderLength != 7 || h.Blocks != 1 || h.Length <= 7 || h.Length > len(data) {
			t.Errorf("%s: %+v, want %+v of one block", c.name, h, want)
		}
		if err := h.Decodable(); (err == nil) != (c.channels > 0) {
			t.Errorf("%s: decodable: %v", c.name, err)
		}
		// The frames follow one another to the end of the file.
		n := 0
		for b := data; len(b) > 0; n++ {
			h, err := ParseADTS(b)
			if err != nil || h.Length > len(b) || h.Config.SampleRate != c.rate {
				t.Fatalf("%s: frame %d: %+v, %v", c.name, n, h, err)
			}
			b = b[h.Length:]
		}
		if n < 10 {
			t.Errorf("%s: %d frames", c.name, n)
		}
	}

	// A protected MPEG-2 frame of LC at 44.1 kHz in stereo, of 300 bytes
	// and two raw data blocks.
	b := header(1, 4, 2, 300, 1)
	b[1] |= 8  // ID: MPEG-2
	b[1] &^= 1 // protection_absent: 0
	h, err := ParseADTS(b)
	want := ADTSHeader{Config: Config{ObjectType: ObjectTypeLC, SampleRate: 44100, Channels: 2}, Length: 300, HeaderLength: 9, Blocks: 2}
	if err != nil || h != want {
		t.Errorf("a protected frame: %+v, %v; want %+v", h, err, want)
	}
	if err := h.Decodable(); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("two raw data blocks: %v", err)
	}
	// AAC Main, and 5.1 surround.
	for _, h := range []ADTSHeader{
		{Config: Config{ObjectType: ObjectTypeMain, SampleRate: 44100, Channels: 2}, Blocks: 1},
		{Config: Config{ObjectType: ObjectTypeLC, SampleRate: 44100, Channels: 6}, Blocks: 1},
	} {
		if err := h.Decodable(); !errors.Is(err, errors.ErrUnsupported) {
			t.Errorf("%+v: %v", h, err)
		}
	}

	for _, c := range []struct {
		name string
		b    []byte
	}{
		{"short", header(1, 4, 2, 300, 0)[:6]},
		{"an ID3 tag", []byte("ID3\x04\x00\x00\x00\x00\x00\x3f")},
		{"half a syncword", []byte{0xFF, 0xE1, 0x50, 0x80, 0x2E, 0x1F, 0xFC}},
		{"layer 1", func() []byte { b := header(1, 4, 2, 300, 0); b[1] |= 2; return b }()},
		{"a reserved sampling frequency", header(1, 13, 2, 300, 0)},
		{"a frame shorter than its header", header(1, 4, 2, 7, 0)},
		{"a protected frame shorter than its header", func() []byte { b := header(1, 4, 2, 9, 0); b[1] &^= 1; return b }()},
	} {
		if h, err := ParseADTS(c.b); !errors.Is(err, ErrFormat) {
			t.Errorf("%s: %+v, %v", c.name, h, err)
		}
	}
}

// header makes the 7 bytes of an unprotected MPEG-4 ADTS header of profile,
// sampling frequency index rate, channel configuration channels, a frame
// of length bytes, and blocks+1 raw data blocks.
func header(profile, rate, channels, length, blocks int) []byte {
	return []byte{
		0xFF, 0xF1,
		byte(profile<<6 | rate<<2 | channels>>2),
		byte(channels&3<<6 | length>>11),
		byte(length >> 3),
		byte(length&7<<5 | 0x1F),
		byte(0xFC | blocks),
	}
}
