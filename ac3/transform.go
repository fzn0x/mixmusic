package ac3

import "math"

// transform is the inverse transform of section 7.9.4: the IMDCT of a
// block of 512 samples, or of two of 256, by a complex IFFT of 128 or 64
// points between the pre- and post-IFFT multiplies, then the window and
// the overlap with the last block.
type transform struct {
	// window is w[n] of Table 7.33, the first half of the window.
	window [256]float32
	// The pre- and post-IFFT multipliers: xcos1, xsin1 of the long
	// transform, xcos2, xsin2 of the short ones.
	cos1, sin1 [128]float32
	cos2, sin2 [64]float32
	// The IFFT's: e^(j*2*pi*k/128), and the bit reversal of 128 and 64.
	twr, twi [64]float32
	rev128   [128]uint8
	rev64    [64]uint8
	re, im   [128]float32 // Z, z and y of the long transform
	re2, im2 [64]float32  // of the second short one
	x        [512]float32 // the windowed samples
}

// kbdAlpha is the alpha of the Kaiser-Bessel-derived window whose first
// half Table 7.33 gives, to its five digits.
const kbdAlpha = 5

// kbdWindow returns the first n values of a Kaiser-Bessel-derived window of
// 2n values, of alpha.
func kbdWindow(n int, alpha float64) []float64 {
	kernel := make([]float64, n+1)
	var sum float64
	for j := range kernel {
		r := 2*float64(j)/float64(n) - 1
		kernel[j] = bessel0(math.Pi * alpha * math.Sqrt(1-r*r))
		sum += kernel[j]
	}
	w := make([]float64, n)
	var acc float64
	for j := range w {
		acc += kernel[j]
		w[j] = math.Sqrt(acc / sum)
	}
	return w
}

// bessel0 is the modified Bessel function of the first kind, of order 0.
func bessel0(x float64) float64 {
	sum, term := 1.0, 1.0
	for k := 1; k < 64; k++ {
		term *= x * x / 4 / float64(k*k)
		sum += term
		if term < sum*1e-17 {
			break
		}
	}
	return sum
}

func (t *transform) init() {
	for i, v := range kbdWindow(256, kbdAlpha) {
		t.window[i] = float32(v)
	}
	const n = 512
	for k := range 128 {
		a := 2 * math.Pi * float64(8*k+1) / (8 * n)
		t.cos1[k] = float32(-math.Cos(a))
		t.sin1[k] = float32(-math.Sin(a))
	}
	for k := range 64 {
		a := 2 * math.Pi * float64(8*k+1) / (4 * n)
		t.cos2[k] = float32(-math.Cos(a))
		t.sin2[k] = float32(-math.Sin(a))
		b := 2 * math.Pi * float64(k) / 128
		t.twr[k] = float32(math.Cos(b))
		t.twi[k] = float32(math.Sin(b))
	}
	for i := range 128 {
		t.rev128[i] = uint8(reverse(i, 7))
	}
	for i := range 64 {
		t.rev64[i] = uint8(reverse(i, 6))
	}
}

// reverse reverses the bits bits of i.
func reverse(i, bits int) int {
	r := 0
	for range bits {
		r = r<<1 | i&1
		i >>= 1
	}
	return r
}

// ifft computes in place z[n] = sum over k of Z[k] * e^(j*2*pi*k*n/len),
// of 128 points or 64, whose bit reversal is rev; stride is the step of
// the twiddles of 128 points that its own are: 1 or 2.
func (t *transform) ifft(re, im []float32, rev []uint8, stride int) {
	n := len(re)
	for i := range n {
		if j := int(rev[i]); j > i {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		half := size >> 1
		step := stride * (n / size)
		for start := 0; start < n; start += size {
			for k := range half {
				wr, wi := t.twr[k*step], t.twi[k*step]
				a, b := start+k, start+k+half
				xr := re[b]*wr - im[b]*wi
				xi := re[b]*wi + im[b]*wr
				re[b], im[b] = re[a]-xr, im[a]-xi
				re[a], im[a] = re[a]+xr, im[a]+xi
			}
		}
	}
}

// inverse transforms the coefficients of a block, of one 512-sample
// transform or of two 256-sample ones when short, into the 256 samples of
// out: their first half windowed, overlapped with the second half of the
// last block's, which overlap holds and which it then takes this block's.
func (t *transform) inverse(coef, overlap *[256]float32, short bool, out *[256]float32) {
	w := &t.window
	x := &t.x
	if !short {
		re, im := t.re[:], t.im[:]
		for k := range 128 {
			xr, xi := coef[255-2*k], coef[2*k]
			re[k] = xr*t.cos1[k] - xi*t.sin1[k]
			im[k] = xi*t.cos1[k] + xr*t.sin1[k]
		}
		t.ifft(re, im, t.rev128[:], 1)
		for n := range 128 {
			zr, zi := re[n], im[n]
			re[n] = zr*t.cos1[n] - zi*t.sin1[n]
			im[n] = zi*t.cos1[n] + zr*t.sin1[n]
		}
		for n := range 64 {
			x[2*n] = -im[64+n] * w[2*n]
			x[2*n+1] = re[64-n-1] * w[2*n+1]
			x[128+2*n] = -re[n] * w[128+2*n]
			x[128+2*n+1] = im[128-n-1] * w[128+2*n+1]
			x[256+2*n] = -re[64+n] * w[256-2*n-1]
			x[256+2*n+1] = im[64-n-1] * w[256-2*n-2]
			x[384+2*n] = im[n] * w[128-2*n-1]
			x[384+2*n+1] = -re[128-n-1] * w[128-2*n-2]
		}
	} else {
		re1, im1 := t.re[:64], t.im[:64]
		re2, im2 := t.re2[:], t.im2[:]
		for k := range 64 {
			// X1[k] = X[2k] and X2[k] = X[2k+1].
			ar, ai := coef[2*(128-2*k-1)], coef[2*(2*k)]
			br, bi := coef[2*(128-2*k-1)+1], coef[2*(2*k)+1]
			re1[k] = ar*t.cos2[k] - ai*t.sin2[k]
			im1[k] = ai*t.cos2[k] + ar*t.sin2[k]
			re2[k] = br*t.cos2[k] - bi*t.sin2[k]
			im2[k] = bi*t.cos2[k] + br*t.sin2[k]
		}
		t.ifft(re1, im1, t.rev64[:], 2)
		t.ifft(re2, im2, t.rev64[:], 2)
		for n := range 64 {
			zr, zi := re1[n], im1[n]
			re1[n] = zr*t.cos2[n] - zi*t.sin2[n]
			im1[n] = zi*t.cos2[n] + zr*t.sin2[n]
			zr, zi = re2[n], im2[n]
			re2[n] = zr*t.cos2[n] - zi*t.sin2[n]
			im2[n] = zi*t.cos2[n] + zr*t.sin2[n]
		}
		for n := range 64 {
			x[2*n] = -im1[n] * w[2*n]
			x[2*n+1] = re1[64-n-1] * w[2*n+1]
			x[128+2*n] = -re1[n] * w[128+2*n]
			x[128+2*n+1] = im1[64-n-1] * w[128+2*n+1]
			x[256+2*n] = -re2[n] * w[256-2*n-1]
			x[256+2*n+1] = im2[64-n-1] * w[256-2*n-2]
			x[384+2*n] = im2[n] * w[128-2*n-1]
			x[384+2*n+1] = -re2[64-n-1] * w[128-2*n-2]
		}
	}
	for n := range 256 {
		out[n] = 2 * (x[n] + overlap[n])
		overlap[n] = x[256+n]
	}
}
