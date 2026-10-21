// Port of silk/SigProc_FIX.h, silk/macros.h, silk/Inlines.h and silk/typedef.h: the generic C macros, as built with OPUS_FAST_INT64.

package silk

import "math/bits"

// Limits of typedef.h.
const (
	silkInt32Max int32 = 0x7FFFFFFF
	silkInt32Min int32 = -0x80000000
	silkInt16Max int32 = 0x7FFF
	silkInt16Min int32 = -0x8000
)

// Pseudo-random generator constants.
const (
	randMultiplier = 196314165
	randIncrement  = 907633515
)

// smulwb is silk_SMULWB: (a32 * (opus_int32)((opus_int16)(b32))) >> 16.
func smulwb(a32, b32 int32) int32 {
	return int32((int64(a32) * int64(int16(b32))) >> 16)
}

// smlawb is silk_SMLAWB: a32 + ((b32 * (opus_int32)((opus_int16)(c32))) >> 16).
func smlawb(a32, b32, c32 int32) int32 {
	return int32(int64(a32) + ((int64(b32) * int64(int16(c32))) >> 16))
}

// smulbb is silk_SMULBB: (opus_int32)((opus_int16)(a32)) * (opus_int32)((opus_int16)(b32)).
func smulbb(a32, b32 int32) int32 {
	return int32(int16(a32)) * int32(int16(b32))
}

// smlabb is silk_SMLABB: a32 + (opus_int32)((opus_int16)(b32)) * (opus_int32)((opus_int16)(c32)).
func smlabb(a32, b32, c32 int32) int32 {
	return a32 + int32(int16(b32))*int32(int16(c32))
}

// smulww is silk_SMULWW: (a32 * b32) >> 16.
func smulww(a32, b32 int32) int32 {
	return int32((int64(a32) * int64(b32)) >> 16)
}

// smlaww is silk_SMLAWW: a32 + ((b32 * c32) >> 16).
func smlaww(a32, b32, c32 int32) int32 {
	return int32(int64(a32) + ((int64(b32) * int64(c32)) >> 16))
}

// smultt is silk_SMULTT: (a32 >> 16) * (b32 >> 16).
func smultt(a32, b32 int32) int32 {
	return (a32 >> 16) * (b32 >> 16)
}

// smull is silk_SMULL: (opus_int64)(a32) * b32.
func smull(a32, b32 int32) int64 {
	return int64(a32) * int64(b32)
}

// smmul is silk_SMMUL: (opus_int32)silk_RSHIFT64(silk_SMULL(a32, b32), 32).
func smmul(a32, b32 int32) int32 {
	return int32(smull(a32, b32) >> 32)
}

// add32Ovflw is silk_ADD32_ovflw.
func add32Ovflw(a, b int32) int32 {
	return int32(uint32(a) + uint32(b))
}

// sub32Ovflw is silk_SUB32_ovflw.
func sub32Ovflw(a, b int32) int32 {
	return int32(uint32(a) - uint32(b))
}

// mlaOvflw is silk_MLA_ovflw: a32 + b32 * c32, allowed to overflow.
func mlaOvflw(a32, b32, c32 int32) int32 {
	return add32Ovflw(a32, int32(uint32(b32)*uint32(c32)))
}

// smlabbOvflw is silk_SMLABB_ovflw.
func smlabbOvflw(a32, b32, c32 int32) int32 {
	return add32Ovflw(a32, int32(int16(b32))*int32(int16(c32)))
}

// sat16 is silk_SAT16.
func sat16(a int32) int32 {
	if a > silkInt16Max {
		return silkInt16Max
	}
	if a < silkInt16Min {
		return silkInt16Min
	}
	return a
}

// addSat16 is silk_ADD_SAT16: (opus_int16)silk_SAT16( silk_ADD32( (opus_int32)(a), (b) ) ).
func addSat16(a, b int32) int16 {
	return int16(sat16(a + b))
}

// addSat32 is silk_ADD_SAT32.
func addSat32(a, b int32) int32 {
	if (uint32(a)+uint32(b))&0x80000000 == 0 {
		if (uint32(a)&uint32(b))&0x80000000 != 0 {
			return silkInt32Min
		}
		return a + b
	}
	if (uint32(a)|uint32(b))&0x80000000 == 0 {
		return silkInt32Max
	}
	return a + b
}

// subSat32 is silk_SUB_SAT32.
func subSat32(a, b int32) int32 {
	if (uint32(a)-uint32(b))&0x80000000 == 0 {
		if uint32(a)&(uint32(b)^0x80000000)&0x80000000 != 0 {
			return silkInt32Min
		}
		return a - b
	}
	if (uint32(a)^0x80000000)&uint32(b)&0x80000000 != 0 {
		return silkInt32Max
	}
	return a - b
}

// lshiftSat32 is silk_LSHIFT_SAT32: saturates before shifting.
func lshiftSat32(a, shift int32) int32 {
	return limit(a, silkInt32Min>>shift, silkInt32Max>>shift) << shift
}

// rshiftRound is silk_RSHIFT_ROUND; shift > 0.
func rshiftRound(a, shift int32) int32 {
	if shift == 1 {
		return (a >> 1) + (a & 1)
	}
	return ((a >> (shift - 1)) + 1) >> 1
}

