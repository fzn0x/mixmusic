package celt

// Port of celt/bands.c, decoder side: the bands' shapes, split
// recursively, folded from lower bands when they get no pulses, and their
// stereo.

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

var (
	orderyTable = [...]int{
		1, 0,
		3, 0, 2, 1,
		7, 0, 4, 3, 6, 1, 5, 2,
		15, 0, 8, 7, 12, 3, 11, 4, 14, 1, 9, 6, 13, 2, 10, 5,
	}
	bitInterleaveTable   = [16]uint8{0, 1, 1, 1, 2, 3, 3, 3, 2, 3, 3, 3, 2, 3, 3, 3}
	bitDeinterleaveTable = [16]uint8{
		0x00, 0x03, 0x0C, 0x0F, 0x30, 0x33, 0x3C, 0x3F,
		0xC0, 0xC3, 0xCC, 0xCF, 0xF0, 0xF3, 0xFC, 0xFF,
	}
	exp2Table8 = [8]int{16384, 17866, 19483, 21247, 23170, 25267, 27554, 30048}
)

// fracMul16 is FRAC_MUL16: a Q15 product of 16-bit values.
func fracMul16(a, b int) int { return int((16384 + int32(int16(a))*int32(int16(b))) >> 15) }

// bitexactCos is bitexact_cos: a cosine that is the same on every
// platform, as the bit allocation depends on it.
func bitexactCos(x int16) int16 {
	tmp := (4096 + int32(x)*int32(x)) >> 13
	x2 := int(int16(tmp))
	x2 = int(int16(32767 - x2 + fracMul16(x2, -7651+fracMul16(x2, 8277+fracMul16(-626, x2)))))
	return int16(1 + x2)
}

// bitexactLog2tan is bitexact_log2tan.
func bitexactLog2tan(isin, icos int) int {
	lc := ilog(uint32(icos))
	ls := ilog(uint32(isin))
	icos <<= 15 - lc
	isin <<= 15 - ls
	return (ls-lc)*(1<<11) +
		fracMul16(isin, fracMul16(isin, -2597)+7932) -
		fracMul16(icos, fracMul16(icos, -2597)+7932)
}

// denormaliseBands is denormalise_bands: the MDCT of a channel, freq, from
// the shapes x and the energies bandLogE of its bands.
func (m *mode) denormaliseBands(x, freq, bandLogE []float32, start, end, mm, downsample int, silence bool) {
	eBands := m.eBands
	n := mm * m.shortMdctSize
	bound := mm * int(eBands[end])
	if downsample != 1 {
		bound = min(bound, n/downsample)
	}
	if silence {
		bound = 0
		start, end = 0, 0
	}
	f := 0
	xi := mm * int(eBands[start])
	if start != 0 {
		for i := 0; i < mm*int(eBands[start]); i++ {
			freq[f] = 0
			f++
		}
	} else {
		f += mm * int(eBands[start])
	}
	for i := start; i < end; i++ {
		j := mm * int(eBands[i])
		bandEnd := mm * int(eBands[i+1])
		lg := bandLogE[i] + eMeans[i]
		g := celtExp2(min(32, lg))
		for {
			freq[f] = x[xi] * g
			f++
			xi++
			j++
			if j >= bandEnd {
				break
			}
		}
	}
	clear(freq[bound:n])
}

