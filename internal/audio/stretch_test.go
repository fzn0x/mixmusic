package audio

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// sine is secs seconds of a sine of hz at amplitude 0.5, in stereo.
func sine(hz, secs float64) []float32 {
	n := int(secs * Rate)
	out := make([]float32, 2*n)
	for f := range n {
		v := float32(0.5 * math.Sin(2*math.Pi*hz*float64(f)/Rate))
		out[2*f], out[2*f+1] = v, v
	}
	return out
}

// chirp is secs seconds of a sine whose frequency rises evenly from f0 to
// f1, which tells where in it a moment is.
func chirp(f0, f1, secs float64) []float32 {
	n := int(secs * Rate)
	out := make([]float32, 2*n)
	phase := 0.0
	for f := range n {
		phase += 2 * math.Pi * (f0 + (f1-f0)*float64(f)/float64(n)) / Rate
		v := float32(0.5 * math.Sin(phase))
		out[2*f], out[2*f+1] = v, v
	}
	return out
}

// stretched is what a stretcher played of a track, a quantum at a time.
type stretched struct {
	out []float32
	// pos is the frame of the track that played at the start of each
	// quantum; bypassed tells which quanta it played as the ring had them.
	pos      []int64
	bypassed []bool
	// most is the most input the stretcher held.
	most int
}

// stretch plays in, interleaved stereo, through a stretcher at the speed
// speedAt gives each quantum, as the audio thread does, with the ring
// filled as the decoder fills it, to the end of the track.
func stretch(t testing.TB, in []float32, speedAt func(q int) float64) stretched {
	t.Helper()
	r := newRing(0)
	s := new(stretcher)
	var st stretched
	frames, written := len(in)/2, 0
	buf := make([]float32, 2*quantum)
	never := func(<-chan struct{}) bool { return false }
	limit := 20 + 4*frames/quantum
	for q := 0; ; q++ {
		if q > limit {
			t.Fatalf("the stretch plays on: %d quanta of %d frames", q, frames)
		}
		if written < frames {
			n := min(ringFrames-int(r.readable()), frames-written)
			r.writeFrom(in[2*written:2*(written+n)], never)
			if written += n; written == frames {
				r.done.Store(true)
			}
		}
		speed := speedAt(q)
		st.pos = append(st.pos, s.position(r))
		st.bypassed = append(st.bypassed, speed == 1 && (s.src != r || s.mode != stretching))
		n := s.read(r, buf, speed)
		st.out = append(st.out, buf[:2*n]...)
		st.most = max(st.most, s.have)
		if n < quantum {
			if !r.done.Load() || r.readable() > 0 || s.pending(r) {
				t.Fatalf("quantum %d: %d frames before the end", q, n)
			}
			return st
		}
	}
}

func at(speed float64) func(int) float64 { return func(int) float64 { return speed } }

var speeds = []float64{0.5, 0.75, 1.25, 1.5, 2, 3}

// zeroCrossHz is the frequency of the left channel of buf from frame f0 to
// f1, by its zero crossings.
func zeroCrossHz(buf []float32, f0, f1 int) float64 {
	n := 0
	for f := f0 + 1; f < f1; f++ {
		if (buf[2*(f-1)] < 0) != (buf[2*f] < 0) {
			n++
		}
	}
	return float64(n) / 2 / (float64(f1-f0) / Rate)
}

// maxStep is the largest step from a sample to the next of buf's left
// channel.
func maxStep(buf []float32) float64 {
	m := 0.0
	for f := 1; f < len(buf)/2; f++ {
		m = max(m, math.Abs(float64(buf[2*f]-buf[2*(f-1)])))
	}
	return m
}

// endStep is maxStep of a track that ends, into the silence after it too:
// a tone that stops abruptly ends in a step of its own.
func endStep(buf []float32) float64 { return maxStep(append(slices.Clone(buf), 0, 0)) }

// lengthSlack is how far from the length of the input over the speed the
// stretch of a track may end: its windows start within stretchSearch of
// where the speed puts them, the last one too, and end on a hop.
func lengthSlack(speed float64) float64 { return stretchSearch/speed + stretchHop }

// TestStretchPassThrough plays at 1× the ring as it is, sample for sample.
func TestStretchPassThrough(t *testing.T) {
	in := chirp(100, 9000, 3)
	got := stretch(t, in, at(1)).out
	if len(got) != len(in) {
		t.Fatalf("%d frames, want %d", len(got)/2, len(in)/2)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], in[i])
		}
	}
}

