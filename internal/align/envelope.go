// Package align finds where a stretch of one recording's audio is in
// another recording's: where a song starts in its music video, which may
// open with a scene before it or leave out its first seconds, so that the
// song's synced lyrics keep time with the video (#565). It compares the
// two recordings' envelopes: how loud each third of an octave from 100 Hz
// to 3.2 kHz is, every 10 ms, which a mastering of its own, the video's
// sound effects and the encoders' noise leave much alike where the
// waveforms would differ. A band louder or quieter throughout, as another
// mastering's equalizer leaves it, changes nothing, as the comparison
// takes away each band's mean. It all runs on the computer, in a few
// hundred milliseconds of work for a video of some minutes.
package align

import (
	"math"
	"time"
)

const (
	// Hop is the time between the frames of an envelope.
	Hop = 10 * time.Millisecond
	// Bands is the number of bands of a frame: the thirds of an octave
	// from 100 Hz up, to 3.2 kHz.
	Bands = 16
	// filterRate is the rate the bands' filters run at: the audio given is
	// taken down to it, by the mean of each run of its samples.
	filterRate = 8000
	// hopSamples is the samples at filterRate a hop holds.
	hopSamples = filterRate / 100
	// floor is added to the energy of a band, some 45 dB under a loud
	// one's, so that silence has a level, as the hiss of another copy of
	// the same silence has.
	floor = 1e-4
)

// Envelope is the envelope of a stretch of a recording: Bands levels a
// frame, a frame every Hop, from At.
type Envelope struct {
	// At is the time, in the recording, of the first frame.
	At time.Duration
	v  []float32
}

// Frames returns the number of frames of the envelope.
func (e Envelope) Frames() int { return len(e.v) / Bands }

// From returns the envelope from its first frame at t or after it.
func (e Envelope) From(t time.Duration) Envelope {
	skip := 0
	if t > e.At {
		skip = min(e.Frames(), int((t-e.At+Hop-1)/Hop))
	}
	return Envelope{At: e.At + time.Duration(skip)*Hop, v: e.v[skip*Bands:]}
}

// first returns the envelope's first n frames, all of them when it has
// fewer.
func (e Envelope) first(n int) Envelope {
	return Envelope{At: e.At, v: e.v[:min(len(e.v), n*Bands)]}
}

// Levels makes the envelope of the audio written to it: the logarithm of
// the energy of each band over the last 30 ms, every Hop.
type Levels struct {
	at   time.Duration
	down int
	// sum is of the samples of the filters' sample under way, n of them.
	sum float64
	n   int

	filters [Bands]biquad
	// energy is of the hop under way, k samples of it, and before of the
	// two hops before.
	energy [Bands]float64
	k      int
	before [Bands][2]float64
	v      []float32
}

// NewLevels returns the levels of mono audio at rate, a multiple of 8000,
// as 48000 is, whose first sample is at at in its recording.
func NewLevels(rate int, at time.Duration) *Levels {
	if rate < filterRate || rate%filterRate != 0 {
		panic("align: the rate isn't a multiple of 8000")
	}
	l := &Levels{at: at, down: rate / filterRate}
	// A third of an octave apart, each as wide: a Q of 4.3.
	q := math.Sqrt(math.Cbrt(2)) / (math.Cbrt(2) - 1)
	for b := range l.filters {
		l.filters[b] = bandpass(100*math.Pow(2, float64(b)/3), q)
	}
	return l
}

// Write takes in the samples that follow those written before.
func (l *Levels) Write(mono []float32) {
	for _, x := range mono {
		l.sum += float64(x)
		if l.n++; l.n < l.down {
			continue
		}
		s := l.sum / float64(l.down)
		l.sum, l.n = 0, 0
		for b := range l.filters {
			y := l.filters[b].run(s)
			l.energy[b] += y * y
		}
		if l.k++; l.k == hopSamples {
			l.frame()
		}
	}
}

// frame ends the hop under way.
func (l *Levels) frame() {
	for b := range Bands {
		e := l.energy[b] + l.before[b][0] + l.before[b][1]
		l.before[b][1], l.before[b][0] = l.before[b][0], l.energy[b]
		l.v = append(l.v, float32(math.Log(e+floor)))
		l.energy[b] = 0
	}
	l.k = 0
}

// Envelope returns the envelope of the hops written whole.
func (l *Levels) Envelope() Envelope { return Envelope{At: l.at, v: l.v} }

// biquad is the band-pass filter of Robert Bristow-Johnson's Audio EQ
// Cookbook that passes its centre as it is, at filterRate.
type biquad struct {
	b0, b2, a1, a2 float64
	x1, x2, y1, y2 float64
}

func bandpass(f0, q float64) biquad {
	w0 := 2 * math.Pi * f0 / filterRate
	alpha := math.Sin(w0) / (2 * q)
	a0 := 1 + alpha
	return biquad{b0: alpha / a0, b2: -alpha / a0, a1: -2 * math.Cos(w0) / a0, a2: (1 - alpha) / a0}
}

func (f *biquad) run(x float64) float64 {
	y := f.b0*x + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2
	f.x2, f.x1, f.y2, f.y1 = f.x1, x, f.y1, y
	return y
}
