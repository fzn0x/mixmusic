package main

import "testing"

// A band's number shows the half decibels the slider steps by.
func TestEqGainLabel(t *testing.T) {
	for v, want := range map[float64]string{
		0: "+0", 0.5: "+0.5", -0.5: "-0.5", 1: "+1", 1.5: "+1.5",
		2: "+2", 2.5: "+2.5", -12: "-12", 12: "+12", -11.5: "-11.5",
	} {
		if got := eqGainLabel(v); got != want {
			t.Errorf("eqGainLabel(%v) = %q, want %q", v, got, want)
		}
	}
}

// TestEqualizerHalfDecibels renders the numbers over the bands: the
// preamp's +1.5 and a band's -0.5 show their halves, a whole one doesn't.
func TestEqualizerHalfDecibels(t *testing.T) {
	s := newTestShell(t)
	s.set.Equalizer.Enabled = true
	s.set.Equalizer.Preamp = 1.5
	s.set.Equalizer.Gains = make([]float64, 10)
	s.set.Equalizer.Gains[1], s.set.Equalizer.Gains[2] = -0.5, 3
	tt := testerOf(t, s, 1280, 3600)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"+1.5", "-0.5", "+3"} {
		if !tt.HasText(want) {
			t.Errorf("no number %q: %q", want, tt.Texts())
		}
	}
}
