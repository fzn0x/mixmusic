package volume

import (
	"math"
	"testing"
)

func db(gain float64) float64 { return 20 * math.Log10(gain) }

// TestGain checks the ends, the middle, and the even steps of the curve.
func TestGain(t *testing.T) {
	if g := Gain(0); g != 0 {
		t.Errorf("Gain(0) = %v", g)
	}
	if g := Gain(1); math.Abs(g-1) > 1e-12 {
		t.Errorf("Gain(1) = %v", g)
	}
	if d := db(Gain(0.5)); math.Abs(d+30) > 0.5 {
		t.Errorf("the middle is %.2f dB, want -30", d)
	}
	// One press of up or down, 0.05, is about 3 dB down to -40 dB.
	for p := 1.0; Gain(p) > 0.01; p -= 0.05 {
		if step := db(Gain(p)) - db(Gain(p-0.05)); step < 2.9 || step > 3.5 {
			t.Errorf("from %.2f, a step down is %.2f dB", p, step)
		}
	}
}

// TestSaved checks that a saved volume plays as loud as before, where the
// gain was the square of the position, and that it goes back and forth.
func TestSaved(t *testing.T) {
	for _, c := range []struct{ saved, at float64 }{{0.8, 0.94}, {0.5, 0.80}, {0.1, 0.35}, {0, 0}, {1, 1}} {
		p := Of(c.saved)
		if math.Abs(p-c.at) > 0.005 {
			t.Errorf("Of(%v) = %.3f, want %.2f", c.saved, p, c.at)
		}
		if g := Gain(p); math.Abs(g-c.saved*c.saved) > 1e-12 {
			t.Errorf("a saved %v plays at %v, want %v", c.saved, g, c.saved*c.saved)
		}
		if v := Saved(p); math.Abs(v-c.saved) > 1e-12 {
			t.Errorf("Saved(Of(%v)) = %v", c.saved, v)
		}
	}
	if v := Saved(0.5); math.Abs(v-0.18) > 0.005 {
		t.Errorf("the middle sends Spotify Connect %.3f, want 0.18", v)
	}
}

// TestOutOfRange keeps what's outside 0 to 1 at its end.
func TestOutOfRange(t *testing.T) {
	if Gain(-1) != 0 || Gain(2) != Gain(1) || Of(-1) != 0 || Of(2) != Of(1) {
		t.Errorf("Gain(-1) %v, Gain(2) %v, Of(-1) %v, Of(2) %v", Gain(-1), Gain(2), Of(-1), Of(2))
	}
}
