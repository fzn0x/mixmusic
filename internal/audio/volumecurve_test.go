package audio

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/volume"
)

// TestEngineVolumeCurve plays the volume's position on the curve chosen
// (#383): the middle of the slider on Linear at half the level, then, the
// curve changed as it plays, on Natural at its -30 dB, the position
// staying where it was. Each change glides.
func TestEngineVolumeCurve(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	e.SetVolume(0.5)
	e.SetVolumeCurve(volume.Linear)
	e.Load(cuesFront, Cue{Autoplay: true})
	buffered(t, e, 0, ringFrames/2)
	at := func(got []float32, from int, gain float64, what string) {
		t.Helper()
		// After the glide, half a second in.
		for i := Rate; i < 2*Rate; i++ {
			if d := float64(got[i]) - float64(want[from+i])*gain; d > 1e-6 || d < -1e-6 {
				t.Fatalf("%s, frame %d: %v, want %v", what, (from+i)/2, got[i], float64(want[from+i])*gain)
			}
		}
	}
	at(dev.play(t, Rate), 0, 0.5, "on Linear")
	e.SetVolumeCurve(volume.Natural)
	at(dev.play(t, Rate), 2*Rate, volume.Gain(0.5), "back on Natural")
}

// TestEngineVolumeOnTheCurve plays a volume set after the curve on it.
func TestEngineVolumeOnTheCurve(t *testing.T) {
	e, _, _ := newTestEngine(t)
	e.SetVolumeCurve(volume.Linear)
	e.SetVolume(0.25)
	e.mu.Lock()
	target := e.master.target
	e.mu.Unlock()
	if target != 0.25 {
		t.Errorf("0.25 on Linear plays at %v", target)
	}
	e.SetVolumeCurve(volume.Natural)
	e.mu.Lock()
	target = e.master.target
	e.mu.Unlock()
	if target != volume.Gain(0.25) {
		t.Errorf("0.25 back on Natural plays at %v, not %v", target, volume.Gain(0.25))
	}
}
