package audio

import (
	"math"
	"testing"
)

// headroomCases are bands to find the headroom of: boosts of one band,
// of several, of all, and cuts alone.
var headroomCases = map[string][len(Bands)]float64{
	"bass boost": {6, 5, 4, 2, 0, 0, 0, 0, 0, 0},
	"treble":     {0, 0, 0, 0, 0, 1, 2, 4, 5, 6},
	"one peak":   {0, 0, 0, 0, 0, 12, 0, 0, 0, 0},
	"lo-fi":      {-3, 0, 3, 4, 2, 0, -2, -4, -6, -8},
	"all up":     {12, 12, 12, 12, 12, 12, 12, 12, 12, 12},
	"zigzag":     {12, -12, 12, -12, 12, -12, 12, -12, 12, -12},
	"half step":  {0.5, 0, 0, 0, 0, 0, 0, 0, 0, 0},
}

// finePeak is the peak of the bands' response over a grid finer than
// Headroom's, and apart from it: 0 Hz and half the rate, and 500 to an
// octave between, from 3 Hz.
func finePeak(gains [len(Bands)]float64) float64 {
	peak := max(Response(gains, 0), Response(gains, Rate/2))
	for hz := 3.0; hz < Rate/2; hz *= math.Pow(2, 1.0/500) {
		peak = max(peak, Response(gains, hz))
	}
	return peak
}

// TestHeadroom cuts the preamp by the peak of the bands' summed response,
// and half a decibel: no frequency comes out louder than it went in, and
// the cut is no deeper than that, to a tenth.
func TestHeadroom(t *testing.T) {
	for name, gains := range headroomCases {
		h := Headroom(gains)
		peak := finePeak(gains)
		if got := peak + h; got > -headroomMargin+0.01 || got < -headroomMargin-0.11 {
			t.Errorf("%s: the peak, %.3f dB, with the preamp at %v, comes out at %.3f dB", name, peak, h, got)
		}
		if h < -30 {
			t.Errorf("%s: the preamp at %v, below its range", name, h)
		}
		if h != math.Round(h*10)/10 {
			t.Errorf("%s: the preamp at %v, not in tenths", name, h)
		}
	}
	for name, gains := range map[string][len(Bands)]float64{
		"flat": {},
		"cuts": {-6, -3, 0, 0, -1, 0, 0, -12, 0, -2},
	} {
		if h := Headroom(gains); h != 0 {
			t.Errorf("%s: the preamp at %v, though the bands boost nothing", name, h)
		}
	}
}

// TestHeadroomOnTheEngine plays a sine at the peak of the bands' response
// between 20 Hz and 20 kHz through the equalizer, its preamp at Headroom:
// it comes out as the response and the preamp tell, and quieter than it
// went in, by half a decibel at least.
func TestHeadroomOnTheEngine(t *testing.T) {
	for name, gains := range headroomCases {
		peakHz, peak := 0.0, math.Inf(-1)
		for hz := 20.0; hz < 20000; hz *= math.Pow(2, 1.0/48) {
			if r := Response(gains, hz); r > peak {
				peakHz, peak = hz, r
			}
		}
		h := Headroom(gains)
		eq := newEqualizer()
		eq.set(true, gains, h)
		eq.preamp.value = eq.preamp.target
		for i := range eq.gains {
			eq.gains[i].value = eq.gains[i].target
		}
		const amp = 0.5
		n := Rate
		in := soundTone(n, 2, peakHz, [maxChannels]float64{amp, amp}, 0)
		for i := 0; i < len(in); i += 2 * quantum {
			eq.process(in[i:i+2*quantum], 2)
		}
		out := 0.0
		for _, v := range in[len(in)/2:] { // past the filters' start
			out = max(out, math.Abs(float64(v)))
		}
		db := 20 * math.Log10(out/amp)
		if want := peak + h; db > -headroomMargin+0.05 || math.Abs(db-want) > 0.1 {
			t.Errorf("%s: %.0f Hz comes out at %.2f dB, want %.2f", name, peakHz, db, want)
		}
	}
}

// TestResponse sums the bands' own: a band alone peaks at its gain, at its
// frequency, and two add up in dB.
func TestResponse(t *testing.T) {
	var one [len(Bands)]float64
	one[5] = 6
	if r := Response(one, 1000); math.Abs(r-6) > 0.01 {
		t.Errorf("+6 dB at 1 kHz: %.3f dB there", r)
	}
	var other [len(Bands)]float64
	other[7] = -4
	both := one
	both[7] = -4
	for _, hz := range []float64{100, 700, 1000, 3000, 4000} {
		if got, want := Response(both, hz), Response(one, hz)+Response(other, hz); math.Abs(got-want) > 1e-9 {
			t.Errorf("%v Hz: %v dB, want %v", hz, got, want)
		}
	}
}
