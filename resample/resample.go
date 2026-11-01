// Package resample converts the sample rate of audio, as MixMusic's audio
// engine, which plays everything at 48 kHz, converts the 44.1 kHz of
// YouTube's AAC and the other rates AAC has.
//
// A Converter is a polyphase filter of Kaiser-windowed sincs. The frames of
// its output fall between those of its input at a few phases, as many as
// the output's rate divided by the greatest common divisor of the two rates:
// 160 from 44.1 to 48 kHz, 320 from 22.05 kHz, 2 from 24 kHz, 3 from 32
// kHz. The filter has a set of coefficients for each, computed exactly
// rather than interpolated between a few, which would add noise of its own.
// It passes what is below 0.907 of the lower rate's Nyquist frequency, 20
// kHz of 22.05 kHz, as it is, within 0.001 dB, and keeps the images and
// aliases it makes 100 dB below it.
package resample

import (
	"errors"
	"fmt"
	"math"
)

// ErrBadArg is why New refuses a rate or a count of channels out of range.
var ErrBadArg = errors.New("resample: invalid argument")

const (
	// passband is the share of the Nyquist frequency of the lower rate that
	// the filter passes untouched: 20 kHz of 22.05 kHz. The filter cuts at
	// the Nyquist frequency, and stops as far above it as it passes below
	// it: the images and aliases of what it passes fall where it stops.
	passband = 20000.0 / 22050
	// attenuation is how far below the sound the filter keeps images and
	// aliases, in dB, as the Kaiser window of its design gives it.
	attenuation = 100
	// block is the frames of input the converter takes in at a time.
	block = 1024
	// maxKernel bounds the coefficients of a filter: rates in a ratio of
	// large integers, which need many phases, are refused rather than
	// taking megabytes.
	maxKernel = 1 << 18
	// maxChannels is the most channels a converter converts, those of AAC's
	// 7.1.
	maxChannels = 8
	// tiny is the magnitude below which input samples are taken as 0:
	// 2^-100, 600 dB below full scale.
	tiny = 0x1p-100
)

// Converter converts interleaved audio of a few channels from one sample
// rate to another. It is not safe for concurrent use.
//
// Its output is aligned with its input: the output's frame n is the sound
// at n/to seconds after the input's first frame, which it computes as if
// silence came before that frame and after the last. A stream of k frames
// gives ceil(k·to/from) frames, the last of which Flush gives.
type Converter struct {
	channels int
	// up and down are the rates divided by their greatest common divisor:
	// the converter makes up frames for each down frames it reads. A
	// converter between equal rates has no kernel, and copies.
	up, down int
	// From one frame of output to the next, the input moves on by step
	// frames and frac phases, of up to a frame.
	step, frac int
	// taps is the count of coefficients of a phase, 2·half: the window of
	// an output frame at frame i of the input and a phase spans the input's
	// frames i-half+1 to i+half.
	half, taps int
	// kernel has the taps coefficients of each of up phases, one after the
	// other.
	kernel []float32
	// buf holds the input's frames that windows still need, interleaved:
	// fill of them, of a capacity of taps+block.
	buf  []float32
	fill int
	// pos is the frame of buf that the window of the next output frame
	// starts at, and phase its phase.
	pos, phase int
	// in and out are the frames read and written since the last Reset.
	in, out int64
}

// New returns a converter of channels channels, from 1 to 8, from rate
// from to rate to, in Hz. It refuses rates in a ratio of large integers,
// whose filter would be too large, with an error wrapping
// errors.ErrUnsupported.
func New(from, to, channels int) (*Converter, error) {
	if from <= 0 || to <= 0 || channels < 1 || channels > maxChannels {
		return nil, ErrBadArg
	}
	c := &Converter{channels: channels, up: 1, down: 1}
	if from == to {
		return c, nil
	}
	g := gcd(from, to)
	c.up, c.down = to/g, from/g
	c.step, c.frac = c.down/c.up, c.down%c.up

	// The filter cuts at half the lower rate, in cycles per frame of the
	// input, and goes from passing to stopping over width.
	cutoff := 0.5 * min(1, float64(to)/float64(from))
	width := 2 * (1 - passband) * cutoff
	// Kaiser's estimate of the length of the filter that stops by
	// attenuation over width, in frames of the input: taps, made even.
	n := math.Ceil((attenuation - 7.95) / (14.36 * width))
	c.half = int(min(n, maxKernel)+1) / 2
	c.taps = 2 * c.half
	if n > maxKernel || c.up > maxKernel/c.taps {
		return nil, fmt.Errorf("resample: %w: %d Hz to %d Hz", errors.ErrUnsupported, from, to)
	}
	c.kernel = make([]float32, c.up*c.taps)
	c.makeKernel(cutoff)
	c.buf = make([]float32, (c.taps+block)*channels)
	c.Reset()
	return c, nil
}

