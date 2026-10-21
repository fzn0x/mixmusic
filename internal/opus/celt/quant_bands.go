package celt

// Port of celt/quant_bands.c, decoder side: the band energies, in log2
// units, coarse then fine.

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

var (
	// predCoef and betaCoef are the time and frequency prediction of the
	// inter coarse energy, by frame size.
	predCoef  = [4]float32{29440 / 32768., 26112 / 32768., 21248 / 32768., 16384 / 32768.}
	betaCoef  = [4]float32{30147 / 32768., 22282 / 32768., 12124 / 32768., 6554 / 32768.}
	betaIntra = float32(4915 / 32768.)

	smallEnergyICDF = []uint8{2, 1, 0}
)

// unquantCoarseEnergy is unquant_coarse_energy.
func (m *mode) unquantCoarseEnergy(start, end int, oldEBands []float32, intra bool, dec *rangedec.Decoder, c, lm int) {
	probModel := eProbModel[lm][b2i(intra)][:]
	var prev [2]float32
	var coef, beta float32
	if intra {
		coef = 0
		beta = betaIntra
	} else {
		beta = betaCoef[lm]
		coef = predCoef[lm]
	}
	budget := int32(dec.Storage() * 8)
	// Decode at a fixed coarse resolution.
	for i := start; i < end; i++ {
		for ch := 0; ch < c; ch++ {
			var qi int
			tell := int32(dec.Tell())
			switch {
			case budget-tell >= 15:
				pi := 2 * min(i, 20)
				qi = laplaceDecode(dec, uint32(probModel[pi])<<7, int(probModel[pi+1])<<6)
			case budget-tell >= 2:
				qi = dec.ICDF(smallEnergyICDF, 2)
				qi = (qi >> 1) ^ -(qi & 1)
			case budget-tell >= 1:
				qi = -b2i(dec.BitLogp(1))
			default:
				qi = -1
			}
			q := float32(qi)
			k := i + ch*m.nbEBands
			oldEBands[k] = max(-9, oldEBands[k])
			tmp := coef*oldEBands[k] + prev[ch] + q
			oldEBands[k] = tmp
			prev[ch] = prev[ch] + q - beta*q
		}
	}
}

// unquantFineEnergy is unquant_fine_energy without prev_quant, as the
// decoder calls it.
func (m *mode) unquantFineEnergy(start, end int, oldEBands []float32, extraQuant []int, dec *rangedec.Decoder, c int) {
	for i := start; i < end; i++ {
		extra := extraQuant[i]
		if extra <= 0 {
			continue
		}
		if int32(dec.Tell()+c*extra) > int32(dec.Storage())*8 {
			continue
		}
		for ch := 0; ch < c; ch++ {
			q2 := dec.Bits(uint(extra))
			offset := (float32(q2)+.5)*float32(int(1)<<(14-extra))*(1./16384) - .5
			offset *= float32(1<<14) * (1. / 16384)
			oldEBands[i+ch*m.nbEBands] += offset
		}
	}
}

// unquantEnergyFinalise is unquant_energy_finalise: it spends the bits left
// on more fine energy, by priority.
func (m *mode) unquantEnergyFinalise(start, end int, oldEBands []float32, fineQuant, finePriority []int, bitsLeft int, dec *rangedec.Decoder, c int) {
	for prio := 0; prio < 2; prio++ {
		for i := start; i < end && bitsLeft >= c; i++ {
			if fineQuant[i] >= maxFineBits || finePriority[i] != prio {
				continue
			}
			for ch := 0; ch < c; ch++ {
				q2 := dec.Bits(1)
				offset := (float32(q2) - .5) * float32(int(1)<<(14-fineQuant[i]-1)) * (1. / 16384)
				if oldEBands != nil {
					oldEBands[i+ch*m.nbEBands] += offset
				}
				bitsLeft--
			}
		}
	}
}
