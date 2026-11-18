package audio

import (
	"math"
	"slices"
	"testing"
)

// The correction of headphones (#616), sample by sample.

// correctionOfTest is a correction of each type of filter, as AutoEq's
// profiles have them, with a low-pass and a high-pass besides.
var correctionOfTest = Correction{Preamp: -5.9, Filters: []CorrectionFilter{
	{CorrectionLowShelf, 105, 0.6, 0.7},
	{CorrectionPeak, 127, -3.6, 0.21},
	{CorrectionPeak, 620, 5.4, 1.06},
	{CorrectionPeak, 4564, -2.3, 3.38},
	{CorrectionHighShelf, 10000, 2.2, 0.7},
	{CorrectionHighPass, 20, 0, 0.7071},
	{CorrectionLowPass, 20000, 0, 0.7071},
}}

// runCorrection passes buf through f a quantum at a time, as the engine
// does.
func runCorrection(f *correction, buf []float32, ch int) []float32 {
	out := slices.Clone(buf)
	for i := 0; i < len(out); i += ch * quantum {
		f.process(out[i:min(i+ch*quantum, len(out))], ch)
	}
	return out
}

// settledCorrection is a correction stage playing c, its fade done, its
// filters' memory clear.
func settledCorrection(c Correction, ch int) *correction {
	f := newCorrection()
	f.setTo(c)
	runCorrection(f, make([]float32, ch*Rate), ch) // silence, fifty time constants
	if f.fading {
		panic("the fade didn't end")
	}
	return f
}

// TestCorrectionNoneIsSkipped leaves every sample as it was while there is
// no correction: at first, and once one faded out again.
func TestCorrectionNoneIsSkipped(t *testing.T) {
	for _, ch := range []int{2, 6} {
		in := soundNoise(20*quantum, ch, 21)
		if got := runCorrection(newCorrection(), in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: no correction changed the samples", ch)
		}
		f := newCorrection()
		f.setTo(correctionOfTest)
		runCorrection(f, soundNoise(20*quantum, ch, 22), ch)
		f.setTo(Correction{Filters: []CorrectionFilter{}})
		runCorrection(f, soundNoise(Rate, ch, 23), ch)
		if f.cur != nil || f.fading {
			t.Errorf("%d channels: none still plays a chain", ch)
		}
		if got := runCorrection(f, in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: back at none, the samples change", ch)
		}
	}
}

// TestCorrectionPlaysItsFilters plays the preamp, then each filter in
// order, on every channel of stereo and of 5.1, as the cookbook's filters
// do, sample for sample.
func TestCorrectionPlaysItsFilters(t *testing.T) {
	for _, ch := range []int{2, 6} {
		in := soundNoise(Rate/4, ch, 24)
		got := runCorrection(settledCorrection(correctionOfTest, ch), in, ch)
		gain := float32(math.Pow(10, correctionOfTest.Preamp/20))
		for k := range ch {
			x := make([]float32, 0, len(in)/ch)
			for i := k; i < len(in); i += ch {
				x = append(x, in[i]*gain)
			}
			for _, f := range correctionOfTest.Filters {
				if f.Type == CorrectionPeak {
					// Blink's peaking filter, which is the cookbook's.
					var b biquad
					b.set(peaking, f.Hz, f.Q, f.Gain)
					b.process(x, 1)
					continue
				}
				for i, v := range cookbookRun(correctionKinds[f.Type], f.Hz, f.Q, f.Gain, x) {
					x[i] = float32(v)
				}
			}
			for i, want := range x {
				if g := got[i*ch+k]; math.Abs(float64(g-want)) > 1e-6*max(1, math.Abs(float64(want))) {
					t.Fatalf("%d channels, channel %d, sample %d: %v, want %v", ch, k, i, g, want)
				}
			}
		}
	}
}

// TestCorrectionResponse shapes a sine as the filters' response says: the
// preamp and the sum of each filter's gain in dB.
func TestCorrectionResponse(t *testing.T) {
	for _, hz := range []float64{50, 127, 620, 1000, 4564, 12000} {
		want := correctionOfTest.Preamp
		for _, f := range correctionOfTest.Filters {
			var b biquad
			b.set(correctionKinds[f.Type], f.Hz, f.Q, f.Gain)
			want += 10 * math.Log10(b.gainAt(hz))
		}
		in := soundTone(Rate, 2, hz, [maxChannels]float64{0.3, 0.3}, 0)
		got := runCorrection(settledCorrection(correctionOfTest, 2), in, 2)
		var pin, pout float64
		for i := len(in) / 2; i < len(in); i++ {
			pin += float64(in[i]) * float64(in[i])
			pout += float64(got[i]) * float64(got[i])
		}
		if db := 10 * math.Log10(pout/pin); math.Abs(db-want) > 0.05 {
			t.Errorf("%v Hz comes out at %.3f dB, want %.3f", hz, db, want)
		}
	}
}

