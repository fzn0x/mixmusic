package ac3

import (
	"fmt"
	"math"
)

// Exponent strategies, of Table 7.4.
const (
	expReuse = 0
	expD15   = 1
	expD25   = 2
	expD45   = 3
)

// Delta bit allocation states, of Table 5.16.
const (
	deltaReuse = 0
	deltaNew   = 1
	deltaNone  = 2
)

// parseBlock reads audio block blk of the syncframe r reads, and turns it
// into the transform coefficients of its channels.
func (d *Decoder) parseBlock(blk int) error {
	r := &d.r
	h := &d.h
	nf := h.nfchans
	for c := range nf {
		d.ch[c].blksw = r.flag()
	}
	d.ditherAt, d.phsflginuAt, d.phsflgAt = r.pos, -1, -1
	for c := range nf {
		d.ch[c].dithflag = r.flag()
	}
	if r.flag() { // dynrnge
		d.dynrng = dynamicRange(r.read(8))
	} else if blk == 0 {
		d.dynrng = 1
	}
	if h.acmod == ModeDualMono {
		if r.flag() { // dynrng2e
			d.dynrng2 = dynamicRange(r.read(8))
		} else if blk == 0 {
			d.dynrng2 = 1
		}
	}

	// The coupling strategy. Coupling information is never shared across
	// syncframes: in block 0, all of it is new.
	wasCoupled := d.cplinu && blk > 0
	var wasIn [maxFull]bool
	if wasCoupled {
		wasIn = d.chincpl
	}
	moved := !wasCoupled // whether the coupling's range changed
	newBands := false    // whether its bands did
	if r.flag() {        // cplstre
		m, b, err := d.parseCouplingStrategy()
		if err != nil {
			return err
		}
		moved = moved || m
		newBands = b
	} else if blk == 0 {
		return fmt.Errorf("%w: no coupling strategy in block 0", ErrFormat)
	}
	if d.cplinu {
		var fresh [maxFull]bool
		for c := range nf {
			fresh[c] = moved || newBands || !wasIn[c]
		}
		if err := d.parseCouplingCoordinates(&fresh); err != nil {
			return err
		}
	}

	// Rematrixing.
	if h.acmod == ModeStereo {
		if r.flag() { // rematstr
			for i := range d.rematBands() {
				d.rematflg[i] = r.flag()
			}
		} else if blk == 0 {
			return fmt.Errorf("%w: no rematrixing strategy in block 0", ErrFormat)
		}
	}

	// The exponent strategies, and the bandwidths.
	cpl := &d.ch[cplIndex]
	cplexpstr := expReuse
	if d.cplinu {
		cplexpstr = r.read(2)
		if cplexpstr == expReuse && moved {
			return fmt.Errorf("%w: coupling exponents reused", ErrFormat)
		}
		cpl.start = 37 + 12*d.cplbegf
		cpl.end = 37 + 12*(d.cplendf+3)
	}
	var chexpstr [maxFull]int
	for c := range nf {
		chexpstr[c] = r.read(2)
	}
	d.expstr = chexpstr
	d.cplexpstr = cplexpstr
	lfeexpstr := expReuse
	if h.lfeon {
		lfeexpstr = r.read(1)
	}
	for c := range nf {
		ch := &d.ch[c]
		end := ch.end
		if d.chincpl[c] {
			end = cpl.start
		} else if chexpstr[c] != expReuse {
			chbwcod := r.read(6)
			if chbwcod > 60 {
				return fmt.Errorf("%w: chbwcod %d", ErrFormat, chbwcod)
			}
			end = 37 + 3*(chbwcod+12)
		}
		if chexpstr[c] == expReuse && (blk == 0 || end != ch.end) {
			return fmt.Errorf("%w: exponents of channel %d reused", ErrFormat, c)
		}
		ch.start, ch.end = 0, end
	}
	if h.lfeon {
		if lfeexpstr == expReuse && blk == 0 {
			return fmt.Errorf("%w: LFE exponents reused", ErrFormat)
		}
		d.ch[lfeIndex].start, d.ch[lfeIndex].end = 0, 7
	}

	// The exponents.
	if d.cplinu && cplexpstr != expReuse {
		absexp := r.read(4) << 1
		groups := (cpl.end - cpl.start) / (3 << (cplexpstr - 1))
		if err := d.exponents(cpl, absexp, groups, cplexpstr, cpl.start); err != nil {
			return err
		}
	}
	for c := range nf {
		if chexpstr[c] == expReuse {
			continue
		}
		ch := &d.ch[c]
		absexp := r.read(4)
		ch.exp[0] = uint8(absexp)
		var groups int
		switch chexpstr[c] {
		case expD15:
			groups = (ch.end - 1) / 3
		case expD25:
			groups = (ch.end - 1 + 3) / 6
		case expD45:
			groups = (ch.end - 1 + 9) / 12
		}
		if err := d.exponents(ch, absexp, groups, chexpstr[c], 1); err != nil {
			return err
		}
		r.skip(2) // gainrng
	}
	if h.lfeon && lfeexpstr != expReuse {
		lfe := &d.ch[lfeIndex]
		absexp := r.read(4)
		lfe.exp[0] = uint8(absexp)
		if err := d.exponents(lfe, absexp, 2, expD15, 1); err != nil {
			return err
		}
	}

	// The bit allocation's parameters.
	if r.flag() { // baie
		d.sdcycod = r.read(2)
		d.fdcycod = r.read(2)
		d.sgaincod = r.read(2)
		d.dbpbcod = r.read(2)
		d.floorcod = r.read(3)
	} else if blk == 0 {
		return fmt.Errorf("%w: no bit allocation parameters in block 0", ErrFormat)
	}
	if r.flag() { // snroffste
		d.csnroffst = r.read(6)
		if d.cplinu {
			cpl.fsnroffst = r.read(4)
			cpl.fgaincod = r.read(3)
		}
		for c := range nf {
			d.ch[c].fsnroffst = r.read(4)
			d.ch[c].fgaincod = r.read(3)
		}
		if h.lfeon {
			d.ch[lfeIndex].fsnroffst = r.read(4)
			d.ch[lfeIndex].fgaincod = r.read(3)
		}
	} else if blk == 0 {
		return fmt.Errorf("%w: no SNR offsets in block 0", ErrFormat)
	}
	if d.cplinu {
		if r.flag() { // cplleake
			d.cplfleak = r.read(3)
			d.cplsleak = r.read(3)
		} else if !wasCoupled {
			return fmt.Errorf("%w: no coupling leak", ErrFormat)
		}
	}
	if d.cplinu && !wasCoupled {
		cpl.delta = false
	}
	d.deltaAt = r.pos
	if r.flag() { // deltbaie
		if d.cplinu {
			cpl.deltbae = r.read(2)
		}
		for c := range nf {
			d.ch[c].deltbae = r.read(2)
		}
		if d.cplinu {
			if err := d.parseDelta(cpl, blk, moved); err != nil {
				return err
			}
		}
		for c := range nf {
			if err := d.parseDelta(&d.ch[c], blk, false); err != nil {
				return err
			}
		}
	} else if blk == 0 {
		cpl.delta = false
		for c := range nf {
			d.ch[c].delta = false
		}
	} else if d.cplinu && moved && cpl.delta {
		return fmt.Errorf("%w: no coupling delta bit allocation", ErrFormat)
	}
	if r.flag() { // skiple
		r.skip(8 * r.read(9))
	}
	if r.over() {
		return fmt.Errorf("%w: syncframe too short", ErrFormat)
	}

	// The bit allocation.
	zero := d.csnroffst == 0 && (!d.cplinu || cpl.fsnroffst == 0) && (!h.lfeon || d.ch[lfeIndex].fsnroffst == 0)
	for c := range nf {
		zero = zero && d.ch[c].fsnroffst == 0
	}
	if d.cplinu {
		if err := d.allocate(cpl, zero); err != nil {
			return err
		}
	}
	for c := range nf {
		if err := d.allocate(&d.ch[c], zero); err != nil {
			return err
		}
	}
	if h.lfeon {
		if err := d.allocate(&d.ch[lfeIndex], zero); err != nil {
			return err
		}
	}

	// The mantissas.
	d.mantAt = r.pos
	var g groups
	gotCpl := false
	for c := range nf {
		ch := &d.ch[c]
		if err := d.mantissas(&g, ch, ch.dithflag); err != nil {
			return err
		}
		clear(ch.coef[ch.end:])
		if d.cplinu && d.chincpl[c] && !gotCpl {
			if err := d.mantissas(&g, cpl, false); err != nil {
				return err
			}
			gotCpl = true
		}
	}
	if h.lfeon {
		lfe := &d.ch[lfeIndex]
		if err := d.mantissas(&g, lfe, false); err != nil {
			return err
		}
		clear(lfe.coef[lfe.end:])
	}
	if r.over() {
		return fmt.Errorf("%w: syncframe too short", ErrFormat)
	}

	if d.cplinu {
		d.decouple()
	}
	if h.acmod == ModeStereo {
		d.rematrix()
	}
	d.compress()
	if d.onBlock != nil {
		d.onBlock(blk)
	}
	return nil
}