// makeKernel computes the coefficients of each phase: the sinc of a
// low-pass filter at cutoff, in cycles per frame of the input, under a
// Kaiser window of the converter's taps frames, at the offset of the phase.
// Each phase passes DC with a gain of 1.
func (c *Converter) makeKernel(cutoff float64) {
	beta := 0.1102 * (attenuation - 8.7)
	i0beta := bessel0(beta)
	for p := range c.up {
		k := c.kernel[p*c.taps : (p+1)*c.taps]
		offset := float64(p) / float64(c.up)
		var sum float64
		for m := range k {
			// The time of the coefficient's frame from the output frame's,
			// in frames of the input.
			t := float64(m-c.half+1) - offset
			x := t / float64(c.half)
			w := bessel0(beta*math.Sqrt(max(0, 1-x*x))) / i0beta
			v := w * 2 * cutoff * sinc(2*cutoff*t)
			k[m] = float32(v)
			sum += v
		}
		// The coefficients are normalized, as floats, and the rounding left
		// added to the largest, so that they sum to 1 as nearly as floats
		// can.
		var rounded float64
		largest := 0
		for m := range k {
			k[m] = float32(float64(k[m]) / sum)
			rounded += float64(k[m])
			if k[m] > k[largest] {
				largest = m
			}
		}
		k[largest] = float32(float64(k[largest]) + 1 - rounded)
	}
}

// Latency returns the most frames of output that the converter holds back,
// in the frames of the rate it converts to: the output of a moment comes
// once the input of Latency frames of output after it has come, and Flush
// gives what the input's end leaves. It delays no sound, which stays where
// it was in time; it is 0 between equal rates.
func (c *Converter) Latency() int {
	return int(ceilDiv(int64(c.half)*int64(c.up), int64(c.down)))
}

// MaxOutput returns the most frames Process writes when it reads frames
// frames of input: an out of that many frames takes all of in at once.
func (c *Converter) MaxOutput(frames int) int {
	return int(ceilDiv(int64(frames)*int64(c.up), int64(c.down)))
}

// Process converts the frames of in, interleaved, into out, as many as out
// holds, and returns the frames it read from in and those it wrote to out.
// It reads all of in unless out fills up; a partial frame at the end of
// either is left alone. Its output lags its input by up to Latency frames,
// which further input or Flush gives.
func (c *Converter) Process(in, out []float32) (read, written int) {
	ch := c.channels
	frames, room := len(in)/ch, len(out)/ch
	if c.kernel == nil {
		n := min(frames, room)
		copy(out[:n*ch], in[:n*ch])
		return n, n
	}
	for {
		written += c.produce(out[written*ch : room*ch])
		if written == room || read == frames {
			break
		}
		if c.fill == len(c.buf)/ch {
			c.compact()
		}
		n := min(len(c.buf)/ch-c.fill, frames-read)
		dst := c.buf[c.fill*ch:]
		for i, v := range in[read*ch : (read+n)*ch] {
			// Samples this small, 600 dB down, make subnormal products,
			// a hundred times slower to compute: they are silence.
			if v < tiny && v > -tiny {
				v = 0
			}
			dst[i] = v
		}
		c.fill += n
		read += n
		c.in += int64(n)
	}
	return read, written
}

