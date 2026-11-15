package audio

import (
	"testing"
	"time"
)

// TestEngineCrossfadePausedPastItsEnd pauses a crossfade halfway for
// longer than the fade lasts: the track fading out stays, and fades on
// from where it was once played again (#549).
func TestEngineCrossfadePausedPastItsEnd(t *testing.T) {
	e, dev, _ := newTestEngine(t)
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	dev.play(t, 188*quantum)
	e.Preload(speech)
	buffered(t, e, 1, ringFrames/2)
	const fade = 300 * time.Millisecond
	if err := e.Crossfade(speech, fade, Cue{}); err != nil {
		t.Fatal(err)
	}
	dev.play(t, int(frames(fade))/2)
	e.mu.Lock()
	out := e.decks[1-e.active]
	e.mu.Unlock()
	e.Pause()
	dev.play(t, 4800)
	// The clock passes the fade's end while it is paused.
	kept := func() bool {
		e.report()
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.fading && e.decks[1-e.active] == out
	}
	for paused := time.Now(); time.Since(paused) < 2*fade; time.Sleep(10 * time.Millisecond) {
		if !kept() {
			t.Fatalf("the track fading out was dropped %v into the pause", time.Since(paused).Round(time.Millisecond))
		}
	}
	e.mu.Lock()
	gain, heard := out.gain.value, out.heard()
	e.mu.Unlock()
	if gain <= 0 || gain >= 1 {
		t.Fatalf("the track fading out held at gain %v", gain)
	}
	e.Play()
	dev.play(t, int(frames(fade))+4800)
	e.mu.Lock()
	moved := out.heard() - heard
	e.mu.Unlock()
	if moved < frames(fade)/4 {
		t.Errorf("the track fading out played %d frames on once played again", moved)
	}
	if kept() {
		t.Error("the fade didn't end once the track faded out")
	}
}
