package audio

// The AnalyserNode of the web player, as Blink's RealtimeAnalyser computes
// it (third_party/blink/renderer/modules/webaudio/realtime_analyser.cc, under
// the BSD license of LICENSE.blink): fftSize 256, smoothingTimeConstant 0.8,
// and the default decibel range, -100 to -30 dB.

import (
	"math"
	"sync"
)

const (
	// FFTSize is the analyser's fftSize; it gives FFTSize/2 frequency bins.
	FFTSize      = 256
	smoothing    = 0.8
	minDecibels  = -100
	maxDecibels  = -30
	analyserBins = FFTSize / 2
)

// Analysis is what the analyser gives, as an AnalyserNode does.
type Analysis struct {
	// Bytes are the decibels of each frequency bin, from 0 for -100 dB to
	// 255 for -30 dB, as getByteFrequencyData gives them.
	Bytes [analyserBins]byte
	// Decibels are those of getFloatFrequencyData: -Inf for silence.
	Decibels [analyserBins]float32
	// Waveform is the last FFTSize samples, as getFloatTimeDomainData
	// gives them.
	Waveform [FFTSize]float32
}

// analyser keeps the last FFTSize samples of the mono mix of what plays, and
// the smoothed magnitudes of their spectrum.
type analyser struct {
	mu      sync.Mutex
	input   [FFTSize]float32 // a ring of the last samples
	write   int
	written int64 // the samples written so far

	// analysing serializes the analyses, which own what follows; the audio
	// thread never waits for one.
	analysing sync.Mutex
	// analysed is written at the last analysis: Blink analyses again only
	// when time has advanced.
	analysed int64

	// magnitude is the smoothed spectrum, of the analyses so far.
	magnitude [analyserBins]float32
	// window is the Blackman window; cos and sin are the twiddles.
	window   [FFTSize]float64
	cos, sin [FFTSize]float64
	samples  [FFTSize]float32
}

func newAnalyser() *analyser {
	a := &analyser{}
	const alpha = 0.16
	const a0, a1, a2 = 0.5 * (1 - alpha), 0.5, 0.5 * alpha
	for i := range FFTSize {
		x := float64(i) / FFTSize
		a.window[i] = a0 - a1*math.Cos(2*math.Pi*x) + a2*math.Cos(2*math.Pi*2*x)
		a.cos[i] = math.Cos(2 * math.Pi * x)
		a.sin[i] = math.Sin(2 * math.Pi * x)
	}
	return a
}

// writeInput adds interleaved stereo frames to the analyser, mixed down to
// mono as Web Audio mixes speakers: (L+R)/2.
func (a *analyser) writeInput(buf []float32) {
	a.mu.Lock()
	for i := 0; i < len(buf); i += 2 {
		a.input[a.write] = 0.5 * (buf[i] + buf[i+1])
		a.write = (a.write + 1) % FFTSize
	}
	a.written += int64(len(buf) / 2)
	a.mu.Unlock()
}

// silence makes the analyser hear silence from now on, as the web player's
// does while nothing plays: its spectrum then falls to nothing.
func (a *analyser) silence() {
	a.mu.Lock()
	a.input = [FFTSize]float32{}
	a.written += FFTSize
	a.mu.Unlock()
}

// byteFrequencyData writes the analysis's bytes into data.
func (a *analyser) byteFrequencyData(data *[analyserBins]byte) {
	var out Analysis
	a.analyse(&out)
	*data = out.Bytes
}

// analyse analyses the last FFTSize samples, when samples came since the
// last analysis, as Blink does when time has advanced, and writes them and
// their spectrum into out.
func (a *analyser) analyse(out *Analysis) {
	a.analysing.Lock()
	defer a.analysing.Unlock()
	a.mu.Lock()
	n := copy(a.samples[:], a.input[a.write:])
	copy(a.samples[n:], a.input[:a.write])
	written := a.written
	a.mu.Unlock()
	out.Waveform = a.samples
	if written != a.analysed {
		a.analysed = written
		a.transform()
	}
	for k, m := range a.magnitude {
		db := linearToDecibels(m)
		out.Decibels[k] = db
		scaled := 255 * (float64(db) - minDecibels) * (1.0 / (maxDecibels - minDecibels))
		out.Bytes[k] = byte(min(max(scaled, 0), 255))
	}
}

// transform updates the smoothed magnitudes with the spectrum of samples.
func (a *analyser) transform() {
	// The windowed samples, as floats, as Blink keeps them.
	var x [FFTSize]float32
	for i, s := range a.samples {
		x[i] = s * float32(a.window[i])
	}
	const scale = 1.0 / FFTSize
	for k := range analyserBins {
		var re, im float64
		t := 0
		for _, v := range x {
			re += float64(v) * a.cos[t]
			im -= float64(v) * a.sin[t]
			t = (t + k) % FFTSize
		}
		if k == 0 {
			im = 0 // the packed Nyquist bin, which Blink blows away
		}
		magnitude := math.Hypot(re, im) * scale
		m := float32(smoothing*float64(a.magnitude[k]) + (1-smoothing)*magnitude)
		if math.IsNaN(float64(m)) || math.IsInf(float64(m), 0) {
			m = 0
		}
		a.magnitude[k] = m
	}
}