// antiCollapse is anti_collapse: it fills the short MDCTs of transients
// that got no pulses with noise, so their energy doesn't collapse.
func (m *mode) antiCollapse(x []float32, collapseMasks []uint8, lm, c, size, start, end int,
	logE, prev1LogE, prev2LogE []float32, pulses []int, seed uint32) {
	for i := start; i < end; i++ {
		n0 := int(m.eBands[i+1]) - int(m.eBands[i])
		// The depth in 1/8 bits.
		depth := int(uint32(1+pulses[i])/uint32(n0)) >> lm
		thresh := .5 * celtExp2(-.125*float32(depth))
		sqrt1 := celtRsqrt(float32(n0 << lm))
		for ch := 0; ch < c; ch++ {
			prev1 := prev1LogE[ch*m.nbEBands+i]
			prev2 := prev2LogE[ch*m.nbEBands+i]
			if c == 1 {
				prev1 = max(prev1, prev1LogE[m.nbEBands+i])
				prev2 = max(prev2, prev2LogE[m.nbEBands+i])
			}
			ediff := logE[ch*m.nbEBands+i] - min(prev1, prev2)
			ediff = max(0, ediff)
			// Short blocks have less energy than long ones: r needs 2 or
			// 2*sqrt(2), by LM.
			r := 2 * celtExp2(-ediff)
			if lm == 3 {
				r *= 1.41421356
			}
			r = min(thresh, r)
			r = r * sqrt1
			xo := ch*size + int(m.eBands[i])<<lm
			renormalize := false
			for k := 0; k < 1<<lm; k++ {
				// A collapse: fill with noise.
				if collapseMasks[i*c+ch]&(1<<k) == 0 {
					for j := 0; j < n0; j++ {
						seed = celtLCGRand(seed)
						if seed&0x8000 != 0 {
							x[xo+(j<<lm)+k] = r
						} else {
							x[xo+(j<<lm)+k] = -r
						}
					}
					renormalize = true
				}
			}
			// Energy was added: normalize again.
			if renormalize {
				renormaliseVector(x[xo:], n0<<lm, q31One)
			}
		}
	}
}

func stereoMerge(x, y []float32, mid float32, n int) {
	// The norms of X+Y and X-Y, as |X|^2 + |Y|^2 +/- sum(xy).
	xp := innerProd(y, x, n)
	side := innerProd(y, y, n)
	// Compensate for the mid's normalization.
	xp = mid * xp
	el := mid*mid + side - 2*xp
	er := mid*mid + side + 2*xp
	if er < 6e-4 || el < 6e-4 {
		copy(y[:n], x[:n])
		return
	}
	lgain := celtRsqrt(el)
	rgain := celtRsqrt(er)
	for j := 0; j < n; j++ {
		// Apply the mid's scaling; the side's is done.
		l := mid * x[j]
		r := y[j]
		x[j] = lgain * (l - r)
		y[j] = rgain * (l + r)
	}
}

func deinterleaveHadamard(x, tmp []float32, n0, stride int, hadamard bool) {
	n := n0 * stride
	tmp = tmp[:n]
	if hadamard {
		ordery := orderyTable[stride-2:]
		for i := 0; i < stride; i++ {
			for j := 0; j < n0; j++ {
				tmp[ordery[i]*n0+j] = x[j*stride+i]
			}
		}
	} else {
		for i := 0; i < stride; i++ {
			for j := 0; j < n0; j++ {
				tmp[i*n0+j] = x[j*stride+i]
			}
		}
	}
	copy(x[:n], tmp)
}

func interleaveHadamard(x, tmp []float32, n0, stride int, hadamard bool) {
	n := n0 * stride
	tmp = tmp[:n]
	if hadamard {
		ordery := orderyTable[stride-2:]
		for i := 0; i < stride; i++ {
			for j := 0; j < n0; j++ {
				tmp[j*stride+i] = x[ordery[i]*n0+j]
			}
		}
	} else {
		for i := 0; i < stride; i++ {
			for j := 0; j < n0; j++ {
				tmp[j*stride+i] = x[i*n0+j]
			}
		}
	}
	copy(x[:n], tmp)
}

func haar1(x []float32, n0, stride int) {
	n0 >>= 1
	for i := 0; i < stride; i++ {
		for j := 0; j < n0; j++ {
			tmp1 := .70710678 * x[stride*2*j+i]
			tmp2 := .70710678 * x[stride*(2*j+1)+i]
			x[stride*2*j+i] = tmp1 + tmp2
			x[stride*(2*j+1)+i] = tmp1 - tmp2
		}
	}
}

func computeQn(n, b, offset, pulseCap int, stereo bool) int {
	n2 := 2*n - 1
	if stereo && n == 2 {
		n2--
	}
	// The upper limit leaves enough bits, in a stereo split with
	// itheta==16384, for a pulse in the side, which would else collapse.
	qb := (b + n2*offset) / n2
	qb = min(b-pulseCap-(4<<bitRes), qb)
	qb = min(8<<bitRes, qb)
	if qb < (1<<bitRes)>>1 {
		return 1
	}
	qn := exp2Table8[qb&0x7] >> (14 - (qb >> bitRes))
	return (qn + 1) >> 1 << 1
}

