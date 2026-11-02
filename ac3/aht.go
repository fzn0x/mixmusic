package ac3

import (
	"fmt"
	"math"
)

// The adaptive hybrid transform of E-AC-3, AHT: section E3.4 of A/52,
// E.2.4 of ETSI TS 102 366. A channel of AHT has the exponents of block 0
// alone, and has the mantissas of all six blocks in block 0, as a DCT
// across the blocks of each bin, coded by vector quantization or gain
// adaptive quantization.

// hebapTab is hebaptab, of Table E3.1: the high efficiency bit allocation
// pointer of each address.
var hebapTab = [64]uint8{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 8, 8, 8, 9, 9, 9, 10,
	10, 10, 10, 11, 11, 11, 11, 12, 12, 12, 12, 13, 13, 13, 13, 14,
	14, 14, 14, 15, 15, 15, 15, 16, 16, 16, 16, 17, 17, 17, 17, 18,
	18, 18, 18, 18, 18, 18, 18, 19, 19, 19, 19, 19, 19, 19, 19, 19,
}

// ahtBits are the bits of each hebap's code, of Table E3.2: of the index
// of a vector of six mantissas for hebaps 1 to 7, of a mantissa, m, for 8
// to 19.
var ahtBits = [20]int{0, 2, 3, 4, 5, 7, 8, 9, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 16}

// The remapping constants of the GAQ quantizers, of Table E3.6, as Q15
// fractions, of hebaps 8 to 19: a of the quantizer of gain 1, and a and b,
// of x < 0 and of x >= 0, of the large mantissas of gains 2 and 4, of
// hebaps 8 to 16.
var (
	gaqA1      = [12]int16{0x1249, 0x0889, 0x0421, 0x0208, 0x0102, 0x0081, 0x0040, 0x0020, 0x0010, 0x0008, 0x0002, 0x0000}
	gaqA2      = [9]uint16{0xd555, 0xc925, 0xc444, 0xc211, 0xc104, 0xc081, 0xc040, 0xc020, 0xc010}
	gaqB2      = [9]uint16{0xeaab, 0xd249, 0xc889, 0xc421, 0xc208, 0xc102, 0xc081, 0xc040, 0xc020}
	gaqA4      = [9]uint16{0xedb7, 0xe666, 0xe319, 0xe186, 0xe0c2, 0xe060, 0xe030, 0xe018, 0xe00c}
	gaqB4      = [9]uint16{0xfb6e, 0xeccd, 0xe632, 0xe30c, 0xe183, 0xe0c1, 0xe060, 0xe030, 0xe018}
	gaqB2Plus  = 0x4000
	gaqB4Plus  = 0x2000
	ahtCosines = func() (t [6][6]float32) {
		// The IDCT of section E3.4.5: C(k, m) = sqrt(2) * sum over j of
		// R(j) X(k, j) cos(pi j (2m + 1) / 12), R(0) = 1/sqrt(2), R(j) = 1.
		// The text of the standard's PDF loses the radical of its sqrt(2),
		// which ffmpeg's decoder has, as TestFFmpegWritten checks.
		for m := range t {
			for j := range t[m] {
				r := 1.0
				if j == 0 {
					r = 1 / math.Sqrt2
				}
				t[m][j] = float32(math.Sqrt2 * r * math.Cos(math.Pi*float64(j*(2*m+1))/12))
			}
		}
		return t
	}()
)

// q15 returns a Q15 fraction as a float32.
func q15(v uint16) float32 { return float32(int16(v)) / 32768 }

// fraction reads a mantissa of bits bits, a two's complement fraction.
func (r *bitReader) fraction(bits int) float32 {
	return float32(r.signed(bits)) * expScale[bits-1]
}