// Flush writes into out the frames of output that the converter holds back,
// those whose windows reach past the end of the input, as if silence
// followed it, as many as out holds, and returns how many it wrote. A
// stream ends when Flush writes fewer frames than out holds; the converter
// is then as Reset leaves it, and a further Flush writes nothing.
func (c *Converter) Flush(out []float32) int {
	if c.kernel == nil {
		return 0
	}
	ch := c.channels
	end := ceilDiv(c.in*int64(c.up), int64(c.down))
	room, written := len(out)/ch, 0
	for c.out < end && written < room {
		if c.pos+c.taps > c.fill {
			if c.fill == len(c.buf)/ch {
				c.compact()
			}
			n := min(len(c.buf)/ch-c.fill, c.pos+c.taps-c.fill)
			clear(c.buf[c.fill*ch : (c.fill+n)*ch])
			c.fill += n
			continue
		}
		n := min(int64(room-written), end-c.out)
		written += c.produce(out[written*ch : (written+int(n))*ch])
	}
	if c.out == end {
		c.Reset()
	}
	return written
}

// Reset forgets the input so far, for a stream that starts again, as after
// a seek: the next frame of input is the first, with silence before it.
func (c *Converter) Reset() {
	if c.kernel == nil {
		return
	}
	// The windows of the first frames reach half-1 frames before the
	// input's first.
	c.fill = c.half - 1
	clear(c.buf[:c.fill*c.channels])
	c.pos, c.phase = 0, 0
	c.in, c.out = 0, 0
}

// compact moves the frames of buf that windows still need to its start.
func (c *Converter) compact() {
	ch := c.channels
	copy(c.buf, c.buf[c.pos*ch:c.fill*ch])
	c.fill -= c.pos
	c.pos = 0
}

// produce writes into out the frames of output whose windows buf holds, as
// many as out holds, and returns how many.
func (c *Converter) produce(out []float32) int {
	ch, taps := c.channels, c.taps
	n := 0
	for ; (n+1)*ch <= len(out) && c.pos+taps <= c.fill; n++ {
		k := c.kernel[c.phase*taps:][:taps]
		x := c.buf[c.pos*ch:][:taps*ch]
		y := out[n*ch:][:ch]
		switch ch {
		case 1:
			y[0] = dot(x, k)
		case 2:
			y[0], y[1] = dot2(x, k)
		default:
			for i := range y {
				y[i] = dotStride(x[i:], k, ch)
			}
		}
		c.pos += c.step
		c.phase += c.frac
		if c.phase >= c.up {
			c.phase -= c.up
			c.pos++
		}
	}
	c.out += int64(n)
	return n
}

// dot returns the sum of the products of x and k, of an even length.
func dot(x, k []float32) float32 {
	x = x[:len(k)]
	var s0, s1 float32
	for j := 0; j+1 < len(k); j += 2 {
		s0 += x[j] * k[j]
		s1 += x[j+1] * k[j+1]
	}
	return s0 + s1
}

// dot2 returns the sums of the products of k, of an even length, and each
// channel of x, interleaved stereo.
func dot2(x, k []float32) (float32, float32) {
	x = x[:2*len(k)]
	var l0, r0, l1, r1 float32
	for j := 0; j+1 < len(k); j += 2 {
		f := x[2*j : 2*j+4]
		l0 += f[0] * k[j]
		r0 += f[1] * k[j]
		l1 += f[2] * k[j+1]
		r1 += f[3] * k[j+1]
	}
	return l0 + l1, r0 + r1
}

// dotStride returns the sum of the products of k, of an even length, and
// every stride-th value of x. Like dot2, it adds in the order of dot: a
// channel converts to the same floats whatever the channels beside it.
func dotStride(x, k []float32, stride int) float32 {
	x = x[:(len(k)-1)*stride+1]
	var s0, s1 float32
	for j := 0; j+1 < len(k); j += 2 {
		s0 += x[j*stride] * k[j]
		s1 += x[(j+1)*stride] * k[j+1]
	}
	return s0 + s1
}

// sinc is sin(πx)/(πx).
func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

// bessel0 is I₀, the modified Bessel function of the first kind and order
// 0, of the Kaiser window: the sum of ((x/2)^k / k!)².
func bessel0(x float64) float64 {
	sum, term := 1.0, 1.0
	for k := 1; term > sum*1e-17; k++ {
		h := x / (2 * float64(k))
		term *= h * h
		sum += term
	}
	return sum
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// ceilDiv is a/b rounded up, for a ≥ 0 and b > 0.
func ceilDiv(a, b int64) int64 {
	return (a + b - 1) / b
}
