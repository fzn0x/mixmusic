package audio

import (
	"math"
	"testing"
)

// TestRingOfSixChannels passes frames of 5.1 through a ring, around its
// end.
func TestRingOfSixChannels(t *testing.T) {
	r := newRing(0, 6)
	frame := func(i int) []float32 {
		f := make([]float32, 6)
		for c := range f {
			f[c] = float32(i*10 + c)
		}
		return f
	}
	in := make([]float32, 0, 6*1000)
	for i := range 1000 {
		in = append(in, frame(i)...)
	}
	never := func(<-chan struct{}) bool { return false }
	read := 0
	out := make([]float32, 6*700)
	for range 100 {
		if n := r.writeFrom(in, never); n != 1000 {
			// The ring's room, at its end: what was read made it.
			t.Fatalf("wrote %d frames", n)
		}
		for left := 1000; left > 0; {
			n := r.readTo(out[:6*min(700, left)])
			for i := range n {
				want := frame((1000 - left + i))
				for c := range 6 {
					if out[6*i+c] != want[c] {
						t.Fatalf("frame %d of the ring, channel %d: %v, want %v", read+i, c, out[6*i+c], want[c])
					}
				}
			}
			left -= n
			read += n
		}
	}
	if r.position() != int64(read) || read <= ringFrames {
		t.Errorf("at %d after %d frames", r.position(), read)
	}
}

// surroundDeck returns a deck that plays frames of channels from ring,
// at its full level: frame f of channel c is value(f, c).
func surroundDeck(channels, n int, value func(f, c int) float32, downmix *[2][6]float32) *deck {
	d := &deck{gain: ramp{value: 1, target: 1}, trim: newSmoother(1, glide), changed: make(chan struct{}, 1)}
	d.lkfs.Store(math.Float64bits(math.NaN()))
	r := newRing(0, channels)
	pcm := make([]float32, channels*n)
	for f := range n {
		for c := range channels {
			pcm[channels*f+c] = value(f, c)
		}
	}
	r.writeFrom(pcm, func(<-chan struct{}) bool { return false })
	d.ring.Store(r)
	if downmix != nil {
		d.downmix.Store(downmix)
	}
	return d
}

// renderDecks renders n frames of ch channels of an engine at full volume
// whose first deck is d.
func renderDecks(t *testing.T, ch, n int, decks ...*deck) []float32 {
	t.Helper()
	e := NewEngine(fileOpener, (&fakeDevice{}).start, newRecorder())
	t.Cleanup(e.Close)
	e.master = newSmoother(1, glide)
	e.mu.Lock()
	for i, d := range decks {
		e.decks[i] = d
		e.levels[i] = ramp{value: 1, target: 1}
	}
	e.mu.Unlock()
	out := make([]float32, ch*n)
	e.render(out, ch)
	e.mu.Lock()
	e.decks = [2]*deck{}
	e.mu.Unlock()
	return out
}

// chTone is channel c's sine wave of frame f: each channel its own
// frequency.
func chTone(f, c int) float32 {
	return float32(0.1 * math.Sin(2*math.Pi*float64(f)*float64(300+100*c)/Rate))
}

func TestSurroundDeckOnSixChannels(t *testing.T) {
	const n = 4 * quantum
	got := renderDecks(t, 6, n, surroundDeck(6, n, chTone, nil))
	for f := range n {
		for c := range 6 {
			if got[6*f+c] != chTone(f, c) {
				t.Fatalf("frame %d, channel %d: %v, want %v", f, c, got[6*f+c], chTone(f, c))
			}
		}
	}
}