// ahtMantissas reads the mantissas of the six blocks of a channel of AHT,
// in block 0: its gaqmod and gains, then of each bin a vector index, or
// six mantissas of GAQ, and turns them into the mantissas of each block
// with the IDCT, into ch.aht6.
func (d *Decoder) ahtMantissas(ch *channel) error {
	r := &d.r
	gaqmod := r.read(2)
	endbap := uint8(12)
	if gaqmod >= 2 {
		endbap = 17
	}
	// The gains of the active bins: the bins of hebaps 8 to endbap-1.
	active := 0
	for bin := ch.start; bin < ch.end; bin++ {
		if b := ch.bap[bin]; b > 7 && b < endbap {
			active++
		}
	}
	gains := d.gains[:0]
	switch gaqmod {
	case 1, 2:
		big := uint8(2 * gaqmod) // 2, or 4
		for range active {
			g := uint8(1)
			if r.flag() {
				g = big
			}
			gains = append(gains, g)
		}
	case 3:
		for range (active + 2) / 3 {
			code := r.read(5)
			if code > 26 {
				return fmt.Errorf("%w: GAQ gains %d", ErrFormat, code)
			}
			for _, m := range [3]int{code / 9, code % 9 / 3, code % 3} {
				gains = append(gains, uint8(1)<<m)
			}
		}
	}
	d.gains = gains
	var x [6]float32
	k := 0 // the active bins so far
	for bin := ch.start; bin < ch.end; bin++ {
		b := ch.bap[bin]
		switch {
		case b == 0:
			x = [6]float32{}
		case b <= 7:
			idx := r.read(ahtBits[b])
			if idx >= len(vqTables[b]) {
				return fmt.Errorf("%w: VQ index %d of hebap %d", ErrFormat, idx, b)
			}
			for j, v := range vqTables[b][idx] {
				x[j] = float32(v) / 32768
			}
		default:
			g := uint8(1)
			if b < endbap {
				if k < len(gains) {
					g = gains[k]
				}
				k++
			}
			m := ahtBits[b]
			for j := range x {
				v, err := d.gaqMantissa(b, m, g)
				if err != nil {
					return err
				}
				x[j] = v
			}
		}
		for blk := range 6 {
			c := ahtCosines[blk]
			ch.aht6[blk][bin] = c[0]*x[0] + c[1]*x[1] + c[2]*x[2] + c[3]*x[3] + c[4]*x[4] + c[5]*x[5]
		}
	}
	return nil
}

// gaqMantissa reads a mantissa of GAQ, of hebap b, of m bits, of gain g,
// of Tables E3.5 and E3.6: of gain 1, an m-bit fraction, remapped; of gain
// 2 or 4, a small mantissa of m-1 or m-2 bits, divided by the gain, or
// the tag of the most negative of these, and a large mantissa of m-1 or m
// bits, remapped.
func (d *Decoder) gaqMantissa(b uint8, m int, g uint8) (float32, error) {
	r := &d.r
	if g == 1 {
		x := r.fraction(m)
		return x + x*q15(uint16(gaqA1[b-8])), nil
	}
	small := m - 1
	large := m - 1
	if g == 4 {
		small, large = m-2, m
	}
	if small < 1 || b > 16 {
		return 0, fmt.Errorf("%w: GAQ gain %d of hebap %d", ErrFormat, g, b)
	}
	code := r.read(small)
	if code != 1<<(small-1) {
		return float32(code<<(64-small)>>(64-small)) * expScale[small-1] / float32(g), nil
	}
	x := r.fraction(large)
	a, bn, bp := gaqA2[b-8], gaqB2[b-8], uint16(gaqB2Plus)
	if g == 4 {
		a, bn, bp = gaqA4[b-8], gaqB4[b-8], uint16(gaqB4Plus)
	}
	bias := q15(bp)
	if x < 0 {
		bias = q15(bn)
	}
	return x + x*q15(a) + bias, nil
}

// ahtBlock sets the coefficients of block blk of a channel of AHT, of the
// mantissas ahtMantissas made: mantissas of no bits are dither when dither
// is set.
func (d *Decoder) ahtBlock(ch *channel, blk int, dither bool) {
	for bin := ch.start; bin < ch.end; bin++ {
		v := ch.aht6[blk][bin]
		if ch.bap[bin] == 0 {
			v = 0
			if dither {
				v = d.dither()
			}
		}
		ch.coef[bin] = v * expScale[ch.exp[bin]]
	}
	if ch != &d.ch[cplIndex] {
		clear(ch.coef[ch.end:])
	}
}
