package ac3

import (
	"fmt"
	"math"
)

// Spectral extension, of E-AC-3: section E3.6 of A/52, E.2.6 of ETSI TS
// 102 366. It makes a channel's high frequencies of copies of its low
// ones, blended with noise, at the energies of the stream's coordinates.

// maxSpxBands is the spectral extension sub-bands, and so the most bands.
const maxSpxBands = 17

// spxBand returns the first bin of spectral extension sub-band s, of
// Table E.2.11: of 25 to 228 in sub-bands of 12, and 229 for the end of
// the last.
func spxBand(s int) int { return 25 + 12*s }

// defaultSpxBands is defspxbndstrc, of Table E2.11: whether each sub-band
// continues the band before it.
var defaultSpxBands = [maxSpxBands + 1]bool{8: true, 10: true, 12: true, 14: true, 16: true}

// defaultCplBands is defcplbndstrc, of Table E2.12: whether each coupling
// sub-band continues the band before it.
var defaultCplBands = [18]bool{8: true, 10: true, 11: true, 13: true, 14: true, 15: true, 16: true, 17: true}

// spxAttenuation is spxattentab, of Table E.2.12: the gains of the notch
// filter of each spxattencod, of the two bins before the border and the
// border's own. Its values are 2^(-(i+1)(code+1)/15), as TestSpxAttenuation
// checks against the table.
var spxAttenuation = func() (t [32][3]float32) {
	for code := range t {
		for i := range t[code] {
			t[code][i] = float32(math.Pow(2, -float64((i+1)*(code+1))/15))
		}
	}
	return t
}()

// spectralExtension is the spectral extension of a frame's blocks.
type spectralExtension struct {
	inu     bool
	strtf   int // spxstrtf
	begf    int // spxbegf, the code
	begin   int // spx_begin_subbnd
	end     int // spx_end_subbnd
	bndstrc [maxSpxBands + 1]bool
	nbands  int
	size    [maxSpxBands]int // of each band, in bins
	// Of each channel: whether it uses spectral extension, whether its
	// coordinates come next (the first of a frame always do), and its
	// coordinates and blending factors of each band.
	in           [maxFull]bool
	co           [maxFull][maxSpxBands]float32
	nblend       [maxFull][maxSpxBands]float32
	sblend       [maxFull][maxSpxBands]float32
	rms          [maxSpxBands]float32
	wrap         [maxSpxBands]bool
	noise        uint32 // the state of the generator of its noise
	inspxAtStart int    // for tests: where spxstre is, in bits
}

