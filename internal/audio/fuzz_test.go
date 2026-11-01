package audio

import (
	"bytes"
	"errors"
	"math"
	"os"
	"slices"
	"testing"
	"time"
)

// FuzzTrack opens any bytes as a deck does, as a WebM or an MP4 file, or
// HLS packed audio, and seeks, reads and decodes their track: it never panics, fails only with
// the deck's words, and makes finite frames, as many as it says, of which
// those to play are within them.
func FuzzTrack(f *testing.F) {
	for _, path := range append([]string{cuesFront, monoStream}, aacFiles...) {
		b, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b, int64(0))
		f.Add(b[:len(b)/2], int64(time.Second))
	}
	packed := slices.Concat(packedSegments(f, 4, adtsMono22, adtsClicks)...)
	f.Add(packed, int64(0))
	f.Add(packed[:len(packed)/3], int64(time.Second))
	f.Fuzz(func(t *testing.T, b []byte, at int64) {
		worded := func(err error) bool {
			return errors.Is(err, errUnsupported) || errors.Is(err, errNetwork) || errors.Is(err, errDecode)
		}
		tr, tm, err := openTrack(bytes.NewReader(b))
		if err != nil {
			if !worded(err) {
				t.Fatalf("an error not worded for the player: %v", err)
			}
			return
		}
		if tm.delay < 0 || tm.preRoll <= 0 && !tm.live {
			t.Fatalf("timing %+v", tm)
		}
		check := func(pcm []float32, from, to int) {
			if len(pcm)%2 != 0 || from < 0 || from > to || to > len(pcm)/2 {
				t.Fatalf("%d floats, playing %d to %d", len(pcm), from, to)
			}
			for _, v := range pcm {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatalf("a sample of %v", v)
				}
			}
		}
		for _, to := range []time.Duration{0, time.Duration(at)} {
			if err := tr.seek(max(0, to)); err != nil {
				continue
			}
			for range 200 {
				if tr.read() != nil {
					break
				}
				p, err := tr.decode()
				if err != nil {
					if !worded(err) {
						t.Fatalf("an error not worded for the player: %v", err)
					}
					break
				}
				check(p.pcm, p.from, p.to)
			}
			pcm := tr.end()
			check(pcm, 0, len(pcm)/2)
		}
	})
}
