package ac3

import "fmt"

// exponents reads groups of grouped exponents of a channel, of strategy
// strategy, which follow the absolute exponent absexp, into ch.exp from
// bin first on, as section 7.1.3 decodes them.
func (d *Decoder) exponents(ch *channel, absexp, groups, strategy, first int) error {
	r := &d.r
	size := 1 << (strategy - 1) // the exponents of a group's value
	prev := absexp
	i := first
	if i+3*size*groups > maxBins {
		return fmt.Errorf("%w: %d exponent groups", ErrFormat, groups)
	}
	for range groups {
		g := r.read(7)
		if g > 124 {
			return fmt.Errorf("%w: exponent group %d", ErrFormat, g)
		}
		for _, m := range [3]int{g / 25, g % 25 / 5, g % 5} {
			prev += m - 2
			if prev < 0 || prev > 24 {
				return fmt.Errorf("%w: exponent %d", ErrFormat, prev)
			}
			for range size {
				ch.exp[i] = uint8(prev)
				i++
			}
		}
	}
	return nil
}

// allocate computes the bit allocation of a channel, of section 7.2.2,
// from its exponents of start to end: into ch.bap. zero is the special
// case, of all SNR offsets 0, which allocates no bits.
func (d *Decoder) allocate(ch *channel, zero bool) error {
	start, end := ch.start, ch.end
	if zero {
		clear(ch.bap[start:end])
		return nil
	}
	sdecay := slowDecay[d.sdcycod]
	fdecay := fastDecay[d.fdcycod]
	sgain := slowGain[d.sgaincod]
	dbknee := dbPerBit[d.dbpbcod]
	floor := floorTab[d.floorcod]
	fgain := fastGain[ch.fgaincod]
	snroffset := ((d.csnroffst-15)<<4 + ch.fsnroffst) << 2

	// The exponents mapped into the PSD, and integrated in each band.
	psd := &d.psd
	for bin := start; bin < end; bin++ {
		psd[bin] = 3072 - int(ch.exp[bin])<<7
	}
	bndpsd := &d.bndpsd
	j := start
	for k := int(maskTab[start]); ; k++ {
		last := min(bandStart[k+1], end)
		v := psd[j]
		for j++; j < last; j++ {
			v = logAddition(v, psd[j])
		}
		bndpsd[k] = v
		if end <= last {
			break
		}
	}

	// The excitation function.
	excite := &d.excite
	bndstrt := int(maskTab[start])
	bndend := int(maskTab[end-1]) + 1
	var lowcomp, fastleak, slowleak, begin int
	if bndstrt == 0 {
		// A full-bandwidth channel, or the LFE, whose bands end at 7: its
		// last band has no next to compensate with.
		lfe := bndend == 7
		lowcomp = lowCompensation(lowcomp, bndpsd[0], bndpsd[1], 0)
		excite[0] = bndpsd[0] - fgain - lowcomp
		lowcomp = lowCompensation(lowcomp, bndpsd[1], bndpsd[2], 1)
		excite[1] = bndpsd[1] - fgain - lowcomp
		begin = 7
		for bin := 2; bin < 7; bin++ {
			if !lfe || bin != 6 {
				lowcomp = lowCompensation(lowcomp, bndpsd[bin], bndpsd[bin+1], bin)
			}
			fastleak = bndpsd[bin] - fgain
			slowleak = bndpsd[bin] - sgain
			excite[bin] = fastleak - lowcomp
			if (!lfe || bin != 6) && bndpsd[bin] <= bndpsd[bin+1] {
				begin = bin + 1
				break
			}
		}
		for bin := begin; bin < min(bndend, 22); bin++ {
			if !lfe || bin != 6 {
				lowcomp = lowCompensation(lowcomp, bndpsd[bin], bndpsd[bin+1], bin)
			}
			fastleak = max(fastleak-fdecay, bndpsd[bin]-fgain)
			slowleak = max(slowleak-sdecay, bndpsd[bin]-sgain)
			excite[bin] = max(fastleak-lowcomp, slowleak)
		}
		begin = 22
	} else {
		begin = bndstrt
		fastleak = d.cplfleak<<8 + 768
		slowleak = d.cplsleak<<8 + 768
	}
	for bin := begin; bin < bndend; bin++ {
		fastleak = max(fastleak-fdecay, bndpsd[bin]-fgain)
		slowleak = max(slowleak-sdecay, bndpsd[bin]-sgain)
		excite[bin] = max(fastleak, slowleak)
	}

	// The masking curve.
	mask := &d.mask
	hth := &hearing[d.h.fscod]
	for bin := bndstrt; bin < bndend; bin++ {
		if bndpsd[bin] < dbknee {
			excite[bin] += (dbknee - bndpsd[bin]) >> 2
		}
		mask[bin] = max(excite[bin], hth[bin])
	}

	// The delta bit allocation. Its bands are counted from the channel's
	// first: band 0 of a full-bandwidth channel, the first of the coupling
	// channel's, as section 5.4.3.51 counts "the bit allocation bands of
	// the coupling channel".
	if ch.delta {
		band := bndstrt
		for s := range ch.deltnseg + 1 {
			band += ch.deltoffst[s]
			delta := (ch.deltba[s] - 4) << 7
			if ch.deltba[s] >= 4 {
				delta = (ch.deltba[s] - 3) << 7
			}
			if band+ch.deltlen[s] > 50 {
				return fmt.Errorf("%w: delta bit allocation past band 50", ErrFormat)
			}
			for range ch.deltlen[s] {
				mask[band] += delta
				band++
			}
		}
	}

	// The bit allocation pointers.
	i := start
	for j := int(maskTab[start]); ; j++ {
		last := min(bandStart[j+1], end)
		m := max(mask[j]-snroffset-floor, 0)&0x1fe0 + floor
		for ; i < last; i++ {
			a := (psd[i] - m) >> 5
			ch.bap[i] = bapTab[min(63, max(0, a))]
		}
		if end <= last {
			break
		}
	}
	return nil
}

// logAddition is logadd of section 7.2.2.3.
func logAddition(a, b int) int {
	c := a - b
	addr := min(abs(c)>>1, 255)
	if c >= 0 {
		return a + logAdd[addr]
	}
	return b + logAdd[addr]
}

// lowCompensation is calc_lowcomp of section 7.2.2.4.
func lowCompensation(a, b0, b1, bin int) int {
	switch {
	case bin < 7:
		if b0+256 == b1 {
			a = 384
		} else if b0 > b1 {
			a = max(0, a-64)
		}
	case bin < 20:
		if b0+256 == b1 {
			a = 320
		} else if b0 > b1 {
			a = max(0, a-64)
		}
	default:
		a = max(0, a-128)
	}
	return a
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
