package audio

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// Followed records that the engine followed one track with the next, as
// FollowEvents.
func (r *recorder) Followed(url string) { r.add("followed " + url) }

var _ FollowEvents = (*recorder)(nil)

// count is how many of the events start with prefix.
func (r *recorder) count(prefix string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

// floatWAV writes pcm, interleaved stereo at Rate, as a WAV file of 32-bit
// floats at path, which a deck plays sample for sample.
func floatWAV(t *testing.T, path string, pcm []float32) {
	t.Helper()
	data := make([]byte, 44+4*len(pcm))
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+4*len(pcm)))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 3) // WAVE_FORMAT_IEEE_FLOAT
	binary.LittleEndian.PutUint16(data[22:], 2)
	binary.LittleEndian.PutUint32(data[24:], Rate)
	binary.LittleEndian.PutUint32(data[28:], Rate*8)
	binary.LittleEndian.PutUint16(data[32:], 8)
	binary.LittleEndian.PutUint16(data[34:], 32)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(4*len(pcm)))
	for i, v := range pcm {
		binary.LittleEndian.PutUint32(data[44+4*i:], math.Float32bits(v))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// joined is a track and the one to follow it: Opus in WebM, whose lengths
// are whole quanta, and WAV files whose lengths aren't, so that the join
// falls inside a quantum.
type joined struct {
	name, first, next string
	a, b              []float32
}

func joins(t *testing.T) []joined {
	dir := t.TempDir()
	a, b := chirp(100, 5000, 1.0016), sine(330, 0.75)
	first, next := filepath.Join(dir, "chirp.wav"), filepath.Join(dir, "tone.wav")
	floatWAV(t, first, a)
	floatWAV(t, next, b)
	return []joined{
		{"opus", cuesFront, speech, reference(t, cuesFront), reference(t, speech)},
		{"wav", first, next, a, b},
	}
}

// TestEngineFollows plays a preloaded track armed to follow on the very
// sample after the last one of the track before, at 1× and at 1.25×: the
// output is the one track's frames and then the other's, sample for
// sample, as each plays alone at the speed, with no silence between them
// and no fade-in. The engine tells Followed, rather than a pause and an
// end, and the position is the next track's.
func TestEngineFollows(t *testing.T) {
	for _, j := range joins(t) {
		for _, speed := range []float64{1, 1.25} {
			t.Run(fmt.Sprint(j.name, " at ", speed, "x"), func(t *testing.T) {
				first, next := stretch(t, j.a, at(speed)).out, stretch(t, j.b, at(speed)).out
				want := slices.Concat(first, next)
				join := len(first) / 2
				e, dev, rec := newTestEngine(t)
				if err := e.Load(j.first, Cue{Speed: speed}); err != nil {
					t.Fatal(err)
				}
				buffered(t, e, 0, ringFrames/2)
				e.Preload(j.next, Cue{Speed: speed, Follow: true})
				buffered(t, e, 1, ringFrames/2)
				if err := e.Play(); err != nil {
					t.Fatal(err)
				}
				// To a quarter of a second past the join.
				const past = Rate / 4
				got := dev.play(t, join+past)
				for i := 2 * fadeFrames; i < len(got); i++ {
					if got[i] != want[i] {
						t.Fatalf("frame %d, %d from the join: %v, want %v", i/2, i/2-join, got[i], want[i])
					}
				}
				// The last sample of the one, then the first of the other.
				if got[2*join-2] != j.a[len(j.a)-2] || got[2*join] != j.b[0] {
					t.Errorf("the join is %v, %v, not %v, %v", got[2*join-2], got[2*join], j.a[len(j.a)-2], j.b[0])
				}
				e.report()
				rec.waitFor(t, "followed "+j.next)
				if rec.has("ended") || rec.has("playing false") {
					t.Errorf("the join told an end: %q", rec.events)
				}
				e.mu.Lock()
				d, pending := e.decks[e.active], len(e.pending)/2
				e.mu.Unlock()
				if d == nil || d.url != j.next {
					t.Fatal("the next track isn't the one that plays")
				}
				heard := float64(past + pending)
				if p, want := e.Position(), duration(int64(heard*speed)); p < want-25*time.Millisecond || p > want+25*time.Millisecond {
					t.Errorf("position %v, want %v", p, want)
				}
				// The next track plays to its end, which ends the playback.
				got = append(got, dev.play(t, len(next)/2-past+4800)...)
				for i := 2 * (join + past); i < len(want); i++ {
					if got[i] != want[i] {
						t.Fatalf("frame %d of the next track: %v, want %v", i/2-join, got[i], want[i])
					}
				}
				rec.waitFor(t, "ended")
				if n := rec.count("followed"); n != 1 {
					t.Errorf("it followed %d times: %q", n, rec.events)
				}
			})
		}
	}
}

// deckPlays is what a deck plays of the track of path, whole.
func deckPlays(t *testing.T, path string) []float32 {
	t.Helper()
	d := newDeck(path, 0, fileOpener, func() {})
	defer d.close()
	return drain(t, d)
}

// TestEngineFollowsAAC joins YouTube's AAC in MP4 with its Opus in WebM,
// both ways: what a deck plays of the one track and then of the other,
// sample for sample, with no silence between them and no fade-in.
func TestEngineFollowsAAC(t *testing.T) {
	opus, aac := deckPlays(t, cuesFront), deckPlays(t, aacFragments)
	for _, j := range []joined{{"aac into opus", aacFragments, cuesFront, aac, opus}, {"opus into aac", cuesFront, aacFragments, opus, aac}} {
		e, dev, rec := newTestEngine(t)
		e.Load(j.first, Cue{})
		buffered(t, e, 0, ringFrames/2)
		e.Preload(j.next, Cue{Follow: true})
		buffered(t, e, 1, ringFrames/2)
		e.Play()
		join := len(j.a) / 2
		got := dev.play(t, join+Rate/4)
		want := slices.Concat(j.a, j.b[:2*(Rate/4)])
		for i := 2 * fadeFrames; i < len(got); i++ {
			if got[i] != want[i] {
				t.Fatalf("%s: frame %d, %d from the join: %v, want %v", j.name, i/2, i/2-join, got[i], want[i])
			}
		}
		e.report()
		rec.waitFor(t, "followed "+j.next)
	}
}

// TestEngineFollowsAtTheAlbumsLoudness plays a track that follows another
// under Even loudness: at its own loudness from its first frame, or, as a
// song of the same album, at the loudness of the track before, from where
// it glides to its own, over albumGlide.
func TestEngineFollowsAtTheAlbumsLoudness(t *testing.T) {
	// The first track is turned down by 6 dB, the next by 3.
	ta, tb := math.Pow(10, -6.0/20), math.Pow(10, -3.0/20)
	for _, j := range joins(t) {
		for _, album := range []bool{false, true} {
			e, dev := newMeasuredEngine(t)
			e.Load(j.first+"#-8", Cue{})
			buffered(t, e, 0, ringFrames/2)
			e.Preload(j.next+"#-11", Cue{Follow: true, Glide: album})
			buffered(t, e, 1, ringFrames/2)
			e.Play()
			join := len(j.a) / 2
			got := dev.play(t, join+Rate/2)
			for f := fadeFrames; f < join; f++ {
				for c := range 2 {
					if want := j.a[2*f+c] * float32(ta); got[2*f+c] != want {
						t.Fatalf("%s, album %v: frame %d of the first track: %v, want %v", j.name, album, f, got[2*f+c], want)
					}
				}
			}
			// The next track's trim moves through the quantum it starts in, as
			// that of the track before does, from the quantum's start: the one
			// whose last frames end the track before, when they end it.
			trim := newSmoother(tb, glide)
			if album {
				trim = smoother{value: ta, target: tb, k: albumK}
				off := join % quantum
				if off == 0 {
					off = quantum
				}
				for range off {
					trim.next()
				}
			}
			for f := range Rate / 2 {
				g := float32(1 * 1 * trim.next())
				for c := range 2 {
					if want := float32(0) + j.b[2*f+c]*g; got[2*(join+f)+c] != want {
						t.Fatalf("%s, album %v: frame %d of the next track: %v, want %v", j.name, album, f, got[2*(join+f)+c], want)
					}
				}
			}
		}
	}
}

// TestEngineFollowsAnEndToldLate follows a track whose ring ran out a
// moment before its decoder told it ended, as on a slow network: the
// engine's loop doesn't tell the end meanwhile, and the next track follows
// at the next quantum.
func TestEngineFollowsAnEndToldLate(t *testing.T) {
	j := joins(t)[1]
	e, dev, rec := newTestEngine(t)
	e.Load(j.first, Cue{Autoplay: true})
	e.Preload(j.next, Cue{Follow: true})
	buffered(t, e, 0, ringFrames/2)
	buffered(t, e, 1, ringFrames/2)
	// Past the fade in, in whole quanta: nothing rendered waits to play.
	dev.play(t, 38*quantum)
	// What the audio thread would have read, before it saw the end: the
	// whole track, which the ring holds.
	buffered(t, e, 0, ringFrames)
	e.mu.Lock()
	r := e.decks[0].ring.Load()
	r.read.Store(r.write.Load())
	e.mu.Unlock()
	time.Sleep(2 * tick)
	e.report()
	if rec.has("ended") || rec.has("playing false") {
		t.Fatalf("the end was told: %q", rec.events)
	}
	got := dev.play(t, Rate/10)
	for f := range Rate / 10 {
		if got[2*f] != j.b[2*f] || got[2*f+1] != j.b[2*f+1] {
			t.Fatalf("frame %d: %v, want %v", f, got[2*f:2*f+2], j.b[2*f:2*f+2])
		}
	}
	e.report()
	rec.waitFor(t, "followed "+j.next)
}

// TestEngineFollowsArmedAgain arms the track the idle deck holds again,
// without loading it again: at another speed, and to follow no longer, as
// the next track's speed changes, or the queue does. Not armed, the track
// before ends as it always has.
func TestEngineFollowsArmedAgain(t *testing.T) {
	e, dev, rec := newTestEngine(t)
	e.Load(cuesFront, Cue{Autoplay: true})
	buffered(t, e, 0, ringFrames/2)
	e.Preload(speech, Cue{Follow: true})
	e.mu.Lock()
	preloaded := e.decks[1]
	e.mu.Unlock()
	e.Preload(speech, Cue{Speed: 1.5, Follow: true, Glide: true})
	e.mu.Lock()
	again, speed, armed, glide := e.decks[1], preloaded.speed, e.armed, e.glide
	e.mu.Unlock()
	if again != preloaded || speed != 1.5 || !armed || !glide {
		t.Errorf("armed again: the same deck %v, at %v×, armed %v, gliding %v", again == preloaded, speed, armed, glide)
	}
	e.Preload(speech, Cue{})
	dev.play(t, len(reference(t, cuesFront))/2+4800)
	rec.waitFor(t, "ended")
	e.mu.Lock()
	idle, speed := e.decks[1], preloaded.speed
	e.mu.Unlock()
	if rec.has("followed") || idle != preloaded || speed != 1 {
		t.Errorf("disarmed, it followed: %q", rec.events)
	}
	// Load, Unload, Crossfade and Stop disarm what Preload armed.
	for name, change := range map[string]func(){
		"Load":      func() { e.Load(cuesFront, Cue{}) },
		"Unload":    e.Unload,
		"Crossfade": func() { e.Crossfade(cuesFront, time.Second, Cue{}) },
		"Stop":      e.Stop,
	} {
		e.Load(cuesFront, Cue{})
		e.Preload(speech, Cue{Follow: true})
		change()
		e.mu.Lock()
		armed := e.armed
		e.mu.Unlock()
		if armed {
			t.Errorf("%s left the follow armed", name)
		}
	}
}

// TestEngineFollowsNothingInACrossfade crossfades into the track armed to
// follow as into one only preloaded, sample for sample: a crossfade plays
// as it always has, and follows nothing as the track fading out ends.
func TestEngineFollowsNothingInACrossfade(t *testing.T) {
	const fade = 300 * time.Millisecond
	play := func(follow bool) ([]float32, *recorder) {
		e, dev, rec := newTestEngine(t)
		e.Load(cuesFront, Cue{})
		buffered(t, e, 0, ringFrames/2)
		e.Play()
		dev.play(t, 188*quantum)
		e.Preload(speech, Cue{Follow: follow})
		buffered(t, e, 1, ringFrames/2)
		if err := e.Crossfade(speech, fade, Cue{}); err != nil {
			t.Fatal(err)
		}
		got := dev.play(t, int(frames(fade))+4800)
		time.Sleep(fade)
		e.report()
		return got, rec
	}
	want, _ := play(false)
	got, rec := play(true)
	if !slices.Equal(got, want) {
		t.Error("the crossfade into the track armed to follow differs")
	}
	if rec.has("followed") || rec.has("ended") {
		t.Errorf("events %q", rec.events)
	}
}

// TestEngineFollowsNothingAfterAFailure fails a track whose stream breaks
// off, as before: the track armed to follow doesn't start in its place.
func TestEngineFollowsNothingAfterAFailure(t *testing.T) {
	b, err := os.ReadFile(cuesFront)
	if err != nil {
		t.Fatal(err)
	}
	open := func(ctx context.Context, url string, interrupted func() bool) (Source, error) {
		if url == "broken" {
			return &brokenSource{Reader: bytes.NewReader(b), n: int64(len(b) / 2), why: errors.New("the connection dropped")}, nil
		}
		return fileOpener(ctx, url, interrupted)
	}
	dev := &fakeDevice{}
	rec := newRecorder()
	e := NewEngine(open, dev.start, rec)
	dev.e = e
	t.Cleanup(e.Close)
	e.Load("broken", Cue{Autoplay: true})
	e.Preload(speech, Cue{Follow: true})
	buffered(t, e, 1, ringFrames/2)
	dev.play(t, len(reference(t, cuesFront))/2)
	rec.waitFor(t, "error: ")
	if rec.has("followed") {
		t.Errorf("a track that failed was followed: %q", rec.events)
	}
}

// TestEngineFollowsAtAPartThatEnds plays the track armed to follow where
// the track before reaches a part it passes over that reaches its end,
// with no silence: the part, then that the next track followed, are told.
func TestEngineFollowsAtAPartThatEnds(t *testing.T) {
	a, b := reference(t, cuesFront), reference(t, speech)
	length := lengthOf(a)
	from := length - 2*time.Second
	e, dev, rec := newSkipEngine(t)
	e.SetSkips(cuesFront, []Skip{{From: from, To: length}})
	e.Load(cuesFront, Cue{})
	buffered(t, e, 0, ringFrames/2)
	e.Preload(speech, Cue{Follow: true})
	buffered(t, e, 1, ringFrames/2)
	e.Play()
	got := dev.play(t, int(frames(from))+Rate/2)
	// The track before, to the quantum it reached the part in, at most a
	// quantum of it.
	join := (int(frames(from)) + quantum - 1) / quantum * quantum
	for f := fadeFrames; f < join; f++ {
		if got[2*f] != a[2*f] || got[2*f+1] != a[2*f+1] {
			t.Fatalf("frame %d of the track before: %v, want %v", f, got[2*f:2*f+2], a[2*f:2*f+2])
		}
	}
	for f := join; f < len(got)/2; f++ {
		if got[2*f] != b[2*(f-join)] || got[2*f+1] != b[2*(f-join)+1] {
			t.Fatalf("frame %d of the next track: %v, want %v", f-join, got[2*f:2*f+2], b[2*(f-join):2*(f-join)+2])
		}
	}
	e.report()
	rec.waitFor(t, "followed "+speech)
	rec.mu.Lock()
	events := slices.Clone(rec.events)
	rec.mu.Unlock()
	skipped := slices.IndexFunc(events, func(e string) bool {
		return strings.HasPrefix(e, fmt.Sprintf("skipped %s %v ", cuesFront, duration(frames(from))))
	})
	if followed := slices.Index(events, "followed "+speech); skipped < 0 || skipped > followed || slices.Contains(events, "ended") {
		t.Errorf("events %q", events)
	}
}

// TestEngineFollowAllocs takes over from a track that ended without
// allocating: on the audio thread.
func TestEngineFollowAllocs(t *testing.T) {
	e := &Engine{
		eq: newEqualizer(), comp: newCompressor(), master: newSmoother(1, glide), an: newAnalyser(),
		bus: 2, notify: make(chan struct{}, 1), events: newRecorder(),
	}
	ended, next := newRing(0, 2), newRing(0, 2)
	ended.done.Store(true)
	next.writeFrom(sine(440, 0.1), func(<-chan struct{}) bool { return false })
	from, to := &deck{url: "a", speed: 1, trim: newSmoother(0.5, glide), trimmed: true}, &deck{url: "b", speed: 1}
	from.ring.Store(ended)
	to.ring.Store(next)
	if n := testing.AllocsPerRun(100, func() {
		e.decks, e.active, e.followed, e.armed, e.glide = [2]*deck{from, to}, 0, nil, true, true
		e.levels = [2]ramp{{value: 1, target: 1}, {}}
		next.read.Store(0)
		e.renderQuantum()
	}); n != 0 {
		t.Errorf("%v allocations", n)
	}
	if e.active != 1 || e.followed != from || e.quantumBuf[0] == 0 && e.quantumBuf[2] == 0 {
		t.Error("the next track didn't take over")
	}
}