func TestSurroundDeckMixedDown(t *testing.T) {
	// The stream's own levels: the center at -3 dB, the surrounds at -6,
	// scaled by 1/(1 + 0.707 + 0.5), and an LFE left out.
	const c, s = 0.70710677, 0.5
	k := float32(1 / (1 + c + s))
	m := &[2][6]float32{
		{k, 0, c * k, 0, s * k, 0},
		{0, k, c * k, 0, 0, s * k},
	}
	const n = 4 * quantum
	for _, downmix := range []*[2][6]float32{m, nil} {
		got := renderDecks(t, 2, n, surroundDeck(6, n, chTone, downmix))
		want := downmix
		if want == nil {
			want = &defaultDownmix // a stream that told none yet
		}
		for f := range n {
			for o := range 2 {
				var w float32
				for ch := range 6 {
					w += want[o][ch] * chTone(f, ch)
				}
				if d := math.Abs(float64(got[2*f+o] - w)); d > 1e-7 {
					t.Fatalf("downmix %v, frame %d, side %d: %v, want %v", downmix != nil, f, o, got[2*f+o], w)
				}
			}
		}
	}
	if defaultDownmix[0][2] != defaultDownmix[0][4] || math.Abs(float64(defaultDownmix[0][0]*(1+2*c))-1) > 1e-6 {
		t.Errorf("default downmix %v", defaultDownmix)
	}
}

func TestStereoDeckOnSixChannels(t *testing.T) {
	const n = 4 * quantum
	got := renderDecks(t, 6, n, surroundDeck(2, n, chTone, nil))
	for f := range n {
		for c := range 6 {
			want := float32(0)
			if c < 2 {
				want = chTone(f, c)
			}
			if got[6*f+c] != want {
				t.Fatalf("frame %d, channel %d: %v, want %v", f, c, got[6*f+c], want)
			}
		}
	}
}

// TestStereoAndSurroundDecks mixes a stereo deck fading out and a surround
// one fading in, as a crossfade does, on six channels and on two.
func TestStereoAndSurroundDecks(t *testing.T) {
	const n = 2 * quantum
	stereo := func(f, c int) float32 { return 0.25 }
	five := func(f, c int) float32 { return float32(c+1) / 10 }
	got := renderDecks(t, 6, n, surroundDeck(2, n, stereo, nil), surroundDeck(6, n, five, nil))
	for c, want := range []float32{0.35, 0.45, 0.3, 0.4, 0.5, 0.6} {
		if d := math.Abs(float64(got[6*(n-1)+c] - want)); d > 1e-6 {
			t.Errorf("six channels, channel %d: %v, want %v", c, got[6*(n-1)+c], want)
		}
	}
	got = renderDecks(t, 2, n, surroundDeck(2, n, stereo, nil), surroundDeck(6, n, five, nil))
	m := defaultDownmix
	for o := range 2 {
		want := float32(0.25)
		for c := range 6 {
			want += m[o][c] * five(0, c)
		}
		if d := math.Abs(float64(got[2*(n-1)+o] - want)); d > 1e-6 {
			t.Errorf("two channels, side %d: %v, want %v", o, got[2*(n-1)+o], want)
		}
	}
}

// TestSetSurroundRestartsTheOutput has the output play six channels on a
// device of 5.1 once 5.1 surround is on, two on one of stereo, and two
// again once it is off.
func TestSetSurroundRestartsTheOutput(t *testing.T) {
	for _, five := range []bool{true, false} {
		e, dev, _ := newTestEngine(t)
		dev.surround = five
		if err := e.Load(cuesFront, 0, true); err != nil {
			t.Fatal(err)
		}
		if dev.asked != 2 || dev.channels != 2 || dev.starts != 1 {
			t.Fatalf("asked for %d channels, playing %d, after %d starts", dev.asked, dev.channels, dev.starts)
		}
		e.SetSurround(true)
		want := 2
		if five {
			want = 6
		}
		if dev.asked != 6 || dev.channels != want || dev.starts != 2 || dev.isStopped() {
			t.Errorf("5.1 on a device of 5.1 %v: asked for %d channels, playing %d, after %d starts", five, dev.asked, dev.channels, dev.starts)
		}
		// The stereo track plays on the front left and right.
		out := dev.play(t, 4800)
		ch := dev.channels
		for f := 4000; f < 4800; f++ {
			for c := 2; c < ch; c++ {
				if out[ch*f+c] != 0 {
					t.Fatalf("frame %d plays %v on channel %d", f, out[ch*f+c], c)
				}
			}
		}
		e.SetSurround(true) // no change: no restart
		if dev.starts != 2 {
			t.Errorf("%d starts", dev.starts)
		}
		e.SetSurround(false)
		if dev.asked != 2 || dev.channels != 2 || dev.starts != 3 {
			t.Errorf("5.1 off: asked for %d channels, playing %d, after %d starts", dev.asked, dev.channels, dev.starts)
		}
		e.Stop()
	}
	// While no output plays, the next starts with the channels asked for.
	e, dev, _ := newTestEngine(t)
	dev.surround = true
	e.SetSurround(true)
	if dev.starts != 0 {
		t.Errorf("%d starts with no track", dev.starts)
	}
	if err := e.Load(cuesFront, 0, true); err != nil {
		t.Fatal(err)
	}
	if dev.asked != 6 || dev.channels != 6 || dev.starts != 1 {
		t.Errorf("asked for %d channels, playing %d, after %d starts", dev.asked, dev.channels, dev.starts)
	}
}

