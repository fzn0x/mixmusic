package audio

import (
	"testing"
	"time"
)

// everyEffect is an engine with every stage after the decks shaping the
// sound: the equalizer, the echo, the reverb, width, crossfeed, mono and
// balance, a correction of headphones, and Volume levelling.
func everyEffect(t testing.TB) *Engine {
	dev := &fakeDevice{}
	e := NewEngine(fileOpener, dev.start, newRecorder())
	t.Cleanup(e.Close)
	e.SetEqualizer(true, [len(Bands)]float64{6, 5, 4, 2, 0, -1, 2, 3, 4, 4}, -7)
	e.SetEcho(Echo{On: true, Delay: 500 * time.Millisecond, Feedback: 0.3, Wet: 0.4, Dry: 0.9})
	e.SetReverb(Reverb{On: true, Room: RoomHall, Wet: 0.4, Dry: 0.9})
	e.SetSound(Sound{Balance: 0.2, Mono: true, Crossfeed: 0.5, Width: 1.4})
	e.SetCorrection(Correction{Preamp: -3, Filters: []CorrectionFilter{
		{Type: CorrectionLowShelf, Hz: 105, Gain: 4, Q: 0.7},
		{Type: CorrectionPeak, Hz: 3000, Gain: -2, Q: 2},
		{Type: CorrectionHighShelf, Hz: 10000, Gain: 1.5, Q: 0.7},
	}})
	e.SetNormalize(true)
	return e
}

// TestEffectsDoNotAllocate shapes quanta, on two channels and on six,
// with every effect on, as their settings change, a seek fades the tails
// and the output starts again, without allocating: the audio thread
// never waits for the garbage collector.
func TestEffectsDoNotAllocate(t *testing.T) {
	e := everyEffect(t)
	for _, ch := range []int{2, 6} {
		noise := soundNoise(quantum, ch, 1)
		buf := make([]float32, len(noise))
		rooms := []Room{RoomPlate, RoomLarge, RoomSmall, RoomHall}
		n := 0
		allocs := testing.AllocsPerRun(400, func() {
			copy(buf, noise)
			e.mu.Lock()
			switch n % 100 {
			case 10:
				e.verb.setTo(Reverb{On: true, Room: rooms[n/100%4], Wet: 0.5, Dry: 0.8})
			case 30:
				e.echo.setTo(Echo{On: true, Delay: time.Duration(100+n) * time.Millisecond, Feedback: 0.6, Wet: 0.3, Dry: 1})
			case 50:
				e.echo.cut, e.verb.cut = true, true
			case 70:
				e.echo.stale, e.verb.stale = true, true
			}
			n++
			e.shape(buf, ch)
			e.renderQuantum()
			e.mu.Unlock()
		})
		if allocs != 0 {
			t.Errorf("%d channels: %v allocations a quantum", ch, allocs)
		}
	}
}

// BenchmarkEveryEffect shapes a quantum of noise, 128 frames, with every
// effect on: share is the part of the quantum's 2.67 ms it takes.
func BenchmarkEveryEffect(b *testing.B) {
	for _, ch := range []int{2, 6} {
		name := "stereo"
		if ch == 6 {
			name = "5.1"
		}
		b.Run(name, func(b *testing.B) {
			e := everyEffect(b)
			noise := soundNoise(quantum, ch, 1)
			buf := make([]float32, len(noise))
			e.mu.Lock()
			defer e.mu.Unlock()
			b.ReportAllocs()
			for b.Loop() {
				copy(buf, noise)
				e.shape(buf, ch)
			}
			perQuantum := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
			b.ReportMetric(perQuantum/(quantum*1e9/Rate), "share")
		})
	}
}
