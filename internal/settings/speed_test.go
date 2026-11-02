package settings

import (
	"encoding/json"
	"math"
	"testing"
)

// TestCleanSpeed keeps a speed of the steps, takes the nearest step for
// another, brings one out of range back into it, and plays at a track's
// own speed what has no speed, as settings from before it.
func TestCleanSpeed(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{1, 1}, {1.25, 1.25}, {0.5, 0.5}, {3, 3}, {1.1, 1.1}, {2.35, 2.35},
		{1.234, 1.25}, {1.26, 1.25}, {0.3, 0.5}, {7, 3},
		{0, 1}, {-2, 1}, {math.NaN(), 1}, {math.Inf(1), 1}, {math.Inf(-1), 1},
	} {
		if got := CleanSpeed(c.in); got != c.want {
			t.Errorf("a speed of %v reads as %v, want %v", c.in, got, c.want)
		}
	}
	for file, want := range map[string]float64{`{}`: 1, `{"songSpeed":1.5}`: 1.5, `{"songSpeed":0}`: 1, `{"songSpeed":9}`: 3} {
		s := Default()
		if err := json.Unmarshal([]byte(file), &s); err != nil {
			t.Fatal(err)
		}
		if got := s.Clean().SongSpeed; got != want {
			t.Errorf("%s: songs play at %v, not %v", file, got, want)
		}
	}
	// Every step reads as itself.
	for k := 10; k <= 60; k++ {
		v := float64(k) / 20
		if got := CleanSpeed(v); got != v {
			t.Errorf("the step %v reads as %v", v, got)
		}
	}
}
