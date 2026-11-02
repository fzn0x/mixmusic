package ac3

import (
	"errors"
	"fmt"
)

// E-AC-3, of Annex E of A/52 and of ETSI TS 102 366: its bsi, and its
// audfrm, which tells the strategies of the blocks of a frame.

// Stream types, of strmtyp.
const (
	streamIndependent = 0
	streamDependent   = 1
	streamConverted   = 2 // independent, converted from AC-3
)

// blocksOf are the blocks of a frame of each numblkscod.
var blocksOf = [4]int{1, 2, 3, 6}

// parseEAC3 reads the bsi of an E-AC-3 syncframe, after its syncword.
func (h *header) parseEAC3(r *bitReader) error {
	*h = header{bsid: h.bsid, eac3: true, cmixlev: -1, surmixlev: -1, loroCmixlev: -1, loroSurmixlev: -1, lfemixlevcod: -1}
	h.strmtyp = r.read(2)
	h.substreamid = r.read(3)
	h.size = 2 * (r.read(11) + 1)
	h.fscod = r.read(2)
	numblkscod := r.read(2)
	if h.fscod == 3 {
		// The reduced sample rates of fscod2: the standard doesn't say
		// which hearing threshold their bit allocation takes.
		return fmt.Errorf("ac3: E-AC-3 at %d Hz: %w", [4]int{24000, 22050, 16000}[numblkscod&3], errors.ErrUnsupported)
	}
	h.rate = sampleRates[h.fscod]
	h.numblks = blocksOf[numblkscod]
	h.acmod = r.read(3)
	h.nfchans = modeChannels[h.acmod]
	h.lfeon = r.flag()
	r.skip(5) // bsid
	if h.strmtyp == 3 {
		return fmt.Errorf("%w: strmtyp 3", ErrFormat)
	}
	h.dialnorm = r.read(5)
	if r.flag() { // compre
		r.skip(8)
	}
	if h.acmod == ModeDualMono {
		r.skip(5) // dialnorm2
		if r.flag() {
			r.skip(8) // compr2
		}
	}
	if h.strmtyp == streamDependent && r.flag() { // chanmape
		r.skip(16)
	}
	if r.flag() { // mixmdate
		if h.acmod > ModeStereo {
			r.skip(2) // dmixmod
		}
		if h.acmod&1 != 0 && h.acmod > ModeStereo {
			r.skip(3) // ltrtcmixlev
			h.loroCmixlev = r.read(3)
		}
		if h.acmod&4 != 0 {
			r.skip(3) // ltrtsurmixlev
			h.loroSurmixlev = r.read(3)
		}
		if h.lfeon && r.flag() { // lfemixlevcode
			h.lfemixlevcod = r.read(5)
		}
		if h.strmtyp == streamIndependent {
			if r.flag() { // pgmscle
				r.skip(6)
			}
			if h.acmod == ModeDualMono && r.flag() { // pgmscl2e
				r.skip(6)
			}
			if r.flag() { // extpgmscle
				r.skip(6)
			}
			switch r.read(2) { // mixdef
			case 1:
				r.skip(5) // premixcmpsel, drcsrc, premixcmpscl
			case 2:
				r.skip(12) // mixdata
			case 3:
				r.skip(8 * (r.read(5) + 2)) // mixdeflen, then mixdata
			}
			if h.acmod < ModeStereo {
				if r.flag() { // paninfoe
					r.skip(8 + 6)
				}
				if h.acmod == ModeDualMono && r.flag() { // paninfo2e
					r.skip(8 + 6)
				}
			}
			if r.flag() { // frmmixcfginfoe
				if h.numblks == 1 {
					r.skip(5)
				} else {
					for range h.numblks {
						if r.flag() { // blkmixcfginfoe
							r.skip(5)
						}
					}
				}
			}
		}
	}
	if r.flag() { // infomdate
		r.skip(3 + 1 + 1) // bsmod, copyrightb, origbs
		if h.acmod == ModeStereo {
			r.skip(2 + 2) // dsurmod, dheadphonmod
		}
		if h.acmod >= Mode22 {
			r.skip(2) // dsurexmod
		}
		if r.flag() { // audprodie
			r.skip(5 + 2 + 1)
		}
		if h.acmod == ModeDualMono && r.flag() { // audprodi2e
			r.skip(5 + 2 + 1)
		}
		r.skip(1) // sourcefscod, as fscod < 3
	}
	if h.strmtyp == streamIndependent && h.numblks != 6 {
		r.skip(1) // convsync
	}
	if h.strmtyp == streamConverted {
		blkid := h.numblks == 6 || r.flag()
		if blkid {
			r.skip(6) // frmsizecod
		}
	}
	if r.flag() { // addbsie
		r.skip((r.read(6) + 1) * 8)
	}
	if r.over() {
		return fmt.Errorf("%w: syncframe too short", ErrFormat)
	}
	h.end = r.pos
	return nil
}

