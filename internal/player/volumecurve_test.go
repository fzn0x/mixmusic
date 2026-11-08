package player

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// TestVolumeCurveReachesTheDevice sends the device that plays the volume
// again as the curve changes (#383), for it to play the volume's position
// on the new curve, and leaves the position where it was: the stream, which
// the app gives the curve itself, keeps its volume.
func TestVolumeCurveReachesTheDevice(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = onDevice })
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	h.p.SetVolume(0.4)
	h.wait("the device at 0.4", func(*Player) bool { return h.device.has("volume 0.4") })
	setCurve := func(c volume.Curve) {
		s := h.svc.Settings()
		s.VolumeCurve = c
		h.svc.with(func(f *fakeServices) { f.settings = s })
		h.p.SettingsChanged(s)
	}
	sent := func() int {
		h.device.mu.Lock()
		defer h.device.mu.Unlock()
		n := 0
		for _, c := range h.device.calls {
			if c == "volume 0.4" {
				n++
			}
		}
		return n
	}
	before := sent()
	setCurve(volume.Linear)
	h.wait("the volume sent again", func(*Player) bool { return sent() == before+1 })
	setCurve(volume.Linear) // no change, nothing sent
	h.on(func(*Player) {})
	time.Sleep(20 * time.Millisecond)
	if n := sent(); n != before+1 {
		t.Errorf("the volume was sent %d times more for a curve that didn't change", n-before-1)
	}
	setCurve(volume.Natural)
	h.wait("the volume sent again", func(*Player) bool { return sent() == before+2 })
	if st := h.p.State(); st.Volume != 0.4 || st.Muted {
		t.Errorf("the curve moved the volume to %v, muted %v", st.Volume, st.Muted)
	}

	// Muted, the device hears 0 again, and stays muted.
	h.p.ToggleMute()
	h.wait("muted", func(p *Player) bool { return p.s.Muted })
	setCurve(volume.Linear)
	h.wait("the muted volume sent again", func(*Player) bool { return h.device.has("volume 0") })
	if st := h.p.State(); !st.Muted {
		t.Error("a change of the curve unmuted the player")
	}
}