// bandCtx is band_ctx, for the decoder.
type bandCtx struct {
	m               *mode
	i               int
	intensity       int
	spread          int
	tfChange        int
	dec             *rangedec.Decoder
	remainingBits   int32
	seed            uint32
	disableInv      bool
	avoidSplitNoise bool
	iy              []int
	tmp             []float32
}

// splitCtx is split_ctx.
type splitCtx struct {
	inv    bool
	imid   int
	iside  int
	delta  int
	itheta int
	qalloc int
}

// computeTheta is compute_theta for the decoder: the split of a band in
// two halves or in mid and side, by the angle theta.
func (ctx *bandCtx) computeTheta(sctx *splitCtx, n int, b *int, bb, b0, lm int, stereo bool, fill *int) {
	m := ctx.m
	i := ctx.i
	dec := ctx.dec
	// The resolution of the split parameter theta.
	pulseCap := int(m.logN[i]) + lm*(1<<bitRes)
	offset := pulseCap>>1 - qthetaOffset
	if stereo && n == 2 {
		offset = pulseCap>>1 - qthetaOffsetTwophase
	}
	qn := computeQn(n, *b, offset, pulseCap, stereo)
	if stereo && i >= ctx.intensity {
		qn = 1
	}
	tell := dec.TellFrac()
	itheta := 0
	inv := false
	if qn != 1 {
		// A uniform pdf for the time split, a step for stereo, and a
		// triangle for the rest.
		switch {
		case stereo && n > 2:
			p0 := 3
			x0 := qn / 2
			ft := p0*(x0+1) + x0
			fs := int(dec.Decode(uint32(ft)))
			var x int
			if fs < (x0+1)*p0 {
				x = fs / p0
			} else {
				x = x0 + 1 + (fs - (x0+1)*p0)
			}
			var fl, fh int
			if x <= x0 {
				fl, fh = p0*x, p0*(x+1)
			} else {
				fl, fh = (x-1-x0)+(x0+1)*p0, (x-x0)+(x0+1)*p0
			}
			dec.Update(uint32(fl), uint32(fh), uint32(ft))
			itheta = x
		case b0 > 1 || stereo:
			itheta = int(dec.Uint(uint32(qn + 1)))
		default:
			ft := ((qn >> 1) + 1) * ((qn >> 1) + 1)
			fm := int(dec.Decode(uint32(ft)))
			var fl, fs int
			if fm < ((qn>>1)*((qn>>1)+1))>>1 {
				itheta = int((isqrt32(8*uint32(fm)+1) - 1) >> 1)
				fs = itheta + 1
				fl = itheta * (itheta + 1) >> 1
			} else {
				itheta = int((uint32(2*(qn+1)) - isqrt32(8*uint32(ft-fm-1)+1)) >> 1)
				fs = qn + 1 - itheta
				fl = ft - ((qn + 1 - itheta) * (qn + 2 - itheta) >> 1)
			}
			dec.Update(uint32(fl), uint32(fl+fs), uint32(ft))
		}
		itheta = int(uint32(itheta*16384) / uint32(qn))
	} else if stereo {
		if *b > 2<<bitRes && ctx.remainingBits > 2<<bitRes {
			inv = dec.BitLogp(2)
		}
		// Avoid trouble with downmixing.
		if ctx.disableInv {
			inv = false
		}
		itheta = 0
	}
	qalloc := int(dec.TellFrac() - tell)
	*b -= qalloc
	var imid, iside, delta int
	switch itheta {
	case 0:
		imid, iside = 32767, 0
		*fill &= 1<<bb - 1
		delta = -16384
	case 16384:
		imid, iside = 0, 32767
		*fill &= (1<<bb - 1) << bb
		delta = 16384
	default:
		imid = int(bitexactCos(int16(itheta)))
		iside = int(bitexactCos(int16(16384 - itheta)))
		// The mid vs side allocation that minimizes the squared error.
		delta = fracMul16((n-1)<<7, bitexactLog2tan(iside, imid))
	}
	*sctx = splitCtx{inv: inv, imid: imid, iside: iside, delta: delta, itheta: itheta, qalloc: qalloc}
}

// quantBandN1 is quant_band_n1: a band of one sample, its sign.
func (ctx *bandCtx) quantBandN1(x, y, lowbandOut []float32) uint32 {
	stereo := y != nil
	xs := x
	for c := 0; c < 1+b2i(stereo); c++ {
		sign := uint32(0)
		if ctx.remainingBits >= 1<<bitRes {
			sign = ctx.dec.Bits(1)
			ctx.remainingBits -= 1 << bitRes
		}
		if sign != 0 {
			xs[0] = -normScale
		} else {
			xs[0] = normScale
		}
		xs = y
	}
	if lowbandOut != nil {
		lowbandOut[0] = x[0]
	}
	return 1
}

