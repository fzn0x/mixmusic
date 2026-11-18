package audio

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

// noise is n frames of ch channels of noise, from -1 to 1.
func noise(n, ch int, seed uint64) []float32 {
	r := rand.New(rand.NewPCG(seed, 1))
	buf := make([]float32, n*ch)
	for i := range buf {
		buf[i] = float32(r.Float64()*2 - 1)
	}
	return buf
}

// soundTone is n frames of ch channels, each a sine of hz at amp, the first
// frame at sample from.
func soundTone(n, ch int, hz float64, amp [maxChannels]float64, from int) []float32 {
	buf := make([]float32, n*ch)
	for f := range n {
		v := math.Sin(2 * math.Pi * hz * float64(from+f) / Rate)
		for c := range ch {
			buf[f*ch+c] = float32(amp[c] * v)
		}
	}
	return buf
}

// run passes buf through f a quantum at a time, as the engine does.
func run(f *sound, buf []float32, ch int) []float32 {
	out := slices.Clone(buf)
	for i := 0; i < len(out); i += ch * quantum {
		f.process(out[i:min(i+ch*quantum, len(out))], ch)
	}
	return out
}

// settled is a sound at s, its glides done.
func settledSound(s Sound) *sound {
	f := newSound()
	f.set(s)
	for _, g := range []*smoother{&f.width, &f.crossfeed, &f.mono, &f.balance} {
		g.value = g.target
	}
	return f
}

var neutral = Sound{Width: 1}

// TestSoundNeutralIsSkipped leaves every sample as it was while each stage
// is neutral, at first and once a change glided back, on two channels and
// on six.
func TestSoundNeutralIsSkipped(t *testing.T) {
	for _, ch := range []int{2, 6} {
		in := noise(20*quantum, ch, 1)
		if got := run(newSound(), in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: a new sound changed the samples", ch)
		}
		f := newSound()
		f.set(Sound{Balance: -0.4, Mono: true, Crossfeed: 0.8, Width: 1.6})
		run(f, noise(20*quantum, ch, 2), ch)
		f.set(neutral)
		run(f, noise(Rate, ch, 3), ch) // a second: fifty time constants
		if got := run(f, in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: back at neutral, the stages still change the samples", ch)
		}
	}
}

// TestSoundBalance turns the side the balance leans from down, the other
// staying as it is: on 5.1, the left channels weigh against the right
// ones, and the centre and the bass channel stay.
func TestSoundBalance(t *testing.T) {
	for _, c := range []struct {
		balance float64
		gl, gr  float32
	}{{1, 0, 1}, {-1, 1, 0}, {0.25, 0.75, 1}, {-0.6, 1, 0.4}} {
		for _, ch := range []int{2, 6} {
			in := noise(4*quantum, ch, 4)
			got := run(settledSound(Sound{Balance: c.balance, Width: 1}), in, ch)
			for i := range got {
				want := in[i]
				switch i % ch {
				case 0, 4:
					want *= c.gl
				case 1, 5:
					want *= c.gr
				}
				if got[i] != want {
					t.Fatalf("balance %v, %d channels, sample %d: %v, want %v", c.balance, ch, i, got[i], want)
				}
			}
		}
	}
}

// TestSoundMono plays each pair of channels mixed on both, the centre and
// the bass channel of 5.1 as they were.
func TestSoundMono(t *testing.T) {
	for _, ch := range []int{2, 6} {
		in := noise(4*quantum, ch, 5)
		got := run(settledSound(Sound{Mono: true, Width: 1}), in, ch)
		n := 1
		if ch == 6 {
			n = 2
		}
		for f := 0; f < len(in); f += ch {
			for _, p := range pairs[:n] {
				mid := (in[f+p[0]] + in[f+p[1]]) / 2
				if math.Abs(float64(got[f+p[0]]-mid)) > 1e-6 || math.Abs(float64(got[f+p[1]]-mid)) > 1e-6 {
					t.Fatalf("%d channels, frame %d: %v and %v, want both %v", ch, f/ch, got[f+p[0]], got[f+p[1]], mid)
				}
			}
			if ch == 6 && (got[f+2] != in[f+2] || got[f+3] != in[f+3]) {
				t.Fatalf("frame %d: mono changed the centre or the bass channel", f/ch)
			}
		}
	}
}

// TestSoundGlides moves each stage to a new setting over the glide, a
// little each sample, so as not to click: a steady signal changes by less
// than a thousandth of full scale from one sample to the next.
func TestSoundGlides(t *testing.T) {
	for _, s := range []Sound{
		{Balance: 1, Width: 1}, {Balance: -1, Width: 1}, {Mono: true, Width: 1},
		{Crossfeed: 1, Width: 1}, {Width: 0}, {Width: 2},
	} {
		f := newSound()
		// A steady signal, louder on the left, as the stages hear it:
		// the crossfeed's copies of it are up to speed.
		steady := make([]float32, 2*Rate/10)
		for i := 0; i < len(steady); i += 2 {
			steady[i], steady[i+1] = 0.9, 0.2
		}
		f.set(Sound{Crossfeed: 1e-9, Width: 1})
		run(f, steady, 2)
		f.set(s)
		got := run(f, steady, 2)
		for c := range 2 {
			last := steady[c]
			for i := c; i < len(got); i += 2 {
				if d := math.Abs(float64(got[i] - last)); d > 1e-3 {
					t.Fatalf("%+v, channel %d, frame %d: a step of %v", s, c, i/2, d)
				}
				last = got[i]
			}
		}
		if got[len(got)-2] == steady[0] && got[len(got)-1] == steady[1] {
			t.Errorf("%+v changed nothing", s)
		}
	}
}

