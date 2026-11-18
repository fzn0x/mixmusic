package audio

import "math"

// Auto headroom (#615): the preamp cuts as much as the equalizer's bands
// boost at most, so that no frequency comes out louder than it went in.

// headroomMargin is what Headroom cuts beyond the peak, in dB: room for
// the peaks between samples, and the filters' gliding.
const headroomMargin = 0.5

// responseGrid is the frequencies Response's peak is looked for at: 0 Hz,
// where a low shelf boosts most, then 96 to an octave, from 5 Hz to just
// under half the rate, and each band's own.
var responseGrid = func() []float64 {
	hz := []float64{0}
	for f := 5.0; f < Rate/2; f *= math.Pow(2, 1.0/96) {
		hz = append(hz, f)
	}
	hz = append(hz, Rate/2-1)
	return append(hz, Bands[:]...)
}()

// gainAt returns the gain the filter gives a sine of hz, as a power ratio.
func (f *biquad) gainAt(hz float64) float64 {
	w := 2 * math.Pi * hz / Rate
	c1, c2 := math.Cos(w), math.Cos(2*w)
	num := f.b0*f.b0 + f.b1*f.b1 + f.b2*f.b2 + 2*(f.b0*f.b1+f.b1*f.b2)*c1 + 2*f.b0*f.b2*c2
	den := 1 + f.a1*f.a1 + f.a2*f.a2 + 2*(f.a1+f.a1*f.a2)*c1 + 2*f.a2*c2
	return num / den
}

// bandFilters are the filters of the bands at gains, but those at 0 dB,
// which are the identity.
func bandFilters(gains [len(Bands)]float64) []biquad {
	var fs []biquad
	for i, g := range gains {
		if g != 0 {
			var f biquad
			f.set(bandKind(i), Bands[i], bandQ, g)
			fs = append(fs, f)
		}
	}
	return fs
}

// Response returns the gain, in dB, that the equalizer's bands at gains,
// in dB, give a sine of hz: the sum of each band's.
func Response(gains [len(Bands)]float64, hz float64) float64 {
	p := 1.0
	for _, f := range bandFilters(gains) {
		p *= f.gainAt(hz)
	}
	return 10 * math.Log10(p)
}

// Headroom returns the preamp, in dB, of Auto headroom for the bands at
// gains: the peak of their summed response, plus half a decibel, as a cut,
// rounded down to a tenth; 0 for bands that boost nothing.
func Headroom(gains [len(Bands)]float64) float64 {
	fs := bandFilters(gains)
	peak := 1.0
	for _, hz := range responseGrid {
		p := 1.0
		for i := range fs {
			p *= fs[i].gainAt(hz)
		}
		peak = max(peak, p)
	}
	db := 10 * math.Log10(peak)
	if db < 1e-9 {
		return 0
	}
	return -math.Ceil((db+headroomMargin)*10-1e-9) / 10
}