// TestStretchLength plays 10 s in 10 s over the speed, within
// lengthSlack.
func TestStretchLength(t *testing.T) {
	in := sine(440, 10)
	for _, speed := range speeds {
		st := stretch(t, in, at(speed))
		want := 10 * Rate / speed
		if d := float64(len(st.out)/2) - want; math.Abs(d) > lengthSlack(speed) {
			t.Errorf("at %v×: %d frames, want %.0f", speed, len(st.out)/2, want)
		}
		if st.most > stretchHeld {
			t.Errorf("at %v×: it held %d frames", speed, st.most)
		}
	}
}

// TestStretchKeepsPitch plays a sine of 440 Hz at 440 Hz at every speed.
func TestStretchKeepsPitch(t *testing.T) {
	in := sine(440, 6)
	for _, speed := range speeds {
		out := stretch(t, in, at(speed)).out
		mid := len(out) / 4
		if hz := zeroCrossHz(out, mid, mid+Rate/2); math.Abs(hz-440) > 2 {
			t.Errorf("at %v×: %.1f Hz", speed, hz)
		}
	}
}

// TestStretchNoDropouts plays a steady tone steadily: the level of each
// hop of the output stays within half a decibel of the others, away from
// the start and the end, where the windows run into silence.
func TestStretchNoDropouts(t *testing.T) {
	for _, hz := range []float64{97.3, 440, 2500} {
		in := sine(hz, 6)
		for _, speed := range speeds {
			out := stretch(t, in, at(speed)).out
			lo, hi := math.Inf(1), 0.0
			for f := Rate / 20; f+stretchHop < len(out)/2-Rate/20; f += stretchHop {
				var sum float64
				for i := f; i < f+stretchHop; i++ {
					sum += float64(out[2*i]) * float64(out[2*i])
				}
				r := math.Sqrt(sum / stretchHop)
				lo, hi = min(lo, r), max(hi, r)
			}
			if db := 20 * math.Log10(hi/lo); db > 0.5 {
				t.Errorf("%v Hz at %v×: the level varies by %.2f dB", hz, speed, db)
			}
		}
	}
}

// TestStretchNoClicks splices the windows where they go on in phase: no
// step from a sample to the next is larger than the signal's own.
func TestStretchNoClicks(t *testing.T) {
	two := sine(220, 6)
	for i, v := range sine(331, 6) {
		two[i] = 0.6*two[i] + 0.4*v
	}
	for name, in := range map[string][]float32{"440 Hz": sine(440, 6), "97.3 Hz": sine(97.3, 6), "two tones": two} {
		own := endStep(in)
		for _, speed := range speeds {
			if got := endStep(stretch(t, in, at(speed)).out); got > own*1.02 {
				t.Errorf("%s at %v×: a step of %.5f, the signal's largest is %.5f", name, speed, got, own)
			}
		}
	}
}

// TestStretchPosition tells what plays: on a chirp, whose frequency tells
// the moment of the input, the position is within 25 ms of what plays up
// to 2×, and 45 ms at 3×.
func TestStretchPosition(t *testing.T) {
	const secs, f0, f1 = 10.0, 200.0, 12000.0
	in := chirp(f0, f1, secs)
	for _, speed := range speeds {
		st := stretch(t, in, at(speed))
		worst := 0.0
		const span = 2400 // 50 ms
		for c := Rate / 2; c+span < len(st.out)/2-Rate/2; c += Rate / 10 {
			hz := zeroCrossHz(st.out, c-span/2, c+span/2)
			heard := (hz - f0) / (f1 - f0) * secs
			q := c / quantum
			p := float64(st.pos[q]) + float64(c-q*quantum)*speed
			worst = max(worst, math.Abs(heard-p/Rate))
		}
		limit := 0.025
		if speed > 2 {
			limit = 0.045
		}
		if worst > limit {
			t.Errorf("at %v×: the position is up to %.1f ms off", speed, worst*1000)
		} else {
			t.Logf("at %v×: the position is up to %.1f ms off", speed, worst*1000)
		}
	}
}