// quantPartition is quant_partition: a mono partition, split in two with
// their energy difference while it needs more bits than PVQ codes.
func (ctx *bandCtx) quantPartition(x []float32, n, b, bb int, lowband []float32, lm int, gain float32, fill int) uint32 {
	m := ctx.m
	i := ctx.i
	b0 := bb
	var cm uint32
	// Split the band in two when it needs 1.5 bits more than it can code.
	cache := m.cache.bits[m.cache.index[(lm+1)*m.nbEBands+i]:]
	if lm != -1 && b > int(cache[cache[0]])+12 && n > 2 {
		n >>= 1
		y := x[n:]
		lm--
		if bb == 1 {
			fill = (fill & 1) | (fill << 1)
		}
		bb = (bb + 1) >> 1
		var sctx splitCtx
		ctx.computeTheta(&sctx, n, &b, bb, b0, lm, false, &fill)
		delta := sctx.delta
		itheta := sctx.itheta
		mid := (1. / 32768) * float32(sctx.imid)
		side := (1. / 32768) * float32(sctx.iside)
		// Give low-energy MDCTs more bits than they would deserve.
		if b0 > 1 && itheta&0x3fff != 0 {
			if itheta > 8192 {
				// A rough approximation of pre-echo masking.
				delta -= delta >> (4 - lm)
			} else {
				// A forward-masking slope of 1.5 dB per 10 ms.
				delta = min(0, delta+(n<<bitRes>>(5-lm)))
			}
		}
		mbits := max(0, min(b, (b-delta)/2))
		sbits := b - mbits
		ctx.remainingBits -= int32(sctx.qalloc)
		var nextLowband2 []float32
		if lowband != nil {
			nextLowband2 = lowband[n:]
		}
		rebalance := ctx.remainingBits
		if mbits >= sbits {
			cm = ctx.quantPartition(x, n, mbits, bb, lowband, lm, gain*mid, fill)
			rebalance = int32(mbits) - (rebalance - ctx.remainingBits)
			if rebalance > 3<<bitRes && itheta != 0 {
				sbits += int(rebalance) - (3 << bitRes)
			}
			cm |= ctx.quantPartition(y, n, sbits, bb, nextLowband2, lm, gain*side, fill>>bb) << (b0 >> 1)
		} else {
			cm = ctx.quantPartition(y, n, sbits, bb, nextLowband2, lm, gain*side, fill>>bb) << (b0 >> 1)
			rebalance = int32(sbits) - (rebalance - ctx.remainingBits)
			if rebalance > 3<<bitRes && itheta != 16384 {
				mbits += int(rebalance) - (3 << bitRes)
			}
			cm |= ctx.quantPartition(x, n, mbits, bb, lowband, lm, gain*mid, fill)
		}
		return cm
	}
	// The basic case, without a split.
	q := m.bits2pulses(i, lm, b)
	currBits := m.pulses2bits(i, lm, q)
	ctx.remainingBits -= int32(currBits)
	// Never bust the budget.
	for ctx.remainingBits < 0 && q > 0 {
		ctx.remainingBits += int32(currBits)
		q--
		currBits = m.pulses2bits(i, lm, q)
		ctx.remainingBits -= int32(currBits)
	}
	if q != 0 {
		return algUnquant(x, n, getPulses(q), ctx.spread, bb, ctx.dec, gain, ctx.iy)
	}
	// No pulse: fill the band anyway.
	cmMask := uint32(1)<<bb - 1
	fill &= int(cmMask)
	if fill == 0 {
		clear(x[:n])
		return cm
	}
	if lowband == nil {
		// Noise.
		for j := 0; j < n; j++ {
			ctx.seed = celtLCGRand(ctx.seed)
			x[j] = float32(int32(ctx.seed) >> 20)
		}
		cm = cmMask
	} else {
		// The folded spectrum, with noise about 48 dB below it.
		for j := 0; j < n; j++ {
			ctx.seed = celtLCGRand(ctx.seed)
			tmp := float32(1.0 / 256)
			if ctx.seed&0x8000 == 0 {
				tmp = -tmp
			}
			x[j] = lowband[j] + tmp
		}
		cm = uint32(fill)
	}
	renormaliseVector(x, n, gain)
	return cm
}

