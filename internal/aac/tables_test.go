package aac

import (
	"math"
	"testing"
)

// TestBands checks the offsets of the scalefactor bands of each sampling
// frequency: they rise from 0 to the window's length, by multiples of 4,
// in the number of bands the standard gives.
func TestBands(t *testing.T) {
	long := []int{41, 41, 47, 49, 49, 51, 47, 47, 43, 43, 43, 40}
	short := []int{12, 12, 12, 14, 14, 14, 15, 15, 15, 15, 15, 15}
	for i, r := range rates {
		for _, b := range []struct {
			swb    []uint16
			n, end int
		}{{r.long, long[i], 1024}, {r.short, short[i], 128}} {
			if len(b.swb) != b.n+1 || b.swb[0] != 0 || int(b.swb[b.n]) != b.end {
				t.Errorf("rate %d: %d bands to %d, want %d to %d", sampleRates[i], len(b.swb)-1, b.swb[len(b.swb)-1], b.n, b.end)
			}
			for k := 1; k < len(b.swb); k++ {
				if b.swb[k] <= b.swb[k-1] || b.swb[k]%4 != 0 {
					t.Errorf("rate %d: band %d from %d to %d", sampleRates[i], k-1, b.swb[k-1], b.swb[k])
				}
			}
		}
		if r.tnsLong > len(r.long)-1 || r.tnsShort > len(r.short)-1 {
			t.Errorf("rate %d: TNS up to bands %d and %d", sampleRates[i], r.tnsLong, r.tnsShort)
		}
	}
}

// besselI0 is the modified Bessel function of the first kind, of order 0.
func besselI0(x float64) float64 {
	sum, term := 1.0, 1.0
	for k := 1; term > 1e-20*sum; k++ {
		term *= (x / 2 / float64(k)) * (x / 2 / float64(k))
		sum += term
	}
	return sum
}

// TestWindows checks the windows against their definitions: the sine
// window, and the Kaiser-Bessel derived one, of alpha 4 for long windows
// and 6 for short ones.
func TestWindows(t *testing.T) {
	for _, w := range []struct {
		name  string
		half  []float32
		alpha float64 // 0 for a sine window
	}{
		{"sine1024", sine1024[:], 0},
		{"sine128", sine128[:], 0},
		{"kbd1024", kbd1024[:], 4},
		{"kbd128", kbd128[:], 6},
	} {
		n := 2 * len(w.half)
		want := make([]float64, len(w.half))
		if w.alpha == 0 {
			for i := range want {
				want[i] = math.Sin(math.Pi / float64(n) * (float64(i) + 0.5))
			}
		} else {
			kaiser := make([]float64, n/2+1)
			var total float64
			for i := range kaiser {
				x := 4*float64(i)/float64(n) - 1
				kaiser[i] = besselI0(math.Pi * w.alpha * math.Sqrt(1-x*x))
				total += kaiser[i]
			}
			var sum float64
			for i := range want {
				sum += kaiser[i]
				want[i] = math.Sqrt(sum / total)
			}
		}
		for i, v := range w.half {
			if d := math.Abs(float64(v) - want[i]); d > 1e-7 {
				t.Errorf("%s[%d] = %v, want %v", w.name, i, v, want[i])
				break
			}
		}
		// Princen-Bradley: a window and its mirror add up to 1 in power.
		for i := range w.half {
			a, b := float64(w.half[i]), float64(w.half[len(w.half)-1-i])
			if d := math.Abs(a*a + b*b - 1); d > 1e-6 {
				t.Errorf("%s: %d and its mirror add up to %v", w.name, i, a*a+b*b)
				break
			}
		}
	}
}
