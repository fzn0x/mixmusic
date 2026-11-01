package aac

// Port of Tools/TNS.cs, which reads tns_data. SharpJaad leaves the
// filtering undone: apply follows the decoding process the standard gives
// for it instead.

// tnsMaxOrder is TNS_MAX_ORDER of AAC-LC: the highest order of a filter
// of a long window. A short window's order is coded in 3 bits, at most 7,
// its TNS_MAX_ORDER.
const tnsMaxOrder = 12

// tnsTables are the reflection coefficients a filter's bits code, by
// 2*coef_compress + coef_res. With res bits, 3 + coef_res, the bits code
// a signed c of res - coef_compress bits, and the coefficient is
// sin(c / ((2^(res-1) - 1/2) / (π/2))), or + 1/2 for a negative c.
// SharpJaad's tables hold them negated.
var tnsTables = [4][]float32{tnsCoef0x3[:], tnsCoef0x4[:], tnsCoef1x3[:], tnsCoef1x4[:]}

// tns is tns_data: the filters of each window, from the top band down.
type tns struct {
	present   bool
	filters   [8]int // n_filt
	length    [8][3]int
	order     [8][3]int
	direction [8][3]bool
	// coef are the reflection coefficients of each filter, up to
	// tnsMaxOrder: the standard ignores those after.
	coef [8][3][tnsMaxOrder]float32
}

func (t *tns) parse(r *bitReader, info *icsInfo) {
	filterBits, lengthBits, orderBits := 2, 6, 5
	if info.seq == eightShortSequence {
		filterBits, lengthBits, orderBits = 1, 4, 3
	}
	for w := range info.windows {
		t.filters[w] = r.read(filterBits)
		if t.filters[w] == 0 {
			continue
		}
		coefRes := r.read(1)
		for f := range t.filters[w] {
			t.length[w][f] = r.read(lengthBits)
			order := r.read(orderBits)
			t.order[w][f] = order
			if order == 0 {
				continue
			}
			t.direction[w][f] = r.flag()
			compress := r.read(1)
			bits := coefRes + 3 - compress
			table := tnsTables[2*compress+coefRes]
			for i := range order {
				c := -table[r.read(bits)]
				if i < tnsMaxOrder {
					t.coef[w][f][i] = c
				}
			}
		}
	}
}

// apply filters the spectrum of each window of info with the filters of
// t, in place. maxBands is TNS_MAX_BANDS, which bounds the bands they
// filter.
func (t *tns) apply(spec []float32, info *icsInfo, maxBands int) {
	var lpc, tmp [tnsMaxOrder + 1]float32
	bands := len(info.swb) - 1
	for w := range info.windows {
		x := spec[128*w:]
		bottom := bands
		for f := range t.filters[w] {
			top := bottom
			bottom = max(top-t.length[w][f], 0)
			order := min(t.order[w][f], tnsMaxOrder)
			if order == 0 {
				continue
			}

			// The coefficients of the filter, from its reflection
			// coefficients.
			lpc[0] = 1
			for m := 1; m <= order; m++ {
				k := t.coef[w][f][m-1]
				for i := 1; i < m; i++ {
					tmp[i] = lpc[i] + k*lpc[m-i]
				}
				copy(lpc[1:m], tmp[1:m])
				lpc[m] = k
			}

			start := int(info.swb[min(bottom, maxBands, info.maxSFB)])
			end := int(info.swb[min(top, maxBands, info.maxSFB)])
			size := end - start
			if size <= 0 {
				continue
			}
			inc := 1
			if t.direction[w][f] {
				inc, start = -1, end-1
			}
			// The all-pole filter y(n) = x(n) - lpc[1] y(n-1) - ... -
			// lpc[order] y(n-order), from a state of zeros, in place.
			for n := range size {
				p := start + n*inc
				y := x[p]
				for i := 1; i <= min(n, order); i++ {
					y -= lpc[i] * x[p-i*inc]
				}
				x[p] = y
			}
		}
	}
}