// quantBand is quant_band: a band of one channel, with its time-frequency
// changes.
func (ctx *bandCtx) quantBand(x []float32, n, b, bb int, lowband []float32, lm int, lowbandOut []float32,
	gain float32, lowbandScratch []float32, fill int) uint32 {
	n0 := n
	nB := n
	b0 := bb
	timeDivide := 0
	recombine := 0
	longBlocks := b0 == 1
	tfChange := ctx.tfChange
	nB = int(uint32(nB) / uint32(bb))
	// A single sample.
	if n == 1 {
		return ctx.quantBandN1(x, nil, lowbandOut)
	}
	if tfChange > 0 {
		recombine = tfChange
	}
	// Band recombining increases the frequency resolution.
	if lowbandScratch != nil && lowband != nil && (recombine != 0 || (nB&1 == 0 && tfChange < 0) || b0 > 1) {
		copy(lowbandScratch[:n], lowband[:n])
		lowband = lowbandScratch
	}
	for k := 0; k < recombine; k++ {
		if lowband != nil {
			haar1(lowband, n>>k, 1<<k)
		}
		fill = int(bitInterleaveTable[fill&0xF]) | int(bitInterleaveTable[fill>>4])<<2
	}
	bb >>= recombine
	nB <<= recombine
	// Increasing the time resolution.
	for nB&1 == 0 && tfChange < 0 {
		if lowband != nil {
			haar1(lowband, nB, bb)
		}
		fill |= fill << bb
		bb <<= 1
		nB >>= 1
		timeDivide++
		tfChange++
	}
	b0 = bb
	nB0 := nB
	// The samples in time order instead of frequency order.
	if b0 > 1 && lowband != nil {
		deinterleaveHadamard(lowband, ctx.tmp, nB>>recombine, b0<<recombine, longBlocks)
	}
	cm := ctx.quantPartition(x, n, b, bb, lowband, lm, gain, fill)

	// Back from time order to frequency order.
	if b0 > 1 {
		interleaveHadamard(x, ctx.tmp, nB>>recombine, b0<<recombine, longBlocks)
	}
	// Undo the time-frequency changes.
	nB = nB0
	bb = b0
	for k := 0; k < timeDivide; k++ {
		bb >>= 1
		nB <<= 1
		cm |= cm >> bb
		haar1(x, nB, bb)
	}
	for k := 0; k < recombine; k++ {
		cm = uint32(bitDeinterleaveTable[cm])
		haar1(x, n0>>k, 1<<k)
	}
	bb <<= recombine
	// Scale the output for later folding.
	if lowbandOut != nil {
		nn := celtSqrt(float32(n0))
		for j := 0; j < n0; j++ {
			lowbandOut[j] = nn * x[j]
		}
	}
	return cm & (1<<bb - 1)
}

