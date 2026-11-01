package audio

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDevice starts outputs that the test renders, frame for frame.
type fakeDevice struct {
	e       *Engine
	mu      sync.Mutex
	render  func([]float32)
	failed  func(error)
	starts  int
	stopped bool
	fail    error // what starting returns, if set
}

func (f *fakeDevice) start(render func([]float32), failed func(error)) (Output, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	f.render, f.failed = render, failed
	f.starts++
	f.stopped = false
	return f, nil
}

func (f *fakeDevice) Stop() {
	f.mu.Lock()
	f.stopped = true
	f.render = nil
	f.mu.Unlock()
}

func (f *fakeDevice) Delay() time.Duration { return 0 }

// play renders n frames, as the device would ask for them, 480 at a time.
// The test renders much faster than the device would: before each, it
// waits for the decks that play to have the frames, as they would by then.
func (f *fakeDevice) play(t *testing.T, n int) []float32 {
	t.Helper()
	f.mu.Lock()
	render := f.render
	f.mu.Unlock()
	if render == nil {
		t.Fatal("no output plays")
	}
	out := make([]float32, 2*n)
	for at := 0; at < n; at += 480 {
		f.waitDecks(t, 480+quantum)
		render(out[2*at : 2*min(at+480, n)])
	}
	return out
}

// waitDecks waits for the decks that play to hold n frames, or their whole
// track.
func (f *fakeDevice) waitDecks(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		ready := true
		f.e.mu.Lock()
		for i, d := range f.e.decks {
			l := f.e.levels[i]
			if d == nil || l.value == 0 && l.target == 0 {
				continue
			}
			r := d.ring.Load()
			ready = ready && (r.readable() >= int64(n) || r.done.Load())
		}
		f.e.mu.Unlock()
		if ready {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the decks didn't fill")
		}
		time.Sleep(100 * time.Microsecond)
	}
}

func (f *fakeDevice) isStopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

// recorder records an engine's events.
type recorder struct {
	mu     sync.Mutex
	events []string
	times  []time.Duration
	wake   chan struct{}
}

func newRecorder() *recorder { return &recorder{wake: make(chan struct{}, 1)} }

func (r *recorder) add(e string) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *recorder) Time(position, duration time.Duration) {
	r.mu.Lock()
	r.times = append(r.times, position)
	r.mu.Unlock()
}
func (r *recorder) Ended()                   { r.add("ended") }
func (r *recorder) Error(message string)     { r.add("error: " + message) }
func (r *recorder) Playing(playing bool)     { r.add(fmt.Sprint("playing ", playing)) }
func (r *recorder) Buffering(buffering bool) { r.add(fmt.Sprint("buffering ", buffering)) }

func (r *recorder) has(e string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Contains(r.events, e)
}

// waitFor waits for an event that starts with prefix.
func (r *recorder) waitFor(t *testing.T, prefix string) string {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		r.mu.Lock()
		for _, e := range r.events {
			if strings.HasPrefix(e, prefix) {
				r.mu.Unlock()
				return e
			}
		}
		r.mu.Unlock()
		select {
		case <-r.wake:
		case <-time.After(10 * time.Millisecond):
		case <-deadline:
			t.Fatalf("no %q among %q", prefix, r.events)
		}
	}
}

// newTestEngine returns an engine at full volume, from the first sample.
func newTestEngine(t *testing.T) (*Engine, *fakeDevice, *recorder) {
	dev := &fakeDevice{}
	rec := newRecorder()
	e := NewEngine(fileOpener, dev.start, rec)
	dev.e = e
	e.master = newSmoother(1, glide)
	t.Cleanup(e.Close)
	return e, dev, rec
}

// buffered waits until the active deck holds n frames, or its whole track.
func buffered(t *testing.T, e *Engine, i, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.mu.Lock()
		d := e.decks[i]
		e.mu.Unlock()
		if d != nil {
			r := d.ring.Load()
			if r.readable() >= int64(n) || r.done.Load() {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("the deck didn't fill")
		}
		time.Sleep(time.Millisecond)
	}
}

// fadeFrames is how many frames playing fades in over.
var fadeFrames = int(frames(pauseFade))

func TestEnginePlaysTrack(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, rec := newTestEngine(t)
	if err := e.Load(cuesFront, 0, false); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	got := dev.play(t, len(want)/2+4800)
	// After the fade in, the track itself, then silence.
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
	if !rec.has("playing true") || !rec.has("playing false") {
		t.Errorf("events %q", rec.events)
	}
	if p := e.Position(); p < duration(int64(len(want)/2))-time.Millisecond {
		t.Errorf("position %v at the end", p)
	}
}

