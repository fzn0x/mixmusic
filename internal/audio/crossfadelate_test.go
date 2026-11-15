package audio

import (
	"context"
	"errors"
	"testing"
	"time"
)

// gatedOpener opens files as fileOpener does, but url only once gate is
// closed: its audio comes late, as that of a stream slow to load.
func gatedOpener(url string, gate <-chan struct{}) Opener {
	return func(ctx context.Context, u string, interrupted func() bool) (Source, error) {
		if u == url {
			select {
			case <-gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return fileOpener(ctx, u, interrupted)
	}
}

// newGatedEngine is newTestEngine, whose track at url opens once gate is
// closed.
func newGatedEngine(t *testing.T, url string, gate <-chan struct{}) (*Engine, *fakeDevice, *recorder) {
	dev := &fakeDevice{}
	rec := newRecorder()
	e := NewEngine(gatedOpener(url, gate), dev.start, rec)
	dev.e = e
	e.master = newSmoother(1, glide)
	t.Cleanup(e.Close)
	return e, dev, rec
}

// playDeck renders n frames as fakeDevice.play does, but waits only for
// deck i to hold them, or the rest of its track: the other deck's audio
// hasn't come. The deck may go meanwhile, as one that faded out does.
func playDeck(t *testing.T, e *Engine, dev *fakeDevice, i, n int) []float32 {
	t.Helper()
	dev.mu.Lock()
	render := dev.render
	dev.mu.Unlock()
	if render == nil {
		t.Fatal("no output plays")
	}
	e.mu.Lock()
	d := e.decks[i]
	e.mu.Unlock()
	out := make([]float32, 2*n)
	for at := 0; at < n; at += 480 {
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(100 * time.Microsecond) {
			if r := d.ring.Load(); r.readable() >= 480+quantum || r.done.Load() {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the deck didn't fill")
			}
		}
		render(out[2*at:2*min(at+480, n)], 2)
	}
	return out
}

// waitDone waits until deck i's ring holds the rest of its track.
func waitDone(t *testing.T, e *Engine, i int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.mu.Lock()
		d := e.decks[i]
		e.mu.Unlock()
		if d != nil && d.ring.Load().done.Load() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the deck didn't read its track to its end")
		}
		time.Sleep(time.Millisecond)
	}
}

// checkFade checks got, the frames from the one at which the next track's
// audio came, against a from frame from fading out with ga and b from its
// start fading in with gb.
func checkFade(t *testing.T, got, a, b []float32, from int, ga, gb ramp) {
	t.Helper()
	for f := range len(got) / 2 {
		wa, wb := float32(ga.next()), float32(gb.next())
		for c := range 2 {
			want := float32(0)
			if i := 2*(from+f) + c; i < len(a) {
				want += a[i] * wa
			}
			want += b[2*f+c] * wb
			if got[2*f+c] != want {
				t.Fatalf("frame %d after the next track's audio came: %v, want %v", f, got[2*f+c], want)
			}
		}
	}
}

// TestEngineCrossfadeWaitsForLateTrack crossfades into a track whose audio
// comes after the fade would have ended: the one playing keeps its volume
// until it comes, and the next fades in from silence then, both over the
// whole fade (#549).
func TestEngineCrossfadeWaitsForLateTrack(t *testing.T) {
	a, b := reference(t, cuesFront), reference(t, speech)
	gate := make(chan struct{})
	e, dev, _ := newGatedEngine(t, speech, gate)
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	// Whole numbers of quanta: the fade starts with the next one, and the
	// audio comes between two.
	at := 188 * quantum
	dev.play(t, at)
	const fade = 300 * time.Millisecond
	if err := e.Crossfade(speech, fade, Cue{}); err != nil {
		t.Fatal(err)
	}
	late := 150 * quantum // longer than the fade
	got := playDeck(t, e, dev, 0, late)
	for i := range got {
		if got[i] != a[2*at+i] {
			t.Fatalf("frame %d before the next track's audio came: %v, want %v, at full volume", i/2, got[i], a[2*at+i])
		}
	}
	close(gate)
	buffered(t, e, 1, ringFrames/2)
	got = dev.play(t, int(frames(fade))+4800)
	ga, gb := ramp{value: 1, target: 1}, ramp{}
	ga.to(0, fade)
	gb.to(1, fade)
	checkFade(t, got, a, b, at+late, ga, gb)
}

