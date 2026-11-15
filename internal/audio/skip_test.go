package audio

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// TestSpans passes over the parts that fit the track, of a second or
// more, cut at its end and joined where they overlap or touch.
func TestSpans(t *testing.T) {
	const s = time.Second
	length := 100 * s
	got := Spans([]Skip{
		{From: 50 * s, To: 60 * s, Of: 100 * s},
		{From: 10 * s, To: 20 * s},
		{From: 15 * s, To: 25 * s, Of: 101 * s},
		{From: 25 * s, To: 30 * s},
		{From: 40 * s, To: 40*s + 500*time.Millisecond},
		{From: 70 * s, To: 80 * s, Of: 97 * s},
		{From: 95 * s, To: 120 * s, Of: 0},
	}, length)
	want := []Skip{{From: 10 * s, To: 30 * s}, {From: 50 * s, To: 60 * s}, {From: 95 * s, To: 100 * s}}
	if !slices.Equal(got, want) {
		t.Errorf("Spans = %v, want %v", got, want)
	}
	if !want[2].Ends(length) || want[1].Ends(length) {
		t.Error("Ends tells the wrong part")
	}
	if got := Spans([]Skip{{From: 0, To: s, Of: 104 * s}}, length); got != nil {
		t.Errorf("a part of a track 4 s longer: %v", got)
	}
	// A track whose length isn't known passes over its parts as they are.
	if got := Spans([]Skip{{From: 0, To: 2 * s, Of: 300 * s}}, 0); !slices.Equal(got, []Skip{{From: 0, To: 2 * s}}) {
		t.Errorf("without a length: %v", got)
	}
}

// skipRecorder records an engine's events and the parts it passed over.
type skipRecorder struct{ *recorder }

func (r skipRecorder) Skipped(url string, from, to time.Duration) {
	r.add(fmt.Sprintf("skipped %s %v %v", url, from, to))
}

// newSkipEngine returns an engine at full volume, from the first sample,
// which tells the parts it passes over.
func newSkipEngine(t *testing.T) (*Engine, *fakeDevice, skipRecorder) {
	dev := &fakeDevice{}
	rec := skipRecorder{newRecorder()}
	e := NewEngine(fileOpener, dev.start, rec)
	dev.e = e
	e.master = newSmoother(1, glide)
	t.Cleanup(e.Close)
	return e, dev, rec
}

// lengthOf is the length of a decoded track, of frames.
func lengthOf(pcm []float32) time.Duration { return duration(int64(len(pcm) / 2)) }

