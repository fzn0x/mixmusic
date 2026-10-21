package audio

// The shelving and peaking filters of BiquadFilterNode, as Blink computes
// and runs them (third_party/blink/renderer/platform/audio/biquad.cc, under
// the BSD license of LICENSE.blink), for stereo.

import "math"

type filterKind int

const (
	lowShelf filterKind = iota
	highShelf
	peaking
)

// biquad is a second-order filter of two channels.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	// The filter's memory, by channel: its last inputs and outputs.
	x1, x2, y1, y2 [2]float64
}

// set computes the coefficients of a filter at frequency hz, of quality q
// (for peaking) and gain in dB, as Blink's SetLowShelfParams,
// SetHighShelfParams and SetPeakingParams do.
func (f *biquad) set(kind filterKind, hz, q, gain float64) {
	frequency := min(max(hz/(Rate/2), 0), 1)
	a := pow10(gain / 40)
	switch kind {
	case lowShelf, highShelf:
		if frequency == 1 && kind == lowShelf || frequency == 0 && kind == highShelf {
			f.normalize(a*a, 0, 0, 1, 0, 0)
			return
		}
		if frequency == 1 || frequency == 0 {
			f.normalize(1, 0, 0, 1, 0, 0)
			return
		}
		w0 := math.Pi * frequency
		const s = 1 // the slope: 1 is the most
		alpha := 0.5 * math.Sin(w0) * math.Sqrt((a+1/a)*(1/s-1)+2)
		k := math.Cos(w0)
		k2 := 2 * math.Sqrt(a) * alpha
		ap1, am1 := a+1, a-1
		if kind == lowShelf {
			f.normalize(a*(ap1-am1*k+k2), 2*a*(am1-ap1*k), a*(ap1-am1*k-k2),
				ap1+am1*k+k2, -2*(am1+ap1*k), ap1+am1*k-k2)
		} else {
			f.normalize(a*(ap1+am1*k+k2), -2*a*(am1+ap1*k), a*(ap1+am1*k-k2),
				ap1-am1*k+k2, 2*(am1-ap1*k), ap1-am1*k-k2)
		}
	case peaking:
		q = max(0, q)
		switch {
		case frequency <= 0 || frequency >= 1:
			f.normalize(1, 0, 0, 1, 0, 0)
		case q == 0:
			f.normalize(a*a, 0, 0, 1, 0, 0)
		default:
			w0 := math.Pi * frequency
			alpha := math.Sin(w0) / (2 * q)
			k := math.Cos(w0)
			f.normalize(1+alpha*a, -2*k, 1-alpha*a, 1+alpha/a, -2*k, 1-alpha/a)
		}
	}
}

// pow10 is Blink's 10^x, which it computes in single precision, as
// expf(x·ln 10): the filters at low frequencies are sensitive enough for the
// precision to show.
func pow10(x float64) float64 {
	return float64(float32(math.Exp(float64(float32(x * math.Ln10)))))
}

// normalize is SetNormalizedCoefficients.
func (f *biquad) normalize(b0, b1, b2, a0, a1, a2 float64) {
	inv := 1 / a0
	f.b0, f.b1, f.b2, f.a1, f.a2 = b0*inv, b1*inv, b2*inv, a1*inv, a2*inv
}

// process filters interleaved stereo frames in place. As in Blink, the
// filter computes in double precision, and remembers its outputs as the
// floats it gives.
func (f *biquad) process(buf []float32) {
	for c := range 2 {
		x1, x2, y1, y2 := f.x1[c], f.x2[c], f.y1[c], f.y2[c]
		for i := c; i < len(buf); i += 2 {
			x := buf[i]
			y := float32(f.b0*float64(x) + f.b1*x1 + f.b2*x2 - f.a1*y1 - f.a2*y2)
			buf[i] = y
			x2, x1 = x1, float64(x)
			y2, y1 = y1, float64(y)
		}
		f.x1[c], f.x2[c], f.y1[c], f.y2[c] = flush(x1), flush(x2), flush(y1), flush(y2)
	}
}

// pass lets frames through a filter that is the identity, as a filter at 0
// dB is, keeping its memory as processing them would.
func (f *biquad) pass(buf []float32) {
	if len(buf) < 4 {
		f.process(buf)
		return
	}
	for c := range 2 {
		last, before := float64(buf[len(buf)-2+c]), float64(buf[len(buf)-4+c])
		f.x1[c], f.x2[c], f.y1[c], f.y2[c] = last, before, last, before
	}
}

// flush is DenormalDisabler::FlushDenormalFloatToZero, which keeps
// subnormal numbers, slow to compute, out of the memory of filters.
func flush(v float64) float64 {
	if math.Abs(v) < math.SmallestNonzeroFloat32*(1<<23) {
		return 0
	}
	return v
}
