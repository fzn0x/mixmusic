package celt

// Port of celt/pitch.c and celt/celt_lpc.c: the pitch search and the LPC
// that the packet loss concealment extrapolates with.

// xcorr is celt_pitch_xcorr: xcorr[i] = sum of x[j]*y[i+j] for j < n, for
// i < maxPitch, each sum in the order of j as the reference's kernels.
func xcorr(x, y, out []float32, n, maxPitch int) {
	for i := 0; i < maxPitch; i++ {
		out[i] = innerProd(x, y[i:], n)
	}
}

func findBestPitch(xc, y []float32, length, maxPitch int, bestPitch *[2]int) {
	syy := float32(1)
	bestNum := [2]float32{-1, -1}
	var bestDen [2]float32
	bestPitch[0], bestPitch[1] = 0, 1
	for j := 0; j < length; j++ {
		syy += y[j] * y[j]
	}
	for i := 0; i < maxPitch; i++ {
		if xc[i] > 0 {
			// Small, to neither underflow nor overflow when squared.
			xcorr16 := xc[i] * 1e-12
			num := xcorr16 * xcorr16
			if num*bestDen[1] > bestNum[1]*syy {
				if num*bestDen[0] > bestNum[0]*syy {
					bestNum[1] = bestNum[0]
					bestDen[1] = bestDen[0]
					bestPitch[1] = bestPitch[0]
					bestNum[0] = num
					bestDen[0] = syy
					bestPitch[0] = i
				} else {
					bestNum[1] = num
					bestDen[1] = syy
					bestPitch[1] = i
				}
			}
		}
		syy += y[i+length]*y[i+length] - y[i]*y[i]
		syy = max(1, syy)
	}
}

func celtFIR5(x []float32, num *[5]float32, n int) {
	var mem0, mem1, mem2, mem3, mem4 float32
	for i := 0; i < n; i++ {
		sum := x[i]
		sum += num[0] * mem0
		sum += num[1] * mem1
		sum += num[2] * mem2
		sum += num[3] * mem3
		sum += num[4] * mem4
		mem4 = mem3
		mem3 = mem2
		mem2 = mem1
		mem1 = mem0
		mem0 = x[i]
		x[i] = sum
	}
}

// pitchDownsample is pitch_downsample: x of c channels, low-passed and
// decimated by factor into xLP of length samples, then whitened.
func pitchDownsample(x [2][]float32, xLP []float32, length, c, factor int) {
	offset := factor / 2
	for i := 1; i < length; i++ {
		xLP[i] = .25*x[0][factor*i-offset] + .25*x[0][factor*i+offset] + .5*x[0][factor*i]
	}
	xLP[0] = .25*x[0][offset] + .5*x[0][0]
	if c == 2 {
		for i := 1; i < length; i++ {
			xLP[i] += .25*x[1][factor*i-offset] + .25*x[1][factor*i+offset] + .5*x[1][factor*i]
		}
		xLP[0] += .25*x[1][offset] + .5*x[1][0]
	}
	var ac [5]float32
	celtAutocorr(xLP, ac[:], nil, 0, 4, length)
	// A noise floor of -40 dB.
	ac[0] *= 1.0001
	// Lag windowing.
	for i := 1; i <= 4; i++ {
		ac[i] -= ac[i] * (.008 * float32(i)) * (.008 * float32(i))
	}
	var lpc [4]float32
	celtLPC(lpc[:], ac[:], 4)
	tmp := q15One
	for i := 0; i < 4; i++ {
		tmp = .9 * tmp
		lpc[i] = lpc[i] * tmp
	}
	// Add a zero.
	const c1 = float32(.8)
	lpc2 := [5]float32{
		lpc[0] + .8,
		lpc[1] + c1*lpc[0],
		lpc[2] + c1*lpc[1],
		lpc[3] + c1*lpc[2],
		c1 * lpc[3],
	}
	celtFIR5(xLP, &lpc2, length)
}

