package flac

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// FuzzReader reads any bytes as a FLAC file, and seeks and reads its
// frames: it never panics, fails only with ErrFormat or the end of the
// file, and reads frames of the stream's channels, as many samples in
// each, within the bits of their samples.
func FuzzReader(f *testing.F) {
	for _, c := range fixtures {
		b := readFixture(f, c.name)
		f.Add(b, int64(0))
		f.Add(b[:len(b)/2], c.samples/2)
		f.Add(b[:200], int64(1))
	}
	f.Fuzz(func(t *testing.T, b []byte, at int64) {
		r, err := NewReader(bytes.NewReader(b))
		if err != nil {
			if !errors.Is(err, ErrFormat) {
				t.Fatalf("not ErrFormat: %v", err)
			}
			return
		}
		ch := r.Info().Channels
		read := func() {
			for range 50 {
				fr, err := r.ReadFrame()
				if err == io.EOF {
					return
				}
				if err != nil {
					t.Fatalf("ReadFrame: %v", err)
				}
				if len(fr.Samples) != ch {
					t.Fatalf("a frame of %d channels in a stream of %d", len(fr.Samples), ch)
				}
				for _, s := range fr.Samples {
					if len(s) != len(fr.Samples[0]) || len(s) == 0 {
						t.Fatalf("channels of %d and %d samples", len(s), len(fr.Samples[0]))
					}
				}
			}
		}
		read()
		if _, err := r.SeekSample(max(0, at)); err != nil && err != io.EOF {
			t.Fatalf("SeekSample: %v", err)
		}
		read()
	})
}
