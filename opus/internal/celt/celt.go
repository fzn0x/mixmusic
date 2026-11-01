package celt

// Port of celt/celt.c and celt/celt.h: the pitch post-filter and the
// tables of the decoder.

var (
	trimICDF   = []uint8{126, 124, 119, 109, 87, 41, 19, 9, 4, 2, 0}
	spreadICDF = []uint8{25, 23, 2, 0}
	tapsetICDF = []uint8{2, 1, 0}

	// tfSelectTable are the time-frequency changes: positive values mean
	// better frequency resolution, negative ones better time resolution.
	// The second index is 4*isTransient + 2*tf_select + per_band_flag.
	tfSelectTable = [4][8]int8{
		{0, -1, 0, -1, 0, -1, 0, -1}, // 2.5 ms
		{0, -1, 0, -2, 1, 0, 1, -1},  // 5 ms
		{0, -2, 0, -3, 2, 0, 1, -1},  // 10 ms
		{0, -2, 0, -3, 3, 0, 1, -1},  // 20 ms
	}

	combGains = [3][3]float32{
		{0.3066406250, 0.2170410156, 0.1296386719},
		{0.4638671875, 0.2680664062, 0},
		{0.7998046875, 0.1000976562, 0},
	}
)

// resamplingFactor is the decimation of the decoder's 48 kHz for rate.
func resamplingFactor(rate int) int {
	switch rate {
	case 48000:
		return 1
	case 24000:
		return 2
	case 16000:
		return 3
	case 12000:
		return 4
	case 8000:
		return 6
	}
	return 0
}

func combFilterConst(y []float32, x []float32, xo, t, n int, g10, g11, g12 float32) {
	x4 := x[xo-t-2]
	x3 := x[xo-t-1]
	x2 := x[xo-t]
	x1 := x[xo-t+1]
	for i := 0; i < n; i++ {
		x0 := x[xo+i-t+2]
		y[i] = x[xo+i] + g10*x2 + g11*(x1+x3) + g12*(x0+x4)
		x4 = x3
		x3 = x2
		x2 = x1
		x1 = x0
	}
}

// combFilter is comb_filter: y[i] is x[xo+i] through the pitch filter, its
// period and gain going from T0, g0 to T1, g1 over the overlap. x needs
// T+2 samples of history before xo, and y may be x[xo:], which makes the
// filter recursive.
func combFilter(y []float32, x []float32, xo, t0, t1, n int, g0, g1 float32, tapset0, tapset1 int,
	window []float32, overlap int) {
	if g0 == 0 && g1 == 0 {
		copy(y[:n], x[xo:xo+n])
		return
	}
	// With a zero gain, T0 or T1 is zero: at least 2 avoids reading garbage.
	t0 = max(t0, combfilterMinPeriod)
	t1 = max(t1, combfilterMinPeriod)
	g00 := g0 * combGains[tapset0][0]
	g01 := g0 * combGains[tapset0][1]
	g02 := g0 * combGains[tapset0][2]
	g10 := g1 * combGains[tapset1][0]
	g11 := g1 * combGains[tapset1][1]
	g12 := g1 * combGains[tapset1][2]
	x1 := x[xo-t1+1]
	x2 := x[xo-t1]
	x3 := x[xo-t1-1]
	x4 := x[xo-t1-2]
	// A filter that didn't change needs no overlap.
	if g0 == g1 && t0 == t1 && tapset0 == tapset1 {
		overlap = 0
	}
	i := 0
	for ; i < overlap; i++ {
		x0 := x[xo+i-t1+2]
		f := window[i] * window[i]
		y[i] = x[xo+i] +
			((coefOne-f)*g00)*x[xo+i-t0] +
			((coefOne-f)*g01)*(x[xo+i-t0+1]+x[xo+i-t0-1]) +
			((coefOne-f)*g02)*(x[xo+i-t0+2]+x[xo+i-t0-2]) +
			(f*g10)*x2 +
			(f*g11)*(x1+x3) +
			(f*g12)*(x0+x4)
		x4 = x3
		x3 = x2
		x2 = x1
		x1 = x0
	}
	if g1 == 0 {
		copy(y[overlap:n], x[xo+overlap:xo+n])
		return
	}
	// The part with the constant filter.
	combFilterConst(y[i:], x, xo+i, t1, n-i, g10, g11, g12)
}

// initCaps is init_caps: the most bits each band can use.
func (m *mode) initCaps(cap []int, lm, c int) {
	for i := 0; i < m.nbEBands; i++ {
		n := (int(m.eBands[i+1]) - int(m.eBands[i])) << lm
		cap[i] = (int(m.cache.caps[m.nbEBands*(2*lm+c-1)+i]) + 64) * c * n >> 2
	}
}