// TestEngineEndsAStreamCutInItsLastMoment tells a track whose stream broke
// off in its last moment ended, not failed, and plays it again from its
// start.
func TestEngineEndsAStreamCutInItsLastMoment(t *testing.T) {
	dev := &fakeDevice{}
	rec := newRecorder()
	e := NewEngine(cutOpener(t, cuesFront, 1000, errors.New("the network failed while loading the audio")), dev.start, rec)
	dev.e = e
	t.Cleanup(e.Close)
	if err := e.Load("x", 0, true); err != nil {
		t.Fatal(err)
	}
	dev.play(t, 4*Rate)
	rec.waitFor(t, "ended")
	rec.mu.Lock()
	for _, ev := range rec.events {
		if strings.HasPrefix(ev, "error: ") {
			t.Errorf("events %q", rec.events)
			break
		}
	}
	rec.mu.Unlock()
	e.Seek(0)
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	if out := dev.play(t, Rate); !slices.ContainsFunc(out, func(s float32) bool { return s != 0 }) {
		t.Error("nothing sounds from the start again")
	}
}

func TestEnginePauseHolds(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, rec := newTestEngine(t)
	e.Load(cuesFront, 0, false)
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	dev.play(t, 9600)
	e.Pause()
	rec.waitFor(t, "playing false")
	tail := dev.play(t, 4800)
	// It fades out, then holds silent.
	for i := 2 * fadeFrames; i < len(tail); i++ {
		if tail[i] != 0 {
			t.Fatalf("frame %d after the pause: %v", i/2, tail[i])
		}
	}
	held := e.Position()
	dev.play(t, 4800)
	if p := e.Position(); p != held {
		t.Errorf("position %v while paused, was %v", p, held)
	}
	at := int(frames(held))
	e.Play()
	got := dev.play(t, 4800)
	// From where it held, after the fade in.
	for i := 2 * fadeFrames; i < len(got); i++ {
		if got[i] != want[2*at+i] {
			t.Fatalf("frame %d after playing again: %v, want %v", i/2, got[i], want[2*at+i])
		}
	}
}

func TestEngineCrossfade(t *testing.T) {
	a, b := reference(t, cuesFront), reference(t, speech)
	e, dev, rec := newTestEngine(t)
	e.Load(cuesFront, 0, false)
	buffered(t, e, 0, ringFrames/2)
	e.Play()
	// A whole number of quanta: the fade starts with the next one.
	at := 188 * quantum
	dev.play(t, at)
	e.Preload(speech)
	buffered(t, e, 1, ringFrames/2)
	const fade = 300 * time.Millisecond
	if err := e.Crossfade(speech, fade); err != nil {
		t.Fatal(err)
	}
	n := int(frames(fade)) + 4800
	got := dev.play(t, n)
	// The same ramps, from the same frames.
	ga, gb := ramp{value: 1, target: 1}, ramp{}
	ga.to(0, fade)
	gb.to(1, fade)
	for f := range n {
		wa, wb := float32(ga.next()), float32(gb.next())
		for c := range 2 {
			want := float32(0) + a[2*(at+f)+c]*wa
			want += b[2*f+c] * wb
			if got[2*f+c] != want {
				t.Fatalf("frame %d of the fade: %v, want %v", f, got[2*f+c], want)
			}
		}
	}
	time.Sleep(fade)
	e.report()
	if rec.has("ended") {
		t.Error("the track that faded out ended the playback")
	}
	if e.Position() != duration(int64(n)) {
		t.Errorf("position %v, want %v", e.Position(), duration(int64(n)))
	}
}

func TestEngineLoadTakesThePreload(t *testing.T) {
	want := reference(t, speech)
	e, dev, _ := newTestEngine(t)
	e.Load(cuesFront, 0, false)
	e.Preload(speech)
	buffered(t, e, 1, ringFrames/2)
	if err := e.Load(speech, 0, true); err != nil {
		t.Fatal(err)
	}
	// No waiting: the preload is there.
	got := dev.play(t, 4800)
	for i := 2 * fadeFrames; i < len(got); i++ {
		if got[i] != want[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], want[i])
		}
	}
}

