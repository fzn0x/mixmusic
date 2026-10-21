package celt

// Port of celt/cwrs.c: the decoding of PVQ codewords, the pulses of a
// band's shape.

import "github.com/fzn0x/mixmusic/internal/opus/rangedec"

// pvqRow is CELT_PVQ_U_ROW[r].
func pvqRow(r int) []uint32 { return pvqUData[pvqURowOffsets[r]:] }

// pvqU is U(N,K), the number of PVQ codewords of N dimensions with K
// pulses, as CELT_PVQ_U.
func pvqU(n, k int) uint32 { return pvqRow(min(n, k))[max(n, k)] }

// pvqV is V(N,K) = U(N,K) + U(N,K+1).
func pvqV(n, k int) uint32 { return pvqU(n, k) + pvqU(n, k+1) }

// cwrsi turns codeword i of N dimensions and K pulses into the pulses y,
// and returns the squared norm of y.
func cwrsi(n, k int, i uint32, y []int) float32 {
	var yy float32
	o := 0
	emit := func(val int16) {
		y[o] = int(val)
		o++
		yy += float32(val) * float32(val)
	}
	for n > 2 {
		if k >= n {
			// Lots of pulses.
			row := pvqRow(n)
			// Are the pulses in this dimension negative?
			p := row[k+1]
			s := -b2i(i >= p)
			i -= p & uint32(int32(s))
			// How many pulses are in this dimension?
			k0 := k
			q := row[n]
			if q > i {
				k = n
				for {
					k--
					p = pvqRow(k)[n]
					if p <= i {
						break
					}
				}
			} else {
				for p = row[k]; p > i; p = row[k] {
					k--
				}
			}
			i -= p
			emit(int16((k0 - k + s) ^ s))
		} else {
			// Lots of dimensions: are there pulses in this one at all?
			p := pvqRow(k)[n]
			q := pvqRow(k + 1)[n]
			if p <= i && i < q {
				i -= p
				emit(0)
			} else {
				s := -b2i(i >= q)
				i -= q & uint32(int32(s))
				k0 := k
				for {
					k--
					p = pvqRow(k)[n]
					if p <= i {
						break
					}
				}
				i -= p
				emit(int16((k0 - k + s) ^ s))
			}
		}
		n--
	}
	// n == 2
	p := uint32(2*k + 1)
	s := -b2i(i >= p)
	i -= p & uint32(int32(s))
	k0 := k
	k = int((i + 1) >> 1)
	if k != 0 {
		i -= uint32(2*k - 1)
	}
	emit(int16((k0 - k + s) ^ s))
	// n == 1
	s = -int(i)
	emit(int16((k + s) ^ s))
	return yy
}

// decodePulses is decode_pulses: it reads the pulses y of a band of n
// dimensions and k pulses, and returns their squared norm.
func decodePulses(y []int, n, k int, dec *rangedec.Decoder) float32 {
	return cwrsi(n, k, dec.Uint(pvqV(n, k)), y)
}
