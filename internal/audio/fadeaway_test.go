package audio

import (
	"math"
	"testing"
	"time"
)

// TestEngineFadeAway fades the track out on an equal-power curve, apart
// from the volume, holds it silent, and brings it back with FadeBack.
func TestEngineFadeAway(t *testing.T) {
	want := reference(t, cuesFront)
	frameCount := len(want) / 2
	e, dev, _ := newTestEngine(t)
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	// Past the fade in of playing, and the fade over a quarter of the
	// track: the output rendered at most a quantum ahead of the call.
	start := 4800
	n := frameCount / 4
	if start+2*n+4800 > frameCount {
		t.Fatalf("the track is %d frames", frameCount)
	}
	got := dev.play(t, start)
	e.FadeAway(duration(int64(n)))
	got = append(got, dev.play(t, n+4800)...)
	ratio := func(f int) (float64, bool) {
		w := float64(want[2*f])
		if math.Abs(w) < 0.02 {
			return 0, false
		}
		return float64(got[2*f]) / w, true
	}
	checked := 0
	for k := quantum; k < n; k += 97 {
		r, ok := ratio(start + k)
		if !ok {
			continue
		}
		checked++
		hi := math.Cos(math.Pi/2*float64(k-quantum)/float64(n)) + 1e-4
		lo := math.Cos(math.Pi/2*float64(k)/float64(n)) - 1e-4
		if r > hi || r < lo {
			t.Fatalf("%d frames into the fade, the gain is %v, want from %v to %v", k, r, lo, hi)
		}
	}
	if checked < 20 {
		t.Fatalf("only %d frames checked", checked)
	}
	for f := start + n + quantum; f < start+n+4800; f++ {
		if got[2*f] != 0 || got[2*f+1] != 0 {
			t.Fatalf("frame %d after the fade sounds: %v", f, got[2*f])
		}
	}

	// Back at once: the track plays as it is, at the volume.
	e.FadeBack(0)
	from := len(got) / 2
	got = append(got, dev.play(t, 4800)...)
	for f := from + quantum; f < from+4800; f++ {
		if got[2*f] != want[2*f] {
			t.Fatalf("frame %d after FadeBack: %v, want %v", f, got[2*f], want[2*f])
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if g := e.away.gain(); g != 1 {
		t.Errorf("the gain is %v once back", g)
	}
}

// TestFadeAwayBackMidway comes back from where the fade got to, over the
// time asked, and holds no gain past 1.
func TestFadeAwayBackMidway(t *testing.T) {
	var f fadeAway
	f.phase.to(1, 10*time.Millisecond)
	half := int(frames(5 * time.Millisecond))
	var g float64
	for range half {
		g = f.gain()
	}
	if math.Abs(g-math.Cos(math.Pi/4)) > 1e-3 {
		t.Errorf("half way, the gain is %v", g)
	}
	f.phase.to(0, 5*time.Millisecond)
	prev := g
	for range 2 * half {
		g = f.gain()
		if g < prev-1e-12 || g > 1 {
			t.Fatalf("coming back, the gain went from %v to %v", prev, g)
		}
		prev = g
	}
	if g != 1 {
		t.Errorf("back, the gain is %v", g)
	}
}
