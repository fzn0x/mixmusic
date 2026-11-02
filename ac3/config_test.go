package ac3

import (
	"errors"
	"testing"
)

func TestParseDAC3(t *testing.T) {
	for _, c := range []struct {
		name string
		b    []byte
		want Config
		err  error
	}{
		// fscod 0, bsid 8, bsmod 0, acmod 7, lfeon 1, bit_rate_code 14.
		{"5.1 at 384", []byte{0x10, 0x3D, 0xC0}, Config{SampleRate: 48000, BSID: 8, Mode: Mode32, LFE: true, Bitrate: 384000}, nil},
		// fscod 1, bsid 6, bsmod 0, acmod 2, lfeon 0, bit_rate_code 10.
		{"stereo at 192", []byte{0x4C, 0x11, 0x40}, Config{SampleRate: 44100, BSID: 6, Mode: ModeStereo, Bitrate: 192000}, nil},
		{"short", []byte{0x10, 0x3D}, Config{}, ErrFormat},
		{"fscod 3", []byte{0xD0, 0x3D, 0xC0}, Config{}, ErrFormat},
		{"bit_rate_code 19", []byte{0x10, 0x3E, 0x60}, Config{}, ErrFormat},
		{"E-AC-3's bsid", []byte{0x20, 0x3D, 0xC0}, Config{}, errors.ErrUnsupported},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDAC3(c.b)
			if !errors.Is(err, c.err) || (c.err == nil && err != nil) {
				t.Fatalf("error %v, want %v", err, c.err)
			}
			if got != c.want {
				t.Errorf("%+v, want %+v", got, c.want)
			}
		})
	}
	if n := (Config{Mode: Mode32, LFE: true}).Channels(); n != 6 {
		t.Errorf("5.1 has %d channels", n)
	}
}

func TestParseDEC3(t *testing.T) {
	for _, c := range []struct {
		name string
		b    []byte
		want Config
		err  error
	}{
		// data_rate 384, num_ind_sub 0; fscod 0, bsid 16, asvc 0, bsmod
		// 0, acmod 7, lfeon 1, num_dep_sub 0, reserved.
		{"5.1 at 384", []byte{0x0C, 0x00, 0x20, 0x0F, 0x00}, Config{SampleRate: 48000, BSID: 16, Mode: Mode32, LFE: true, Bitrate: 384000}, nil},
		// data_rate 96; fscod 1, bsid 16, acmod 2, and a substream of 7.1
		// after.
		{"stereo at 96", []byte{0x03, 0x00, 0x60, 0x04, 0x02, 0x01, 0x00}, Config{SampleRate: 44100, BSID: 16, Mode: ModeStereo, Bitrate: 96000}, nil},
		// ffmpeg's, of 5.1 at 192 kbit/s, with a byte of padding.
		{"ffmpeg's", []byte{0x06, 0x00, 0x20, 0x0F, 0x00, 0x00}, Config{SampleRate: 48000, BSID: 16, Mode: Mode32, LFE: true, Bitrate: 192000}, nil},
		{"short", []byte{0x0C, 0x00, 0x20, 0x0F}, Config{}, ErrFormat},
		{"reduced rate", []byte{0x0C, 0x00, 0xE0, 0x0F, 0x00}, Config{}, errors.ErrUnsupported},
		{"AC-3's bsid", []byte{0x0C, 0x00, 0x10, 0x0F, 0x00}, Config{}, ErrFormat},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseDEC3(c.b)
			if !errors.Is(err, c.err) || (c.err == nil && err != nil) {
				t.Fatalf("error %v, want %v", err, c.err)
			}
			if got != c.want {
				t.Errorf("%+v, want %+v", got, c.want)
			}
		})
	}
}
