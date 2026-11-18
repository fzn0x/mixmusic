package audio

import (
	"math"
	"slices"
	"testing"
	"time"
)

// echoOf is an echo at x, its line made and its glides done.
func echoOf(x Echo) *echo {
	f := newEcho()
	f.line = newEchoLine()
	f.setTo(x)
	f.wet.value, f.dry.value, f.feedback.value = f.wet.target, f.dry.target, f.feedback.target
	return f
}

// runEcho passes buf through f a quantum at a time, as the engine does.
func runEcho(f *echo, buf []float32, ch int) []float32 {
	out := slices.Clone(buf)
	for i := 0; i < len(out); i += ch * quantum {
		f.process(out[i:min(i+ch*quantum, len(out))], ch)
	}
	return out
}

// TestEchoNeutralIsSkipped leaves every sample as it was while the echo
// is off, or on at a wet level of 0, at first and once it glided back
// out, on two channels and on six.
func TestEchoNeutralIsSkipped(t *testing.T) {
	for _, ch := range []int{2, 6} {
		in := soundNoise(20*quantum, ch, 1)
		if got := runEcho(newEcho(), in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: a new echo changed the samples", ch)
		}
		if got := runEcho(echoOf(Echo{On: true, Delay: time.Second, Feedback: 0.5, Dry: 1}), in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: a wet level of 0 changed the samples", ch)
		}
		f := echoOf(Echo{On: true, Delay: 100 * time.Millisecond, Feedback: 0.8, Wet: 0.6, Dry: 0.7})
		runEcho(f, soundNoise(Rate/2, ch, 2), ch)
		f.setTo(Echo{})
		runEcho(f, soundNoise(Rate, ch, 3), ch)
		if got := runEcho(f, in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: off again, the echo still changes the samples", ch)
		}
	}
}

// TestEchoRepeats plays an impulse through the echo: the dry sound at
// once, then each repeat a delay after the one before, the first at the
// wet level, each next one the feedback's share of it, and nothing
// between; at each end of the delay's range, and beyond it.
func TestEchoRepeats(t *testing.T) {
	at := rampFrames
	for _, c := range []struct {
		delay    time.Duration
		frames   int
		feedback float64
	}{
		{100 * time.Millisecond, 4800, 0.5},
		{MinEchoDelay, 960, 0.9},
		{MaxEchoDelay, 2 * Rate, 0.25},
		{time.Millisecond, 960, 0},
		{time.Minute, 2 * Rate, 0.75},
	} {
		n := at + 4*c.frames + 1
		got := runEcho(echoOf(Echo{On: true, Delay: c.delay, Feedback: c.feedback, Wet: 0.6, Dry: 0.9}), impulse(n, 2, 0, at), 2)
		w, fb := float32(0.6), float32(c.feedback)
		for f := range n {
			want := float32(0)
			switch k := f - at; {
			case k == 0:
				want = 0.9
			case k > 0 && k%c.frames == 0:
				// Each repeat goes round the line once more.
				tap := float32(1)
				for range k/c.frames - 1 {
					tap *= fb
				}
				want = w * tap
			}
			if got[2*f] != want || got[2*f+1] != 0 {
				t.Fatalf("%v, frame %d: %v and %v, want %v and 0", c.delay, f-at, got[2*f], got[2*f+1], want)
			}
		}
	}
}

// TestEchoFiveOne repeats each channel of 5.1 on its own.
func TestEchoFiveOne(t *testing.T) {
	at, d := rampFrames, 4800
	for c := range maxChannels {
		got := runEcho(echoOf(Echo{On: true, Delay: 100 * time.Millisecond, Feedback: 0.5, Wet: 0.5, Dry: 1}), impulse(at+3*d, 6, c, at), 6)
		for f := range at + 3*d {
			for k := range maxChannels {
				want := float32(0)
				if k == c {
					switch f - at {
					case 0:
						want = 1
					case d:
						want = 0.5
					case 2 * d:
						want = 0.25
					}
				}
				if got[6*f+k] != want {
					t.Fatalf("impulse on channel %d: channel %d plays %v at frame %d, want %v", c, k, got[6*f+k], f-at, want)
				}
			}
		}
	}
}

// TestEchoGlides turns the echo on, changes its delay, its feedback and
// its mix, and turns it off, as a tone plays: none of the changes steps
// the sound by more than half again the most it steps playing steadily,
// before or after, so none clicks.
func TestEchoGlides(t *testing.T) {
	const second = Rate
	tone := soundTone(13*second, 2, 220, [maxChannels]float64{0.5, 0.3}, 0)
	f := echoOf(Echo{})
	changes := []struct {
		at int
		x  Echo
	}{
		{1, Echo{On: true, Delay: 300 * time.Millisecond, Feedback: 0.4, Wet: 0.5, Dry: 0.8}},
		{4, Echo{On: true, Delay: 470 * time.Millisecond, Feedback: 0.4, Wet: 0.5, Dry: 0.8}},
		{7, Echo{On: true, Delay: 470 * time.Millisecond, Feedback: 0.8, Wet: 0.3, Dry: 0.95}},
		{10, Echo{}},
	}
	got := slices.Clone(tone)
	next := 0
	for i := 0; i < len(got); i += 2 * quantum {
		if next < len(changes) && i >= 2*changes[next].at*second {
			f.setTo(changes[next].x)
			next++
		}
		f.process(got[i:min(i+2*quantum, len(got))], 2)
	}
	steps := toneSteps(got, 2, second/10)
	for _, c := range changes {
		at := 10 * c.at
		before, after := slices.Max(steps[at-10:at]), slices.Max(steps[at+25:at+30])
		if during := slices.Max(steps[at : at+10]); during > 1.5*max(before, after) {
			t.Errorf("to %+v: a step of %.4f, against %.4f before and %.4f after", c.x, during, before, after)
		}
	}
}