// TestStretchSpeedChange goes from 1× to 1.5× and back in the middle of a
// tone: without a click, in the time each speed takes, and back to the
// ring as it is, sample for sample.
func TestStretchSpeedChange(t *testing.T) {
	in := sine(440, 6)
	// 2 s at 1×, 2 s at 1.5×, then 1×: 2 + 3 + 1 s of the tone.
	speedAt := func(q int) float64 {
		if q*quantum >= 2*Rate && q*quantum < 4*Rate {
			return 1.5
		}
		return 1
	}
	st := stretch(t, in, speedAt)
	if got, own := endStep(st.out), endStep(in); got > own*1.02 {
		t.Errorf("a step of %.5f, the tone's largest is %.5f", got, own)
	}
	if d := float64(len(st.out)/2 - 5*Rate); math.Abs(d) > lengthSlack(1.5) {
		t.Errorf("%d frames, want %d", len(st.out)/2, 5*Rate)
	}
	back := false
	for q, bypassed := range st.bypassed {
		if !bypassed || q*quantum < 4*Rate {
			continue
		}
		back = true
		p := st.pos[q]
		for i := 2 * q * quantum; i < min(2*(q+1)*quantum, len(st.out)); i++ {
			if want := in[2*p+int64(i-2*q*quantum)]; st.out[i] != want {
				t.Fatalf("quantum %d, back at 1×: %v, want %v", q, st.out[i], want)
			}
		}
	}
	if !back {
		t.Error("it never played the ring as it is again")
	}
	// Speeds a step apart, at every quantum, splice as well.
	wobble := func(q int) float64 { return []float64{1, 1.05, 1.5, 1, 3, 0.5, 2.25}[q/7%7] }
	if got, own := endStep(stretch(t, in, wobble).out), endStep(in); got > own*1.02 {
		t.Errorf("changing the speed often: a step of %.5f, the tone's largest is %.5f", got, own)
	}
}

// TestStretchSeek starts over on the ring of a seek.
func TestStretchSeek(t *testing.T) {
	in := sine(440, 2)
	never := func(<-chan struct{}) bool { return false }
	a, b := newRing(0), newRing(Rate)
	a.writeFrom(in[:2*20000], never)
	b.writeFrom(in[2*Rate/2:2*(Rate/2+20000)], never)
	b2 := newRing(Rate)
	b2.writeFrom(in[2*Rate/2:2*(Rate/2+20000)], never)
	var s, fresh stretcher
	buf := make([]float32, 2*quantum)
	for range 40 {
		s.read(a, buf, 1.5)
	}
	if p := s.position(b); p != 0 {
		t.Errorf("the new ring's position %d", p)
	}
	want := make([]float32, 2*quantum)
	for q := range 40 {
		n := s.read(b, buf, 1.5)
		m := fresh.read(b2, want, 1.5)
		if n != m {
			t.Fatalf("quantum %d: %d frames, want %d", q, n, m)
		}
		for i := range 2 * n {
			if buf[i] != want[i] {
				t.Fatalf("quantum %d: what played before the seek plays on", q)
			}
		}
	}
}

// TestStretchEnd plays a short track to its end, at every speed: what the
// stretch holds plays after the ring ran out.
func TestStretchEnd(t *testing.T) {
	in := sine(440, 0.3)
	for _, speed := range append([]float64{1}, speeds...) {
		r := newRing(0)
		r.writeFrom(in, func(<-chan struct{}) bool { return false })
		r.done.Store(true)
		var s stretcher
		buf := make([]float32, 2*quantum)
		played, heldOn := 0, false
		for range 1000 {
			n := s.read(r, buf, speed)
			played += n
			if r.readable() == 0 && s.pending(r) {
				heldOn = true
			}
			if n < quantum {
				break
			}
		}
		if s.pending(r) {
			t.Errorf("at %v×: it has more to play after it played %d frames", speed, played)
		}
		if speed != 1 && !heldOn {
			t.Errorf("at %v×: it held nothing once the ring ran out", speed)
		}
		if want := 0.3 * Rate / speed; math.Abs(float64(played)-want) > lengthSlack(speed) {
			t.Errorf("at %v×: %d frames, want %.0f", speed, played, want)
		}
		if p := s.position(r); p != int64(len(in)/2) {
			t.Errorf("at %v×: it ends at frame %d of %d", speed, p, len(in)/2)
		}
	}
}

// TestStretchAllocs allocates nothing on the audio thread.
func TestStretchAllocs(t *testing.T) {
	in := sine(440, 1)
	r := newRing(0)
	r.writeFrom(in, func(<-chan struct{}) bool { return false })
	s := new(stretcher)
	buf := make([]float32, 2*quantum)
	speed := 1.5
	if n := testing.AllocsPerRun(100, func() {
		s.read(r, buf, speed)
		speed = 4.5 - speed // 1.5, 3, 1.5…
	}); n != 0 {
		t.Errorf("%v allocations a quantum", n)
	}
}

