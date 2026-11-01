package audio

import (
	"strings"
	"testing"
	"time"
)

// TestEngineSpeed plays a track at 1.5×: as the stretch plays it, sample
// for sample after the fade in, to its end, which ends the track once, at
// its duration.
func TestEngineSpeed(t *testing.T) {
	track := reference(t, cuesFront)
	want := stretch(t, track, at(1.5)).out
	e, dev, rec := newTestEngine(t)
	if err := e.Load(cuesFront, 0, false, 1.5); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	got := dev.play(t, len(want)/2+4800)
	for i := 2 * fadeFrames; i < len(want); i++ {
		if got[i] != want[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], want[i])
		}
	}
	for i := len(want); i < len(got); i++ {
		if got[i] != 0 {
			t.Fatalf("frame %d after the end: %v", i/2, got[i])
		}
	}
	rec.waitFor(t, "ended")
	e.report()
	rec.mu.Lock()
	ended := 0
	for _, ev := range rec.events {
		if ev == "ended" {
			ended++
		}
		if strings.HasPrefix(ev, "buffering true") {
			t.Errorf("it buffered: %q", rec.events)
		}
	}
	rec.mu.Unlock()
	if ended != 1 {
		t.Errorf("it ended %d times", ended)
	}
	if p, end := e.Position(), duration(int64(len(track)/2)); p != end {
		t.Errorf("position %v at the end, want %v", p, end)
	}
}

// TestEngineSpeedSeek seeks at 2×: the track plays on from there, at 2×,
// as the stretch plays what the deck has from there.
func TestEngineSpeedSeek(t *testing.T) {
	// What the deck has from the seek: the track played at 1× from it.
	heard := func(speed float64) []float32 {
		e, dev, _ := newTestEngine(t)
		e.Load(cuesFront, 0, true, speed)
		buffered(t, e, 0, ringFrames/2)
		dev.play(t, 4800)
		e.Seek(time.Second)
		if p := e.Position(); p != time.Second {
			t.Errorf("at %v×, position %v right after the seek", speed, p)
		}
		e.mu.Lock()
		pending := len(e.pending) / 2
		e.mu.Unlock()
		// At 2×, a second of the track that remains plays in half of it.
		n := 3 * Rate
		if speed != 1 {
			n = Rate / 2
		}
		out := dev.play(t, n)[2*pending:]
		if speed != 1 {
			if p, want := e.Position(), time.Second+duration(int64(speed*float64(len(out)/2))); p < want-25*time.Millisecond || p > want+25*time.Millisecond {
				t.Errorf("at %v×, position %v after the seek, want %v", speed, p, want)
			}
		}
		return out
	}
	from := heard(1)
	for len(from) > 0 && from[len(from)-1] == 0 && from[len(from)-2] == 0 {
		from = from[:len(from)-2]
	}
	want := stretch(t, from, at(2)).out
	got := heard(2)
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			t.Fatalf("frame %d after the seek: %v, want %v", i/2, got[i], want[i])
		}
	}
}

// TestEngineCrossfadeSpeeds fades a track at 2× into one at 1×: each plays
// at its own speed, the one fading out too.
func TestEngineCrossfadeSpeeds(t *testing.T) {
	e, dev, _ := newTestEngine(t)
	e.Load(cuesFront, 0, false, 2)
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	dev.play(t, 100*quantum)
	e.Preload(speech)
	buffered(t, e, 1, ringFrames/2)
	e.mu.Lock()
	out := e.decks[e.active]
	before, pending := out.heard(), len(e.pending)/2
	e.mu.Unlock()
	const fade = 300 * time.Millisecond
	if err := e.Crossfade(speech, fade, 1); err != nil {
		t.Fatal(err)
	}
	n := int(frames(fade)) / 2
	dev.play(t, n)
	e.mu.Lock()
	in := e.decks[e.active]
	moved, came := out.heard()-before, in.heard()
	speeds := [2]float64{out.speed, in.speed}
	// The fade starts after what the engine rendered before it, and the
	// engine renders a quantum at a time.
	ahead := len(e.pending)/2 - pending
	e.mu.Unlock()
	if d := moved - int64(2*n); d < -Rate/40 || d > Rate/40 {
		t.Errorf("the track fading out moved on %d frames in %d, want twice as many", moved, n)
	}
	if came != int64(n+ahead) {
		t.Errorf("the track fading in moved on %d frames in %d", came, n)
	}
	if speeds != [2]float64{2, 1} {
		t.Errorf("the decks play at %v", speeds)
	}
}

// TestEngineSetSpeed changes the speed as the track plays, without a gap,
// and while it is paused, which holds it where it was.
func TestEngineSetSpeed(t *testing.T) {
	e, dev, _ := newTestEngine(t)
	e.Load(cuesFront, 0, true, 1)
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, 4800)
	e.SetSpeed(2)
	p0 := e.Position()
	got := dev.play(t, Rate/2)
	// The track sounds throughout: a gap would be a run of silence.
	for f, run := 0, 0; f < len(got)/2; f++ {
		if got[2*f] == 0 && got[2*f+1] == 0 {
			if run++; run > 48 {
				t.Fatalf("silence at frame %d after the change", f)
			}
		} else {
			run = 0
		}
	}
	// What plays is within the search range of where the speed puts it.
	if p := e.Position(); p-p0 < 950*time.Millisecond || p-p0 > 1050*time.Millisecond {
		t.Errorf("500 ms at 2× moved %v on", p-p0)
	}
	e.Pause()
	dev.play(t, 4800)
	held := e.Position()
	e.SetSpeed(0.5)
	dev.play(t, 4800)
	if p := e.Position(); p != held {
		t.Errorf("position %v while paused, was %v", p, held)
	}
	e.Play()
	dev.play(t, Rate)
	if p := e.Position(); p-held < 450*time.Millisecond || p-held > 550*time.Millisecond {
		t.Errorf("a second at 0.5× moved %v on", p-held)
	}
	// Out of range, as no player asks.
	e.SetSpeed(10)
	e.mu.Lock()
	speed := e.decks[e.active].speed
	e.mu.Unlock()
	if speed != MaxSpeed {
		t.Errorf("a speed of 10 plays at %v", speed)
	}
}

// TestEngineLoadsThePreloadAtSpeed takes the track Preload loaded at the
// speed Load asks.
func TestEngineLoadsThePreloadAtSpeed(t *testing.T) {
	e, _, _ := newTestEngine(t)
	e.Load(cuesFront, 0, false, 1)
	e.Preload(speech)
	e.mu.Lock()
	preloaded := e.decks[1]
	e.mu.Unlock()
	if err := e.Load(speech, 0, false, 1.25); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	d := e.decks[e.active]
	e.mu.Unlock()
	if d != preloaded || d.speed != 1.25 {
		t.Errorf("Load took the preload %v, at %v×", d == preloaded, d.speed)
	}
}
