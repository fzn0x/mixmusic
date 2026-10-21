package celt

// Port of celt/rate.h and celt/rate.c: the allocation of bits to bands,
// decoder side.

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

const (
	bitRes                = rangedec.BitRes
	maxPseudo             = 40
	logMaxPseudo          = 6
	celtMaxPulses         = 128
	maxFineBits           = 8
	fineOffset            = 21
	qthetaOffset          = 4
	qthetaOffsetTwophase  = 16
	allocSteps            = 6
	spreadNone            = 0
	spreadLight           = 1
	spreadNormal          = 2
	spreadAggressive      = 3
	combfilterMinPeriod   = 15
	combfilterMaxPeriod   = 1024
	decodeBufferSize      = 2048 // DEC_PITCH_BUF_SIZE
	maxPeriod             = 1024
	plcPitchLagMax        = 720
	plcPitchLagMin        = 100
	celtLPCOrder          = 24
	maxBandsNb            = 21
	frameSizeMax          = 960
	overlapSize           = 120
	logMaxPseudoPlusOne   = logMaxPseudo + 1
	log2FracTableLen      = 24
	defaultNbAllocVectors = 11
)

var log2FracTable = [log2FracTableLen]uint8{
	0,
	8, 13,
	16, 19, 21, 23,
	24, 26, 27, 28, 29, 30, 31, 32,
	32, 33, 34, 34, 35, 36, 36, 37, 37,
}

func getPulses(i int) int {
	if i < 8 {
		return i
	}
	return (8 + (i & 7)) << ((i >> 3) - 1)
}

func (m *mode) bits2pulses(band, lm, bits int) int {
	lm++
	cache := m.cache.bits[m.cache.index[lm*m.nbEBands+band]:]
	lo, hi := 0, int(cache[0])
	bits--
	for i := 0; i < logMaxPseudo; i++ {
		mid := (lo + hi + 1) >> 1
		if int(cache[mid]) >= bits {
			hi = mid
		} else {
			lo = mid
		}
	}
	low := -1
	if lo != 0 {
		low = int(cache[lo])
	}
	if bits-low <= int(cache[hi])-bits {
		return lo
	}
	return hi
}

func (m *mode) pulses2bits(band, lm, pulses int) int {
	lm++
	cache := m.cache.bits[m.cache.index[lm*m.nbEBands+band]:]
	if pulses == 0 {
		return 0
	}
	return int(cache[pulses]) + 1
}

// allocation is what clt_compute_allocation decides of a frame.
type allocation struct {
	codedBands   int
	intensity    int
	dualStereo   bool
	balance      int32
	pulses       [maxBandsNb]int
	fineQuant    [maxBandsNb]int
	finePriority [maxBandsNb]int
}

