package audio

import (
	"math"
	"testing"
)

// The shelves of a Q of their own and the low-pass and high-pass filters of
// the corrections of headphones (#616), sample by sample.

// cookbook returns the coefficients the Audio EQ Cookbook gives a filter
// of kind at hz, of quality q and gain in dB, normalized, computed apart
// from biquad.set: but for the shelves' 10^(gain/40), which is Blink's, in
// single precision, as theirs.
func cookbook(kind filterKind, hz, q, gain float64) (b0, b1, b2, a1, a2 float64) {
	w0 := 2 * math.Pi * hz / Rate
	cw, alpha := math.Cos(w0), math.Sin(w0)/(2*q)
	a := pow10(gain / 40)
	sa := 2 * math.Sqrt(a) * alpha
	var a0 float64
	switch kind {
	case lowPass:
		b0, b1, b2, a0, a1, a2 = (1-cw)/2, 1-cw, (1-cw)/2, 1+alpha, -2*cw, 1-alpha
	case highPass:
		b0, b1, b2, a0, a1, a2 = (1+cw)/2, -(1 + cw), (1+cw)/2, 1+alpha, -2*cw, 1-alpha
	case lowShelfQ:
		b0 = a * ((a + 1) - (a-1)*cw + sa)
		b1 = 2 * a * ((a - 1) - (a+1)*cw)
		b2 = a * ((a + 1) - (a-1)*cw - sa)
		a0 = (a + 1) + (a-1)*cw + sa
		a1 = -2 * ((a - 1) + (a+1)*cw)
		a2 = (a + 1) + (a-1)*cw - sa
	case highShelfQ:
		b0 = a * ((a + 1) + (a-1)*cw + sa)
		b1 = -2 * a * ((a - 1) + (a+1)*cw)
		b2 = a * ((a + 1) + (a-1)*cw - sa)
		a0 = (a + 1) - (a-1)*cw + sa
		a1 = 2 * ((a - 1) - (a+1)*cw)
		a2 = (a + 1) - (a-1)*cw - sa
	}
	return b0 / a0, b1 / a0, b2 / a0, a1 / a0, a2 / a0
}

// cookbookRun filters one channel of x through the cookbook's filter, in
// double precision, as direct form I, its outputs the floats the engine
// plays.
func cookbookRun(kind filterKind, hz, q, gain float64, x []float32) []float64 {
	b0, b1, b2, a1, a2 := cookbook(kind, hz, q, gain)
	y := make([]float64, len(x))
	var x1, x2, y1, y2 float64
	for i, v := range x {
		in := float64(v)
		y[i] = float64(float32(b0*in + b1*x1 + b2*x2 - a1*y1 - a2*y2))
		x2, x1 = x1, in
		y2, y1 = y1, y[i]
	}
	return y
}

// qFilters are filters of the new kinds to test: gentle and steep, low
// and high.
var qFilters = []struct {
	name             string
	kind             filterKind
	hz, q, gain      float64
	dc, nyquist, mid float64 // their gains in dB at 0 Hz, half the rate, and hz
}{
	{"low-pass 1 kHz", lowPass, 1000, math.Sqrt2 / 2, 0, 0, math.Inf(-1), -3.0103},
	{"low-pass 8 kHz, Q 2", lowPass, 8000, 2, 0, 0, math.Inf(-1), 6.0206},
	{"high-pass 30 Hz", highPass, 30, math.Sqrt2 / 2, 0, math.Inf(-1), 0, -3.0103},
	{"high-pass 2 kHz, Q 0.5", highPass, 2000, 0.5, 0, math.Inf(-1), 0, -6.0206},
	{"low shelf +6 dB at 105 Hz, Q 0.7", lowShelfQ, 105, 0.7, 6, 6, 0, 3},
	{"low shelf -9.5 dB at 300 Hz, Q 1.5", lowShelfQ, 300, 1.5, -9.5, -9.5, 0, -4.75},
	{"high shelf +2.2 dB at 10 kHz, Q 0.7", highShelfQ, 10000, 0.7, 2.2, 0, 2.2, 1.1},
	{"high shelf -4 dB at 3 kHz, Q 0.4", highShelfQ, 3000, 0.4, -4, 0, -4, -2},
}