// quantBandStereo is quant_band_stereo: a band of both channels, as mid
// and side.
func (ctx *bandCtx) quantBandStereo(x, y []float32, n, b, bb int, lowband []float32, lm int,
	lowbandOut, lowbandScratch []float32, fill int) uint32 {
	// A single sample.
	if n == 1 {
		return ctx.quantBandN1(x, y, lowbandOut)
	}
	origFill := fill
	var sctx splitCtx
	ctx.computeTheta(&sctx, n, &b, bb, bb, lm, true, &fill)
	delta := sctx.delta
	itheta := sctx.itheta
	mid := (1. / 32768) * float32(sctx.imid)
	side := (1. / 32768) * float32(sctx.iside)
	var cm uint32
	if n == 2 {
		// For N=2, mid and side are orthogonal: the side takes one bit.
		mbits := b
		sbits := 0
		if itheta != 0 && itheta != 16384 {
			sbits = 1 << bitRes
		}
		mbits -= sbits
		ctx.remainingBits -= int32(sctx.qalloc + sbits)
		x2, y2 := x, y
		if itheta > 8192 {
			x2, y2 = y, x
		}
		sign := 0
		if sbits != 0 {
			sign = int(ctx.dec.Bits(1))
		}
		sign = 1 - 2*sign
		// orig_fill, to fold the side even when itheta==16384 cleared the
		// low bits of fill.
		cm = ctx.quantBand(x2, n, mbits, bb, lowband, lm, lowbandOut, q31One, lowbandScratch, origFill)
		// N=2 bands aren't split: cm is 1, or 0 for a fold-collapse.
		y2[0] = float32(-sign) * x2[1]
		y2[1] = float32(sign) * x2[0]
		x[0] = mid * x[0]
		x[1] = mid * x[1]
		y[0] = side * y[0]
		y[1] = side * y[1]
		tmp := x[0]
		x[0] = tmp - y[0]
		y[0] = tmp + y[0]
		tmp = x[1]
		x[1] = tmp - y[1]
		y[1] = tmp + y[1]
	} else {
		mbits := max(0, min(b, (b-delta)/2))
		sbits := b - mbits
		ctx.remainingBits -= int32(sctx.qalloc)
		rebalance := ctx.remainingBits
		if mbits >= sbits {
			// The mid stays normalized, for folding later.
			cm = ctx.quantBand(x, n, mbits, bb, lowband, lm, lowbandOut, q31One, lowbandScratch, fill)
			rebalance = int32(mbits) - (rebalance - ctx.remainingBits)
			if rebalance > 3<<bitRes && itheta != 0 {
				sbits += int(rebalance) - (3 << bitRes)
			}
			// The high bits of fill are zero in a stereo split: the side
			// isn't folded.
			cm |= ctx.quantBand(y, n, sbits, bb, nil, lm, nil, side, nil, fill>>bb)
		} else {
			cm = ctx.quantBand(y, n, sbits, bb, nil, lm, nil, side, nil, fill>>bb)
			rebalance = int32(sbits) - (rebalance - ctx.remainingBits)
			if rebalance > 3<<bitRes && itheta != 16384 {
				mbits += int(rebalance) - (3 << bitRes)
			}
			cm |= ctx.quantBand(x, n, mbits, bb, lowband, lm, lowbandOut, q31One, lowbandScratch, fill)
		}
	}
	if n != 2 {
		stereoMerge(x, y, mid, n)
	}
	if sctx.inv {
		for j := 0; j < n; j++ {
			y[j] = -y[j]
		}
	}
	return cm
}

// specialHybridFolding duplicates enough of the first band's folding data
// to fold the second band; none in CELT-only mode.
func (m *mode) specialHybridFolding(norm, norm2 []float32, start, mm int, dualStereo bool) {
	n1 := mm * (int(m.eBands[start+1]) - int(m.eBands[start]))
	n2 := mm * (int(m.eBands[start+2]) - int(m.eBands[start+1]))
	if n2 <= n1 {
		return
	}
	copy(norm[n1:n2], norm[2*n1-n2:n1])
	if dualStereo {
		copy(norm2[n1:n2], norm2[2*n1-n2:n1])
	}
}

// bandScratch holds the buffers of quantAllBands.
type bandScratch struct {
	norm []float32
	iy   []int
	tmp  []float32
}

func newBandScratch() *bandScratch {
	return &bandScratch{
		norm: make([]float32, 2*8*100),
		iy:   make([]int, 8*22),
		tmp:  make([]float32, 8*22),
	}
}