// BenchmarkStretch plays 10 s of a chirp at 1.5× and 3×, as a deck plays
// it, and tells how many times faster than it plays the stretch runs.
func BenchmarkStretch(b *testing.B) {
	in := chirp(200, 9000, 10)
	never := func(<-chan struct{}) bool { return false }
	buf := make([]float32, 2*quantum)
	for _, speed := range []float64{1.5, 3} {
		b.Run(fmt.Sprint(speed), func(b *testing.B) {
			played := 0
			for b.Loop() {
				r, s := newRing(0), new(stretcher)
				for written := 0; ; {
					if n := min(ringFrames-int(r.readable()), len(in)/2-written); n > 0 {
						r.writeFrom(in[2*written:2*(written+n)], never)
						written += n
					} else if written == len(in)/2 {
						r.done.Store(true)
					}
					n := s.read(r, buf, speed)
					played += n
					if n < quantum {
						break
					}
				}
			}
			b.ReportMetric(float64(played)/Rate/b.Elapsed().Seconds(), "x-realtime")
		})
	}
}

// TestStretchListen writes the test files at each speed to
// MIXMUSIC_STRETCH_WAV, to listen to.
func TestStretchListen(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_STRETCH_WAV")
	if dir == "" {
		t.Skip("MIXMUSIC_STRETCH_WAV names no folder")
	}
	for name, path := range map[string]string{"speech": speech, "stereo": cuesFront} {
		in := reference(t, path)
		for _, speed := range append([]float64{1}, speeds...) {
			out := stretch(t, in, at(speed)).out
			if err := writeWAV(filepath.Join(dir, fmt.Sprintf("%s-%.2fx.wav", name, speed)), out); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// writeWAV writes interleaved stereo at Rate as 16-bit PCM.
func writeWAV(path string, pcm []float32) error {
	data := make([]byte, 44+2*len(pcm))
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*len(pcm)))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], Rate)
	binary.LittleEndian.PutUint32(data[28:], Rate*4)
	binary.LittleEndian.PutUint16(data[32:], 4)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*len(pcm)))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(int16(max(-1, min(1, v))*32767)))
	}
	return os.WriteFile(path, data, 0o644)
}

// TestStretchStartsOnceFilled plays what the ring holds as it is while it
// holds less than the stretch's windows need, as when a track starts, and
// stretches once it holds them, without a seam.
func TestStretchStartsOnceFilled(t *testing.T) {
	in := sine(440, 1)
	never := func(<-chan struct{}) bool { return false }
	r := newRing(0)
	r.writeFrom(in[:2*1000], never)
	var s stretcher
	buf := make([]float32, 2*quantum)
	var out []float32
	for range 4 {
		n := s.read(r, buf, 2)
		out = append(out, buf[:2*n]...)
	}
	if len(out) != 2*4*quantum || s.mode != bypassed {
		t.Fatalf("%d frames played, as the mode %d", len(out)/2, s.mode)
	}
	for i := range out {
		if out[i] != in[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, out[i], in[i])
		}
	}
	r.writeFrom(in[2*1000:2*20000], never)
	for range 40 {
		n := s.read(r, buf, 2)
		out = append(out, buf[:2*n]...)
	}
	if s.mode != stretching {
		t.Errorf("the mode %d once the ring filled", s.mode)
	}
	if got, own := maxStep(out), maxStep(in); got > own*1.02 {
		t.Errorf("a step of %.5f, the tone's largest is %.5f", got, own)
	}
	if p := s.position(r); p < 4*quantum+2*36*quantum-stretchSearch || p > 4*quantum+2*36*quantum+stretchSearch {
		t.Errorf("at frame %d after 4 quanta at 1× and 36 at 2×", p)
	}
}

// TestStretchShortOfInput plays what there is as it is when the ring runs
// short, as when the network is slow, rather than nothing, and stretches
// again once the ring holds enough.
func TestStretchShortOfInput(t *testing.T) {
	in := sine(440, 2)
	never := func(<-chan struct{}) bool { return false }
	r := newRing(0)
	r.writeFrom(in[:2*20000], never)
	var s stretcher
	buf := make([]float32, 2*quantum)
	var out []float32
	stretched := false
	for range 1000 {
		n := s.read(r, buf, 2)
		out = append(out, buf[:2*n]...)
		stretched = stretched || s.mode == stretching
		if n < quantum {
			break
		}
	}
	// It stops only once all the ring held played.
	if p := s.position(r); !stretched || p != 20000 || r.readable() != 0 {
		t.Fatalf("stretched %v, stopped at frame %d with %d left", stretched, p, r.readable())
	}
	r.writeFrom(in[2*20000:2*60000], never)
	stretched = false
	for range 40 {
		n := s.read(r, buf, 2)
		out = append(out, buf[:2*n]...)
		stretched = stretched || s.mode == stretching
	}
	if !stretched {
		t.Error("it didn't stretch again once the ring filled")
	}
	if got, own := maxStep(out), maxStep(in); got > own*1.02 {
		t.Errorf("a step of %.5f, the tone's largest is %.5f", got, own)
	}
}
