package resample

import (
	"math"
	"math/cmplx"
	"sync"
	"testing"
)

// analysed is the frames of output an analysis takes, a power of two: 1.4
// s at 48 kHz, in bins of 0.73 Hz.
const analysed = 1 << 16

// window is a Kaiser window of β 20 over analysed frames, whose leakage
// falls below -150 dB beyond a few bins: far below what it measures.
var window = sync.OnceValues(func() ([]float64, float64) {
	w := make([]float64, analysed)
	var sum float64
	for i := range w {
		x := 2*float64(i)/(analysed-1) - 1
		w[i] = bessel0(20*math.Sqrt(max(0, 1-x*x))) / bessel0(20)
		sum += w[i]
	}
	return w, sum
})

// fft transforms a, of a power of two in length, in place.
func fft(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		step := cmplx.Exp(complex(0, -2*math.Pi/float64(size)))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := range size / 2 {
				u, v := a[start+k], a[start+k+size/2]*w
				a[start+k], a[start+k+size/2] = u+v, u-v
				w *= step
			}
		}
	}
}

// tones returns frames of interleaved stereo at rate: a sine of amplitude
// 0.5 at hz on the left, and a cosine of amplitude 0.25 on the right.
func tones(hz float64, rate, frames int) []float32 {
	out := make([]float32, 2*frames)
	for i := range frames {
		w := 2 * math.Pi * hz * float64(i) / float64(rate)
		out[2*i] = float32(0.5 * math.Sin(w))
		out[2*i+1] = float32(0.25 * math.Cos(w))
	}
	return out
}

// convert runs all of in through c, in one call, and flushes it.
func convert(t testing.TB, c *Converter, in []float32) []float32 {
	t.Helper()
	ch := c.channels
	out := make([]float32, (c.MaxOutput(len(in)/ch)+c.Latency())*ch)
	read, written := c.Process(in, out)
	if read != len(in)/ch {
		t.Fatalf("Process read %d frames of %d", read, len(in)/ch)
	}
	written += c.Flush(out[written*ch:])
	return out[:written*ch]
}

// measure is what an analysis finds of a tone.
type measure struct {
	gain  float64 // the tone's level to the input's, in dB
	delay float64 // how late the tone is, in frames of the output
	spur  float64 // the largest of the rest of the spectrum, in dB of the input's level
	at    float64 // its frequency, in Hz
}

// analyse measures the tone of amplitude amp at hz, a sine of phase phase
// at the input's first frame, in channel ch of out, of channels channels at
// rate, from frame start for analysed frames.
func analyse(out []float32, channels, ch, start, rate int, hz, amp, phase float64) measure {
	w, sum := window()
	a := make([]complex128, analysed)
	var re, im float64
	for i := range a {
		v := float64(out[(start+i)*channels+ch]) * w[i]
		a[i] = complex(v, 0)
		x := 2*math.Pi*hz*float64(start+i)/float64(rate) + phase
		re += v * math.Sin(x)
		im += v * math.Cos(x)
	}
	// The fit of the tone at its frequency: its level, and its phase to
	// the input's, in time.
	var m measure
	m.gain = 20 * math.Log10(2*math.Hypot(re, im)/sum/amp)
	m.delay = -math.Atan2(im, re) / (2 * math.Pi * hz) * float64(rate)
	fft(a)
	bin := hz / float64(rate) * analysed
	full := amp * sum / 2
	for k := range analysed/2 + 1 {
		if math.Abs(float64(k)-bin) <= 12 {
			continue
		}
		if db := 20 * math.Log10(cmplx.Abs(a[k])/full); db > m.spur || m.at == 0 {
			m.spur, m.at = db, float64(k)*float64(rate)/analysed
		}
	}
	return m
}

// passTones are the frequencies of tones the passband tests play at 44.1
// kHz, and at the same share of the Nyquist frequency at other rates.
var passTones = []float64{
	100, 200, 500, 1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000,
	10000, 11000, 12000, 13000, 14000, 15000, 16000, 17000, 18000, 18500,
	19000, 19500, 20000,
}