func (m *mode) interpBits2pulses(a *allocation, start, end, skipStart int, bits1, bits2, thresh, cap []int,
	total int32, skipRsv, intensityRsv, dualStereoRsv, c, lm int, dec *rangedec.Decoder) int {
	bits, ebits, finePriority := a.pulses[:], a.fineQuant[:], a.finePriority[:]
	allocFloor := c << bitRes
	stereo := 0
	if c > 1 {
		stereo = 1
	}
	logM := lm << bitRes
	lo, hi := 0, 1<<allocSteps
	for i := 0; i < allocSteps; i++ {
		mid := (lo + hi) >> 1
		var psum int32
		done := false
		for j := end - 1; j >= start; j-- {
			tmp := bits1[j] + int(int32(mid)*int32(bits2[j])>>allocSteps)
			if tmp >= thresh[j] || done {
				done = true
				// Don't allocate more than we can actually use.
				psum += int32(min(tmp, cap[j]))
			} else if tmp >= allocFloor {
				psum += int32(allocFloor)
			}
		}
		if psum > total {
			hi = mid
		} else {
			lo = mid
		}
	}
	var psum int32
	done := false
	for j := end - 1; j >= start; j-- {
		tmp := bits1[j] + int(int32(lo)*int32(bits2[j])>>allocSteps)
		if tmp < thresh[j] && !done {
			if tmp >= allocFloor {
				tmp = allocFloor
			} else {
				tmp = 0
			}
		} else {
			done = true
		}
		// Don't allocate more than we can actually use.
		tmp = min(tmp, cap[j])
		bits[j] = tmp
		psum += int32(tmp)
	}
	// Decide which bands to skip, working backwards from the end.
	codedBands := end
	for ; ; codedBands-- {
		j := codedBands - 1
		// Never skip the first band, nor a band boosted by dynalloc.
		if j <= skipStart {
			// Give the bit reserved to end skipping back.
			total += int32(skipRsv)
			break
		}
		// The left-over bits this band would get, with those taken back
		// from higher, skipped bands.
		left := total - psum
		percoeff := int32(uint32(left) / uint32(int(m.eBands[codedBands])-int(m.eBands[start])))
		left -= int32(int(m.eBands[codedBands])-int(m.eBands[start])) * percoeff
		rem := max(int(left)-(int(m.eBands[j])-int(m.eBands[start])), 0)
		bandWidth := int(m.eBands[codedBands]) - int(m.eBands[j])
		bandBits := bits[j] + int(percoeff)*bandWidth + rem
		// Only code a skip decision above the threshold of this band;
		// below it, the band is skipped.
		if bandBits >= max(thresh[j], allocFloor+(1<<bitRes)) {
			if dec.BitLogp(1) {
				break
			}
			// A bit was used to skip this band.
			psum += 1 << bitRes
			bandBits -= 1 << bitRes
		}
		// Take back the bits given to this band.
		psum -= int32(bits[j] + intensityRsv)
		if intensityRsv > 0 {
			intensityRsv = int(log2FracTable[j-start])
		}
		psum += int32(intensityRsv)
		if bandBits >= allocFloor {
			// Enough for a fine energy bit per channel.
			psum += int32(allocFloor)
			bits[j] = allocFloor
		} else {
			bits[j] = 0
		}
	}
	// The intensity and dual stereo parameters.
	if intensityRsv > 0 {
		a.intensity = start + int(dec.Uint(uint32(codedBands+1-start)))
	} else {
		a.intensity = 0
	}
	if a.intensity <= start {
		total += int32(dualStereoRsv)
		dualStereoRsv = 0
	}
	if dualStereoRsv > 0 {
		a.dualStereo = dec.BitLogp(1)
	} else {
		a.dualStereo = false
	}
	// Allocate the remaining bits.
	left := total - psum
	percoeff := int32(uint32(left) / uint32(int(m.eBands[codedBands])-int(m.eBands[start])))
	left -= int32(int(m.eBands[codedBands])-int(m.eBands[start])) * percoeff
	for j := start; j < codedBands; j++ {
		bits[j] += int(percoeff) * (int(m.eBands[j+1]) - int(m.eBands[j]))
	}
	for j := start; j < codedBands; j++ {
		tmp := int(min(left, int32(int(m.eBands[j+1])-int(m.eBands[j]))))
		bits[j] += tmp
		left -= int32(tmp)
	}
	var balance int32
	j := start
	for ; j < codedBands; j++ {
		n0 := int(m.eBands[j+1]) - int(m.eBands[j])
		n := n0 << lm
		bit := int32(bits[j]) + balance
		var excess int32
		if n > 1 {
			excess = max(bit-int32(cap[j]), 0)
			bits[j] = int(bit - excess)
			// Compensate for the extra degree of freedom in stereo.
			den := c * n
			if c == 2 && n > 2 && !a.dualStereo && j < a.intensity {
				den++
			}
			nclogn := den * (int(m.logN[j]) + logM)
			// Offset the fine bits by log2(N)/2 + FINE_OFFSET from their
			// fair share of total/N.
			offset := (nclogn >> 1) - den*fineOffset
			// N=2 is the only point off the curve.
			if n == 2 {
				offset += den << bitRes >> 2
			}
			// The offset of the second and third fine energy bit.
			if bits[j]+offset < den*2<<bitRes {
				offset += nclogn >> 2
			} else if bits[j]+offset < den*3<<bitRes {
				offset += nclogn >> 3
			}
			// Divide with rounding.
			ebits[j] = max(0, bits[j]+offset+(den<<(bitRes-1)))
			ebits[j] = int(uint32(ebits[j])/uint32(den)) >> bitRes
			// Make sure not to bust.
			if c*ebits[j] > bits[j]>>bitRes {
				ebits[j] = bits[j] >> stereo >> bitRes
			}
			// More is useless: it is about as far as PVQ can go.
			ebits[j] = min(ebits[j], maxFineBits)
			// Rounded down or capped: a candidate for the final pass.
			finePriority[j] = b2i(ebits[j]*(den<<bitRes) >= bits[j]+offset)
			// The rest goes to PVQ.
			bits[j] -= c * ebits[j] << bitRes
		} else {
			// For N=1, all bits go to fine energy but a sign bit.
			excess = max(0, bit-int32(c<<bitRes))
			bits[j] = int(bit - excess)
			ebits[j] = 0
			finePriority[j] = 1
		}
		// Fine energy can't use the rebalancing of quant_all_bands(): do it
		// here.
		if excess > 0 {
			extraFine := min(int(excess>>(stereo+bitRes)), maxFineBits-ebits[j])
			ebits[j] += extraFine
			extraBits := extraFine * c << bitRes
			finePriority[j] = b2i(int32(extraBits) >= excess-balance)
			excess -= int32(extraBits)
		}
		balance = excess
	}
	// Bits over the cap go to the rebalancing of quant_all_bands().
	a.balance = balance
	// Skipped bands use all their bits for fine energy.
	for ; j < end; j++ {
		ebits[j] = bits[j] >> stereo >> bitRes
		bits[j] = 0
		finePriority[j] = b2i(ebits[j] < 1)
	}
	return codedBands
}

