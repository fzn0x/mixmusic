package aac

import (
	"math"
	"testing"
)

// TestTNSTables checks the coefficients of temporal noise shaping against
// the inverse quantization of the standard.
func TestTNSTables(t *testing.T) {
	for i, table := range tnsTables {
		compress, res := i>>1, 3+i&1
		bits := res - compress
		if len(table) != 1<<bits {
			t.Errorf("table %d has %d coefficients, want %d", i, len(table), 1<<bits)
		}
		for raw, v := range table {
			c := raw
			if c >= 1<<(bits-1) {
				c -= 1 << bits
			}
			iqfac := (float64(int(1)<<(res-1)) - 0.5) / (math.Pi / 2)
			if c < 0 {
				iqfac = (float64(int(1)<<(res-1)) + 0.5) / (math.Pi / 2)
			}
			want := math.Sin(float64(c) / iqfac)
			if d := math.Abs(float64(-v) - want); d > 1e-7 {
				t.Errorf("table %d: coefficient %d is %v, want %v", i, raw, -v, want)
			}
		}
	}
}
