package main

import (
	"math"
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
)

// volumeNear is whether the player plays unmuted at the volume v.
func volumeNear(v float64) func(player.State) bool {
	return func(st player.State) bool { return !st.Muted && math.Abs(st.Volume-v) < 1e-9 }
}

// TestVolumeWheel turns the volume with the wheel over the player bar's
// volume: a notch at a time, and a touchpad once it scrolled far enough.
// Turning it down leaves a muted player muted; turning it up unmutes it, a
// step above where it was.
func TestVolumeWheel(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.p.SetVolume(0.5)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Volume == 0.5 })
	tt := tester(t, s)
	r, ok := tt.Find("Volume")
	if !ok {
		t.Fatalf("no volume slider in the player bar: %q", tt.Texts())
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Wheel(x, y, 0, -100)
	s.settle(t, volumeNear(0.55))
	tt.Wheel(x, y, 0, 100)
	tt.Wheel(x, y, 0, 100)
	s.settle(t, volumeNear(0.45))
	tt.Scroll(x, y, 0, volumeScroll*0.6)
	tt.Scroll(x, y, 0, volumeScroll*0.6)
	s.settle(t, volumeNear(0.4))

	s.p.ToggleMute()
	s.settle(t, func(st player.State) bool { return st.Muted })
	tt.Frame()
	tt.Wheel(x, y, 0, 100)
	tt.Wheel(x, y, 0, -100)
	s.settle(t, volumeNear(0.45))
}