// TestQFiltersSampleBySample filters noise through each kind as the
// cookbook does, sample for sample, on two channels and on six, each
// channel apart.
func TestQFiltersSampleBySample(t *testing.T) {
	for _, c := range qFilters {
		for _, ch := range []int{2, 6} {
			in := soundNoise(Rate/4, ch, 11)
			var f biquad
			f.set(c.kind, c.hz, c.q, c.gain)
			out := append([]float32(nil), in...)
			// A quantum at a time, as the engine runs it: the memory
			// carries across.
			for i := 0; i < len(out); i += ch * quantum {
				f.process(out[i:min(i+ch*quantum, len(out))], ch)
			}
			for k := range ch {
				x := make([]float32, 0, len(in)/ch)
				for i := k; i < len(in); i += ch {
					x = append(x, in[i])
				}
				want := cookbookRun(c.kind, c.hz, c.q, c.gain, x)
				for i, w := range want {
					if got := float64(out[i*ch+k]); math.Abs(got-w) > 1e-6*max(1, math.Abs(w)) {
						t.Fatalf("%s, %d channels, channel %d, sample %d: %v, want %v", c.name, ch, k, i, got, w)
					}
				}
			}
		}
	}
}

// qToneDB is the gain in dB a sine of hz gets from the filter, measured on
// its samples once it settled.
func qToneDB(kind filterKind, hz, q, gain, tone float64) float64 {
	var f biquad
	f.set(kind, hz, q, gain)
	in := soundTone(Rate, 1, tone, [maxChannels]float64{0.25}, 0)
	out := append([]float32(nil), in...)
	f.process(out, 1)
	var pin, pout float64
	for i := len(in) / 2; i < len(in); i++ {
		pin += float64(in[i]) * float64(in[i])
		pout += float64(out[i]) * float64(out[i])
	}
	return 10 * math.Log10(pout/pin)
}

// TestQFiltersResponse gives each filter its gain at 0 Hz, at half the
// rate, and at its own frequency, where a shelf is half way and a
// low-pass or high-pass at the gain of its Q, as the response computed
// from the coefficients says, and as a sine played through it measures.
func TestQFiltersResponse(t *testing.T) {
	db := func(f *biquad, hz float64) float64 { return 10 * math.Log10(f.gainAt(hz)) }
	for _, c := range qFilters {
		var f biquad
		f.set(c.kind, c.hz, c.q, c.gain)
		// At 0 Hz and half the rate, z is 1 and -1: the response is the
		// coefficients' sums, which gainAt's cosines would round.
		dc := (f.b0 + f.b1 + f.b2) / (1 + f.a1 + f.a2)
		nyquist := (f.b0 - f.b1 + f.b2) / (1 - f.a1 + f.a2)
		for _, at := range []struct {
			hz, gain, want float64
		}{{0, dc, c.dc}, {Rate / 2, nyquist, c.nyquist}, {c.hz, math.Sqrt(f.gainAt(c.hz)), c.mid}} {
			if math.IsInf(at.want, -1) {
				if math.Abs(at.gain) > 1e-9 {
					t.Errorf("%s: a gain of %v at %v Hz, want none", c.name, at.gain, at.hz)
				}
				continue
			}
			if got := 20 * math.Log10(math.Abs(at.gain)); math.Abs(got-at.want) > 0.01 {
				t.Errorf("%s: %.4f dB at %v Hz, want %.4f", c.name, got, at.hz, at.want)
			}
		}
		for _, tone := range []float64{60, c.hz, 1000, 7000} {
			if got, want := qToneDB(c.kind, c.hz, c.q, c.gain, tone), db(&f, tone); math.Abs(got-want) > 0.05 {
				t.Errorf("%s: a sine of %v Hz comes out %.3f dB, its response says %.3f", c.name, tone, got, want)
			}
		}
	}
	// A low-pass falls 12 dB an octave, well past its frequency, and a
	// high-pass rises as much well below its own.
	var lp, hp biquad
	lp.set(lowPass, 100, math.Sqrt2/2, 0)
	hp.set(highPass, 8000, math.Sqrt2/2, 0)
	if d := db(&lp, 1000) - db(&lp, 2000); math.Abs(d-12) > 0.2 {
		t.Errorf("the low-pass falls %.2f dB from 1 to 2 kHz", d)
	}
	if d := db(&hp, 1000) - db(&hp, 500); math.Abs(d-12) > 0.2 {
		t.Errorf("the high-pass rises %.2f dB from 500 Hz to 1 kHz", d)
	}
}