// pitchSearch is pitch_search: the pitch of xLP within y, as a lag below
// maxPitch.
func pitchSearch(xLP, y []float32, length, maxPitch int) int {
	lag := length + maxPitch
	xLP4 := make([]float32, length>>2)
	yLP4 := make([]float32, lag>>2)
	xc := make([]float32, maxPitch>>1)
	// Downsample by 2 again.
	for j := 0; j < length>>2; j++ {
		xLP4[j] = xLP[2*j]
	}
	for j := 0; j < lag>>2; j++ {
		yLP4[j] = y[2*j]
	}
	// A coarse search with 4x decimation.
	var bestPitch [2]int
	xcorr(xLP4, yLP4, xc, length>>2, maxPitch>>2)
	findBestPitch(xc, yLP4, length>>2, maxPitch>>2, &bestPitch)
	// A finer search with 2x decimation.
	for i := 0; i < maxPitch>>1; i++ {
		xc[i] = 0
		if abs(i-2*bestPitch[0]) > 2 && abs(i-2*bestPitch[1]) > 2 {
			continue
		}
		sum := innerProd(xLP, y[i:], length>>1)
		xc[i] = max(-1, sum)
	}
	findBestPitch(xc, y, length>>1, maxPitch>>1, &bestPitch)
	// Refine by pseudo-interpolation.
	offset := 0
	if bestPitch[0] > 0 && bestPitch[0] < (maxPitch>>1)-1 {
		a := xc[bestPitch[0]-1]
		b := xc[bestPitch[0]]
		c := xc[bestPitch[0]+1]
		if c-a > .7*(b-a) {
			offset = 1
		} else if a-c > .7*(b-c) {
			offset = -1
		}
	}
	return 2*bestPitch[0] - offset
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// celtLPC is _celt_lpc: the LPC of order p of the autocorrelation ac.
func celtLPC(lpc, ac []float32, p int) {
	errv := ac[0]
	clear(lpc[:p])
	if !(ac[0] > 1e-10) {
		return
	}
	for i := 0; i < p; i++ {
		// This iteration's reflection coefficient.
		var rr float32
		for j := 0; j < i; j++ {
			rr += lpc[j] * ac[i-j]
		}
		rr += ac[i+1]
		r := -fracDiv32(rr, errv)
		// Update the coefficients and the total error.
		lpc[i] = r
		for j := 0; j < (i+1)>>1; j++ {
			tmp1, tmp2 := lpc[j], lpc[i-1-j]
			lpc[j] = tmp1 + r*tmp2
			lpc[i-1-j] = tmp2 + r*tmp1
		}
		errv = errv - (r*r)*errv
		// Bail out once at a gain of 30 dB.
		if errv <= .001*ac[0] {
			break
		}
	}
}

// celtFIR is celt_fir: y[i] = x[xo+i] + sum of num[ord-1-j]*x[xo+i+j-ord],
// reading ord samples of history before xo.
func celtFIR(x []float32, xo int, num []float32, y []float32, n, ord int) {
	for i := 0; i < n; i++ {
		sum := x[xo+i]
		for j := 0; j < ord; j++ {
			sum += num[ord-j-1] * x[xo+i+j-ord]
		}
		y[i] = sum
	}
}

// celtIIR is celt_iir, in place on x: the synthesis filter den, with its
// memory mem of ord samples.
func celtIIR(x []float32, den []float32, n, ord int, mem []float32, scratch []float32) {
	rden := make([]float32, ord)
	y := scratch[:n+ord]
	for i := 0; i < ord; i++ {
		rden[i] = den[ord-i-1]
	}
	i := 0
	for ; i < ord; i++ {
		y[i] = -mem[ord-i-1]
	}
	for ; i < n+ord; i++ {
		y[i] = 0
	}
	for i = 0; i < n-3; i += 4 {
		// Unrolled by 4 as an FIR filter, then patched up as the IIR.
		s0, s1, s2, s3 := x[i], x[i+1], x[i+2], x[i+3]
		for j := 0; j < ord; j++ {
			s0 += rden[j] * y[i+j]
			s1 += rden[j] * y[i+j+1]
			s2 += rden[j] * y[i+j+2]
			s3 += rden[j] * y[i+j+3]
		}
		y[i+ord] = -s0
		x[i] = s0
		s1 += y[i+ord] * den[0]
		y[i+ord+1] = -s1
		x[i+1] = s1
		s2 += y[i+ord+1] * den[0]
		s2 += y[i+ord] * den[1]
		y[i+ord+2] = -s2
		x[i+2] = s2
		s3 += y[i+ord+2] * den[0]
		s3 += y[i+ord+1] * den[1]
		s3 += y[i+ord] * den[2]
		y[i+ord+3] = -s3
		x[i+3] = s3
	}
	for ; i < n; i++ {
		sum := x[i]
		for j := 0; j < ord; j++ {
			sum -= rden[j] * y[i+j]
		}
		y[i+ord] = sum
		x[i] = sum
	}
	for i := 0; i < ord; i++ {
		mem[i] = x[n-i-1]
	}
}

// celtAutocorr is _celt_autocorr: ac[0..lag] of x of n samples, its ends
// windowed by window over overlap samples.
func celtAutocorr(x, ac, window []float32, overlap, lag, n int) {
	fastN := n - lag
	xp := x
	if overlap != 0 {
		xx := make([]float32, n)
		copy(xx, x[:n])
		for i := 0; i < overlap; i++ {
			w := window[i]
			xx[i] = x[i] * w
			xx[n-i-1] = x[n-i-1] * w
		}
		xp = xx
	}
	xcorr(xp, xp, ac, fastN, lag+1)
	for k := 0; k <= lag; k++ {
		var d float32
		for i := k + fastN; i < n; i++ {
			d += xp[i] * xp[i-k]
		}
		ac[k] += d
	}
}
