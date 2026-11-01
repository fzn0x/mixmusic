package celt

// Port of celt/laplace.c: ec_laplace_decode, the Laplace-like distribution
// of the coarse energies.

import "github.com/fzn0x/mixmusic/opus/internal/rangedec"

const (
	laplaceLogMinP = 0
	laplaceMinP    = 1 << laplaceLogMinP
	// laplaceNMin is the minimum number of guaranteed representable
	// energy deltas, in one direction.
	laplaceNMin = 16
)

// laplaceGetFreq1 is the probability of +1 or -1, given that of 0, fs0.
func laplaceGetFreq1(fs0 uint32, decay int) uint32 {
	ft := 32768 - laplaceMinP*(2*laplaceNMin) - fs0
	return ft * uint32(int32(16384-decay)) >> 15
}

// laplaceDecode decodes a value whose probability of 0 is fs/32768 and
// decays geometrically by decay/16384 away from it.
func laplaceDecode(dec *rangedec.Decoder, fs uint32, decay int) int {
	val := 0
	fm := dec.DecodeBin(15)
	fl := uint32(0)
	if fm >= fs {
		val++
		fl = fs
		fs = laplaceGetFreq1(fs, decay) + laplaceMinP
		// Search the decaying part of the PDF.
		for fs > laplaceMinP && fm >= fl+2*fs {
			fs *= 2
			fl += fs
			fs = ((fs - 2*laplaceMinP) * uint32(int32(decay))) >> 15
			fs += laplaceMinP
			val++
		}
		// Everything beyond that has probability laplaceMinP.
		if fs <= laplaceMinP {
			di := (fm - fl) >> (laplaceLogMinP + 1)
			val += int(di)
			fl += 2 * di * laplaceMinP
		}
		if fm < fl+fs {
			val = -val
		} else {
			fl += fs
		}
	}
	dec.Update(fl, min(fl+fs, 32768), 32768)
	return val
}