func TestEngineUnload(t *testing.T) {
	want := reference(t, speech)
	e, dev, rec := newTestEngine(t)
	e.Load(cuesFront, 0, true)
	buffered(t, e, 0, ringFrames/2)
	e.Preload(speech)
	buffered(t, e, 1, ringFrames/2)
	e.mu.Lock()
	preloaded := e.decks[1]
	e.mu.Unlock()
	// A whole number of quanta: nothing rendered waits to play.
	dev.play(t, 188*quantum)
	e.Unload()
	for i, s := range dev.play(t, 4800) {
		if s != 0 {
			t.Fatalf("frame %d after Unload: %v", i/2, s)
		}
	}
	if rec.has("playing false") {
		t.Error("Unload told that the engine paused")
	}
	if p := e.Position(); p != 0 {
		t.Errorf("position %v after Unload", p)
	}
	// The preload stays, for the next track.
	if err := e.Load(speech, 0, true); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	took := e.decks[e.active] == preloaded
	pending := len(e.pending) / 2
	e.mu.Unlock()
	if !took {
		t.Error("Load didn't take the preload")
	}
	got := dev.play(t, 4800)[2*pending:]
	for i := 2 * fadeFrames; i < len(got); i++ {
		if got[i] != want[i] {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], want[i])
		}
	}
}

func TestEngineSeek(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	e.Load(cuesFront, 0, true)
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, 4800)
	e.Seek(2 * time.Second)
	if p := e.Position(); p != 2*time.Second {
		t.Errorf("position %v right after the seek", p)
	}
	// What the engine rendered before the seek plays first.
	e.mu.Lock()
	pending := len(e.pending) / 2
	e.mu.Unlock()
	got := dev.play(t, len(want)/2-2*Rate+4800)
	heard := got[2*pending:]
	for len(heard) > 0 && heard[len(heard)-1] == 0 && heard[len(heard)-2] == 0 {
		heard = heard[:len(heard)-2]
	}
	checkFrom(t, heard, want, 2*Rate, "after the seek")
}

func TestEngineOutputFailures(t *testing.T) {
	t.Run("can't start", func(t *testing.T) {
		e, dev, rec := newTestEngine(t)
		dev.fail = errors.New("no audio output device")
		e.Load(cuesFront, 0, false)
		if err := e.Play(); err == nil {
			t.Fatal("Play played without an output")
		}
		rec.waitFor(t, "error: no audio output device")
	})
	t.Run("stops", func(t *testing.T) {
		e, dev, rec := newTestEngine(t)
		e.Load(cuesFront, 0, true)
		buffered(t, e, 0, 4800)
		dev.play(t, 4800)
		dev.failed(errors.New("the device went away"))
		rec.waitFor(t, "error: "+ErrNoOutput.Error())
		if !dev.isStopped() {
			t.Error("the failed output wasn't stopped")
		}
	})
}

func TestEngineReleasesIdleOutput(t *testing.T) {
	e, dev, _ := newTestEngine(t)
	e.Load(cuesFront, 0, true)
	buffered(t, e, 0, 4800)
	dev.play(t, 4800)
	e.Pause()
	dev.play(t, int(frames(idleTime))+4800)
	deadline := time.Now().Add(2 * time.Second)
	for !dev.isStopped() {
		if time.Now().After(deadline) {
			t.Fatal("the output still plays after a second of silence")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Playing again starts it again.
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	if dev.starts != 2 {
		t.Errorf("%d starts", dev.starts)
	}
}

func TestEngineVolume(t *testing.T) {
	want := reference(t, cuesFront)
	e, dev, _ := newTestEngine(t)
	e.SetVolume(0.5)
	e.Load(cuesFront, 0, true)
	buffered(t, e, 0, ringFrames/2)
	got := dev.play(t, Rate)
	// After the glide, a quarter.
	for i := Rate; i < 2*Rate; i++ {
		if d := float64(got[i]) - float64(want[i])*0.25; d > 1e-6 || d < -1e-6 {
			t.Fatalf("frame %d: %v, want %v", i/2, got[i], want[i]*0.25)
		}
	}
}

func TestEngineStop(t *testing.T) {
	e, dev, rec := newTestEngine(t)
	e.Load(cuesFront, 0, true)
	buffered(t, e, 0, 4800)
	dev.play(t, 4800)
	e.Stop()
	if !dev.isStopped() {
		t.Error("the output plays after Stop")
	}
	rec.waitFor(t, "playing false")
	if e.Position() != 0 {
		t.Errorf("position %v after Stop", e.Position())
	}
}