// TestSoundWidth scales the side signal by the width, and, above 100%, the
// mid signal down by as much, so that no sample can go past full scale.
func TestSoundWidth(t *testing.T) {
	in := noise(Rate/10, 2, 6)
	for _, w := range []float64{0, 0.5, 1.5, 2} {
		got := run(settledSound(Sound{Width: w}), in, 2)
		gm, gs := 1.0, w
		if w > 1 {
			gm, gs = 1/w, 1
		}
		peak := 0.0
		for i := 0; i < len(in); i += 2 {
			m, s := (float64(in[i])+float64(in[i+1]))/2, (float64(in[i])-float64(in[i+1]))/2
			gotM, gotS := (float64(got[i])+float64(got[i+1]))/2, (float64(got[i])-float64(got[i+1]))/2
			if math.Abs(gotM-m*gm) > 1e-6 || math.Abs(gotS-s*gs) > 1e-6 {
				t.Fatalf("width %v, frame %d: mid %v and side %v, want %v and %v", w, i/2, gotM, gotS, m*gm, s*gs)
			}
			peak = max(peak, math.Abs(float64(got[i])), math.Abs(float64(got[i+1])))
		}
		if peak > 1 {
			t.Errorf("width %v: a sample at %v, past full scale", w, peak)
		}
	}
	// The worst case of a widening: a side signal at full scale stays there.
	side := make([]float32, 2*quantum)
	for i := 0; i < len(side); i += 2 {
		side[i], side[i+1] = 1, -1
	}
	if got := run(settledSound(Sound{Width: 2}), side, 2); slices.Max(got) > 1 || slices.Min(got) < -1 {
		t.Errorf("a side signal at full scale, twice as wide, goes from %v to %v", slices.Min(got), slices.Max(got))
	}
}

// TestSoundCrossfeed leaves what plays the same on both sides as it is,
// and feeds a sound on one side to the other later, softer, and duller:
// at the low notes, 4.5 dB below it at full crossfeed, and less of it as
// the notes go up.
func TestSoundCrossfeed(t *testing.T) {
	centre := soundTone(Rate/10, 2, 440, [maxChannels]float64{0.7, 0.7}, 0)
	if got := run(settledSound(Sound{Crossfeed: 1, Width: 1}), centre, 2); !slices.Equal(got, centre) {
		t.Error("crossfeed changed a sound in the middle")
	}

	// An impulse on the left reaches the right no sooner than the delay.
	impulse := make([]float32, 2*quantum)
	impulse[0] = 1
	got := run(settledSound(Sound{Crossfeed: 1, Width: 1}), impulse, 2)
	first := -1
	for f := range quantum {
		if got[2*f+1] != 0 {
			first = f
			break
		}
	}
	if first != crossfeedDelay {
		t.Errorf("the impulse reaches the right %d samples late, want %d", first, crossfeedDelay)
	}

	// A tone on the left alone: the right's level against the left's.
	feed := func(amount, hz float64) float64 {
		in := soundTone(Rate/2, 2, hz, [maxChannels]float64{0.5, 0}, 0)
		got := run(settledSound(Sound{Crossfeed: amount, Width: 1}), in, 2)
		var l, r float64
		for i := len(got) / 2; i < len(got); i += 2 { // past the start
			l += float64(got[i]) * float64(got[i])
			r += float64(got[i+1]) * float64(got[i+1])
		}
		return 10 * math.Log10(r/l)
	}
	if db := feed(1, 40); math.Abs(db+crossfeedFeed) > 0.1 {
		t.Errorf("at full crossfeed, 40 Hz on the left plays %.2f dB on the right, want -%v", db, crossfeedFeed)
	}
	if lo, hi := feed(1, 200), feed(1, 4000); hi > lo-10 {
		t.Errorf("4 kHz feeds %.1f dB, 200 Hz %.1f dB: the copy isn't low-passed", hi, lo)
	}
	if half, full := feed(0.5, 40), feed(1, 40); half > full-3 {
		t.Errorf("half the crossfeed feeds %.1f dB, the whole %.1f dB", half, full)
	}
}

// TestSoundWidthAndCrossfeedOnStereoAlone skips width and crossfeed on 5.1,
// whose channels play from speakers of their own.
func TestSoundWidthAndCrossfeedOnStereoAlone(t *testing.T) {
	in := noise(8*quantum, 6, 7)
	if got := run(settledSound(Sound{Crossfeed: 1, Width: 2}), in, 6); !slices.Equal(got, in) {
		t.Error("width and crossfeed changed six channels")
	}
}

// TestEngineSound plays a track through the engine's stages after the
// equalizer: balanced full right, its left side falls silent over the
// glide and its right one plays as it is.
func TestEngineSound(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, 4800)
	e.SetSound(Sound{Balance: 1, Width: 1})
	got := dev.play(t, Rate/2)
	// After the glide, which settles within 0.3 s.
	for f := 3 * Rate / 10; f < Rate/2; f++ {
		if l, r := got[2*f], got[2*f+1]; math.Abs(float64(l)) > 1e-6 || r != want[2*(4800+f)+1] {
			t.Fatalf("frame %d: %v and %v, want 0 and %v", f, l, r, want[2*(4800+f)+1])
		}
	}
}