// TestOutputChangesChannels renders for an output whose device goes from
// six channels to two and back, as when the default device changes: the
// track plays on, from the frame it was at.
func TestOutputChangesChannels(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	dev.surround = true
	e.SetSurround(true)
	if err := e.Load(cuesFront, 0, false); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	dev.mu.Lock()
	render := dev.render
	dev.mu.Unlock()
	// 6, 2 and 6 channels, each a whole number of quanta.
	at := 0
	for _, ch := range []int{6, 2, 6, 2} {
		n := 10 * quantum
		out := make([]float32, ch*n)
		dev.waitDecks(t, n)
		render(out, ch)
		for f := range n {
			i := at + f
			if i < 2*fadeFrames {
				continue
			}
			for c := range 2 {
				if out[ch*f+c] != want[2*i+c] {
					t.Fatalf("%d channels: frame %d, channel %d: %v, want %v", ch, i, c, out[ch*f+c], want[2*i+c])
				}
			}
			for c := 2; c < ch; c++ {
				if out[ch*f+c] != 0 {
					t.Fatalf("%d channels: frame %d plays on channel %d", ch, i, c)
				}
			}
		}
		at += n
	}
}

// TestEffectsOnSixChannels runs the equalizer and the compressor on 5.1:
// each channel filtered as it is alone, and one detector of all channels.
func TestEffectsOnSixChannels(t *testing.T) {
	gains := [len(Bands)]float64{6, -3, 0, 2, 0, 0, 4, -6, 0, 3}
	const n = 8 * quantum
	five, alone := newEqualizer(), make([]*equalizer, 6)
	five.set(true, gains, 2)
	buf := make([]float32, 6*n)
	for f := range n {
		for c := range 6 {
			buf[6*f+c] = chTone(f, c)
		}
	}
	for q := 0; q < n; q += quantum {
		five.process(buf[6*q:6*(q+quantum)], 6)
	}
	for c := range 6 {
		alone[c] = newEqualizer()
		alone[c].set(true, gains, 2)
		st := make([]float32, 2*n)
		for f := range n {
			st[2*f], st[2*f+1] = chTone(f, c), 0
		}
		for q := 0; q < n; q += quantum {
			alone[c].process(st[2*q:2*(q+quantum)], 2)
		}
		for f := range n {
			if buf[6*f+c] != st[2*f] {
				t.Fatalf("channel %d, frame %d: %v, alone %v", c, f, buf[6*f+c], st[2*f])
			}
		}
	}

	// A loud center turns the fronts down with it, once the compressor
	// attacked.
	const nc = 100 * quantum
	comp := newCompressor()
	loud := make([]float32, 6*nc)
	for f := range nc {
		loud[6*f], loud[6*f+2] = 0.01, float32(0.9*math.Sin(2*math.Pi*float64(f)*200/Rate))
	}
	for q := 0; q < nc; q += quantum {
		comp.process(loud[6*q:6*(q+quantum)], 6, levelThreshold, levelKnee, levelRatio, levelAttack, levelRelease)
	}
	quiet := newCompressor()
	alone2 := make([]float32, 2*nc)
	for f := range nc {
		alone2[2*f] = 0.01
	}
	for q := 0; q < nc; q += quantum {
		quiet.process(alone2[2*q:2*(q+quantum)], 2, levelThreshold, levelKnee, levelRatio, levelAttack, levelRelease)
	}
	if l, a := loud[6*(nc-1)], alone2[2*(nc-1)]; !(l < a*0.8) {
		t.Errorf("the left beside a loud center: %v, alone %v", l, a)
	}
	// forget clears the channels it is told of, and no others.
	five.forget(2)
	for i := range five.filters {
		f := &five.filters[i]
		if f.x1[2] != 0 || f.y1[5] != 0 || f.x1[0] == 0 && f.y1[0] == 0 && gains[i] != 0 {
			t.Fatalf("filter %d after forget(2): %v %v", i, f.x1, f.y1)
		}
	}
	comp.forget(2)
	for j := 2; j < 6; j++ {
		for _, v := range comp.preDelay[j] {
			if v != 0 {
				t.Fatalf("channel %d's look-ahead after forget(2)", j)
			}
		}
	}
}