// rshiftRound64 is silk_RSHIFT_ROUND64; shift > 0.
func rshiftRound64(a int64, shift int32) int64 {
	if shift == 1 {
		return (a >> 1) + (a & 1)
	}
	return ((a >> (shift - 1)) + 1) >> 1
}

// limit is silk_LIMIT (and silk_LIMIT_int, silk_LIMIT_16, silk_LIMIT_32).
func limit(a, limit1, limit2 int32) int32 {
	if limit1 > limit2 {
		if a > limit1 {
			return limit1
		}
		if a < limit2 {
			return limit2
		}
		return a
	}
	if a > limit2 {
		return limit2
	}
	if a < limit1 {
		return limit1
	}
	return a
}

// abs32 is silk_abs: wrong, as in C, for silk_int32_MIN.
func abs32(a int32) int32 {
	if a > 0 {
		return a
	}
	return -a
}

// silkRAND is silk_RAND: the pseudo-random generator.
func silkRAND(seed int32) int32 {
	return mlaOvflw(randIncrement, seed, randMultiplier)
}

// clz32 is silk_CLZ32.
func clz32(in32 int32) int32 {
	return int32(bits.LeadingZeros32(uint32(in32)))
}

// ror32 is silk_ROR32: rotates a32 right by rot bits, left if rot < 0.
func ror32(a32, rot int32) int32 {
	x := uint32(a32)
	r := uint32(rot)
	m := uint32(-rot)
	if rot == 0 {
		return a32
	} else if rot < 0 {
		return int32((x << m) | (x >> (32 - m)))
	}
	return int32((x << (32 - r)) | (x >> r))
}

// clzFrac is silk_CLZ_FRAC: the number of leading zeros and the 7 bits right
// after the leading one.
func clzFrac(in int32) (lz, fracQ7 int32) {
	lzeros := clz32(in)
	return lzeros, ror32(in, 24-lzeros) & 0x7f
}

// sqrtApprox is silk_SQRT_APPROX.
func sqrtApprox(x int32) int32 {
	if x <= 0 {
		return 0
	}

	lz, fracQ7 := clzFrac(x)

	var y int32
	if lz&1 != 0 {
		y = 32768
	} else {
		y = 46214 // 46214 = sqrt(2) * 32768
	}

	// get scaling right
	y >>= lz >> 1

	// increment using fractional part of input
	y = smlawb(y, y, smulbb(213, fracQ7))

	return y
}

// div32VarQ is silk_DIV32_varQ: a good approximation of
// "(a32 << Qres) / b32", Qres >= 0.
func div32VarQ(a32, b32, Qres int32) int32 {
	// Compute number of bits head room and normalize inputs
	aHeadrm := clz32(abs32(a32)) - 1
	a32Nrm := a32 << aHeadrm // Q: a_headrm
	bHeadrm := clz32(abs32(b32)) - 1
	b32Nrm := b32 << bHeadrm // Q: b_headrm

	// Inverse of b32, with 14 bits of precision
	b32Inv := (silkInt32Max >> 2) / (b32Nrm >> 16) // Q: 29 + 16 - b_headrm

	// First approximation
	result := smulwb(a32Nrm, b32Inv) // Q: 29 + a_headrm - b_headrm

	// Compute residual by subtracting product of denominator and first approximation
	// It's OK to overflow because the final value of a32_nrm should always be small
	a32Nrm = sub32Ovflw(a32Nrm, smmul(b32Nrm, result)<<3) // Q: a_headrm

	// Refinement
	result = smlawb(result, a32Nrm, b32Inv) // Q: 29 + a_headrm - b_headrm

	// Convert to Qres domain
	lshift := 29 + aHeadrm - bHeadrm - Qres
	if lshift < 0 {
		return lshiftSat32(result, -lshift)
	}
	if lshift < 32 {
		return result >> lshift
	}
	// Avoid undefined result
	return 0
}

// inverse32VarQ is silk_INVERSE32_varQ: a good approximation of
// "(1 << Qres) / b32", Qres > 0.
func inverse32VarQ(b32, Qres int32) int32 {
	// Compute number of bits head room and normalize input
	bHeadrm := clz32(abs32(b32)) - 1
	b32Nrm := b32 << bHeadrm // Q: b_headrm

	// Inverse of b32, with 14 bits of precision
	b32Inv := (silkInt32Max >> 2) / (b32Nrm >> 16) // Q: 29 + 16 - b_headrm

	// First approximation
	result := b32Inv << 16 // Q: 61 - b_headrm

	// Compute residual by subtracting product of denominator and first approximation from one
	errQ32 := ((int32(1) << 29) - smulwb(b32Nrm, b32Inv)) << 3 // Q32

	// Refinement
	result = smlaww(result, errQ32, b32Inv) // Q: 61 - b_headrm

	// Convert to Qres domain
	lshift := 61 - bHeadrm - Qres
	if lshift <= 0 {
		return lshiftSat32(result, -lshift)
	}
	if lshift < 32 {
		return result >> lshift
	}
	// Avoid undefined result
	return 0
}