// playedFrom is what an engine plays of the track of url loaded at at, n
// frames of it after its fade in, from fadeFrames past at: what passing
// over a part to at must sound like, as a seek there decodes from a little
// before it.
func playedFrom(t *testing.T, url string, at time.Duration, n int) []float32 {
	t.Helper()
	e, dev, _ := newTestEngine(t)
	if err := e.Load(url, Cue{Start: at}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	return dev.play(t, n+fadeFrames)[2*fadeFrames:]
}

// TestEngineStartsPastAPart starts a track that opens with a part at the
// part's end, with no gap, and tells it passed over it.
func TestEngineStartsPastAPart(t *testing.T) {
	want := playedFrom(t, cuesFront, time.Second, Rate/2)
	e, dev, rec := newSkipEngine(t)
	e.SetSkips(cuesFront, []Skip{{From: 0, To: time.Second}})
	if err := e.Load(cuesFront, Cue{}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	got := dev.play(t, Rate/2+fadeFrames)[2*fadeFrames:]
	if !slices.Equal(got, want) {
		t.Error("it doesn't start as the track loaded at the part's end does")
	}
	rec.waitFor(t, "skipped "+cuesFront+" 0s 1s")
}

// TestEngineSkipsAPartItReaches passes over a part the engine is told of
// as the track plays, as it reaches it: what it rendered of the part is a
// quantum at most, then silence, then the track from the part's end.
// Seeking back into the part plays it.
func TestEngineSkipsAPartItReaches(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, rec := newSkipEngine(t)
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, 480*10)
	// A late answer.
	from, to := time.Second, 2*time.Second
	e.SetSkips(cuesFront, []Skip{{From: from, To: to}})
	start := 480 * 10
	f := int(frames(from)) - start
	// Up to the part, and a chunk of it, which the deck holds at, until
	// the loop moved it past.
	got := dev.play(t, f+480)
	rec.waitFor(t, fmt.Sprintf("skipped %s %v %v", cuesFront, from, to))
	got = append(got, dev.play(t, Rate/2)...)
	if s := snr(got[2*2*fadeFrames:2*f], want[2*(start+2*fadeFrames):2*(start+f)]); s < 40 {
		t.Errorf("SNR %.1f dB before the part", s)
	}
	after := got[2*(f+quantum):]
	gap := 0
	for gap < len(after)/2 && after[2*gap] == 0 && after[2*gap+1] == 0 {
		gap++
	}
	if gap > 480+quantum {
		t.Fatalf("%d frames of silence at the part", gap)
	}
	rest := after[2*gap:]
	if s := snr(rest[2*fadeFrames:2*(fadeFrames+Rate/4)], playedFrom(t, cuesFront, to, Rate/4)); s < 100 {
		t.Errorf("SNR %.1f dB after the part, %d frames of silence on, against the track loaded at its end", s, gap)
	}
	// Back into the part, which played once, it plays.
	e.Seek(from)
	e.mu.Lock()
	pending := len(e.pending) / 2
	e.mu.Unlock()
	back := dev.play(t, Rate/4+pending+fadeFrames)[2*(pending+fadeFrames):]
	if s := snr(back, playedFrom(t, cuesFront, from, Rate/4)); s < 100 {
		t.Errorf("SNR %.1f dB back in the part, against the track loaded there", s)
	}
}

// TestEngineEndsAtAPartThatEndsTheTrack ends a track as it reaches a part
// that reaches its end, without playing it.
func TestEngineEndsAtAPartThatEndsTheTrack(t *testing.T) {
	want := reference(t, cuesFront)
	length := lengthOf(want)
	e, dev, rec := newSkipEngine(t)
	from := length - time.Second - 500*time.Millisecond
	e.SetSkips(cuesFront, []Skip{{From: from, To: length - 500*time.Millisecond}})
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	got := dev.play(t, len(want)/2)
	rec.waitFor(t, "ended")
	rec.waitFor(t, "skipped "+cuesFront)
	at := int(frames(from)) + quantum
	for i := 2 * at; i < len(got); i++ {
		if got[i] != 0 {
			t.Fatalf("frame %d of the part plays", i/2)
		}
	}
	if p := e.Position(); p > from+time.Duration(quantum)*time.Second/Rate {
		t.Errorf("position %v, past the part's start %v", p, from)
	}
	// Played again from its start, the track plays.
	e.Seek(0)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	if out := dev.play(t, Rate/2); !slices.ContainsFunc(out, func(s float32) bool { return s != 0 }) {
		t.Error("nothing sounds from the start again")
	}
}

// TestEngineLoadTakesAPreloadPastAPart plays a preloaded track that opens
// with a part from the part's end at once, as Load takes the preload.
func TestEngineLoadTakesAPreloadPastAPart(t *testing.T) {
	want := playedFrom(t, speech, time.Second, Rate/4)
	e, dev, rec := newSkipEngine(t)
	e.SetSkips(speech, []Skip{{From: 0, To: time.Second}})
	e.Load(cuesFront, Cue{})
	e.Preload(speech)
	buffered(t, e, 1, ringFrames/2)
	e.mu.Lock()
	preloaded := e.decks[1]
	e.mu.Unlock()
	if err := e.Load(speech, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	took := e.decks[e.active] == preloaded
	e.mu.Unlock()
	if !took {
		t.Fatal("Load didn't take the preload")
	}
	got := dev.play(t, Rate/4+fadeFrames)[2*fadeFrames:]
	if !slices.Equal(got, want) {
		t.Error("it doesn't start as the track loaded at the part's end does")
	}
	rec.waitFor(t, "skipped "+speech+" 0s 1s")
}

// TestEngineSkipsLeaveOutWhatDoesntFit passes over no part marked on a
// track of another length, nor one shorter than minSkip.
func TestEngineSkipsLeaveOutWhatDoesntFit(t *testing.T) {
	want := reference(t, cuesFront)
	length := lengthOf(want)
	e, dev, rec := newSkipEngine(t)
	e.SetSkips(cuesFront, []Skip{{From: 0, To: time.Second, Of: length + 3*time.Second}, {From: time.Second, To: 1500 * time.Millisecond}})
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	got := dev.play(t, 2*Rate)
	for i := 2 * fadeFrames; i < len(got); i++ {
		if got[i] != want[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], want[i])
		}
	}
	if rec.has("skipped") {
		t.Errorf("events %q", rec.events)
	}
}

// TestEngineCrossfadesPastAPart fades into a track that opens with a part
// from the part's end.
func TestEngineCrossfadesPastAPart(t *testing.T) {
	e, dev, rec := newSkipEngine(t)
	e.SetSkips(speech, []Skip{{From: 0, To: time.Second}})
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	dev.play(t, 4800)
	if err := e.Crossfade(speech, 100*time.Millisecond, Cue{}); err != nil {
		t.Fatal(err)
	}
	dev.play(t, Rate/2)
	rec.waitFor(t, "skipped "+speech+" 0s 1s")
	if p := e.Position(); p < time.Second {
		t.Errorf("position %v of the track faded into", p)
	}
}

// wholeSource is a file's Source that records being asked to keep its
// download whole.
type wholeSource struct {
	Source
	kept *atomic.Bool
}

func (s wholeSource) KeepWhole() { s.kept.Store(true) }

// TestDeckKeepsTheDownloadWhole asks the source of a track the deck passes
// over parts of to go on downloading it from its start, and not that of a
// track it passes over nothing of.
func TestDeckKeepsTheDownloadWhole(t *testing.T) {
	for _, skips := range [][]Skip{nil, {{From: time.Second, To: 2 * time.Second}}} {
		var kept atomic.Bool
		open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
			src, err := fileOpener(ctx, url, interrupted)
			return wholeSource{src, &kept}, err
		}
		d := newDeckSkipping(cuesFront, 0, skips, open, func() {})
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, ready := d.info(); ready {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the deck didn't open the track")
			}
			time.Sleep(time.Millisecond)
		}
		if kept.Load() != (skips != nil) {
			t.Errorf("parts %v: kept whole %v", skips, kept.Load())
		}
		d.close()
	}
}