// TestShelfQ gives a shelf of Q 1/√2 Blink's coefficients, whose slope of
// 1 it is, and a shelf of a higher Q the bump and dip about its frequency
// that Blink's never has.
func TestShelfQ(t *testing.T) {
	for _, kinds := range [][2]filterKind{{lowShelf, lowShelfQ}, {highShelf, highShelfQ}} {
		var blink, q biquad
		blink.set(kinds[0], 250, bandQ, 7.5)
		q.set(kinds[1], 250, math.Sqrt2/2, 7.5)
		for i, v := range [][2]float64{{blink.b0, q.b0}, {blink.b1, q.b1}, {blink.b2, q.b2}, {blink.a1, q.a1}, {blink.a2, q.a2}} {
			if math.Abs(v[0]-v[1]) > 1e-12 {
				t.Errorf("kind %d, coefficient %d: %v, Blink's %v", kinds[1], i, v[1], v[0])
			}
		}
	}
	peak := func(q float64) (most, least float64) {
		var f biquad
		f.set(lowShelfQ, 200, q, 6)
		most, least = math.Inf(-1), math.Inf(1)
		for hz := 5.0; hz < Rate/2; hz *= math.Pow(2, 1.0/48) {
			g := 10 * math.Log10(f.gainAt(hz))
			most, least = max(most, g), min(least, g)
		}
		return most, least
	}
	if most, least := peak(math.Sqrt2 / 2); most > 6.001 || least < -0.001 {
		t.Errorf("Q 0.707 goes from %.3f to %.3f dB, past the shelf's", least, most)
	}
	if most, least := peak(2); most < 7 || least > -0.5 {
		t.Errorf("Q 2 goes from %.3f to %.3f dB: no bump and dip", least, most)
	}
}

// TestQFiltersAtTheEdges keeps the filters at 0 Hz and half the rate from
// dividing by zero: a low-pass there passes all or nothing, and a Q of 0
// computes as the least.
func TestQFiltersAtTheEdges(t *testing.T) {
	for _, c := range []struct {
		kind filterKind
		hz   float64
		b0   float64
	}{{lowPass, Rate / 2, 1}, {lowPass, 0, 0}, {highPass, 0, 1}, {highPass, Rate, 0}} {
		var f biquad
		f.set(c.kind, c.hz, 0.7, 0)
		if f.b0 != c.b0 || f.b1 != 0 || f.b2 != 0 || f.a1 != 0 || f.a2 != 0 {
			t.Errorf("kind %d at %v Hz: %+v", c.kind, c.hz, f)
		}
	}
	for _, kind := range []filterKind{lowPass, highPass, lowShelfQ, highShelfQ} {
		var f biquad
		f.set(kind, 1000, 0, 6)
		for _, v := range []float64{f.b0, f.b1, f.b2, f.a1, f.a2} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Errorf("kind %d of Q 0: %+v", kind, f)
				break
			}
		}
	}
}
