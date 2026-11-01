package ac3

import "fmt"

// The mantissa values of the symmetric quantizers, of Tables 7.19 to 7.23,
// by their codes.
var (
	levels3  = [3]float32{-2. / 3, 0, 2. / 3}
	levels5  = [5]float32{-4. / 5, -2. / 5, 0, 2. / 5, 4. / 5}
	levels7  = [7]float32{-6. / 7, -4. / 7, -2. / 7, 0, 2. / 7, 4. / 7, 6. / 7}
	levels11 = [11]float32{-10. / 11, -8. / 11, -6. / 11, -4. / 11, -2. / 11, 0, 2. / 11, 4. / 11, 6. / 11, 8. / 11, 10. / 11}
	levels15 = [15]float32{-14. / 15, -12. / 15, -10. / 15, -8. / 15, -6. / 15, -4. / 15, -2. / 15, 0, 2. / 15, 4. / 15, 6. / 15, 8. / 15, 10. / 15, 12. / 15, 14. / 15}
)

// mantissaBits are the bits of the mantissas of the asymmetric quantizers,
// of baps 6 to 15: qntztab of Table 7.18.
var mantissaBits = [16]int{6: 5, 7: 6, 8: 7, 9: 8, 10: 9, 11: 10, 12: 11, 13: 12, 14: 14, 15: 16}

// expScale is 2^-exp, of each exponent.
var expScale = func() (s [25]float32) {
	for e := range s {
		s[e] = pow2(-e)
	}
	return s
}()

// groups holds the mantissas of the grouped codes of a block not used
// yet: of the 3-, 5- and 11-level quantizers, whose groups the exponent
// sets of a block share, as section 7.3.5 tells.
type groups struct {
	n1, n2, n4 int // how many are left
	v1, v2     [3]float32
	v4         [2]float32
}

// ditherScale scales the dither, of a uniform distribution of [-1, 1), to
// that of [-0.707, 0.707), as section 7.3.4 recommends.
const ditherScale = 0.70710677

// dither returns the next value of the dither.
func (d *Decoder) dither() float32 {
	d.random = d.random*1664525 + 1013904223
	return float32(int32(d.random)) * (ditherScale / (1 << 31))
}

// mantissas reads the mantissas of a channel's bins, of start to end, and
// sets their coefficients: a mantissa of no bits is dither when dither is
// set, else 0.
func (d *Decoder) mantissas(g *groups, ch *channel, dither bool) error {
	r := &d.r
	for bin := ch.start; bin < ch.end; bin++ {
		var m float32
		switch b := ch.bap[bin]; b {
		case 0:
			if dither {
				m = d.dither()
			}
		case 1:
			if g.n1 == 0 {
				code := r.read(5)
				if code > 26 {
					return fmt.Errorf("%w: 3-level mantissa group %d", ErrFormat, code)
				}
				g.v1 = [3]float32{levels3[code/9], levels3[code%9/3], levels3[code%3]}
				g.n1 = 3
			}
			m = g.v1[3-g.n1]
			g.n1--
		case 2:
			if g.n2 == 0 {
				code := r.read(7)
				if code > 124 {
					return fmt.Errorf("%w: 5-level mantissa group %d", ErrFormat, code)
				}
				g.v2 = [3]float32{levels5[code/25], levels5[code%25/5], levels5[code%5]}
				g.n2 = 3
			}
			m = g.v2[3-g.n2]
			g.n2--
		case 3:
			code := r.read(3)
			if code == 7 {
				return fmt.Errorf("%w: 7-level mantissa 7", ErrFormat)
			}
			m = levels7[code]
		case 4:
			if g.n4 == 0 {
				code := r.read(7)
				if code > 120 {
					return fmt.Errorf("%w: 11-level mantissa group %d", ErrFormat, code)
				}
				g.v4 = [2]float32{levels11[code/11], levels11[code%11]}
				g.n4 = 2
			}
			m = g.v4[2-g.n4]
			g.n4--
		case 5:
			code := r.read(4)
			if code == 15 {
				return fmt.Errorf("%w: 15-level mantissa 15", ErrFormat)
			}
			m = levels15[code]
		default:
			bits := mantissaBits[b]
			m = float32(r.signed(bits)) * expScale[bits-1]
		}
		ch.coef[bin] = m * expScale[ch.exp[bin]]
	}
	return nil
}

// decouple makes the coefficients of the coupled channels of a block, in
// the range of the coupling channel, of its coefficients and of their
// coupling coordinates, as section 7.4.3 does, and dithers those of no
// bits, in each channel of its own.
func (d *Decoder) decouple() {
	cpl := &d.ch[cplIndex]
	for c := range d.h.nfchans {
		if !d.chincpl[c] {
			continue
		}
		ch := &d.ch[c]
		for s := d.cplbegf; s < d.cplendf+3; s++ {
			co := d.cplco[c][s] * 8
			if c == 1 && d.h.acmod == ModeStereo && d.phsflg[s] {
				co = -co
			}
			for bin := 37 + 12*s; bin < 49+12*s; bin++ {
				v := cpl.coef[bin]
				if cpl.bap[bin] == 0 && ch.dithflag {
					v = d.dither() * expScale[cpl.exp[bin]]
				}
				ch.coef[bin] = v * co
			}
		}
	}
}

// rematBounds are the first bins of the rematrixing bands, of Tables 7.25
// to 7.28, and the end of the last.
var rematBounds = [5]int{13, 25, 37, 61, 253}

// rematrix restores the left and right channels of the bands of a 2/0
// block whose rematrix flags are set, as section 7.5.4 does.
func (d *Decoder) rematrix() {
	l, r := &d.ch[0], &d.ch[1]
	nb := d.rematBands()
	end := min(l.end, r.end)
	for i := range nb {
		if !d.rematflg[i] {
			continue
		}
		hi := rematBounds[i+1]
		if d.cplinu && i == nb-1 {
			hi = 37 + 12*d.cplbegf
		}
		for bin := rematBounds[i]; bin < min(hi, end); bin++ {
			a, b := l.coef[bin], r.coef[bin]
			l.coef[bin], r.coef[bin] = a+b, a-b
		}
	}
}

// compress applies the dynamic range gains of the block to the
// coefficients of its channels: dynrng2 to Ch2 of a 1+1 stream, dynrng to
// the others.
func (d *Decoder) compress() {
	for c := range d.h.nfchans {
		g := d.dynrng
		if c == 1 && d.h.acmod == ModeDualMono {
			g = d.dynrng2
		}
		scale(&d.ch[c].coef, g)
	}
	if d.h.lfeon {
		scale(&d.ch[lfeIndex].coef, d.dynrng)
	}
}

func scale(coef *[maxBins]float32, g float32) {
	if g == 1 {
		return
	}
	for i := range coef {
		coef[i] *= g
	}
}