// audioFrame is what the audfrm of an E-AC-3 syncframe tells of its
// blocks.
type audioFrame struct {
	expstre, ahte                              bool
	snroffststr                                int
	transproce, blkswe, dithflage, bamode      bool
	frmfgaincode, dbaflde, skipflde, spxattene bool
	cplstre, cplinu                            [6]bool
	cplexpstr                                  [6]int
	chexpstr                                   [6][maxFull]int
	lfeexpstr                                  [6]int
	cplahtinu, lfeahtinu                       bool
	chahtinu                                   [maxFull]bool
	frmcsnroffst, frmfsnroffst                 int
	spxattencod                                [maxFull]int // -1 without attenuation
	firstspxcos, firstcplcos                   [maxFull]bool
	firstcplleak                               bool
	dithflageAt                                int // for tests: where dithflage is, in bits
	cplblocks                                  int // the blocks of coupling so far
}

// frameStrategies are the exponent strategies of the six blocks of each
// frmchexpstr and frmcplexpstr, of Table E2.10.
var frameStrategies = func() (t [32][6]uint8) {
	const r, a, b, c = expReuse, expD15, expD25, expD45
	rows := [32][6]uint8{
		{a, r, r, r, r, r}, {a, r, r, r, r, c}, {a, r, r, r, b, r}, {a, r, r, r, c, c},
		{b, r, r, b, r, r}, {b, r, r, b, r, c}, {b, r, r, c, b, r}, {b, r, r, c, c, c},
		{b, r, a, r, r, r}, {b, r, b, r, r, c}, {b, r, b, r, b, r}, {b, r, b, r, c, c},
		{b, r, c, b, r, r}, {b, r, c, b, r, c}, {b, r, c, c, b, r}, {b, r, c, c, c, c},
		{c, a, r, r, r, r}, {c, a, r, r, r, c}, {c, b, r, r, b, r}, {c, b, r, r, c, c},
		{c, b, r, b, r, r}, {c, b, r, b, r, c}, {c, b, r, c, b, r}, {c, b, r, c, c, c},
		{c, c, a, r, r, r}, {c, c, b, r, r, c}, {c, c, b, r, b, r}, {c, c, b, r, c, c},
		{c, c, c, b, r, r}, {c, c, c, b, r, c}, {c, c, c, c, b, r}, {c, c, c, c, c, c},
	}
	return rows
}()

