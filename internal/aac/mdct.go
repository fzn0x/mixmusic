package aac

// Port of Filterbank/MDCT.cs and Filterbank/FFT.cs: the inverse MDCT, by a
// complex FFT of a quarter of its size. SharpJaad holds the twiddles and
// the roots of unity in tables; this package computes them.

import "math"

// mdct holds the tables of an inverse MDCT of n samples.
type mdct struct {
	n int
	// twiddle are sqrt(2/n) (cos, sin)(2π(k + 1/8)/n), for k up to n/4,
	// which turn the coefficients into the input of the FFT and its output
	// into the samples, and scale them by the 2/n of the inverse MDCT.
	twiddle [][2]float32
	// roots are e^(2πik/m) for k up to m/2, of the FFT of m = n/4 points.
	roots [][2]float32
	// bitrev is the permutation of the FFT's input, which reverses the
	// bits of each index.
	bitrev []uint16
}

var mdctLong, mdctShort mdct

func (t *mdct) init(n int) {
	t.n = n
	m := n / 4
	scale := math.Sqrt(2 / float64(n))
	t.twiddle = make([][2]float32, m)
	for k := range t.twiddle {
		a := 2 * math.Pi * (float64(k) + 1.0/8) / float64(n)
		t.twiddle[k] = [2]float32{float32(scale * math.Cos(a)), float32(scale * math.Sin(a))}
	}
	t.roots = make([][2]float32, m/2)
	for k := range t.roots {
		a := 2 * math.Pi * float64(k) / float64(m)
		t.roots[k] = [2]float32{float32(math.Cos(a)), float32(math.Sin(a))}
	}
	t.bitrev = make([]uint16, m)
	bits := 0
	for 1<<bits < m {
		bits++
	}
	for i := range t.bitrev {
		r := 0
		for b := range bits {
			r |= (i >> b & 1) << (bits - 1 - b)
		}
		t.bitrev[i] = uint16(r)
	}
}

// inverse computes the inverse MDCT of the n/2 coefficients of in into
// the n samples of out, with buf, of n/4 entries, for the FFT:
// out(i) = 2/n Σ in(k) cos(2π/n (i + n0) (k + 1/2)), n0 = (n/2 + 1)/2.
func (t *mdct) inverse(in, out []float32, buf [][2]float32) {
	n2, n4, n8 := t.n/2, t.n/4, t.n/8
	in, out, buf = in[:n2], out[:t.n], buf[:n4]
	tw := t.twiddle

	// Pre-FFT complex multiplication.
	for k := range n4 {
		x0, x1 := in[2*k], in[n2-1-2*k]
		c, s := tw[k][0], tw[k][1]
		buf[k] = [2]float32{x1*c - x0*s, x0*c + x1*s}
	}

	t.fft(buf)

	// Post-FFT complex multiplication.
	for k := range n4 {
		re, im := buf[k][0], buf[k][1]
		c, s := tw[k][0], tw[k][1]
		buf[k] = [2]float32{re*c - im*s, im*c + re*s}
	}

	// Reordering.
	for k := 0; k < n8; k += 2 {
		out[2*k] = buf[n8+k][1]
		out[2+2*k] = buf[n8+1+k][1]

		out[1+2*k] = -buf[n8-1-k][0]
		out[3+2*k] = -buf[n8-2-k][0]

		out[n4+2*k] = buf[k][0]
		out[n4+2+2*k] = buf[1+k][0]

		out[n4+1+2*k] = -buf[n4-1-k][1]
		out[n4+3+2*k] = -buf[n4-2-k][1]

		out[n2+2*k] = buf[n8+k][0]
		out[n2+2+2*k] = buf[n8+1+k][0]

		out[n2+1+2*k] = -buf[n8-1-k][1]
		out[n2+3+2*k] = -buf[n8-2-k][1]

		out[n2+n4+2*k] = -buf[k][1]
		out[n2+n4+2+2*k] = -buf[1+k][1]

		out[n2+n4+1+2*k] = buf[n4-1-k][0]
		out[n2+n4+3+2*k] = buf[n4-2-k][0]
	}
}

// fft is the inverse complex FFT of x, unscaled, in place: radix 2, with
// a first round of radix 4, after the bit-reversal permutation.
func (t *mdct) fft(x [][2]float32) {
	m := len(x)
	for i, j := range t.bitrev {
		if i < int(j) {
			x[i], x[j] = x[j], x[i]
		}
	}

	// The first round, of radix 4.
	for i := 0; i < m; i += 4 {
		a0, a1 := x[i][0]+x[i+1][0], x[i][1]+x[i+1][1]
		b0, b1 := x[i+2][0]+x[i+3][0], x[i+2][1]+x[i+3][1]
		c0, c1 := x[i][0]-x[i+1][0], x[i][1]-x[i+1][1]
		d0, d1 := x[i+2][0]-x[i+3][0], x[i+2][1]-x[i+3][1]
		x[i] = [2]float32{a0 + b0, a1 + b1}
		x[i+2] = [2]float32{a0 - b0, a1 - b1}
		x[i+1] = [2]float32{c0 - d1, c1 + d0}
		x[i+3] = [2]float32{c0 + d1, c1 - d0}
	}

	// The rounds from the bottom up.
	for i := 4; i < m; i <<= 1 {
		step := m / (2 * i)
		for j := 0; j < m; j += 2 * i {
			for k := range i {
				r := t.roots[k*step]
				p, q := &x[j+k], &x[i+j+k]
				z0 := q[0]*r[0] - q[1]*r[1]
				z1 := q[0]*r[1] + q[1]*r[0]
				q[0], q[1] = p[0]-z0, p[1]-z1
				p[0], p[1] = p[0]+z0, p[1]+z1
			}
		}
	}
}
