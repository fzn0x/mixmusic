package ac3

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestWindow checks that the window the decoder computes, the
// Kaiser-Bessel-derived one of alpha 5, is the window of Table 7.33 to the
// five digits the table gives, and that it reconstructs: w[n]^2 +
// w[255-n]^2 = 1, the second half of the window being the first reversed.
func TestWindow(t *testing.T) {
	var tf transform
	tf.init()
	for n, want := range windowTable {
		if got := float64(tf.window[n]); math.Abs(got-want) > 0.5e-5+1e-9 {
			t.Errorf("w[%d] = %.7f, Table 7.33 has %.5f", n, got, want)
		}
	}
	for n := range 128 {
		a, b := float64(tf.window[n]), float64(tf.window[255-n])
		if e := a*a + b*b - 1; math.Abs(e) > 1e-6 {
			t.Errorf("w[%d]^2 + w[%d]^2 - 1 = %g", n, 255-n, e)
		}
	}
}

// standardInverse is the inverse transform of section 7.9.4 as its
// equations give it, in float64, with the complex IFFTs as the sums they
// are: of a long block, or two short ones.
func standardInverse(X []float64, delay []float64, short bool, w []float64) []float64 {
	const N = 512
	x := make([]float64, N)
	ifft := func(Z []complex128, step float64) []complex128 {
		z := make([]complex128, len(Z))
		for n := range z {
			for k, v := range Z {
				a := step * math.Pi * float64(k*n) / N
				z[n] += v * complex(math.Cos(a), math.Sin(a))
			}
		}
		return z
	}
	win := func(i int) float64 { return w[i] }
	if !short {
		Z := make([]complex128, N/4)
		for k := range Z {
			c := complex(-math.Cos(2*math.Pi*float64(8*k+1)/(8*N)), -math.Sin(2*math.Pi*float64(8*k+1)/(8*N)))
			Z[k] = complex(X[N/2-2*k-1], X[2*k]) * c
		}
		z := ifft(Z, 8)
		y := make([]complex128, N/4)
		for n := range y {
			c := complex(-math.Cos(2*math.Pi*float64(8*n+1)/(8*N)), -math.Sin(2*math.Pi*float64(8*n+1)/(8*N)))
			y[n] = z[n] * c
		}
		yr := func(i int) float64 { return real(y[i]) }
		yi := func(i int) float64 { return imag(y[i]) }
		for n := range N / 8 {
			x[2*n] = -yi(N/8+n) * win(2*n)
			x[2*n+1] = yr(N/8-n-1) * win(2*n+1)
			x[N/4+2*n] = -yr(n) * win(N/4+2*n)
			x[N/4+2*n+1] = yi(N/4-n-1) * win(N/4+2*n+1)
			x[N/2+2*n] = -yr(N/8+n) * win(N/2-2*n-1)
			x[N/2+2*n+1] = yi(N/8-n-1) * win(N/2-2*n-2)
			x[3*N/4+2*n] = yi(n) * win(N/4-2*n-1)
			x[3*N/4+2*n+1] = -yr(N/4-n-1) * win(N/4-2*n-2)
		}
	} else {
		X1, X2 := make([]float64, N/4), make([]float64, N/4)
		for k := range N / 4 {
			X1[k], X2[k] = X[2*k], X[2*k+1]
		}
		Z1, Z2 := make([]complex128, N/8), make([]complex128, N/8)
		for k := range Z1 {
			c := complex(-math.Cos(2*math.Pi*float64(8*k+1)/(4*N)), -math.Sin(2*math.Pi*float64(8*k+1)/(4*N)))
			Z1[k] = complex(X1[N/4-2*k-1], X1[2*k]) * c
			Z2[k] = complex(X2[N/4-2*k-1], X2[2*k]) * c
		}
		z1, z2 := ifft(Z1, 16), ifft(Z2, 16)
		y1, y2 := make([]complex128, N/8), make([]complex128, N/8)
		for n := range y1 {
			c := complex(-math.Cos(2*math.Pi*float64(8*n+1)/(4*N)), -math.Sin(2*math.Pi*float64(8*n+1)/(4*N)))
			y1[n], y2[n] = z1[n]*c, z2[n]*c
		}
		for n := range N / 8 {
			x[2*n] = -imag(y1[n]) * win(2*n)
			x[2*n+1] = real(y1[N/8-n-1]) * win(2*n+1)
			x[N/4+2*n] = -real(y1[n]) * win(N/4+2*n)
			x[N/4+2*n+1] = imag(y1[N/8-n-1]) * win(N/4+2*n+1)
			x[N/2+2*n] = -real(y2[n]) * win(N/2-2*n-1)
			x[N/2+2*n+1] = imag(y2[N/8-n-1]) * win(N/2-2*n-2)
			x[3*N/4+2*n] = imag(y2[n]) * win(N/4-2*n-1)
			x[3*N/4+2*n+1] = -real(y2[N/8-n-1]) * win(N/4-2*n-2)
		}
	}
	pcm := make([]float64, N/2)
	for n := range N / 2 {
		pcm[n] = 2 * (x[n] + delay[n])
		delay[n] = x[N/2+n]
	}
	return pcm
}

// TestTransform checks the decoder's inverse transform, of long blocks and
// short ones in any order, against the equations of section 7.9.4, on
// blocks of random coefficients: they agree to float32's precision, about
// 140 dB below the samples' level.
func TestTransform(t *testing.T) {
	var tf transform
	tf.init()
	w := make([]float64, 256)
	for i := range w {
		w[i] = float64(tf.window[i])
	}
	rng := rand.New(rand.NewPCG(1, 2))
	var coef, overlap, out [256]float32
	delay := make([]float64, 256)
	var sig, noise float64
	for blk := range 40 {
		short := blk%3 == 1 || blk%7 == 0
		X := make([]float64, 256)
		for k := range X {
			// Coefficients fall off with frequency, as audio's do.
			coef[k] = float32(rng.NormFloat64() * 0.1 / (1 + float64(k)/16))
			X[k] = float64(coef[k])
		}
		tf.inverse(&coef, &overlap, short, &out)
		want := standardInverse(X, delay, short, w)
		for n, v := range want {
			sig += v * v
			e := float64(out[n]) - v
			noise += e * e
		}
	}
	snr := 10 * math.Log10(sig/noise)
	t.Logf("SNR %.1f dB", snr)
	if snr < 130 {
		t.Errorf("SNR %.1f dB", snr)
	}
}