// TestAnalyserOfSixChannels draws a stereo track on six channels as on two.
func TestAnalyserOfSixChannels(t *testing.T) {
	two, six := newAnalyser(), newAnalyser()
	st := make([]float32, 2*FFTSize)
	five := make([]float32, 6*FFTSize)
	for f := range FFTSize {
		st[2*f], st[2*f+1] = chTone(f, 0), chTone(f, 1)
		five[6*f], five[6*f+1] = chTone(f, 0), chTone(f, 1)
	}
	two.writeInput(st, 2)
	six.writeInput(five, 6)
	var a, b Analysis
	two.analyse(&a)
	six.analyse(&b)
	if a != b {
		t.Errorf("stereo on six channels analyses otherwise")
	}
	// 5.1's center counts at -3 dB, its surrounds at -9, its LFE not.
	clear(five)
	five[2], five[3], five[4] = 1, 1, 1
	six.writeInput(five[:6], 6)
	six.analyse(&b)
	if got := b.Waveform[FFTSize-1]; math.Abs(float64(got)-(0.70710677+0.35355338)) > 1e-6 {
		t.Errorf("the mono mix of C, LFE and Ls at 1: %v", got)
	}
}

// BenchmarkRenderSurround renders 10 ms periods of a surround deck on six
// channels, and mixed down to two, through the equalizer and the
// compressor: nothing may be allocated.
func BenchmarkRenderSurround(b *testing.B) {
	for _, ch := range []int{6, 2} {
		b.Run(map[int]string{6: "six", 2: "two"}[ch], func(b *testing.B) {
			e := NewEngine(fileOpener, (&fakeDevice{}).start, newRecorder())
			defer e.Close()
			e.master = newSmoother(1, glide)
			e.normalize = true
			e.eq.set(true, [len(Bands)]float64{3, 0, -2, 0, 1, 0, 0, 2, 0, -1}, 0)
			d := surroundDeck(6, 1, chTone, nil)
			e.mu.Lock()
			e.decks[0] = d
			e.levels[0] = ramp{value: 1, target: 1}
			e.mu.Unlock()
			pcm := make([]float32, 6*480)
			for f := range 480 {
				for c := range 6 {
					pcm[6*f+c] = chTone(f, c)
				}
			}
			out := make([]float32, ch*480)
			never := func(<-chan struct{}) bool { return false }
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				d.ring.Load().writeFrom(pcm, never)
				e.render(out, ch)
			}
			b.StopTimer()
			if allocs := testing.AllocsPerRun(100, func() {
				d.ring.Load().writeFrom(pcm, never)
				e.render(out, ch)
			}); allocs > 0 {
				b.Errorf("%v allocations a period", allocs)
			}
			e.mu.Lock()
			e.decks = [2]*deck{}
			e.mu.Unlock()
		})
	}
}