// quantAllBands is quant_all_bands for the decoder: the shapes of the bands
// from start to end, of x and, in stereo, y.
func (m *mode) quantAllBands(start, end int, x, y []float32, collapseMasks []uint8, pulses []int,
	shortBlocks bool, spread int, dualStereo bool, intensity int, tfRes []int, totalBits, balance int32,
	dec *rangedec.Decoder, lm, codedBands int, seed *uint32, disableInv bool, sc *bandScratch) {
	eBands := m.eBands
	mm := 1 << lm
	bb := 1
	if shortBlocks {
		bb = mm
	}
	c := 1
	if y != nil {
		c = 2
	}
	normOffset := mm * int(eBands[start])
	// The last band needs no norm: nothing folds from it.
	normLen := mm*int(eBands[m.nbEBands-1]) - normOffset
	norm := sc.norm[:c*normLen]
	norm2 := norm[normLen:]
	// The last band of x is scratch space until it is decoded.
	lowbandScratch := x[mm*int(eBands[m.effEBands-1]):]
	lowbandOffset := 0
	updateLowband := true
	ctx := bandCtx{
		m: m, dec: dec, intensity: intensity, seed: *seed, spread: spread,
		disableInv: disableInv, iy: sc.iy, tmp: sc.tmp,
		// No noise in the first band of transients.
		avoidSplitNoise: bb > 1,
	}
	for i := start; i < end; i++ {
		ctx.i = i
		last := i == end-1
		xb := x[mm*int(eBands[i]):]
		var yb []float32
		if y != nil {
			yb = y[mm*int(eBands[i]):]
		}
		n := mm*int(eBands[i+1]) - mm*int(eBands[i])
		tell := int32(dec.TellFrac())
		// The bits to give this band.
		if i != start {
			balance -= tell
		}
		remainingBits := totalBits - tell - 1
		ctx.remainingBits = remainingBits
		b := 0
		if i <= codedBands-1 {
			currBalance := balance / int32(min(3, codedBands-i))
			b = max(0, min(16383, min(int(remainingBits+1), pulses[i]+int(currBalance))))
		}
		if (mm*int(eBands[i])-n >= mm*int(eBands[start]) || i == start+1) && (updateLowband || lowbandOffset == 0) {
			lowbandOffset = i
		}
		if i == start+1 {
			m.specialHybridFolding(norm, norm2, start, mm, dualStereo)
		}
		tfChange := tfRes[i]
		ctx.tfChange = tfChange
		if i >= m.effEBands {
			xb = norm
			if y != nil {
				yb = norm
			}
			lowbandScratch = nil
		}
		if last {
			lowbandScratch = nil
		}
		// A conservative estimate of the collapse masks of the bands to
		// fold from.
		effectiveLowband := -1
		var xCm, yCm uint32
		if lowbandOffset != 0 && (spread != spreadAggressive || bb > 1 || tfChange < 0) {
			// Never repeat spectral content within one band.
			effectiveLowband = max(0, mm*int(eBands[lowbandOffset])-normOffset-n)
			foldStart := lowbandOffset
			for {
				foldStart--
				if mm*int(eBands[foldStart]) <= effectiveLowband+normOffset {
					break
				}
			}
			foldEnd := lowbandOffset - 1
			for {
				foldEnd++
				if !(foldEnd < i && mm*int(eBands[foldEnd]) < effectiveLowband+normOffset+n) {
					break
				}
			}
			for foldI := foldStart; ; {
				xCm |= uint32(collapseMasks[foldI*c+0])
				yCm |= uint32(collapseMasks[foldI*c+c-1])
				foldI++
				if foldI >= foldEnd {
					break
				}
			}
		} else {
			// The LCG folds: all blocks are (almost always) nonzero.
			xCm = 1<<bb - 1
			yCm = xCm
		}
		if dualStereo && i == intensity {
			// Switch dual stereo off for intensity.
			dualStereo = false
			for j := 0; j < mm*int(eBands[i])-normOffset; j++ {
				norm[j] = .5 * (norm[j] + norm2[j])
			}
		}
		var lowband, lowband2, out, out2 []float32
		if effectiveLowband != -1 {
			lowband = norm[effectiveLowband:]
			if c == 2 {
				lowband2 = norm2[effectiveLowband:]
			}
		}
		if !last {
			out = norm[mm*int(eBands[i])-normOffset:]
			if c == 2 {
				out2 = norm2[mm*int(eBands[i])-normOffset:]
			}
		}
		if dualStereo {
			xCm = ctx.quantBand(xb, n, b/2, bb, lowband, lm, out, q31One, lowbandScratch, int(xCm))
			yCm = ctx.quantBand(yb, n, b/2, bb, lowband2, lm, out2, q31One, lowbandScratch, int(yCm))
		} else {
			if yb != nil {
				xCm = ctx.quantBandStereo(xb, yb, n, b, bb, lowband, lm, out, lowbandScratch, int(xCm|yCm))
			} else {
				xCm = ctx.quantBand(xb, n, b, bb, lowband, lm, out, q31One, lowbandScratch, int(xCm|yCm))
			}
			yCm = xCm
		}
		collapseMasks[i*c+0] = uint8(xCm)
		collapseMasks[i*c+c-1] = uint8(yCm)
		balance += int32(pulses[i]) + tell
		// Update the folding position only while at 1 bit/sample depth.
		updateLowband = b > n<<bitRes
		// Only the first band needs to avoid noise on a split; then there
		// is folding.
		ctx.avoidSplitNoise = false
	}
	*seed = ctx.seed
}
