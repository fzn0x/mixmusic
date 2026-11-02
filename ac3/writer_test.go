package ac3

import (
	"encoding/binary"
	"math"
	"testing"
)

// A writer of E-AC-3 frames, for tests: it captures what the decoder reads
// of the blocks of a stream ffmpeg's encoder made, then writes them again
// as E-AC-3 frames of the syntax and the tools ffmpeg's encoder never
// uses, their mantissas quantized again as the bit allocation of their new
// parameters asks. ffmpeg's decoder then decodes them, as the decoder does.

// capturedBlock is what the decoder read of a block.
type capturedBlock struct {
	cplinu     bool
	chincpl    [maxFull]bool
	cplbegf    int
	cplendf    int
	cplbndstrc [18]bool
	cplco      [maxFull][18]float32
	rematflg   [4]bool
	// Of each state: its bins, exponents, and mantissas, of the bins of
	// bits, before the exponents.
	start, end [maxStates]int
	exp        [maxStates][maxBins]uint8
	mant       [maxStates][maxBins]float32
}

// capturedStream is what the decoder read of the frames of a stream.
type capturedStream struct {
	acmod, nf int
	lfeon     bool
	fscod     int
	blocks    []capturedBlock
}

// capture decodes a stream of AC-3 or E-AC-3 of no dither, and keeps what
// it read of each block.
func capture(t testing.TB, data []byte) *capturedStream {
	t.Helper()
	cs := &capturedStream{}
	d := NewDecoder()
	d.onBlock = func(blk int) {
		h := &d.h
		cs.acmod, cs.nf, cs.lfeon, cs.fscod = h.acmod, h.nfchans, h.lfeon, h.fscod
		if d.dynrng != 1 {
			t.Fatal("a block has a dynrng")
		}
		var b capturedBlock
		b.cplinu, b.chincpl = d.cplinu, d.chincpl
		b.cplbegf, b.cplendf, b.cplbndstrc, b.cplco = d.cplbegf, d.cplendf, d.cplbndstrc, d.cplco
		b.rematflg = d.rematflg
		if h.acmod == ModeStereo {
			// The coefficients of the rematrixed bands, back to what the
			// bit stream holds.
			l, r := &d.ch[0], &d.ch[1]
			nb := d.rematBands()
			for i := range nb {
				if !d.rematflg[i] {
					continue
				}
				hi := rematBounds[i+1]
				if d.cplinu && i == nb-1 {
					hi = 37 + 12*d.cplbegf
				}
				for bin := rematBounds[i]; bin < min(hi, l.end, r.end); bin++ {
					a, c := l.coef[bin], r.coef[bin]
					l.coef[bin], r.coef[bin] = (a+c)/2, (a-c)/2
				}
			}
		}
		for s := range maxStates {
			ch := &d.ch[s]
			if (s < h.nfchans) || (s == lfeIndex && h.lfeon) || (s == cplIndex && d.cplinu) {
				b.start[s], b.end[s], b.exp[s] = ch.start, ch.end, ch.exp
				for bin := ch.start; bin < ch.end; bin++ {
					if ch.bap[bin] != 0 {
						b.mant[s][bin] = ch.coef[bin] / expScale[ch.exp[bin]]
					}
				}
			}
		}
		cs.blocks = append(cs.blocks, b)
	}
	pcm := make([]float32, MaxChannels*FrameLength)
	for i, f := range frames(t, data) {
		if _, err := d.Decode(f, pcm); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	return cs
}

// writeOptions are the syntax and tools of the frames writeEAC3 writes.
type writeOptions struct {
	numblks     int
	snroffststr int  // 1 or 2: of each block, else of the frame
	bamode      bool // write the bit allocation parameters
	fgain       int  // the fast gain code of channel 0, written when not 0 or 4
	delta       []segment
	skip        bool // a skip field of 3 bytes in block 1
	blksw       bool // short blocks
	transproc   bool // transient pre-noise processing data, which decoders may ignore
	// Spectral extension of every full-bandwidth channel, of a stream
	// without coupling: of sub-bands spxbegf to spxendf, copied from
	// spxstrtf, in one band, of spxblnd, and the notch of attenuation
	// code atten, unless it is -1.
	spx                                bool
	spxstrtf, spxbegf, spxendf, spxbnd int
	spxblnd, atten                     int
	// AHT of channel 0, of exponents of block 0 alone, and its GAQ
	// mode, which gaq gives.
	aht bool
	gaq int
	// csnr is the coarse SNR offset, 15 when 0.
	csnr int
	// ahtX, when set, sets the mantissas of AHT of each bin, in place of
	// those of the stream; flatExp, when not 0, sets the exponents of the
	// channel of AHT all to it, for a bit allocation alike in every bin.
	ahtX    func(bin int, x *[6]float64)
	flatExp int
	// Mixing metadata: the Lo/Ro levels and the LFE's mix level code, or
	// -1.
	mix                     bool
	loroC, loroS, lfeMixLev int
}

// bitWriter writes bits, most significant first.
type bitWriter struct{ s bitstring }

func (w *bitWriter) put(v, n int) {
	for b := n - 1; b >= 0; b-- {
		w.s = append(w.s, byte(v>>b&1))
	}
}

func (w *bitWriter) flag(v bool) {
	if v {
		w.put(1, 1)
	} else {
		w.put(0, 1)
	}
}

// coordinate returns the exponent and the mantissa of a coupling
// coordinate co, of master mstr, the nearest of their values.
func coordinate(co float32, mstr int) (exp, mant int) {
	best := math.Inf(1)
	for e := range 16 {
		for m := range 16 {
			v := float64(m+16) / 32
			if e == 15 {
				v = float64(m) / 16
			}
			v *= math.Ldexp(1, -(e + 3*mstr))
			if d := math.Abs(v - float64(co)); d < best {
				best, exp, mant = d, e, m
			}
		}
	}
	return exp, mant
}

// writeExponents writes the exponents of bins from to end of a state as
// D15, after its absolute exponent abs: groups of three differences. A
// difference of more than 2 can't be written: the exponents become those
// the decoder reads.
func writeExponents(w *bitWriter, exp *[maxBins]uint8, abs, from, end int) {
	prev := abs
	for i := from; i < end; i += 3 {
		var m [3]int
		for k := range 3 {
			e := prev
			if i+k < end {
				e = int(exp[i+k])
			}
			dexp := min(2, max(-2, e-prev))
			m[k] = dexp + 2
			prev += dexp
			if i+k < end {
				exp[i+k] = uint8(prev)
			}
		}
		w.put(25*m[0]+5*m[1]+m[2], 7)
	}
}

// writeEAC3 writes the blocks of cs as frames of E-AC-3, as o asks, and
// returns the stream.
func writeEAC3(t testing.TB, cs *capturedStream, o writeOptions) []byte {
	t.Helper()
	nf := cs.nf
	nb := o.numblks
	// A copy, whose exponents the writing may change.
	c := *cs
	c.blocks = append([]capturedBlock{}, cs.blocks...)
	cs = &c
	if o.flatExp != 0 {
		for i := range c.blocks {
			b := &c.blocks[i]
			for bin := range maxBins {
				m := float64(b.mant[0][bin]) * math.Ldexp(1, o.flatExp-int(b.exp[0][bin]))
				b.mant[0][bin] = float32(min(0.99, max(-0.99, m)))
				b.exp[0][bin] = uint8(o.flatExp)
			}
		}
	}
	if cs.acmod == ModeDualMono {
		t.Fatal("no writer of 1+1 streams")
	}
	var out []byte
	d := NewDecoder() // its bit allocation
	d.h.fscod = cs.fscod
	for first := 0; first+nb <= len(cs.blocks); first += nb {
		blocks := cs.blocks[first : first+nb]
		var w bitWriter
		w.put(syncWord, 16)
		w.put(streamIndependent, 2)
		w.put(0, 3)  // substreamid
		w.put(0, 11) // frmsiz, set at the end
		w.put(cs.fscod, 2)
		w.put([7]int{1: 0, 2: 1, 3: 2, 6: 3}[nb], 2)
		w.put(cs.acmod, 3)
		w.flag(cs.lfeon)
		w.put(16, 5) // bsid
		w.put(31, 5) // dialnorm
		w.put(0, 1)  // compre
		w.flag(o.mix)
		if o.mix {
			if cs.acmod > ModeStereo {
				w.put(2, 2) // dmixmod: Lo/Ro
			}
			if cs.acmod&1 != 0 && cs.acmod > ModeStereo {
				w.put(4, 3) // ltrtcmixlev
				w.put(o.loroC, 3)
			}
			if cs.acmod&4 != 0 {
				w.put(4, 3) // ltrtsurmixlev
				w.put(o.loroS, 3)
			}
			if cs.lfeon {
				w.flag(o.lfeMixLev >= 0)
				if o.lfeMixLev >= 0 {
					w.put(o.lfeMixLev, 5)
				}
			}
			w.put(0, 1) // pgmscle
			w.put(0, 1) // extpgmscle
			w.put(0, 2) // mixdef
			if cs.acmod < ModeStereo {
				w.put(0, 1) // paninfoe
			}
			w.put(0, 1) // frmmixcfginfoe
		}
		w.put(0, 1) // infomdate
		if nb != 6 {
			w.put(0, 1) // convsync
		}
		w.put(0, 1) // addbsie

		// audfrm.
		aht := o.aht && nb == 6
		if nb == 6 {
			w.put(1, 1) // expstre
			w.flag(aht)
		}
		snrstr := o.snroffststr
		w.put(snrstr, 2)
		w.flag(o.transproc)
		w.flag(o.blksw)
		w.put(1, 1) // dithflage
		w.flag(o.bamode)
		fgain := o.fgain != 0 && o.fgain != 4
		w.flag(fgain)
		w.flag(o.delta != nil)
		w.flag(o.skip)
		w.flag(o.spx && o.atten >= 0)
		if cs.acmod > ModeMono {
			w.flag(blocks[0].cplinu)
			for blk := 1; blk < nb; blk++ {
				w.put(1, 1) // cplstre
				w.flag(blocks[blk].cplinu)
			}
		}
		// The exponent strategies: D15 in block 0, reused while the
		// exponents are, of a channel of AHT always.
		var expstr [6][maxStates]int
		for blk := range nb {
			for s := range maxStates {
				expstr[blk][s] = expD15
				if blk > 0 && (s == 0 && aht || blocks[blk].exp[s] == blocks[blk-1].exp[s] && blocks[blk].end[s] == blocks[blk-1].end[s] &&
					blocks[blk].start[s] == blocks[blk-1].start[s] && blocks[blk].cplinu == blocks[blk-1].cplinu) {
					expstr[blk][s] = expReuse
				}
				if o.spx && s < nf {
					// The bandwidth changes when spectral extension starts.
					if blk == 0 {
						expstr[blk][s] = expD15
					}
				}
			}
			if blocks[blk].cplinu {
				w.put(expstr[blk][cplIndex], 2)
			}
			for c := range nf {
				w.put(expstr[blk][c], 2)
			}
		}
		if cs.lfeon {
			for blk := range nb {
				w.put(expstr[blk][lfeIndex]&1, 1)
			}
		}
		if nb == 6 {
			w.put(0, 5*nf) // convexpstr, as convexpstre is 1 of 6 blocks
		} else {
			w.put(0, 1) // convexpstre
		}
		if aht {
			if blocks[0].cplinu {
				// cplahtinu is there only of coupling of one exponent set.
				t.Fatal("AHT of a stream of coupling")
			}
			// chahtinu, and lfeahtinu, of a channel of exponents of block 0
			// alone.
			regs := func(s int) int {
				n := 0
				for blk := range 6 {
					if expstr[blk][s] != expReuse {
						n++
					}
				}
				return n
			}
			for c := range nf {
				if regs(c) == 1 {
					w.flag(c == 0)
				} else if c == 0 {
					t.Fatal("AHT of a channel of exponents of more blocks than one")
				}
			}
			if cs.lfeon && regs(lfeIndex) == 1 {
				w.put(0, 1)
			}
		}
		csnr, fsnr := 15, 0
		if o.csnr != 0 {
			csnr = o.csnr
		}
		if snrstr == 0 {
			w.put(csnr, 6)
			w.put(fsnr, 4)
		}
		if o.transproc {
			for c := range nf {
				w.flag(c == 0)
				if c == 0 {
					w.put(300, 10)
					w.put(40, 8)
				}
			}
		}
		if o.spx && o.atten >= 0 {
			for range nf {
				w.put(1, 1)
				w.put(o.atten, 5)
			}
		}
		if nb > 1 {
			w.put(0, 1) // blkstrtinfoe
		}

		// The blocks.
		var firstCos [maxFull]bool // firstcplcos
		for c := range firstCos {
			firstCos[c] = true
		}
		for blk := range nb {
			b := &blocks[blk]
			if o.blksw {
				for c := range nf {
					w.flag((blk+c)%3 == 1)
				}
			}
			for range nf {
				w.put(0, 1) // dithflag
			}
			w.put(0, 1) // dynrnge
			if cs.acmod == ModeDualMono {
				w.put(0, 1)
			}
			// Spectral extension.
			spxBegin, spxEnd := 0, 0
			if o.spx {
				if o.spxbegf < 6 {
					spxBegin = o.spxbegf + 2
				} else {
					spxBegin = o.spxbegf*2 - 3
				}
				if o.spxendf < 3 {
					spxEnd = o.spxendf + 5
				} else {
					spxEnd = o.spxendf*2 + 3
				}
			}
			if blk > 0 {
				w.put(0, 1) // spxstre: the strategy of block 0 holds
			} else {
				w.flag(o.spx) // spxinu
				if o.spx {
					if cs.acmod != ModeMono {
						for range nf {
							w.put(1, 1)
						}
					}
					w.put(o.spxstrtf, 2)
					w.put(o.spxbegf, 3)
					w.put(o.spxendf, 3)
					w.put(1, 1) // spxbndstrce
					for s := spxBegin + 1; s < spxEnd; s++ {
						w.flag((s-spxBegin)%o.spxbnd != 0)
					}
				}
			}
			if o.spx {
				nbands := 0
				for s := spxBegin; s < spxEnd; s++ {
					if (s-spxBegin)%o.spxbnd == 0 {
						nbands++
					}
				}
				for c := range nf {
					if blk > 0 {
						w.flag(blk == 3) // spxcoe
						if blk != 3 {
							continue
						}
					}
					w.put(o.spxblnd, 5)
					w.put(c%2, 2) // mstrspxco
					for bnd := range nbands {
						w.put((bnd+c+blk)%4, 4) // spxcoexp
						w.put((bnd+2*c)%4, 2)   // spxcomant
					}
				}
			}
			// Coupling.
			if b.cplinu {
				w.put(0, 1) // ecplinu
				if cs.acmod != ModeStereo {
					for c := range nf {
						w.flag(b.chincpl[c])
					}
				} else {
					w.put(0, 1) // phsflginu
				}
				w.put(b.cplbegf, 4)
				if !o.spx {
					w.put(b.cplendf, 4)
				}
				w.put(1, 1) // cplbndstrce
				for s := b.cplbegf + 1; s < b.cplendf+3; s++ {
					w.flag(b.cplbndstrc[s])
				}
				for c := range nf {
					if !b.chincpl[c] {
						firstCos[c] = true
						continue
					}
					if !firstCos[c] {
						w.put(1, 1) // cplcoe
					}
					firstCos[c] = false
					mstr := 0
					w.put(mstr, 2)
					for s := b.cplbegf; s < b.cplendf+3; s++ {
						if s == b.cplbegf || !b.cplbndstrc[s] {
							e, m := coordinate(b.cplco[c][s], mstr)
							w.put(e, 4)
							w.put(m, 4)
						}
					}
				}
			} else {
				for c := range firstCos {
					firstCos[c] = true
				}
			}
			// Rematrixing.
			if cs.acmod == ModeStereo {
				nr := 4
				switch {
				case b.cplinu && b.cplbegf == 0:
					nr = 2
				case b.cplinu && b.cplbegf < 3:
					nr = 3
				case !b.cplinu && o.spx && o.spxbegf < 2:
					nr = 3
				}
				if blk > 0 {
					w.put(1, 1) // rematstr
				}
				for i := range nr {
					w.flag(b.rematflg[i])
				}
			}
			// The bandwidths, and the exponents.
			var start, end [maxStates]int
			start, end = b.start, b.end
			for c := range nf {
				if o.spx {
					end[c] = spxBand(spxBegin)
				}
				if expstr[blk][c] != expReuse && !b.chincpl[c] && !o.spx {
					w.put((end[c]-73)/3, 6) // chbwcod
				}
			}
			if b.cplinu && expstr[blk][cplIndex] != expReuse {
				abs := int(b.exp[cplIndex][b.start[cplIndex]]) &^ 1
				w.put(abs>>1, 4)
				writeExponents(&w, &b.exp[cplIndex], abs, b.start[cplIndex], b.end[cplIndex])
			}
			for c := range nf {
				if expstr[blk][c] == expReuse {
					continue
				}
				w.put(int(b.exp[c][0]), 4)
				writeExponents(&w, &b.exp[c], int(b.exp[c][0]), 1, end[c])
				w.put(0, 2) // gainrng
			}
			if cs.lfeon && expstr[blk][lfeIndex] != expReuse {
				w.put(int(b.exp[lfeIndex][0]), 4)
				writeExponents(&w, &b.exp[lfeIndex], int(b.exp[lfeIndex][0]), 1, 7)
			}
			// The bit allocation's parameters.
			sdcy, fdcy, sgain, dbpb, floor := 2, 1, 1, 2, 7
			if o.bamode {
				sdcy, fdcy, sgain, dbpb, floor = 3, 2, 2, 1, 5
				w.flag(blk == 0) // baie
				if blk == 0 {
					w.put(sdcy, 2)
					w.put(fdcy, 2)
					w.put(sgain, 2)
					w.put(dbpb, 2)
					w.put(floor, 3)
				}
			}
			fine := [maxStates]int{}
			for s := range fine {
				fine[s] = fsnr
			}
			switch snrstr {
			case 1:
				if blk > 0 {
					w.put(1, 1) // snroffste
				}
				w.put(csnr, 6)
				w.put(fsnr+blk%3, 4) // blkfsnroffst
				for s := range fine {
					fine[s] = fsnr + blk%3
				}
			case 2:
				if blk > 0 {
					w.put(1, 1) // snroffste
				}
				w.put(csnr, 6)
				if b.cplinu {
					fine[cplIndex] = 2
					w.put(2, 4)
				}
				for c := range nf {
					fine[c] = (c + blk) % 4
					w.put(fine[c], 4)
				}
				if cs.lfeon {
					fine[lfeIndex] = 1
					w.put(1, 4)
				}
			}
			fgaincod := [maxStates]int{4, 4, 4, 4, 4, 4, 4}
			if fgain {
				w.put(1, 1) // fgaincode
				fgaincod[0] = o.fgain
				if b.cplinu {
					w.put(4, 3)
				}
				for c := range nf {
					w.put(fgaincod[c], 3)
				}
				if cs.lfeon {
					w.put(4, 3)
				}
			}
			w.put(0, 1) // convsnroffste
			if b.cplinu {
				if blk > 0 && blocks[blk-1].cplinu {
					w.put(0, 1) // cplleake: those of block 0 hold
				}
				if blk == 0 || !blocks[blk-1].cplinu {
					w.put(0, 3) // cplfleak
					w.put(0, 3) // cplsleak
				}
			}
			if o.delta != nil {
				w.flag(blk == 0) // deltbaie, of block 0, which the others reuse
				if blk == 0 {
					if b.cplinu {
						w.put(deltaNone, 2)
					}
					for c := range nf {
						if c == 0 {
							w.put(deltaNew, 2)
						} else {
							w.put(deltaNone, 2)
						}
					}
					w.put(len(o.delta)-1, 3)
					for _, g := range o.delta {
						w.put(g.offset, 5)
						w.put(g.length, 4)
						w.put(g.ba, 3)
					}
				}
			}
			if o.skip {
				w.flag(blk == 1) // skiple
				if blk == 1 {
					w.put(3, 9)
					w.put(0xA5A5A5, 24)
				}
			}

			// The bit allocation, and the mantissas.
			var bc blockCopy
			got := false
			for c := range nf {
				bc.order = append(bc.order, c)
				if b.cplinu && b.chincpl[c] && !got {
					bc.order = append(bc.order, cplIndex)
					got = true
				}
			}
			if cs.lfeon {
				bc.order = append(bc.order, lfeIndex)
			}
			d.sdcycod, d.fdcycod, d.sgaincod, d.dbpbcod, d.floorcod = sdcy, fdcy, sgain, dbpb, floor
			d.csnroffst, d.cplfleak, d.cplsleak = csnr, 0, 0
			for _, s := range bc.order {
				ch := channel{start: start[s], end: end[s], fsnroffst: fine[s], fgaincod: fgaincod[s]}
				ch.exp = b.exp[s]
				if aht && s == 0 {
					ch.exp = blocks[0].exp[s]
					ch.aht = true
				}
				if s == 0 && o.delta != nil {
					ch.delta = true
					ch.deltnseg = len(o.delta) - 1
					for i, g := range o.delta {
						ch.deltoffst[i], ch.deltlen[i], ch.deltba[i] = g.offset, g.length, g.ba
					}
				}
				if err := d.allocate(&ch, false); err != nil {
					t.Fatal(err)
				}
				set := &bc.sets[s]
				set.start, set.end, set.bap = ch.start, ch.end, ch.bap
				set.value = b.mant[s]
				if aht && s == 0 {
					if blk == 0 {
						w.s = append(w.s, writeAHT(&ch, blocks, o.gaq, o.ahtX)...)
					}
					set.end = set.start // its mantissas are of block 0, written
				}
			}
			w.s = append(w.s, writeMantissas(&bc)...)
		}
		// auxdata and errorcheck: to whole words, then auxdatae, encinfo
		// and crc2.
		for (len(w.s)+18)%16 != 0 {
			w.put(0, 1)
		}
		w.put(0, 18)
		f := w.s.bytes()
		words := len(f) / 2
		if words > 2048 {
			t.Fatalf("a frame of %d words", words)
		}
		binary.BigEndian.PutUint16(f[2:], uint16(words-1)&0x7FF) // strmtyp and substreamid are 0
		setCRC(f)
		out = append(out, f...)
	}
	return out
}

// writeAHT writes the gaqmod and the mantissas of channel 0 of AHT, of the
// six blocks: the DCT across the blocks of each bin's mantissas, of the
// exponents of block 0, quantized as the high efficiency bit allocation
// ch.bap asks: by the nearest vector of the VQ tables, or GAQ of gaqmod.
func writeAHT(ch *channel, blocks []capturedBlock, gaqmod int, set func(int, *[6]float64)) bitstring {
	var w bitWriter
	w.put(gaqmod, 2)
	endbap := uint8(12)
	if gaqmod >= 2 {
		endbap = 17
	}
	// The DCT of the blocks' mantissas, the forward transform of
	// ahtCosines' inverse: X(j) = sqrt(2) R(j) / 6 * sum over m of C(m)
	// cos(pi j (2m + 1) / 12).
	var x [maxBins][6]float64
	for bin := ch.start; bin < ch.end; bin++ {
		for j := range 6 {
			r := 1.0
			if j == 0 {
				r = 1 / math.Sqrt2
			}
			var sum float64
			for m := range 6 {
				c := float64(blocks[m].mant[0][bin]) * math.Ldexp(1, int(ch.exp[bin])-int(blocks[m].exp[0][bin]))
				sum += min(1, max(-1, c)) * math.Cos(math.Pi*float64(j*(2*m+1))/12)
			}
			x[bin][j] = math.Sqrt2 * r / 6 * sum
		}
		if set != nil {
			set(bin, &x[bin])
		}
	}
	// The gains of the active bins: 2 where a mantissa is small, of gaqmod
	// 1, 4 of gaqmod 2, and 1, 2 or 4 by turns of gaqmod 3.
	var gains []int
	for bin := ch.start; bin < ch.end; bin++ {
		if b := ch.bap[bin]; b > 7 && b < endbap {
			g := 1
			switch gaqmod {
			case 1:
				if math.Abs(x[bin][0]) < 0.3 {
					g = 2
				}
			case 2:
				if math.Abs(x[bin][0]) < 0.2 {
					g = 4
				}
			case 3:
				g = 1 << (len(gains) % 3)
			}
			gains = append(gains, g)
		}
	}
	switch gaqmod {
	case 1, 2:
		for _, g := range gains {
			w.flag(g != 1)
		}
	case 3:
		for len(gains)%3 != 0 {
			gains = append(gains, 1)
		}
		for i := 0; i < len(gains); i += 3 {
			code := 0
			for _, g := range gains[i : i+3] {
				code = code*3 + map[int]int{1: 0, 2: 1, 4: 2}[g]
			}
			w.put(code, 5)
		}
	}
	k := 0
	for bin := ch.start; bin < ch.end; bin++ {
		b := ch.bap[bin]
		switch {
		case b == 0:
		case b <= 7:
			best, bestIdx := math.Inf(1), 0
			for idx, v := range vqTables[b] {
				var dist float64
				for j := range 6 {
					e := float64(v[j])/32768 - x[bin][j]
					dist += e * e
				}
				if dist < best {
					best, bestIdx = dist, idx
				}
			}
			w.put(bestIdx, ahtBits[b])
		default:
			g := 1
			if b < endbap {
				if k < len(gains) {
					g = gains[k]
				}
				k++
			}
			m := ahtBits[b]
			for j := range 6 {
				writeGAQ(&w, x[bin][j], b, m, g)
			}
		}
	}
	return w.s
}

// writeGAQ writes the GAQ code of value v, of hebap b, of m bits, of gain
// g, the code whose remapped value is nearest v, of the codes around the
// one v rounds to of each quantizer.
func writeGAQ(w *bitWriter, v float64, b uint8, m, g int) {
	frac := func(c, n int) float64 { return float64(c<<(64-n)>>(64-n)) / float64(int(1)<<(n-1)) }
	type code struct {
		bits []int // values and widths, in pairs
		val  float64
	}
	var best *code
	try := func(c code) {
		if best == nil || math.Abs(c.val-v) < math.Abs(best.val-v) {
			best = &c
		}
	}
	// near returns the codes of n bits around the one whose fraction times
	// scale is v.
	near := func(n int, scale float64) []int {
		q := int(math.Round(v / scale * float64(int(1)<<(n-1))))
		var cs []int
		for _, c := range []int{q - 1, q, q + 1} {
			if c >= -(1<<(n-1)) && c < 1<<(n-1) {
				cs = append(cs, c&(1<<n-1))
			}
		}
		return cs
	}
	if g == 1 {
		// Of its 2^m - 1 levels: the most negative code is none.
		a := float64(gaqA1[b-8]) / 32768
		for _, c := range near(m, 1+a) {
			if c != 1<<(m-1) {
				try(code{[]int{c, m}, frac(c, m) * (1 + a)})
			}
		}
	} else {
		small, large := m-1, m-1
		a, bn, bp := gaqA2[b-8], gaqB2[b-8], uint16(gaqB2Plus)
		if g == 4 {
			small, large = m-2, m
			a, bn, bp = gaqA4[b-8], gaqB4[b-8], uint16(gaqB4Plus)
		}
		tag := 1 << (small - 1)
		for _, c := range near(small, 1/float64(g)) {
			if c != tag {
				try(code{[]int{c, small}, frac(c, small) / float64(g)})
			}
		}
		af := float64(int16(a)) / 32768
		for _, bias := range []float64{float64(int16(bp)) / 32768, float64(int16(bn)) / 32768} {
			q := int(math.Round((v - bias) / (1 + af) * float64(int(1)<<(large-1))))
			for _, c := range []int{q - 1, q, q + 1} {
				if c < -(1<<(large-1)) || c >= 1<<(large-1) {
					continue
				}
				x := frac(c&(1<<large-1), large)
				bb := float64(int16(bp)) / 32768
				if x < 0 {
					bb = float64(int16(bn)) / 32768
				}
				try(code{[]int{tag, small, c & (1<<large - 1), large}, x + x*af + bb})
			}
		}
	}
	for i := 0; i < len(best.bits); i += 2 {
		w.put(best.bits[i], best.bits[i+1])
	}
}
