package celt

// Port of celt/vq.c, decoder side: the shape of a band from its pulses,
// spread by rotations.

import "github.com/fzn0x/mixmusic/opus/internal/rangedec"

var spreadFactor = [3]int{15, 10, 5}

func expRotation1(x []float32, length, stride int, c, s float32) {
	ms := -s
	for i := 0; i < length-stride; i++ {
		x1, x2 := x[i], x[i+stride]
		x[i+stride] = c*x2 + s*x1
		x[i] = c*x1 + ms*x2
	}
	for i := length - 2*stride - 1; i >= 0; i-- {
		x1, x2 := x[i], x[i+stride]
		x[i+stride] = c*x2 + s*x1
		x[i] = c*x1 + ms*x2
	}
}

// expRotation is exp_rotation: it spreads the energy of a band's pulses,
// more when there are few, or undoes that for dir 1.
func expRotation(x []float32, length, dir, stride, k, spread int) {
	if 2*k >= length || spread == spreadNone {
		return
	}
	factor := spreadFactor[spread-1]
	gain := float32(length) / float32(length+factor*k)
	theta := .5 * (gain * gain)
	c := celtCosNorm(theta)
	s := celtCosNorm(q15One - theta) // sin(theta)
	stride2 := 0
	if length >= 8*stride {
		stride2 = 1
		// sqrt(length/stride) rounded: increment while (stride2+0.5)^2 < length/stride.
		for (stride2*stride2+stride2)*stride+(stride>>2) < length {
			stride2++
		}
	}
	length = int(uint32(length) / uint32(stride))
	for i := 0; i < stride; i++ {
		xi := x[i*length:]
		if dir < 0 {
			if stride2 != 0 {
				expRotation1(xi, length, stride2, s, c)
			}
			expRotation1(xi, length, 1, c, s)
		} else {
			expRotation1(xi, length, 1, c, -s)
			if stride2 != 0 {
				expRotation1(xi, length, stride2, s, -c)
			}
		}
	}
}

func normaliseResidual(iy []int, x []float32, n int, ryy, gain float32) {
	g := (1 / celtSqrt(ryy)) * gain
	for i := 0; i < n; i++ {
		x[i] = float32(iy[i]) * g
	}
}

func extractCollapseMask(iy []int, n, b int) uint32 {
	if b <= 1 {
		return 1
	}
	n0 := int(uint32(n) / uint32(b))
	var mask uint32
	for i := 0; i < b; i++ {
		tmp := 0
		for j := 0; j < n0; j++ {
			tmp |= iy[i*n0+j]
		}
		if tmp != 0 {
			mask |= 1 << i
		}
	}
	return mask
}

// algUnquant is alg_unquant: it decodes the k pulses of a band of n
// samples into x, with the norm gain, and returns its collapse mask.
func algUnquant(x []float32, n, k, spread, b int, dec *rangedec.Decoder, gain float32, iy []int) uint32 {
	iy = iy[:n]
	ryy := decodePulses(iy, n, k, dec)
	normaliseResidual(iy, x, n, ryy, gain)
	expRotation(x, n, -1, b, k, spread)
	return extractCollapseMask(iy, n, b)
}

// innerProd is celt_inner_prod: the sum of x[i]*y[i], in order.
func innerProd(x, y []float32, n int) float32 {
	var xy float32
	for i := 0; i < n; i++ {
		xy += x[i] * y[i]
	}
	return xy
}

// renormaliseVector is renormalise_vector: it scales x to the norm gain.
func renormaliseVector(x []float32, n int, gain float32) {
	e := epsilon + innerProd(x, x, n)
	g := (1 / celtSqrt(e)) * gain
	for i := 0; i < n; i++ {
		x[i] = g * x[i]
	}
}