// parseAudioFrame reads the audfrm of the E-AC-3 syncframe whose bsi r
// read.
func (d *Decoder) parseAudioFrame() error {
	r := &d.r
	h := &d.h
	f := &d.frm
	nf := h.nfchans
	nb := h.numblks
	f.expstre, f.ahte = true, false
	if nb == 6 {
		f.expstre = r.flag()
		f.ahte = r.flag()
	}
	f.snroffststr = r.read(2)
	if f.snroffststr == 3 {
		return fmt.Errorf("%w: snroffststr 3", ErrFormat)
	}
	f.transproce = r.flag()
	f.blkswe = r.flag()
	f.dithflageAt = r.pos
	f.dithflage = r.flag()
	f.bamode = r.flag()
	f.frmfgaincode = r.flag()
	f.dbaflde = r.flag()
	f.skipflde = r.flag()
	f.spxattene = r.flag()

	f.cplstre, f.cplinu = [6]bool{}, [6]bool{}
	ncplblks := 0
	if h.acmod > ModeMono {
		f.cplstre[0] = true
		f.cplinu[0] = r.flag()
		for blk := 1; blk < nb; blk++ {
			f.cplstre[blk] = r.flag()
			if f.cplstre[blk] {
				f.cplinu[blk] = r.flag()
			} else {
				f.cplinu[blk] = f.cplinu[blk-1]
			}
		}
	}
	for blk := range nb {
		if f.cplinu[blk] {
			ncplblks++
		}
	}

	// The exponent strategies.
	if f.expstre {
		for blk := range nb {
			if f.cplinu[blk] {
				f.cplexpstr[blk] = r.read(2)
			}
			for c := range nf {
				f.chexpstr[blk][c] = r.read(2)
			}
		}
	} else {
		if h.acmod > ModeMono && ncplblks > 0 {
			s := r.read(5)
			for blk := range 6 {
				f.cplexpstr[blk] = int(frameStrategies[s][blk])
			}
		}
		for c := range nf {
			s := r.read(5)
			for blk := range 6 {
				f.chexpstr[blk][c] = int(frameStrategies[s][blk])
			}
		}
	}
	if h.lfeon {
		for blk := range nb {
			f.lfeexpstr[blk] = r.read(1)
		}
	}
	if h.strmtyp == streamIndependent {
		convexpstre := nb == 6 || r.flag()
		if convexpstre {
			r.skip(5 * nf) // convexpstr
		}
	}

	// AHT, which only a channel of exponents of block 0 alone can use.
	f.cplahtinu, f.lfeahtinu = false, false
	f.chahtinu = [maxFull]bool{}
	if f.ahte {
		ncplregs := 0
		for blk := range 6 {
			if f.cplstre[blk] || f.cplexpstr[blk] != expReuse {
				ncplregs++
			}
		}
		if ncplblks == 6 && ncplregs == 1 {
			f.cplahtinu = r.flag()
		}
		for c := range nf {
			n := 0
			for blk := range 6 {
				if f.chexpstr[blk][c] != expReuse {
					n++
				}
			}
			if n == 1 {
				f.chahtinu[c] = r.flag()
			}
		}
		if h.lfeon {
			n := 0
			for blk := range 6 {
				if f.lfeexpstr[blk] != expReuse {
					n++
				}
			}
			if n == 1 {
				f.lfeahtinu = r.flag()
			}
		}
	}

	if f.snroffststr == 0 {
		f.frmcsnroffst = r.read(6)
		f.frmfsnroffst = r.read(4)
	}
	if f.transproce {
		// Transient pre-noise processing, which this decoder leaves
		// out: the standard's decoder may improve transients with it.
		for range nf {
			if r.flag() { // chintransproc
				r.skip(10 + 8) // transprocloc, transproclen
			}
		}
	}
	for c := range nf {
		f.spxattencod[c] = -1
		if f.spxattene && r.flag() { // chinspxatten
			f.spxattencod[c] = r.read(5)
		}
	}
	if nb > 1 && r.flag() { // blkstrtinfoe
		words := h.size / 2
		bits := 0
		for 1<<bits < words {
			bits++
		}
		r.skip((nb - 1) * (4 + bits))
	}
	for c := range nf {
		f.firstspxcos[c] = true
		f.firstcplcos[c] = true
	}
	f.firstcplleak = true
	f.cplblocks = 0
	if r.over() {
		return fmt.Errorf("%w: syncframe too short", ErrFormat)
	}
	return nil
}
