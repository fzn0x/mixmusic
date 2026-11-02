package ac3

import (
	"errors"
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

// parseBlock reads audio block blk of the syncframe r reads, of AC-3 or
// of E-AC-3, and turns it into the transform coefficients of its channels.
func (d *Decoder) parseBlock(blk int) error {
	r := &d.r
	h := &d.h
	f := &d.frm
	eac3 := h.eac3
	nf := h.nfchans
	for c := range nf {
		d.ch[c].blksw = (!eac3 || f.blkswe) && r.flag()
	}
	d.ditherAt, d.phsflginuAt, d.phsflgAt = r.pos, -1, -1
	for c := range nf {
		d.ch[c].dithflag = eac3 && !f.dithflage || r.flag()
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
	if eac3 {
		if err := d.parseSpectralExtension(blk); err != nil {
			return err
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
	cplstre := f.cplstre[blk]
	if !eac3 {
		cplstre = r.flag()
	}
	if cplstre {
		m, b, err := d.parseCouplingStrategy(blk)
		if err != nil {
			return err
		}
		moved = moved || m
		newBands = b
	} else if eac3 && blk == 0 {
		// A frame of one channel, or two independent ones.
		d.cplinu = false
		d.chincpl = [maxFull]bool{}
	} else if blk == 0 {
		return fmt.Errorf("%w: no coupling strategy in block 0", ErrFormat)
	}
	if d.cplinu {
		var fresh [maxFull]bool
		for c := range nf {
			fresh[c] = !eac3 && (moved || newBands || !wasIn[c])
		}
		if err := d.parseCouplingCoordinates(&fresh); err != nil {
			return err
		}
		f.cplblocks++
	}

	// Rematrixing.
	if h.acmod == ModeStereo {
		if (eac3 && blk == 0) || r.flag() { // rematstr
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
	var chexpstr [maxFull]int
	lfeexpstr := expReuse
	if eac3 {
		if d.cplinu {
			cplexpstr = f.cplexpstr[blk]
		}
		chexpstr = f.chexpstr[blk]
		if h.lfeon {
			lfeexpstr = f.lfeexpstr[blk]
		}
	} else {
		if d.cplinu {
			cplexpstr = r.read(2)
		}
		for c := range nf {
			chexpstr[c] = r.read(2)
		}
		if h.lfeon {
			lfeexpstr = r.read(1)
		}
	}
	if d.cplinu {
		if cplexpstr == expReuse && moved {
			return fmt.Errorf("%w: coupling exponents reused", ErrFormat)
		}
		cpl.start = 37 + 12*d.cplbegf
		cpl.end = 37 + 12*(d.cplendf+3)
	}
	d.expstr = chexpstr
	d.cplexpstr = cplexpstr
	for c := range nf {
		ch := &d.ch[c]
		end := ch.end
		switch {
		case d.chincpl[c]:
			end = cpl.start
		case d.chinspx[c]:
			end = spxBand(d.spxBegin)
		case chexpstr[c] != expReuse:
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
	switch {
	case eac3 && !f.bamode:
		d.sdcycod, d.fdcycod, d.sgaincod, d.dbpbcod, d.floorcod = 2, 1, 1, 2, 7
	case r.flag(): // baie
		d.sdcycod = r.read(2)
		d.fdcycod = r.read(2)
		d.sgaincod = r.read(2)
		d.dbpbcod = r.read(2)
		d.floorcod = r.read(3)
	case blk == 0:
		return fmt.Errorf("%w: no bit allocation parameters in block 0", ErrFormat)
	}
	if err := d.parseOffsets(blk); err != nil {
		return err
	}
	if eac3 && h.strmtyp == streamIndependent && r.flag() { // convsnroffste
		r.skip(10)
	}
	if d.cplinu {
		cplleake := eac3 && f.firstcplleak
		f.firstcplleak = false
		if cplleake || r.flag() {
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
	switch {
	case eac3 && !f.dbaflde:
		cpl.delta = false
		for c := range nf {
			d.ch[c].delta = false
		}
	case r.flag(): // deltbaie
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
	case blk == 0:
		cpl.delta = false
		for c := range nf {
			d.ch[c].delta = false
		}
	case d.cplinu && moved && cpl.delta:
		return fmt.Errorf("%w: no coupling delta bit allocation", ErrFormat)
	}
	if (!eac3 || f.skipflde) && r.flag() { // skiple
		r.skip(8 * r.read(9))
	}
	if r.over() {
		return fmt.Errorf("%w: syncframe too short", ErrFormat)
	}

	// The bit allocation, and the mantissas.
	zero := d.csnroffst == 0 && (!d.cplinu || cpl.fsnroffst == 0) && (!h.lfeon || d.ch[lfeIndex].fsnroffst == 0)
	for c := range nf {
		zero = zero && d.ch[c].fsnroffst == 0
	}
	d.mantAt = r.pos
	var g groups
	gotCpl := false
	for c := range nf {
		ch := &d.ch[c]
		if err := d.channelMantissas(&g, ch, blk, zero, ch.dithflag); err != nil {
			return err
		}
		if d.cplinu && d.chincpl[c] && !gotCpl {
			if err := d.channelMantissas(&g, cpl, blk, zero, false); err != nil {
				return err
			}
			gotCpl = true
		}
	}
	if h.lfeon {
		if err := d.channelMantissas(&g, &d.ch[lfeIndex], blk, zero, false); err != nil {
			return err
		}
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
	if d.spxinu {
		d.extend()
	}
	d.compress()
	if d.onBlock != nil {
		d.onBlock(blk)
	}
	return nil
}

// parseOffsets reads the SNR offsets and the fast gains of a block, of
// AC-3 or of the SNR offset strategy of an E-AC-3 frame.
func (d *Decoder) parseOffsets(blk int) error {
	r := &d.r
	h := &d.h
	f := &d.frm
	nf := h.nfchans
	cpl := &d.ch[cplIndex]
	lfe := &d.ch[lfeIndex]
	if !h.eac3 {
		if !r.flag() { // snroffste
			if blk == 0 {
				return fmt.Errorf("%w: no SNR offsets in block 0", ErrFormat)
			}
			return nil
		}
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
			lfe.fsnroffst = r.read(4)
			lfe.fgaincod = r.read(3)
		}
		return nil
	}
	setFine := func(v int) {
		cpl.fsnroffst, lfe.fsnroffst = v, v
		for c := range nf {
			d.ch[c].fsnroffst = v
		}
	}
	switch {
	case f.snroffststr == 0:
		d.csnroffst = f.frmcsnroffst
		setFine(f.frmfsnroffst)
	case blk == 0 || r.flag(): // snroffste
		d.csnroffst = r.read(6)
		if f.snroffststr == 1 {
			setFine(r.read(4)) // blkfsnroffst
		} else {
			if d.cplinu {
				cpl.fsnroffst = r.read(4)
			}
			for c := range nf {
				d.ch[c].fsnroffst = r.read(4)
			}
			if h.lfeon {
				lfe.fsnroffst = r.read(4)
			}
		}
	}
	if f.frmfgaincode && r.flag() { // fgaincode
		if d.cplinu {
			cpl.fgaincod = r.read(3)
		}
		for c := range nf {
			d.ch[c].fgaincod = r.read(3)
		}
		if h.lfeon {
			lfe.fgaincod = r.read(3)
		}
	} else {
		cpl.fgaincod, lfe.fgaincod = 4, 4
		for c := range nf {
			d.ch[c].fgaincod = 4
		}
	}
	return nil
}

// channelMantissas computes the bit allocation of a channel's exponents
// and reads its mantissas, of AC-3's quantizers or, in block 0 of a
// channel of AHT, those of the six blocks. zero is the special case of
// all SNR offsets 0, which allocates no bits; dither tells whether
// mantissas of no bits are dither.
func (d *Decoder) channelMantissas(g *groups, ch *channel, blk int, zero, dither bool) error {
	if ch.aht {
		if blk == 0 {
			if err := d.allocate(ch, zero); err != nil {
				return err
			}
			if err := d.ahtMantissas(ch); err != nil {
				return err
			}
		}
		d.ahtBlock(ch, blk, dither)
		return nil
	}
	if err := d.allocate(ch, zero); err != nil {
		return err
	}
	if err := d.mantissas(g, ch, dither); err != nil {
		return err
	}
	if ch != &d.ch[cplIndex] {
		clear(ch.coef[ch.end:])
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

// parseCouplingStrategy reads the coupling strategy of block blk, whose
// cplstre is set. It tells whether the coupling's range changed, and
// whether its bands did.
func (d *Decoder) parseCouplingStrategy(blk int) (moved, newBands bool, err error) {
	r := &d.r
	h := &d.h
	f := &d.frm
	if h.eac3 {
		d.cplinu = f.cplinu[blk]
	} else {
		d.cplinu = r.flag()
	}
	if !d.cplinu {
		d.chincpl = [maxFull]bool{}
		if h.eac3 {
			for c := range h.nfchans {
				f.firstcplcos[c] = true
			}
			f.firstcplleak = true
			d.phsflginu = false
		}
		return false, false, nil
	}
	if h.acmod < ModeStereo {
		return false, false, fmt.Errorf("%w: coupling in audio coding mode %d", ErrFormat, h.acmod)
	}
	if h.eac3 && r.flag() { // ecplinu
		return false, false, fmt.Errorf("ac3: enhanced coupling: %w", errors.ErrUnsupported)
	}
	coupled := 0
	for c := range h.nfchans {
		d.chincpl[c] = h.eac3 && h.acmod == ModeStereo || r.flag()
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
	begf := r.read(4)
	var endf int
	switch {
	case !h.eac3 || !d.spxinu:
		endf = r.read(4)
	case d.spxbegf < 6:
		endf = d.spxbegf - 2 // the coupling ends where spectral extension starts
	default:
		endf = d.spxbegf*2 - 7
	}
	if begf > endf+2 {
		return false, false, fmt.Errorf("%w: coupling from sub-band %d to %d", ErrFormat, begf, endf+2)
	}
	moved = begf != d.cplbegf || endf != d.cplendf
	d.cplbegf, d.cplendf = begf, endf
	// E-AC-3 may leave out the band structure: the structure of the
	// block before then holds, or the default one, which the frame starts
	// with.
	strce := !h.eac3 || r.flag()
	d.ncplbnd = 0
	for s := begf; s < endf+3; s++ {
		strc := d.cplbndstrc[s]
		if strce {
			strc = s > begf && r.flag()
			newBands = newBands || strc != d.cplbndstrc[s]
			d.cplbndstrc[s] = strc
		}
		if s == begf || !strc {
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
	f := &d.frm
	any := false
	for c := range h.nfchans {
		if !d.chincpl[c] {
			f.firstcplcos[c] = true
			continue
		}
		// E-AC-3's first block of coupling of a channel has its
		// coordinates, without cplcoe.
		cplcoe := h.eac3 && f.firstcplcos[c]
		f.firstcplcos[c] = false
		if !cplcoe && !r.flag() { // cplcoe
			if fresh[c] {
				return fmt.Errorf("%w: coupling coordinates of channel %d reused", ErrFormat, c)
			}
			continue
		}
		any = true
		mstr := 3 * r.read(2)
		var co float32
		for s := d.cplbegf; s < d.cplendf+3; s++ {
			if s == d.cplbegf || !d.cplbndstrc[s] {
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
			if s == d.cplbegf || !d.cplbndstrc[s] {
				flag = r.flag()
			}
			d.phsflg[s] = flag
		}
	}
	return nil
}

// pow2 returns 2^e.
func pow2(e int) float32 { return float32(math.Ldexp(1, e)) }

// rematBands returns the rematrixing bands of a 2/0 block, of Table 5.15,
// and of section E3.3.2 with spectral extension.
func (d *Decoder) rematBands() int {
	switch {
	case d.cplinu && d.cplbegf == 0:
		return 2
	case d.cplinu && d.cplbegf < 3:
		return 3
	case !d.cplinu && d.spxinu && d.spxbegf < 2:
		return 3
	default:
		return 4
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