// parseSpectralExtension reads the spectral extension strategy and the
// coordinates of block blk of an E-AC-3 frame.
func (d *Decoder) parseSpectralExtension(blk int) error {
	r := &d.r
	h := &d.h
	f := &d.frm
	x := &d.spx
	nf := h.nfchans
	x.inspxAtStart = r.pos
	if blk == 0 || r.flag() { // spxstre
		x.inu = r.flag()
		if x.inu {
			for c := range nf {
				x.in[c] = h.acmod == ModeMono || r.flag()
			}
			x.strtf = r.read(2)
			x.begf = r.read(3)
			endf := r.read(3)
			if x.begf < 6 {
				x.begin = x.begf + 2
			} else {
				x.begin = x.begf*2 - 3
			}
			if endf < 3 {
				x.end = endf + 5
			} else {
				x.end = endf*2 + 3
			}
			if x.begin >= x.end || x.strtf >= x.begin {
				return fmt.Errorf("%w: spectral extension of sub-bands %d to %d, copied from %d", ErrFormat, x.begin, x.end, x.strtf)
			}
			if r.flag() { // spxbndstrce
				for s := x.begin + 1; s < x.end; s++ {
					x.bndstrc[s] = r.flag()
				}
			}
			x.nbands = 0
			for s := x.begin; s < x.end; s++ {
				if s == x.begin || !x.bndstrc[s] {
					x.size[x.nbands] = 12
					x.nbands++
				} else {
					x.size[x.nbands-1] += 12
				}
			}
		} else {
			for c := range nf {
				x.in[c] = false
				f.firstspxcos[c] = true
			}
		}
	}
	d.spxinu = x.inu
	d.spxbegf = x.begf
	d.spxBegin = x.begin
	for c := range nf {
		d.chinspx[c] = x.inu && x.in[c]
	}
	if !x.inu {
		return nil
	}
	for c := range nf {
		if !x.in[c] {
			f.firstspxcos[c] = true
			continue
		}
		spxcoe := f.firstspxcos[c]
		f.firstspxcos[c] = false
		if !spxcoe && !r.flag() { // spxcoe
			continue
		}
		noffset := float64(r.read(5)) / 32 // spxblnd
		mstr := 3 * r.read(2)              // mstrspxco
		mant := spxBand(x.begin)
		for b := range x.nbands {
			exp, m := r.read(4), r.read(2)
			var co float32
			if exp == 15 {
				co = float32(m) / 4
			} else {
				co = float32(m+4) / 8
			}
			x.co[c][b] = co * pow2(-(exp + mstr))
			size := x.size[b]
			nratio := (float64(mant)+0.5*float64(size))/float64(spxBand(x.end)) - noffset
			nratio = min(1, max(0, nratio))
			x.nblend[c][b] = float32(math.Sqrt(nratio))
			x.sblend[c][b] = float32(math.Sqrt(1 - nratio))
			mant += size
		}
	}
	return nil
}

// extend synthesizes the high frequencies of the channels of the block in
// spectral extension, of section E.2.6.4: their coefficients from the
// start of the extension on are those of a lower range, copied, blended
// with noise, and scaled.
func (d *Decoder) extend() {
	x := &d.spx
	f := &d.frm
	copyStart, copyEnd := spxBand(x.strtf), spxBand(x.begin)
	for c := range d.h.nfchans {
		if !x.in[c] {
			continue
		}
		tc := &d.ch[c].coef
		// The translation, and the RMS energy of each band.
		from, to := copyStart, copyEnd
		for b := range x.nbands {
			size := x.size[b]
			x.wrap[b] = from+size > copyEnd
			if x.wrap[b] {
				from = copyStart
			}
			var energy float32
			for range size {
				if from == copyEnd {
					from = copyStart
				}
				v := tc[from]
				tc[to] = v
				energy += v * v
				from++
				to++
			}
			x.rms[b] = float32(math.Sqrt(float64(energy / float32(size))))
		}
		// The notch filter at the border of the extension, and where the
		// copies wrap.
		if code := f.spxattencod[c]; code >= 0 {
			a := &spxAttenuation[code]
			notch := func(border int) {
				tc[border-2] *= a[0]
				tc[border-1] *= a[1]
				tc[border] *= a[2]
				tc[border+1] *= a[1]
				tc[border+2] *= a[0]
			}
			border := copyEnd
			notch(border)
			for b := 1; b < x.nbands; b++ {
				border += x.size[b-1]
				if x.wrap[b] {
					notch(border)
				}
			}
		}
		// The noise blending, and the scaling by the coordinates.
		bin := copyEnd
		for b := range x.nbands {
			nscale := x.rms[b] * x.nblend[c][b]
			sscale := x.sblend[c][b]
			co := x.co[c][b] * 32
			for range x.size[b] {
				tc[bin] = (tc[bin]*sscale + d.noise()*nscale) * co
				bin++
			}
		}
	}
}

// noise returns the next value of the noise of spectral extension: of
// zero mean and unit variance, uniform over [-sqrt(3), sqrt(3)).
func (d *Decoder) noise() float32 {
	x := &d.spx
	x.noise = x.noise*1664525 + 1013904223
	return float32(int32(x.noise)) * (1.7320508 / (1 << 31))
}