// TestCorrectionFades moves from no correction to one, from one to
// another, and back to none, without a click: a low sine changes by
// little more than it does itself from one sample to the next. A change
// asked for while a fade is under way plays once it ends.
func TestCorrectionFades(t *testing.T) {
	loud := Correction{Preamp: 6, Filters: []CorrectionFilter{{CorrectionLowShelf, 300, 9, 0.7}}}
	quiet := Correction{Preamp: -12, Filters: []CorrectionFilter{{CorrectionHighPass, 400, 0, 2}}}
	const hz, amp = 110.0, 0.2
	// A sine of hz changes by at most 2π·hz/Rate of its amplitude a sample,
	// which the corrections raise by 6 and 9 dB at most.
	most := 2 * math.Pi * hz / Rate * amp * math.Pow(10, 15.0/20) * 1.5
	f := newCorrection()
	at := 0
	last := float32(0)
	play := func(what string, n int) []float32 {
		in := soundTone(n, 2, hz, [maxChannels]float64{amp, amp}, at)
		at += n
		got := runCorrection(f, in, 2)
		for i := 0; i < len(got); i += 2 {
			if d := math.Abs(float64(got[i] - last)); d > most {
				t.Fatalf("%s, frame %d: a step of %.4f, past %.4f", what, i/2, d, most)
			}
			last = got[i]
		}
		return got
	}
	play("none", Rate/10)
	f.setTo(loud)
	play("to loud", Rate/2)
	if f.fading || f.cur == nil || f.cur.gain != float32(math.Pow(10, 6.0/20)) {
		t.Fatalf("the fade to loud didn't end: %+v", f)
	}
	f.setTo(quiet)
	play("toward quiet", Rate/50)
	f.setTo(loud) // under way: plays once the fade to quiet ends
	f.setTo(quiet)
	f.setTo(Correction{})
	play("to none", Rate)
	if f.fading || f.cur != nil {
		t.Errorf("the last asked, none, doesn't play: %+v", f)
	}
}

// TestCorrectionForgets clears the filters' memory of the channels a
// device of fewer channels left.
func TestCorrectionForgets(t *testing.T) {
	f := settledCorrection(correctionOfTest, 6)
	runCorrection(f, soundNoise(4*quantum, 6, 25), 6)
	f.forget(2)
	for _, b := range f.cur.filters {
		for c := 2; c < maxChannels; c++ {
			if b.x1[c] != 0 || b.x2[c] != 0 || b.y1[c] != 0 || b.y2[c] != 0 {
				t.Fatalf("channel %d keeps its memory", c)
			}
		}
		if b.y1[0] == 0 && b.y1[1] == 0 {
			t.Fatal("the first two channels lost theirs")
		}
	}
}

// TestCorrectionSkipsWhatItDoesntKnow plays the filters of the types it
// knows, the first MaxCorrectionFilters of them.
func TestCorrectionSkipsWhatItDoesntKnow(t *testing.T) {
	c := Correction{Filters: []CorrectionFilter{{"BP", 1000, 0, 1}, {CorrectionPeak, 1000, 3, 1}}}
	if ch := newChain(c); len(ch.filters) != 1 || ch.gain != 1 {
		t.Errorf("a band-pass and a peak: %+v", ch)
	}
	c.Filters = make([]CorrectionFilter, 30)
	for i := range c.Filters {
		c.Filters[i] = CorrectionFilter{CorrectionPeak, 1000, 1, 1}
	}
	if ch := newChain(c); len(ch.filters) != MaxCorrectionFilters {
		t.Errorf("30 filters play %d", len(ch.filters))
	}
	if newChain(Correction{}) != nil || newChain(Correction{Filters: []CorrectionFilter{}}) != nil {
		t.Error("none has a chain")
	}
}

// TestEngineCorrection plays a track through the engine's correction: a
// preamp of -6.02 dB alone halves every sample, once the fade ends, and
// the correction set reads back.
func TestEngineCorrection(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, 4800)
	half := Correction{Preamp: 20 * math.Log10(0.5), Filters: []CorrectionFilter{}}
	e.SetCorrection(half)
	if got := e.Correction(); !got.equal(half) {
		t.Errorf("the correction reads back as %+v", got)
	}
	got := dev.play(t, Rate/2)
	for f := 3 * Rate / 10; f < Rate/2; f++ {
		for c := range 2 {
			w := want[2*(4800+f)+c] * float32(math.Pow(10, half.Preamp/20))
			if g := got[2*f+c]; math.Abs(float64(g-w)) > 1e-7 {
				t.Fatalf("frame %d, channel %d: %v, want %v", f, c, g, w)
			}
		}
	}
}
