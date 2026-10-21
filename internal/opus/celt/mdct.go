package celt

// Port of celt/mdct.c: the inverse MDCT, clt_mdct_backward_c.

// mdctBackward turns the coefficients in[0], in[stride], in[2*stride]...
// into out, windowed and overlapped for TDAC with the overlap the previous
// frame left at its start.
func mdctBackward(l *mdctLookup, in []float32, out []float32, window []float32, overlap, shift, stride int) {
	n := l.n
	trig := l.trig
	for i := 0; i < shift; i++ {
		n >>= 1
		trig = trig[n:]
	}
	n2 := n >> 1
	n4 := n >> 2

	// Pre-rotate, storing in bit-reversed order for the FFT.
	yp := out[overlap>>1:]
	bitrev := l.kfft[shift].bitrev
	xp1, xp2 := 0, stride*(n2-1)
	for i := 0; i < n4; i++ {
		rev := int(bitrev[i])
		x1, x2 := in[xp1], in[xp2]
		yr := x2*trig[i] + x1*trig[n4+i]
		yi := x1*trig[i] - x2*trig[n4+i]
		// Real and imaginary swap: an FFT stands for an inverse FFT.
		yp[2*rev+1] = yr
		yp[2*rev] = yi
		xp1 += 2 * stride
		xp2 -= 2 * stride
	}

	fftImpl(l.kfft[shift], yp)

	// Post-rotate and de-shuffle from both ends of the buffer at once, in
	// place.
	y0, y1 := overlap>>1, overlap>>1+n2-2
	for i := 0; i < (n4+1)>>1; i++ {
		re, im := out[y0+1], out[y0]
		t0, t1 := trig[i], trig[n4+i]
		yr := re*t0 + im*t1
		yi := re*t1 - im*t0
		re, im = out[y1+1], out[y1]
		out[y0] = yr
		out[y1+1] = yi
		t0, t1 = trig[n4-i-1], trig[n2-i-1]
		yr = re*t0 + im*t1
		yi = re*t1 - im*t0
		out[y1] = yr
		out[y0+1] = yi
		y0 += 2
		y1 -= 2
	}

	// Mirror on both sides for TDAC.
	x1i, y1i := overlap-1, 0
	w1, w2 := 0, overlap-1
	for i := 0; i < overlap/2; i++ {
		x1, x2 := out[x1i], out[y1i]
		out[y1i] = x2*window[w2] - x1*window[w1]
		out[x1i] = x2*window[w1] + x1*window[w2]
		y1i++
		x1i--
		w1++
		w2--
	}
}