// dynamicRange returns the gain of a dynrng, as section 7.7.1.2 tells:
// 2^(X+1) times 0.1Y in binary, of its 3 bits X, signed, and 5 bits Y.
func dynamicRange(v int) float32 {
	x := v >> 5
	if x >= 4 {
		x -= 8
	}
	g := float32(32+v&31) / 64
	if x+1 >= 0 {
		return g * float32(int(1)<<(x+1))
	}
	return g / float32(int(1)<<-(x+1))
}

// parseCouplingStrategy reads the coupling strategy of a block whose
// cplstre is set. It tells whether the coupling's range changed, and
// whether its bands did.
func (d *Decoder) parseCouplingStrategy() (moved, newBands bool, err error) {
	r := &d.r
	h := &d.h
	d.cplinu = r.flag()
	if !d.cplinu {
		d.chincpl = [maxFull]bool{}
		return false, false, nil
	}
	if h.acmod < ModeStereo {
		return false, false, fmt.Errorf("%w: coupling in audio coding mode %d", ErrFormat, h.acmod)
	}
	coupled := 0
	for c := range h.nfchans {
		d.chincpl[c] = r.flag()
		if d.chincpl[c] {
			coupled++
		}
	}
	if coupled < 2 {
		return false, false, fmt.Errorf("%w: %d channels in coupling", ErrFormat, coupled)
	}
	d.phsflginu = false
	if h.acmod == ModeStereo {
		d.phsflginuAt = r.pos
		d.phsflginu = r.flag()
	}
	begf, endf := r.read(4), r.read(4)
	if begf > endf+2 {
		return false, false, fmt.Errorf("%w: coupling from sub-band %d to %d", ErrFormat, begf, endf+2)
	}
	moved = begf != d.cplbegf || endf != d.cplendf
	d.cplbegf, d.cplendf = begf, endf
	d.ncplbnd = 0
	for s := begf; s < endf+3; s++ {
		strc := s > begf && r.flag()
		newBands = newBands || strc != d.cplbndstrc[s]
		d.cplbndstrc[s] = strc
		if !strc {
			d.ncplbnd++
		}
	}
	return moved, newBands, nil
}

