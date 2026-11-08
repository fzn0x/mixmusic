package volume

import (
	"math"
	"testing"
)

// TestNatural is the curve of Gain, which the volume always had.
func TestNatural(t *testing.T) {
	for p := 0.0; p <= 1; p += 0.01 {
		if Natural.Gain(p) != Gain(p) {
			t.Fatalf("at %.2f, Natural plays %v, not %v", p, Natural.Gain(p), Gain(p))
		}
	}
	// A curve it doesn't know, as from an edit of settings.json, is
	// Natural.
	if Curve("").Gain(0.5) != Gain(0.5) || Curve("loud").Gain(0.5) != Gain(0.5) {
		t.Errorf("an unknown curve plays the middle at %v and %v, not %v", Curve("").Gain(0.5), Curve("loud").Gain(0.5), Gain(0.5))
	}
}

// TestLinear plays each position at its own gain: the middle 6 dB down,
// the top at full and the foot silent.
func TestLinear(t *testing.T) {
	for _, c := range []struct{ p, gain float64 }{{0, 0}, {0.25, 0.25}, {0.5, 0.5}, {0.8, 0.8}, {1, 1}, {-1, 0}, {2, 1}} {
		if g := Linear.Gain(c.p); math.Abs(g-c.gain) > 1e-12 {
			t.Errorf("Linear.Gain(%v) = %v, want %v", c.p, g, c.gain)
		}
	}
	if d := db(Linear.Gain(0.5)); math.Abs(d+6.02) > 0.01 {
		t.Errorf("the middle is %.2f dB, want -6", d)
	}
	// Louder than Natural all the way between the ends, which they share.
	for p := 0.05; p < 1; p += 0.05 {
		if Linear.Gain(p) <= Natural.Gain(p) {
			t.Errorf("at %.2f, Linear plays %v, no louder than Natural's %v", p, Linear.Gain(p), Natural.Gain(p))
		}
	}
}

// TestShare sends a Spotify Connect device the square root of the gain on
// each curve: on Natural, what Saved keeps, as before.
func TestShare(t *testing.T) {
	for p := 0.0; p <= 1; p += 0.05 {
		if Natural.Share(p) != Saved(p) {
			t.Errorf("at %.2f, Natural sends %v, not %v", p, Natural.Share(p), Saved(p))
		}
		if s := Linear.Share(p); math.Abs(s*s-Linear.Gain(p)) > 1e-12 {
			t.Errorf("at %.2f, Linear sends %v for a gain of %v", p, s, Linear.Gain(p))
		}
	}
	if s := Linear.Share(0.5); math.Abs(s-0.707) > 0.001 {
		t.Errorf("Linear's middle sends Spotify Connect %.3f, want 0.707", s)
	}
}

// TestSavedWhateverTheCurve keeps a position as settings.json keeps it
// whatever the curve: changing the curve leaves the slider where it was.
func TestSavedWhateverTheCurve(t *testing.T) {
	for _, p := range []float64{0, 0.3, 0.5, 0.94, 1} {
		if back := Of(Saved(p)); math.Abs(back-p) > 1e-9 {
			t.Errorf("the position %v comes back from settings.json as %v", p, back)
		}
	}
}

// TestCurves offers Natural first, the curve of earlier versions.
func TestCurves(t *testing.T) {
	if len(Curves) != 2 || Curves[0] != Natural || Curves[1] != Linear {
		t.Errorf("Curves = %v", Curves)
	}
}