// TestEchoCut fades the repeats out over the quantum a seek asks it to,
// and leaves nothing of them after; an output that started again starts
// from silence too.
func TestEchoCut(t *testing.T) {
	silence := make([]float32, 2*quantum)
	for _, how := range []string{"cut", "stale"} {
		f := echoOf(Echo{On: true, Delay: 50 * time.Millisecond, Feedback: 0.9, Wet: 1, Dry: 1})
		runEcho(f, soundNoise(Rate, 2, 7), 2)
		if got := runEcho(f, silence, 2); slices.Max(got) == 0 {
			t.Fatal("no repeats after the noise")
		}
		if how == "cut" {
			f.cut = true
			got := runEcho(f, silence, 2)
			if got[len(got)-2] != 0 || got[len(got)-1] != 0 {
				t.Errorf("the quantum of the seek ends at %v and %v", got[len(got)-2], got[len(got)-1])
			}
			q := len(got) / 4
			if first, last := power(got[:q], 1, 0, 0), power(got[3*q:], 1, 0, 0); last > first/10 {
				t.Errorf("the repeats don't fade over the seek's quantum: %v, then %v", first, last)
			}
		} else {
			f.stale = true
		}
		if got := runEcho(f, make([]float32, 2*Rate), 2); slices.Max(got) != 0 || slices.Min(got) != 0 {
			t.Errorf("%s: the repeats play on", how)
		}
	}
}

// TestEchoNoSubnormals feeds the repeats back until they fade out: the
// line never holds a subnormal number, and in the end only zeros.
func TestEchoNoSubnormals(t *testing.T) {
	f := echoOf(Echo{On: true, Delay: MinEchoDelay, Feedback: MaxEchoFeedback, Wet: 1, Dry: 1})
	runEcho(f, impulse(rampFrames+quantum, 2, 0, rampFrames), 2)
	// 0.9 a repeat, 50 repeats a second: float32's subnormals, 760 dB
	// down, in 17 s.
	silence := make([]float32, 2*Rate)
	for range 20 {
		runEcho(f, silence, 2)
		if slices.ContainsFunc(f.line, subnormal) {
			t.Fatal("a subnormal number in the line")
		}
	}
	if slices.ContainsFunc(f.line, func(v float32) bool { return v != 0 }) {
		t.Error("the repeats never went to silence")
	}
	if f.ringing() {
		t.Error("the echo rings in silence")
	}
}

// TestEchoRinging tells the engine the echo still sounds while its line
// holds a repeat to play, however long the delay, and not once its
// repeats fell under -100 dBFS.
func TestEchoRinging(t *testing.T) {
	f := echoOf(Echo{On: true, Delay: MaxEchoDelay, Feedback: 0.5, Wet: 1, Dry: 1})
	runEcho(f, soundNoise(Rate/10, 2, 8), 2)
	// Nearly two seconds of silence: the repeat is still to come.
	silence := make([]float32, 2*quantum)
	for range 2*Rate/quantum - 10 {
		runEcho(f, silence, 2)
	}
	if !f.ringing() {
		t.Fatal("the echo doesn't ring before its first repeat")
	}
	rang := 0
	for f.ringing() && rang < 60*Rate {
		runEcho(f, silence, 2)
		rang += quantum
	}
	// From 0 dB to -100 dB, 6 dB a repeat: 17 repeats.
	if s := float64(rang) / Rate; s < 30 || s > 40 {
		t.Errorf("the echo rang %.1f s more", s)
	}
	if echoOf(Echo{}).ringing() {
		t.Error("an echo that's off rings")
	}
}

// TestEngineEcho plays a track's repeats, as SetEcho sets them.
func TestEngineEcho(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	x := Echo{On: true, Delay: 100 * time.Millisecond, Wet: 0.5, Dry: 1}
	e.SetEcho(x)
	if got := e.Echo(); got != x {
		t.Errorf("Echo() is %+v", got)
	}
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	got := dev.play(t, Rate)
	// Once the wet level glided up from 0: the track, and half of it 100
	// ms late.
	for f := Rate / 2; f < Rate; f++ {
		for c := range 2 {
			w := float64(want[2*f+c]) + 0.5*float64(want[2*(f-4800)+c])
			if d := math.Abs(float64(got[2*f+c]) - w); d > 1e-6 {
				t.Fatalf("frame %d: %v, want %v", f, got[2*f+c], w)
			}
		}
	}
}