// TestPassband converts tones across the band from each rate AAC has to
// 48 kHz: they keep their level and their place in time, and the images
// and aliases the conversion makes stay far below them.
func TestPassband(t *testing.T) {
	const to = 48000
	for _, from := range []int{44100, 32000, 24000, 22050, 16000, 11025, 8000} {
		c, err := New(from, to, 2)
		if err != nil {
			t.Fatal(err)
		}
		skip := 4 * c.Latency()
		frames := (skip+analysed+4*c.Latency())*from/to + 1
		var worstGain, worstDelay, worstSpur float64 = 0, 0, -400
		var worstSpurAt, worstSpurHz float64
		for _, hz := range passTones {
			hz := hz * float64(from) / 44100
			out := convert(t, c, tones(hz, from, frames))
			for ch, amp := range []float64{0.5, 0.25} {
				m := analyse(out, 2, ch, skip, to, hz, amp, float64(ch)*math.Pi/2)
				if math.Abs(m.gain) > math.Abs(worstGain) {
					worstGain = m.gain
				}
				worstDelay = max(worstDelay, math.Abs(m.delay))
				if m.spur > worstSpur {
					worstSpur, worstSpurAt, worstSpurHz = m.spur, m.at, hz
				}
				if math.Abs(m.gain) > 0.01 {
					t.Errorf("%d Hz: a tone at %.0f Hz has a gain of %+.4f dB", from, hz, m.gain)
				}
				if math.Abs(m.delay) > 1e-3 {
					t.Errorf("%d Hz: a tone at %.0f Hz is %.5f frames late", from, hz, m.delay)
				}
				if m.spur > -90 {
					t.Errorf("%d Hz: a tone at %.0f Hz makes %.1f dB at %.0f Hz", from, hz, m.spur, m.at)
				}
			}
		}
		t.Logf("%5d Hz, %d taps by %d phases: gain within %+.5f dB, delay within %.1e frames, spurs at most %.1f dB (%.0f Hz, of %.0f Hz)",
			from, c.taps, c.up, worstGain, worstDelay, worstSpur, worstSpurAt, worstSpurHz)
	}
}

// TestAliases converts down, from rates above 48 kHz, tones the output
// can't hold: what they leave, aliased below its Nyquist frequency, stays
// far below them. Tones it can hold keep their level, as up.
func TestAliases(t *testing.T) {
	for _, r := range []struct{ from, to int }{{96000, 48000}, {88200, 48000}, {64000, 48000}, {48000, 44100}} {
		c, err := New(r.from, r.to, 2)
		if err != nil {
			t.Fatal(err)
		}
		nyquist := float64(r.to) / 2
		skip := 4 * c.Latency()
		frames := (skip+analysed+4*c.Latency())*r.from/r.to + 1
		worstGain, worstAlias, worstAt := 0.0, -400.0, 0.0
		for _, share := range []float64{0.01, 0.1, 0.3, 0.5, 0.7, 0.8, 0.85, 0.9, passband} {
			hz := share * nyquist
			out := convert(t, c, tones(hz, r.from, frames))
			m := analyse(out, 2, 0, skip, r.to, hz, 0.5, 0)
			if math.Abs(m.gain) > math.Abs(worstGain) {
				worstGain = m.gain
			}
			if math.Abs(m.gain) > 0.01 || m.spur > -90 {
				t.Errorf("%d to %d Hz: a tone at %.0f Hz has a gain of %+.4f dB, and makes %.1f dB at %.0f Hz", r.from, r.to, hz, m.gain, m.spur, m.at)
			}
		}
		// Tones from the stopband, as far above the Nyquist frequency as
		// the passband ends below it, to that of the input.
		for _, share := range []float64{2 - passband, 1.1, 1.2, 1.4, 1.6, 1.8, 1.95} {
			hz := share * nyquist
			if hz >= float64(r.from)/2 {
				continue
			}
			out := convert(t, c, tones(hz, r.from, frames))
			// The tone's alias, if it were let through; the analysis
			// finds whatever is left, there or elsewhere.
			alias := math.Abs(math.Remainder(hz, float64(r.to)))
			m := analyse(out, 2, 0, skip, r.to, alias, 0.5, 0)
			level := max(m.gain, m.spur)
			if level > worstAlias {
				worstAlias, worstAt = level, hz
			}
			if level > -90 {
				t.Errorf("%d to %d Hz: a tone at %.0f Hz leaves %.1f dB", r.from, r.to, hz, level)
			}
		}
		if worstAt == 0 {
			t.Logf("%d to %d Hz, %d taps by %d phases: gain within %+.5f dB; the input holds nothing above the stopband",
				r.from, r.to, c.taps, c.up, worstGain)
			continue
		}
		t.Logf("%d to %d Hz, %d taps by %d phases: gain within %+.5f dB, aliases at most %.1f dB (of %.0f Hz)",
			r.from, r.to, c.taps, c.up, worstGain, worstAlias, worstAt)
	}
}
