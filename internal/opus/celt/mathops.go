package celt

// Port of the float build's celt/mathops.h and the arithmetic of
// celt/arch.h, without FLOAT_APPROX.

import (
	"math"
	"math/bits"
)

const (
	q15One     = float32(1)
	epsilon    = float32(1e-15)
	verySmall  = float32(1e-30)
	sigScale   = float32(32768)
	pi         = 3.1415926535897931
	normScale  = float32(1)
	normShift  = 14 // only scales fixed-point values; kept for SHL32 calls
	coefOne    = float32(1)
	q31One     = float32(1)
	celtLPCOrd = 24
)

func celtSqrt(x float32) float32  { return float32(math.Sqrt(float64(x))) }
func celtRsqrt(x float32) float32 { return 1 / celtSqrt(x) }
func celtRcp(x float32) float32   { return 1 / x }

// celtCosNorm is cos(pi/2*x).
func celtCosNorm(x float32) float32 { return float32(math.Cos(0.5 * pi * float64(x))) }

func celtLog2(x float32) float32 { return float32(1.442695040888963387 * math.Log(float64(x))) }
func celtExp2(x float32) float32 { return float32(math.Exp(0.6931471805599453094 * float64(x))) }

// fracDiv32 is frac_div32.
func fracDiv32(a, b float32) float32 { return a / b }

// sig2res is SIG2RES: a sample of the decoder's scale to one of [-1, 1].
func sig2res(a float32) float32 { return (1 / sigScale) * a }

// celtLCGRand is celt_lcg_rand, the pseudo-random generator of folding and
// concealment.
func celtLCGRand(seed uint32) uint32 { return 1664525*seed + 1013904223 }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// isqrt32 is the integer square root of 0 < n < 2^32, rounded down.
func isqrt32(n uint32) uint32 {
	var g uint32
	bshift := (ilog(n) - 1) >> 1
	b := uint32(1) << bshift
	for {
		t := ((g << 1) + b) << bshift
		if t <= n {
			g += b
			n -= t
		}
		b >>= 1
		bshift--
		if bshift < 0 {
			break
		}
	}
	return g
}

// ilog is EC_ILOG: the number of bits needed to write x, 0 for 0.
func ilog(x uint32) int { return bits.Len32(x) }