// parseCouplingCoordinates reads the coupling coordinates of a block, and
// its phase flags. Those of a channel can be reused only when it isn't
// fresh: when it was in coupling in the block before, of the same bands.
func (d *Decoder) parseCouplingCoordinates(fresh *[maxFull]bool) error {
	r := &d.r
	h := &d.h
	any := false
	for c := range h.nfchans {
		if !d.chincpl[c] {
			continue
		}
		if !r.flag() { // cplcoe
			if fresh[c] {
				return fmt.Errorf("%w: coupling coordinates of channel %d reused", ErrFormat, c)
			}
			continue
		}
		any = true
		mstr := 3 * r.read(2)
		var co float32
		for s := d.cplbegf; s < d.cplendf+3; s++ {
			if !d.cplbndstrc[s] {
				exp, mant := r.read(4), r.read(4)
				if exp == 15 {
					co = float32(mant) / 16
				} else {
					co = float32(mant+16) / 32
				}
				co *= pow2(-(exp + mstr))
			}
			d.cplco[c][s] = co
		}
	}
	if h.acmod == ModeStereo && any {
		d.phsflgAt = r.pos
	}
	if !d.phsflginu {
		d.phsflg = [18]bool{}
	} else if any {
		var flag bool
		for s := d.cplbegf; s < d.cplendf+3; s++ {
			if !d.cplbndstrc[s] {
				flag = r.flag()
			}
			d.phsflg[s] = flag
		}
	}
	return nil
}

// pow2 returns 2^e.
func pow2(e int) float32 { return float32(math.Ldexp(1, e)) }

// rematBands returns the rematrixing bands of a 2/0 block, of Table 5.15.
func (d *Decoder) rematBands() int {
	switch {
	case !d.cplinu || d.cplbegf > 2:
		return 4
	case d.cplbegf > 0:
		return 3
	default:
		return 2
	}
}

// parseDelta reads the delta bit allocation of a channel, as its deltbae,
// read already, tells. It can't be reused in block 0, nor where the
// coupling channel's range moved (moved).
func (d *Decoder) parseDelta(ch *channel, blk int, moved bool) error {
	r := &d.r
	switch ch.deltbae {
	case deltaReuse:
		if blk == 0 || (moved && ch.delta) {
			return fmt.Errorf("%w: delta bit allocation reused", ErrFormat)
		}
	case deltaNew:
		ch.delta = true
		ch.deltnseg = r.read(3)
		for s := range ch.deltnseg + 1 {
			ch.deltoffst[s] = r.read(5)
			ch.deltlen[s] = r.read(4)
			ch.deltba[s] = r.read(3)
		}
	case deltaNone:
		ch.delta = false
	default:
		return fmt.Errorf("%w: deltbae 3", ErrFormat)
	}
	return nil
}
