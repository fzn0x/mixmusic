package audio

// The shelving and peaking filters of BiquadFilterNode, as Blink computes
// and runs them (third_party/blink/renderer/platform/audio/biquad.cc, under
// the BSD license of LICENSE.blink), for stereo and 5.1. The corrections of
// headphones (#616) add shelves of a Q of their own, and low-pass and
// high-pass filters, as Robert Bristow-Johnson's Audio EQ Cookbook makes
// them.

import "math"

type filterKind int

const (
	lowShelf filterKind = iota
	highShelf
	peaking
	// lowShelfQ and highShelfQ are shelves of a Q of their own: Blink's
	// have a slope of 1, which is a Q of 1/√2.
	lowShelfQ
	highShelfQ
	// lowPass and highPass are of a Q as the cookbook has it, not as
	// Blink's, whose Q is a resonance in dB.
	lowPass
	highPass
)

// leastQ is the least Q a filter computes with: a Q of 0 would divide by
// zero.
const leastQ = 1e-3

// biquad is a second-order filter of up to maxChannels channels.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	// The filter's memory, by channel: its last inputs and outputs.
	x1, x2, y1, y2 [maxChannels]float64
}

// set computes the coefficients of a filter at frequency hz, of quality q
// (for peaking, the shelves of a Q and the low-pass and high-pass) and gain
// in dB (for peaking and the shelves), as Blink's SetLowShelfParams,
// SetHighShelfParams and SetPeakingParams do, and the cookbook's LPF and
// HPF.
func (f *biquad) set(kind filterKind, hz, q, gain float64) {
	frequency := min(max(hz/(Rate/2), 0), 1)
	a := pow10(gain / 40)
	switch kind {
	case lowShelf, highShelf, lowShelfQ, highShelfQ:
		low := kind == lowShelf || kind == lowShelfQ
		if frequency == 1 && low || frequency == 0 && !low {
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
		if kind == lowShelfQ || kind == highShelfQ {
			alpha = math.Sin(w0) / (2 * max(q, leastQ))
		}
		k := math.Cos(w0)
		k2 := 2 * math.Sqrt(a) * alpha
		ap1, am1 := a+1, a-1
		if low {
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
	case lowPass, highPass:
		switch {
		case frequency == 1 && kind == lowPass || frequency == 0 && kind == highPass:
			f.normalize(1, 0, 0, 1, 0, 0) // all of it passes
		case frequency == 1 || frequency == 0:
			f.normalize(0, 0, 0, 1, 0, 0) // none of it does
		default:
			w0 := math.Pi * frequency
			alpha := math.Sin(w0) / (2 * max(q, leastQ))
			k := math.Cos(w0)
			if kind == lowPass {
				f.normalize((1-k)/2, 1-k, (1-k)/2, 1+alpha, -2*k, 1-alpha)
			} else {
				f.normalize((1+k)/2, -(1 + k), (1+k)/2, 1+alpha, -2*k, 1-alpha)
			}
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

// process filters frames of ch channels interleaved in place. As in Blink,
// the filter computes in double precision, and remembers its outputs as
// the floats it gives.
func (f *biquad) process(buf []float32, ch int) {
	for c := range ch {
		x1, x2, y1, y2 := f.x1[c], f.x2[c], f.y1[c], f.y2[c]
		for i := c; i < len(buf); i += ch {
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
func (f *biquad) pass(buf []float32, ch int) {
	if len(buf) < 2*ch {
		f.process(buf, ch)
		return
	}
	for c := range ch {
		last, before := float64(buf[len(buf)-ch+c]), float64(buf[len(buf)-2*ch+c])
		f.x1[c], f.x2[c], f.y1[c], f.y2[c] = last, before, last, before
	}
}

// forget clears the memory of the channels from from on.
func (f *biquad) forget(from int) {
	for c := from; c < maxChannels; c++ {
		f.x1[c], f.x2[c], f.y1[c], f.y2[c] = 0, 0, 0, 0
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