// TestEngineCrossfadeLateNearTheEnd has the next track's audio come with
// under half a second of the one playing left: it starts at full volume,
// and the one playing fades out over what is left of it (#549).
func TestEngineCrossfadeLateNearTheEnd(t *testing.T) {
	a, b := reference(t, cuesFront), reference(t, speech)
	gate := make(chan struct{})
	e, dev, _ := newGatedEngine(t, speech, gate)
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	at := (len(a)/2 - int(frames(330*time.Millisecond))) / quantum * quantum
	dev.play(t, at)
	const fade = time.Second // longer than what is left
	if err := e.Crossfade(speech, fade, Cue{}); err != nil {
		t.Fatal(err)
	}
	late := 10 * quantum
	playDeck(t, e, dev, 0, late)
	waitDone(t, e, 0)
	close(gate)
	buffered(t, e, 1, ringFrames/2)
	left := len(a)/2 - at - late
	got := dev.play(t, left+4800)
	ga, gb := ramp{value: 1, target: 1}, ramp{value: 1, target: 1}
	ga.to(0, duration(int64(left)))
	checkFade(t, got, a, b, at+late, ga, gb)
}

// TestEngineCrossfadeLateAfterTheEnd has the one playing end before the
// next track's audio comes: it plays to its end at full volume, the fade
// ends with it, and the next track starts at full volume (#549).
func TestEngineCrossfadeLateAfterTheEnd(t *testing.T) {
	a, b := reference(t, cuesFront), reference(t, speech)
	gate := make(chan struct{})
	e, dev, rec := newGatedEngine(t, speech, gate)
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	at := (len(a)/2 - int(frames(200*time.Millisecond))) / quantum * quantum
	dev.play(t, at)
	const fade = 300 * time.Millisecond
	if err := e.Crossfade(speech, fade, Cue{}); err != nil {
		t.Fatal(err)
	}
	got := playDeck(t, e, dev, 0, len(a)/2-at+4800)
	for i := range got {
		want := float32(0)
		if 2*at+i < len(a) {
			want = a[2*at+i]
		}
		if got[i] != want {
			t.Fatalf("frame %d of the end: %v, want %v, at full volume", i/2, got[i], want)
		}
	}
	e.report()
	e.mu.Lock()
	fading, out := e.fading, e.decks[1-e.active]
	e.mu.Unlock()
	if fading || out != nil {
		t.Error("the fade didn't end as the track fading out did")
	}
	close(gate)
	buffered(t, e, 1, ringFrames/2)
	e.mu.Lock()
	pending := len(e.pending) / 2
	e.mu.Unlock()
	got = dev.play(t, 4800)[2*pending:]
	for i := range got {
		if got[i] != b[i] {
			t.Fatalf("frame %d of the next track: %v, want %v, at full volume", i/2, got[i], b[i])
		}
	}
	if rec.has("ended") {
		t.Error("the end of the track fading out ended the playback")
	}
}

// TestEngineCrossfadeIntoFailure fails the track faded into before its
// audio came: the wait for it ends with its failure.
func TestEngineCrossfadeIntoFailure(t *testing.T) {
	dev := &fakeDevice{}
	rec := newRecorder()
	why := errors.New("the network failed while loading the audio")
	e := NewEngine(func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
		if url == "x" {
			return nil, why
		}
		return fileOpener(ctx, url, interrupted)
	}, dev.start, rec)
	dev.e = e
	t.Cleanup(e.Close)
	e.Load(cuesFront, Cue{Autoplay: true})
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, 4800)
	if err := e.Crossfade("x", 300*time.Millisecond, Cue{}); err != nil {
		t.Fatal(err)
	}
	rec.waitFor(t, "error: "+why.Error())
}

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
