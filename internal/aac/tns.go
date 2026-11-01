package aac

// Port of Tools/TNS.cs, which reads tns_data.

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
