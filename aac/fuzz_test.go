package aac

import (
	"errors"
	"math"
	"slices"
	"testing"
)

// fuzzInput builds an input of FuzzDecode: the decoder's settings, then
// each frame after its length, 16 bits.
func fuzzInput(settings byte, frames ...[]byte) []byte {
	in := []byte{settings}
	for _, f := range frames {
		in = append(in, byte(len(f)>>8), byte(len(f)))
		in = append(in, f...)
	}
	return in
}

// fuzzSettings is the settings byte of a decoder of cfg: bit 0 for
// stereo, then the sampling frequency index.
func fuzzSettings(cfg Config) byte {
	return byte(slices.Index(sampleRates[:], cfg.SampleRate))<<1 | byte(cfg.Channels-1)
}

// FuzzDecode decodes any bytes as a sequence of frames, and checks that
// the decoder doesn't panic, decodes a frame to FrameLength samples or
// refuses it with ErrFormat or errors.ErrUnsupported, and makes only
// finite samples.
func FuzzDecode(f *testing.F) {
	// Frames of each fixture, in a row, at a few places, and for the
	// decoder of another stream.
	for _, e := range fixtures {
		cfg, frames := fixtureFrames(f, e.name)
		for i := 0; i+3 <= len(frames); i += max(1, len(frames)/4) {
			f.Add(fuzzInput(fuzzSettings(cfg), frames[i:i+3]...))
			f.Add(fuzzInput(fuzzSettings(cfg)^1, frames[i:i+2]...))
		}
	}
	// Built frames: pulses, and the elements ffmpeg doesn't write.
	var a, b frameWriter
	pulsed := a.sce([4]int{20, 0, -40, 3}, &pulseSpec{0, []int{0, 2, 0, 1}, []int{15, 1, 2, 7}}).end()
	b.bits(elemDSE, 3).bits(0, 4).bits(1, 1).bits(3, 8).align().zeros(3)
	b.bits(elemPCE, 3).bits(0, 4).bits(1, 2).bits(4, 4).bits(1, 4).bits(0, 4).bits(0, 4).bits(0, 2).bits(0, 3).bits(0, 4)
	b.bits(0, 3).bits(0, 1).bits(0, 4).align().bits(1, 8).zeros(1)
	padded := b.sce([4]int{3, -2, 20, 1}, nil).bits(elemFIL, 3).bits(15, 4).bits(2, 8).zeros(16).end()
	f.Add(fuzzInput(fuzzSettings(mono44), pulsed, padded, pulsed))
	f.Add(fuzzInput(fuzzSettings(mono44)|1, pulsed))
	f.Add(fuzzInput(0))
	f.Add(fuzzInput(25, []byte{0xE0}))

	f.Fuzz(func(t *testing.T, in []byte) {
		if len(in) < 1 {
			return
		}
		cfg := Config{
			ObjectType: ObjectTypeLC,
			SampleRate: sampleRates[int(in[0]>>1)%len(sampleRates)],
			Channels:   int(in[0]&1) + 1,
		}
		d, err := NewDecoder(cfg)
		if err != nil {
			t.Fatal(err)
		}
		pcm := make([]float32, 2*FrameLength)
		in = in[1:]
		for len(in) >= 2 {
			n := min(int(in[0])<<8|int(in[1]), len(in)-2)
			frame := in[2 : 2+n]
			in = in[2+n:]
			got, err := d.Decode(frame, pcm)
			if err != nil {
				if got != 0 || !errors.Is(err, ErrFormat) && !errors.Is(err, errors.ErrUnsupported) {
					t.Fatalf("Decode = %d, %v", got, err)
				}
				continue
			}
			if got != FrameLength {
				t.Fatalf("decoded %d samples", got)
			}
			for i, v := range pcm {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatalf("sample %d is %v", i, v)
				}
			}
		}
	})
}

// FuzzParseConfig reads any bytes as an AudioSpecificConfig, and checks
// that ParseConfig doesn't panic, refuses it with ErrFormat or
// errors.ErrUnsupported, or returns a config a decoder decodes.
func FuzzParseConfig(f *testing.F) {
	for _, asc := range [][]byte{
		{0x12, 0x10},
		{0x11, 0x88},
		{0x2b, 0x92, 0x08, 0},
		{0x12, 0x00, 0x05, 0x04, 0x00, 0x00, 0x20, 0x01, 'L', 0x56, 0xe5, 0x00},
		asc([2]int{2, 5}, [2]int{15, 4}, [2]int{44056, 24}, [2]int{2, 4}, [2]int{0, 3}),
		asc([2]int{2, 5}, [2]int{7, 4}, [2]int{2, 4}, [2]int{0, 3}, [2]int{0x2B7, 11}, [2]int{5, 5}, [2]int{1, 1}, [2]int{4, 4}),
		asc([2]int{31, 5}, [2]int{1, 6}, [2]int{4, 4}, [2]int{2, 4}),
	} {
		f.Add(asc)
	}
	f.Fuzz(func(t *testing.T, asc []byte) {
		cfg, err := ParseConfig(asc)
		if err != nil {
			if !errors.Is(err, ErrFormat) && !errors.Is(err, errors.ErrUnsupported) {
				t.Fatalf("ParseConfig: %v", err)
			}
			return
		}
		if _, err := NewDecoder(cfg); err != nil {
			t.Fatalf("NewDecoder(%+v): %v", cfg, err)
		}
	})
}
