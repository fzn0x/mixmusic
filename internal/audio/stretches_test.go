package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// wavOf is a WAV file of 16-bit PCM of ch channels at rate, of pcm's
// interleaved frames.
func wavOf(rate, ch int, pcm []float32) []byte {
	data := make([]byte, 44+2*len(pcm))
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*len(pcm)))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], uint16(ch))
	binary.LittleEndian.PutUint32(data[24:], uint32(rate))
	binary.LittleEndian.PutUint32(data[28:], uint32(rate*ch*2))
	binary.LittleEndian.PutUint16(data[32:], uint16(ch*2))
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*len(pcm)))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(int16(max(-1, min(1, v))*32767)))
	}
	return data
}

// readStretch reads the stretch from from to to of file.
func readStretch(t *testing.T, file []byte, from, to time.Duration) []float32 {
	t.Helper()
	s, err := OpenStretches(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	var out []float32
	if err := s.Read(from, to, func(mono []float32) { out = append(out, mono...) }); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestStretchesReadTheFramesOfTheirTimes reads the frames of a stretch of
// a file at Rate, from the frame of its start, and mixes its channels
// down to their mean.
func TestStretchesReadTheFramesOfTheirTimes(t *testing.T) {
	const n = 3 * Rate
	pcm := make([]float32, 2*n)
	ramp := func(i int) float32 { return float32(i%1000)/1000 - 0.5 }
	for i := range n {
		pcm[2*i], pcm[2*i+1] = ramp(i)+0.25, ramp(i)-0.25
	}
	file := wavOf(Rate, 2, pcm)
	s, err := OpenStretches(bytes.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	if s.Length() != 3*time.Second {
		t.Errorf("length = %v", s.Length())
	}
	got := readStretch(t, file, 1500*time.Millisecond, 2*time.Second)
	if len(got) != Rate/2 {
		t.Fatalf("%d frames, want %d", len(got), Rate/2)
	}
	for i, v := range got {
		if want := ramp(Rate*3/2 + i); math.Abs(float64(v-want)) > 2.0/32767 {
			t.Fatalf("frame %d = %v, want %v", i, v, want)
		}
	}
	// A stretch past the end stops at it.
	if got := readStretch(t, file, 2900*time.Millisecond, 4*time.Second); len(got) != Rate/10 {
		t.Errorf("%d frames to the end, want %d", len(got), Rate/10)
	}
}

// TestStretchesOfAFileAtAnotherRate converts it to Rate as a deck does: a
// click at 1.05 s of a file at 8 kHz is at 1.05 s of its stretch.
func TestStretchesOfAFileAtAnotherRate(t *testing.T) {
	const rate = 8000
	pcm := make([]float32, 2*rate)
	pcm[rate*105/100] = 0.9
	got := readStretch(t, wavOf(rate, 1, pcm), time.Second, 1100*time.Millisecond)
	if len(got) != Rate/10 {
		t.Fatalf("%d frames, want %d", len(got), Rate/10)
	}
	peak := 0
	for i, v := range got {
		if math.Abs(float64(v)) > math.Abs(float64(got[peak])) {
			peak = i
		}
	}
	if want := Rate / 20; peak < want-6 || peak > want+6 {
		t.Errorf("the click is at frame %d of the stretch, want %d", peak, want)
	}
}

// TestStretchesOfOpusPlayAsTheWholeTrack reads a stretch of an Opus track
// as decoding the track from its start plays it, from the frame of its
// time, to the millisecond WebM tells the times of packets in.
func TestStretchesOfOpusPlayAsTheWholeTrack(t *testing.T) {
	file, err := os.ReadFile(filepath.Join("..", "..", "webm", "testdata", "stereo-cues-front.webm"))
	if err != nil {
		t.Fatal(err)
	}
	whole := readStretch(t, file, 0, 3*time.Second)
	part := readStretch(t, file, time.Second, 1500*time.Millisecond)
	if len(part) != Rate/2 || len(whole) < Rate*3/2 {
		t.Fatalf("%d and %d frames", len(part), len(whole))
	}
	// differs is how much of the stretch differs from the track lag frames
	// on from its time: the decoder settles within the seek's pre-roll.
	differs := func(lag int) float64 {
		var diff, power float64
		for i, v := range part {
			d := float64(v - whole[Rate+lag+i])
			diff += d * d
			power += float64(v) * float64(v)
		}
		return diff / power
	}
	best := 0
	for lag := -2 * Rate / 1000; lag <= 2*Rate/1000; lag++ {
		if differs(lag) < differs(best) {
			best = lag
		}
	}
	if best < -Rate/1000 || best > Rate/1000 || differs(best) > 1e-3 {
		t.Errorf("the stretch is the track %d frames on, and differs by %g", best, differs(best))
	}
}