// computeAllocation is clt_compute_allocation for the decoder.
func (m *mode) computeAllocation(a *allocation, start, end int, offsets, cap []int, allocTrim int,
	total int32, c, lm int, dec *rangedec.Decoder) {
	total = max(total, 0)
	nb := m.nbEBands
	skipStart := start
	// A bit to signal the end of skipped bands.
	skipRsv := 0
	if total >= 1<<bitRes {
		skipRsv = 1 << bitRes
	}
	total -= int32(skipRsv)
	// Bits for the intensity and dual stereo parameters.
	intensityRsv, dualStereoRsv := 0, 0
	if c == 2 {
		intensityRsv = int(log2FracTable[end-start])
		if int32(intensityRsv) > total {
			intensityRsv = 0
		} else {
			total -= int32(intensityRsv)
			if total >= 1<<bitRes {
				dualStereoRsv = 1 << bitRes
			}
			total -= int32(dualStereoRsv)
		}
	}
	var bits1, bits2, thresh, trimOffset [maxBandsNb]int
	for j := start; j < end; j++ {
		w := int(m.eBands[j+1]) - int(m.eBands[j])
		// Below this, no PVQ bits for sure.
		thresh[j] = max(c<<bitRes, (3*w<<lm<<bitRes)>>4)
		// The tilt of the allocation curve.
		trimOffset[j] = c * w * (allocTrim - 5 - lm) * (end - j - 1) * (1 << (lm + bitRes)) >> 6
		// Single-coefficient bands gain more from a coarse value each.
		if w<<lm == 1 {
			trimOffset[j] -= c << bitRes
		}
	}
	lo, hi := 1, m.nbAllocVectors-1
	for {
		done := false
		psum := 0
		mid := (lo + hi) >> 1
		for j := end - 1; j >= start; j-- {
			n := int(m.eBands[j+1]) - int(m.eBands[j])
			bitsj := c * n * int(m.allocVectors[mid*nb+j]) << lm >> 2
			if bitsj > 0 {
				bitsj = max(0, bitsj+trimOffset[j])
			}
			bitsj += offsets[j]
			if bitsj >= thresh[j] || done {
				done = true
				psum += min(bitsj, cap[j])
			} else if bitsj >= c<<bitRes {
				psum += c << bitRes
			}
		}
		if int32(psum) > total {
			hi = mid - 1
		} else {
			lo = mid + 1
		}
		if lo > hi {
			break
		}
	}
	hi = lo
	lo--
	for j := start; j < end; j++ {
		n := int(m.eBands[j+1]) - int(m.eBands[j])
		bits1j := c * n * int(m.allocVectors[lo*nb+j]) << lm >> 2
		var bits2j int
		if hi >= m.nbAllocVectors {
			bits2j = cap[j]
		} else {
			bits2j = c * n * int(m.allocVectors[hi*nb+j]) << lm >> 2
		}
		if bits1j > 0 {
			bits1j = max(0, bits1j+trimOffset[j])
		}
		if bits2j > 0 {
			bits2j = max(0, bits2j+trimOffset[j])
		}
		if lo > 0 {
			bits1j += offsets[j]
		}
		bits2j += offsets[j]
		if offsets[j] > 0 {
			skipStart = j
		}
		bits2j = max(0, bits2j-bits1j)
		bits1[j] = bits1j
		bits2[j] = bits2j
	}
	a.codedBands = m.interpBits2pulses(a, start, end, skipStart, bits1[:], bits2[:], thresh[:], cap,
		total, skipRsv, intensityRsv, dualStereoRsv, c, lm, dec)
}
