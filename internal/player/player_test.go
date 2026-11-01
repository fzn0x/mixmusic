package player

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

func init() {
	failDelay = 20 * time.Millisecond
	reportEvery = 10 * time.Millisecond
	saveEvery = 10 * time.Millisecond
	saveVolumeAfter = 20 * time.Millisecond
}

// fakeServices is the app, with YouTube tracks that play as streams.
type fakeServices struct {
	mu       sync.Mutex
	settings settings.Settings
	// prepare, when set, finds how a track plays.
	prepare func(media.Track) (Source, error)
	// streamErr fails PrepareStream.
	streamErr error
	failure   string
	radio     []media.Track
	played    []string
	reports   []Now
	volumes   []float64
	saved     []Saved
	prepared  []string
}

func newServices() *fakeServices {
	s := settings.Default()
	s.Autoplay = false
	s.Crossfade = 0
	return &fakeServices{settings: s}
}

func streamOf(t media.Track) Source { return Source{Mode: ModeStream, URL: "url:" + t.ID, Track: t} }

func (f *fakeServices) Prepare(_ context.Context, t media.Track) (Source, error) {
	f.mu.Lock()
	f.prepared = append(f.prepared, t.ID)
	prepare := f.prepare
	f.mu.Unlock()
	if prepare != nil {
		return prepare(t)
	}
	return streamOf(t), nil
}

func (f *fakeServices) PrepareStream(_ context.Context, t media.Track) (Source, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.streamErr != nil {
		return Source{}, f.streamErr
	}
	return streamOf(t), nil
}

func (f *fakeServices) StreamFailure(media.Track) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failure
}

func (f *fakeServices) RecordPlay(t media.Track) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.played = append(f.played, t.ID)
}

func (f *fakeServices) Radio(context.Context, media.Track) ([]media.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.radio, nil
}

func (f *fakeServices) Settings() settings.Settings {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.settings
}

func (f *fakeServices) SaveVolume(v float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.volumes = append(f.volumes, v)
}

func (f *fakeServices) Report(now Now) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reports = append(f.reports, now)
}

func (f *fakeServices) SaveQueue(s Saved) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved = append(f.saved, s)
}

func (f *fakeServices) with(fn func(*fakeServices)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

// fakeStream is the engine: it tells the player it plays as the engine
// does, from within Play and Pause.
type fakeStream struct {
	mu     sync.Mutex
	p      *Player
	calls  []string
	paused bool
	url    string
}

func (s *fakeStream) record(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, fmt.Sprintf(format, args...))
}

func (s *fakeStream) Load(url string, start time.Duration, autoplay bool) error {
	s.record("load %s %v %v", url, start, autoplay)
	s.mu.Lock()
	s.url, s.paused = url, true
	s.mu.Unlock()
	if autoplay {
		return s.Play()
	}
	return nil
}

func (s *fakeStream) Preload(url string) { s.record("preload %s", url) }

func (s *fakeStream) Crossfade(url string, d time.Duration) error {
	s.record("crossfade %s %v", url, d)
	return nil
}

func (s *fakeStream) Play() error {
	s.record("play")
	s.mu.Lock()
	was := s.paused
	s.paused = false
	s.mu.Unlock()
	if was {
		s.p.Playing(true)
	}
	return nil
}

func (s *fakeStream) Pause() {
	s.record("pause")
	s.mu.Lock()
	was := s.paused
	s.paused = true
	s.mu.Unlock()
	if !was {
		s.p.Playing(false)
	}
}

func (s *fakeStream) Stop() {
	s.record("stop")
	s.mu.Lock()
	was := s.paused
	s.paused, s.url = true, ""
	s.mu.Unlock()
	if !was {
		s.p.Playing(false)
	}
}

func (s *fakeStream) Unload() {
	s.record("unload")
	s.mu.Lock()
	s.url = ""
	s.mu.Unlock()
}

// sounding is the URL the engine plays, or "".
func (s *fakeStream) sounding() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused {
		return ""
	}
	return s.url
}

func (s *fakeStream) Seek(t time.Duration) { s.record("seek %v", t) }
func (s *fakeStream) SetVolume(v float64)  { s.record("volume %v", v) }
func (s *fakeStream) lastLoad() string     { return s.last("load") }
func (s *fakeStream) has(call string) bool { return slices.Contains(s.copy(), call) }
func (s *fakeStream) count(pre string) int { return len(s.with(pre)) }
func (s *fakeStream) with(pre string) []string {
	var out []string
	for _, c := range s.copy() {
		if strings.HasPrefix(c, pre) {
			out = append(out, c)
		}
	}
	return out
}

func (s *fakeStream) last(pre string) string {
	w := s.with(pre)
	if len(w) == 0 {
		return ""
	}
	return w[len(w)-1]
}

func (s *fakeStream) copy() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

// fakeDevice is a Spotify Connect device, which reports what its commands
// made it play, or what the test sets.
type fakeDevice struct {
	mu      sync.Mutex
	calls   []string
	state   *DeviceState
	playErr error
}

func (d *fakeDevice) record(format string, args ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, fmt.Sprintf(format, args...))
}

func (d *fakeDevice) Play(_ context.Context, t media.Track, at time.Duration) error {
	d.record("play %s %v", t.ID, at)
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.playErr == nil {
		d.state = &DeviceState{Playing: true, Progress: at, Track: &t}
	}
	return d.playErr
}

func (d *fakeDevice) Pause(context.Context) error {
	d.record("pause")
	d.change(func(s *DeviceState) { s.Playing = false })
	return nil
}

func (d *fakeDevice) Resume(context.Context) error {
	d.record("resume")
	d.change(func(s *DeviceState) { s.Playing = true })
	return nil
}

func (d *fakeDevice) Seek(_ context.Context, t time.Duration) error {
	d.record("seek %v", t)
	d.change(func(s *DeviceState) { s.Progress = t })
	return nil
}

// change changes what the device reports, when it plays something.
func (d *fakeDevice) change(fn func(*DeviceState)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.state != nil {
		st := *d.state
		fn(&st)
		d.state = &st
	}
}

// set sets what the device reports.
func (d *fakeDevice) set(s *DeviceState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state = s
}

func (d *fakeDevice) SetVolume(_ context.Context, v float64) error {
	d.record("volume %v", v)
	return nil
}

func (d *fakeDevice) State(context.Context) (*DeviceState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state, nil
}

func (d *fakeDevice) has(call string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Contains(d.calls, call)
}

type harness struct {
	t      *testing.T
	p      *Player
	svc    *fakeServices
	stream *fakeStream
	device *fakeDevice
	mu     sync.Mutex
	notes  []Notice
}

func newHarness(t *testing.T, saved Saved, setup ...func(*fakeServices)) *harness {
	t.Helper()
	h := &harness{t: t, svc: newServices(), stream: &fakeStream{}, device: &fakeDevice{}}
	for _, fn := range setup {
		fn(h.svc)
	}
	h.p = New(h.svc, h.stream, h.device, saved)
	h.stream.p = h.p
	h.p.Notices(func(n Notice) {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.notes = append(h.notes, n)
	})
	t.Cleanup(h.p.Close)
	return h
}

func (h *harness) notices() []Notice {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.notes)
}

// wait waits until ok, which runs on the player's goroutine.
func (h *harness) wait(what string, ok func(*Player) bool) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		done := make(chan bool)
		h.p.do(func() { done <- ok(h.p) })
		if <-done {
			return
		}
		if time.Now().After(deadline) {
			h.device.mu.Lock()
			device := slices.Clone(h.device.calls)
			h.device.mu.Unlock()
			h.t.Fatalf("%s didn't happen; the stream did %q, the device %q", what, h.stream.copy(), device)
		}
		time.Sleep(time.Millisecond)
	}
}

// on runs fn on the player's goroutine, and waits for it.
func (h *harness) on(fn func(*Player)) {
	done := make(chan struct{})
	h.p.do(func() {
		fn(h.p)
		close(done)
	})
	<-done
}

func track(id string, artist ...string) media.Track {
	t := media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: "Song " + id, DurationMs: 200000}
	for _, a := range artist {
		t.Artists = append(t.Artists, media.Ref{Name: a})
	}
	return t
}

func tracks(ids ...string) []media.Track {
	out := make([]media.Track, len(ids))
	for i, id := range ids {
		out[i] = track(id, "Artist "+id)
	}
	return out
}

// playing is the track id playing, once loaded: until then nothing
// plays.
func playing(id string) func(*Player) bool {
	return func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == "yt:"+id && p.s.Playing && !p.s.Buffering &&
			p.s.Source != nil && p.s.Source.Track.ID == cur.Track.ID
	}
}

func TestPlayTracks(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c"), 1)
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
		t.Errorf("loaded %q", got)
	}
	s := h.p.State()
	if len(s.Queue) != 3 || s.Index != 1 || s.Buffering || s.Source == nil || s.Source.URL != "url:yt:b" {
		t.Errorf("state %+v", s)
	}
	h.svc.with(func(f *fakeServices) {
		last := f.reports[len(f.reports)-1]
		if last.Track == nil || last.Track.ID != "yt:b" || !last.Playing {
			t.Errorf("reported %+v", last)
		}
	})
}

func TestTransport(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Toggle()
	h.wait("pausing", func(p *Player) bool { return !p.s.Playing })
	h.p.Toggle()
	h.wait("playing again", playing("a"))
	if n := h.stream.count("load"); n != 1 {
		t.Errorf("playing again loaded the track %d times", n)
	}

	h.p.Next()
	h.wait("b playing", playing("b"))
	// Early in a track, Previous goes to the one before.
	h.p.Previous()
	h.wait("a playing", playing("a"))
	// Later in it, to its start.
	h.p.Time(10*time.Second, 200*time.Second)
	h.p.Previous()
	h.wait("a back at 0", func(p *Player) bool { return p.s.Position == 0 })
	if !h.stream.has("seek 0s") {
		t.Error("Previous didn't seek to the start")
	}

	h.p.Seek(500 * time.Second)
	h.wait("seeking to the end", func(p *Player) bool { return p.s.Position == 200*time.Second })
	h.p.Seek(-time.Second)
	h.wait("seeking to 0", func(p *Player) bool { return p.s.Position == 0 })

	// ToggleOnce takes two presses at once as one.
	h.p.ToggleOnce()
	h.p.ToggleOnce()
	h.wait("pausing", func(p *Player) bool { return !p.s.Playing })
	time.Sleep(50 * time.Millisecond)
	h.on(func(p *Player) {
		if p.s.Playing {
			t.Error("the second toggle played again")
		}
	})
}

// TestSwitchSilences stops the track that played as soon as another is
// chosen, while the other's stream is found, and when it can't be.
func TestSwitchSilences(t *testing.T) {
	// b shows its error for failDelay before c plays: 20 ms, less than
	// wait can take between two looks on a busy runner. Restored after the
	// player closes, as cleanups run last registered first.
	delay := failDelay
	failDelay = 500 * time.Millisecond
	t.Cleanup(func() { failDelay = delay })
	found := make(chan struct{})
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:b" {
				<-found
				return Source{}, errors.New("offline.")
			}
			return streamOf(t), nil
		}
	})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	h.p.Next()
	h.wait("b chosen", func(p *Player) bool { return p.s.Index == 1 })
	if got := h.stream.sounding(); got != "" {
		close(found)
		t.Fatalf("%s still plays while b is found", got)
	}
	h.on(func(p *Player) {
		if !p.s.Buffering || p.s.Position != 0 {
			t.Errorf("finding b: buffering %v, position %v", p.s.Buffering, p.s.Position)
		}
	})
	close(found)
	h.wait("b failing", func(p *Player) bool { return p.s.Error != "" })
	if got := h.stream.sounding(); got != "" {
		t.Errorf("%s plays under b's error", got)
	}
	h.wait("c playing", playing("c"))
}

// TestLoadingPlays has a track that loads play: it can't be paused
// meanwhile, only left for another track.
func TestLoadingPlays(t *testing.T) {
	// a is found once released.
	loading := func(t *testing.T) (*harness, func()) {
		found := make(chan struct{})
		release := sync.OnceFunc(func() { close(found) })
		h := newHarness(t, Saved{}, func(f *fakeServices) {
			f.prepare = func(t media.Track) (Source, error) {
				if t.ID == "yt:a" {
					<-found
				}
				return streamOf(t), nil
			}
		})
		t.Cleanup(release)
		h.p.PlayTracks(tracks("a", "b"), 0)
		h.wait("a loading", func(p *Player) bool { return p.s.Index == 0 && p.s.Loading })
		return h, release
	}

	t.Run("no pause", func(t *testing.T) {
		h, release := loading(t)
		h.p.Toggle()
		h.p.Pause()
		h.p.Play()
		h.p.ToggleOnce()
		// A seek moves where it plays from, and it still plays.
		h.p.Seek(30 * time.Second)
		h.on(func(p *Player) {
			if !p.s.Loading || p.s.Index != 0 {
				t.Errorf("after the presses: loading %v, index %d", p.s.Loading, p.s.Index)
			}
		})
		if h.stream.has("pause") {
			t.Errorf("the load was paused: %q", h.stream.copy())
		}
		release()
		h.wait("a playing", playing("a"))
		if got := h.stream.lastLoad(); got != "load url:yt:a 30s true" {
			t.Errorf("loaded %q", got)
		}
		if h.p.State().Loading {
			t.Error("still loading once it plays")
		}
		// Once it plays, it pauses.
		h.p.Toggle()
		h.wait("pausing", func(p *Player) bool { return !p.s.Playing })
	})

	t.Run("another track", func(t *testing.T) {
		h, release := loading(t)
		h.p.Next()
		h.wait("b playing", playing("b"))
		release()
		time.Sleep(20 * time.Millisecond)
		h.on(func(p *Player) {
			if cur, _ := p.s.Current(); cur.Track.ID != "yt:b" || h.stream.lastLoad() != "load url:yt:b 0s true" {
				t.Errorf("a, found late, took over: %q", h.stream.copy())
			}
		})
	})

	t.Run("failing", func(t *testing.T) {
		h := newHarness(t, Saved{}, func(f *fakeServices) {
			f.prepare = func(media.Track) (Source, error) { return Source{}, errors.New("offline.") }
		})
		h.p.PlayTracks(tracks("a"), 0)
		h.wait("a failing", func(p *Player) bool { return p.s.Error != "" })
		if h.p.State().Loading {
			t.Error("still loading once it failed")
		}
	})
}

func TestEnd(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.wait("b playing", playing("b"))
	// The end of the queue stops there.
	h.p.Ended()
	h.wait("stopping", func(p *Player) bool { return !p.s.Playing && p.s.Position == 0 && p.s.Index == 1 })
	// Play then plays that track again, from its start, which the engine,
	// having ended it, goes to once seeked.
	h.p.Play()
	h.wait("b again from its start", func(*Player) bool {
		calls := h.stream.copy()
		n := len(calls)
		return n >= 2 && calls[n-2] == "seek 0s" && calls[n-1] == "play"
	})

	h.on(func(p *Player) { p.s.Repeat = RepeatAll })
	h.p.Ended()
	h.wait("a playing again", playing("a"))

	h.on(func(p *Player) { p.s.Repeat = RepeatOne })
	loads := h.stream.count("load")
	h.p.Ended()
	h.wait("a once more", func(p *Player) bool { return h.stream.count("load") == loads+1 })
	h.on(func(p *Player) {
		if p.s.Index != 0 {
			t.Errorf("repeat one moved to %d", p.s.Index)
		}
	})
	// A skip moves on even when repeating one.
	h.p.Next()
	h.wait("b playing", playing("b"))
}

// TestEndReportsLoading tells the app of the next track as loading, not
// paused, when one ends: the engine pauses at the end of a track before it
// tells of the end.
func TestEndReportsLoading(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Playing(false)
	h.p.Ended()
	h.wait("b playing", playing("b"))
	h.svc.with(func(f *fakeServices) {
		var b []Now
		for _, r := range f.reports {
			if r.Track != nil && r.Track.ID == "yt:b" {
				b = append(b, r)
			}
		}
		if len(b) < 2 || b[0].Playing || !b[0].Loading {
			t.Fatalf("b was reported as %+v", b)
		}
		if last := b[len(b)-1]; !last.Playing || last.Loading {
			t.Errorf("b playing was reported as %+v", last)
		}
	})
}

// TestFailureReports tells the app that a track that failed doesn't load
// any more.
func TestFailureReports(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(media.Track) (Source, error) { return Source{}, errors.New("offline.") }
	})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a failing", func(p *Player) bool { return p.s.Error != "" })
	h.svc.with(func(f *fakeServices) {
		n := len(f.reports)
		if n < 2 || !f.reports[n-2].Loading {
			t.Fatalf("a loading wasn't reported: %+v", f.reports)
		}
		if last := f.reports[n-1]; last.Track == nil || last.Track.ID != "yt:a" || last.Playing || last.Loading {
			t.Errorf("a failing was reported as %+v", last)
		}
	})
}

// TestPlayID plays the entry of the queue a deep link names, and tells
// why one the queue doesn't hold doesn't play.
func TestPlayID(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.PlayID("yt:b")
	h.wait("b playing", playing("b"))
	h.p.PlayID("yt:z")
	h.wait("the notice", func(*Player) bool {
		n := h.notices()
		return len(n) == 1 && strings.Contains(n[0].Message, "album or playlist")
	})
}

func TestRepeatAllPrevious(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c"), 0)
	h.wait("a playing", playing("a"))
	h.p.CycleRepeat()
	h.p.Previous()
	h.wait("c playing", playing("c"))
	h.p.CycleRepeat()
	h.p.CycleRepeat()
	h.wait("repeat off", func(p *Player) bool { return p.s.Repeat == RepeatOff })
}

func TestAutoplay(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.settings.Autoplay = true
		f.radio = []media.Track{
			track("a", "Artist a"), // queued
			{ID: "yt:av", Title: "Song a (Official Video)", Artists: []media.Ref{{Name: "Artist a"}}, Video: true}, // the same song
			track("x", "X"),
			{ID: "yt:xv", Title: "Song x [Official Music Video]", Artists: []media.Ref{{Name: "X"}}, Video: true},
			track("y", "Y"),
		}
	})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Ended()
	h.wait("x playing", playing("x"))
	s := h.p.State()
	var ids []string
	for _, it := range s.Queue {
		ids = append(ids, it.Track.ID)
		if it.Track.ID != "yt:a" && !it.Radio {
			t.Errorf("%s isn't marked as the radio's", it.Track.ID)
		}
	}
	if !slices.Equal(ids, []string{"yt:a", "yt:x", "yt:y"}) {
		t.Errorf("queue %v", ids)
	}
	// Enqueued tracks go before the radio's.
	h.p.Enqueue(tracks("e"))
	h.wait("e queued", func(p *Player) bool { return len(p.s.Queue) == 4 })
	if s := h.p.State(); s.Queue[2].Track.ID != "yt:e" {
		t.Errorf("enqueued at %v", s.Queue)
	}
}

func TestRadioTracks(t *testing.T) {
	queued := []media.Track{{ID: "yt:1", Title: "Blue", Artists: []media.Ref{{Name: "Ann"}}}}
	found := []media.Track{
		{ID: "yt:1", Title: "Blue", Artists: []media.Ref{{Name: "Ann"}}},
		{ID: "yt:2", Title: "Blue (Lyrics)", Artists: []media.Ref{{Name: "ANN"}}},
		{ID: "yt:3v", Title: "Red (Official Video)", Artists: []media.Ref{{Name: "Bo"}}, Video: true},
		{ID: "yt:3", Title: "Red", Artists: []media.Ref{{Name: "Bo"}}},
		{ID: "yt:4", Title: "Green (Live)", Artists: []media.Ref{{Name: "Cy"}}},
	}
	var ids []string
	for _, tr := range RadioTracks(found, queued) {
		ids = append(ids, tr.ID)
	}
	// The song over its video, in the radio's order; a live version stays.
	if !slices.Equal(ids, []string{"yt:3", "yt:4"}) {
		t.Errorf("kept %v", ids)
	}
	if a, b := SongKey(found[0]), SongKey(found[1]); a != b {
		t.Errorf("song keys %q and %q", a, b)
	}
}

func TestStreamError(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.failure = "YouTube refused it" })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	// The first error plays the stream again, freshly found, where it was.
	h.p.Error("the stream failed")
	h.wait("a retried", func(p *Player) bool { return strings.HasPrefix(h.stream.lastLoad(), "load url:yt:a?retry=") })
	if got := h.stream.lastLoad(); !strings.HasSuffix(got, " 42s true") {
		t.Errorf("retried with %q", got)
	}
	// The second fails, with why the stream tells, and moves on.
	h.p.Error("the stream failed")
	h.wait("b playing", playing("b"))
	n := h.notices()
	if len(n) != 1 || n[0].Message != "Couldn't play Song a: YouTube refused it" || !n[0].Error {
		t.Errorf("notices %+v", n)
	}
}

func TestFailuresStop(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(media.Track) (Source, error) { return Source{}, errors.New("offline.") }
	})
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	h.wait("three failures", func(p *Player) bool { return p.failures == maxFailures })
	time.Sleep(10 * failDelay)
	h.on(func(p *Player) {
		if p.s.Index != 2 || p.s.Playing || p.s.Error != "offline." {
			t.Errorf("after three failures: index %d, playing %v, error %q", p.s.Index, p.s.Playing, p.s.Error)
		}
	})
	n := h.notices()
	if len(n) != 3 || n[2].Message != "Couldn't play Song c: offline. Stopped, as 3 tracks in a row couldn't play" {
		t.Errorf("notices %+v", n)
	}
	// Play tries the track again.
	h.svc.with(func(f *fakeServices) { f.prepare = nil })
	h.p.Play()
	h.wait("c playing", playing("c"))
}

func TestProgress(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b preloaded", func(p *Player) bool { return h.stream.has("preload url:yt:b") })
	played := func() []string {
		var out []string
		h.svc.with(func(f *fakeServices) { out = slices.Clone(f.played) })
		return out
	}
	h.p.Time(31*time.Second, 200*time.Second)
	h.wait("a counted", func(p *Player) bool { return slices.Equal(played(), []string{"yt:a"}) })
	// Once.
	h.p.Time(35*time.Second, 200*time.Second)
	h.on(func(*Player) {})
	time.Sleep(20 * time.Millisecond)
	if got := played(); !slices.Equal(got, []string{"yt:a"}) {
		t.Errorf("played %v", got)
	}
	h.p.Time(196*time.Second, 200*time.Second)
	h.wait("b crossfading in", playing("b"))
	if got := h.stream.last("crossfade"); got != "crossfade url:yt:b 5s" {
		t.Errorf("crossfaded %q", got)
	}
	if got := h.stream.count("load"); got != 1 {
		t.Errorf("%d loads", got)
	}
}

func TestNextTrackFoundAtStart(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	prepared := func() []string {
		var out []string
		h.svc.with(func(f *fakeServices) { out = slices.Clone(f.prepared) })
		return out
	}
	h.p.Time(1500*time.Millisecond, 200*time.Second)
	h.wait("b prepared", func(*Player) bool { return slices.Contains(prepared(), "yt:b") })
	// Once, and without its audio yet.
	h.p.Time(2500*time.Millisecond, 200*time.Second)
	h.on(func(*Player) {})
	if got := prepared(); !slices.Equal(got, []string{"yt:a", "yt:b"}) || h.stream.count("preload") != 0 {
		t.Errorf("prepared %v, %d preloads", got, h.stream.count("preload"))
	}
	h.p.Time(11*time.Second, 200*time.Second)
	h.wait("b preloaded", func(*Player) bool { return h.stream.has("preload url:yt:b") })
}

func TestSlowStartLogged(t *testing.T) {
	var logs strings.Builder
	var mu sync.Mutex
	log.SetOutput(writerFunc(func(b []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return logs.Write(b)
	}))
	saved := slowStart
	slowStart = 50 * time.Millisecond
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		slowStart = saved
	})
	logged := func() string {
		mu.Lock()
		defer mu.Unlock()
		return logs.String()
	}

	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:a" {
				time.Sleep(100 * time.Millisecond)
			}
			return streamOf(t), nil
		}
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(250*time.Millisecond, 200*time.Second)
	h.p.Time(500*time.Millisecond, 200*time.Second)
	h.on(func(*Player) {})
	if got := logged(); strings.Count(got, "yt:a started") != 1 || !strings.Contains(got, "after it was asked to play") {
		t.Errorf("log:\n%s", got)
	}
	h.p.Next()
	h.wait("b playing", playing("b"))
	h.p.Time(250*time.Millisecond, 200*time.Second)
	h.on(func(*Player) {})
	if got := logged(); strings.Contains(got, "yt:b") {
		t.Errorf("a quick start logged:\n%s", got)
	}
}

// writerFunc is an io.Writer of a function.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) { return f(b) }

func TestShortTrackCounts(t *testing.T) {
	h := newHarness(t, Saved{})
	short := track("s", "S")
	short.DurationMs = 40000
	h.p.PlayTracks([]media.Track{short}, 0)
	h.wait("s playing", playing("s"))
	h.p.Time(21*time.Second, 40*time.Second)
	h.wait("s counted", func(p *Player) bool { return p.counted != "" })
}

func TestQueue(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b", "c"), 1)
	h.wait("b playing", playing("b"))
	ids := func() []string {
		var out []string
		for _, it := range h.p.State().Queue {
			out = append(out, strings.TrimPrefix(it.Track.ID, "yt:"))
		}
		return out
	}
	h.p.PlayNext(tracks("n"))
	h.p.Enqueue(tracks("e"))
	h.on(func(*Player) {})
	if got := ids(); !slices.Equal(got, []string{"a", "b", "n", "c", "e"}) {
		t.Fatalf("queue %v", got)
	}
	h.p.RemoveAt(1) // the current one stays
	h.p.RemoveAt(0)
	h.on(func(p *Player) {
		if p.s.Index != 0 {
			t.Errorf("index %d after removing the one before", p.s.Index)
		}
	})
	if got := ids(); !slices.Equal(got, []string{"b", "n", "c", "e"}) {
		t.Fatalf("queue %v", got)
	}
	h.p.Move(0, 3)
	h.on(func(p *Player) {
		if p.s.Index != 3 {
			t.Errorf("the current track moved to %d", p.s.Index)
		}
	})
	if got := ids(); !slices.Equal(got, []string{"n", "c", "e", "b"}) {
		t.Fatalf("queue %v", got)
	}
	h.p.Move(3, 1)
	h.p.ClearUpcoming()
	h.on(func(*Player) {})
	if got := ids(); !slices.Equal(got, []string{"n", "b"}) {
		t.Fatalf("queue %v", got)
	}
}

func TestShuffle(t *testing.T) {
	h := newHarness(t, Saved{})
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	h.p.PlayTracks(tracks(ids...), 2)
	h.wait("c playing", playing("c"))
	order := func() []string {
		var out []string
		for _, it := range h.p.State().Queue {
			out = append(out, strings.TrimPrefix(it.Track.ID, "yt:"))
		}
		return out
	}
	h.p.SetShuffle(true)
	h.on(func(*Player) {})
	got := order()
	if !slices.Equal(got[:3], ids[:3]) || len(got) != len(ids) {
		t.Errorf("shuffling moved what played: %v", got)
	}
	h.p.Enqueue(tracks("z"))
	h.p.SetShuffle(false)
	h.on(func(p *Player) {
		if cur, _ := p.s.Current(); cur.Track.ID != "yt:c" {
			t.Errorf("unshuffling moved the current track to %d", p.s.Index)
		}
	})
	// The order comes back, with what was added since at its end.
	if got := order(); !slices.Equal(got, append(slices.Clone(ids), "z")) {
		t.Errorf("unshuffled %v", got)
	}
	// A list played shuffled starts with its chosen track.
	h.p.SetShuffle(true)
	h.p.PlayTracks(tracks(ids...), 5)
	h.wait("f playing", playing("f"))
	if got := order(); got[0] != "f" || len(got) != len(ids) {
		t.Errorf("played shuffled: %v", got)
	}
	h.p.SetShuffle(false)
	h.on(func(*Player) {})
	if got := order(); !slices.Equal(got, ids) {
		t.Errorf("unshuffled %v", got)
	}
}

func TestVolume(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.SetVolume(0.5)
	h.p.SetVolume(2)
	h.wait("the volume saved", func(*Player) bool {
		var v []float64
		h.svc.with(func(f *fakeServices) { v = slices.Clone(f.volumes) })
		return slices.Equal(v, []float64{1})
	})
	h.p.ToggleMute()
	h.wait("muted", func(p *Player) bool { return p.s.Muted })
	if got := h.stream.last("volume"); got != "volume 0" {
		t.Errorf("muted: %q", got)
	}
	h.p.SetVolume(0.25)
	h.wait("unmuted", func(p *Player) bool { return !p.s.Muted })
	if got := h.stream.last("volume"); got != "volume 0.25" {
		t.Errorf("unmuted: %q", got)
	}
}

func TestRestore(t *testing.T) {
	saved := Saved{Tracks: tracks("a", "b"), Index: 1, Position: 42000, Shuffle: true, Repeat: RepeatOne}
	h := newHarness(t, saved)
	s := h.p.State()
	if len(s.Queue) != 2 || s.Index != 1 || s.Position != 42*time.Second || !s.Shuffle || s.Repeat != RepeatOne || s.Playing {
		t.Fatalf("restored %+v", s)
	}
	// Seeking with nothing loaded holds the track there; Play plays it.
	h.p.Seek(50 * time.Second)
	h.wait("b held", func(p *Player) bool { return h.stream.lastLoad() == "load url:yt:b 50s false" })
	h.p.Play()
	h.wait("b playing", playing("b"))
	h.wait("the queue saved", func(*Player) bool {
		var last Saved
		h.svc.with(func(f *fakeServices) {
			if len(f.saved) > 0 {
				last = f.saved[len(f.saved)-1]
			}
		})
		return len(last.Tracks) == 2 && last.Index == 1 && last.Position == 50000 && last.Repeat == RepeatOne
	})
}

func spotifyTrack(id string) media.Track {
	return media.Track{ID: "spotify:" + id, Source: media.SourceSpotify, Title: "Spotify " + id,
		Artists: []media.Ref{{Name: "Sp"}}, DurationMs: 200000}
}

func onDevice(t media.Track) (Source, error) {
	if t.Source == media.SourceSpotify {
		return Source{Mode: ModeConnect, Track: t}, nil
	}
	return streamOf(t), nil
}

func TestConnect(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = onDevice })
	sp := spotifyTrack("1")
	h.p.PlayTracks([]media.Track{sp, track("b", "B")}, 0)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	if !h.device.has("play spotify:1 0s") || !h.stream.has("stop") {
		t.Errorf("device %v, stream %v", h.device.calls, h.stream.copy())
	}
	// The device reports its progress; a relinked copy is the same track.
	relinked := sp
	relinked.ID = "spotify:relinked"
	h.device.set(&DeviceState{Playing: true, Progress: 100 * time.Second, Track: &relinked})
	h.wait("following the device", func(p *Player) bool { return p.s.Position >= 100*time.Second })
	h.p.Pause()
	h.wait("pausing", func(p *Player) bool { return !p.s.Playing && h.device.has("pause") })
	h.p.Seek(197 * time.Second)
	h.wait("seeking", func(p *Player) bool { return h.device.has("seek 3m17s") })
	h.p.Play()
	h.wait("resuming", func(p *Player) bool { return p.s.Playing && h.device.has("resume") })
	// Near its end, the device moving on ends the track.
	h.wait("ending", func(p *Player) bool { return p.connect.ending })
	h.device.set(&DeviceState{Playing: true, Progress: time.Second, Track: &media.Track{ID: "spotify:other"}})
	h.wait("b playing", playing("b"))
	if h.p.connect.active() {
		t.Error("the device is still followed")
	}
}

func TestConnectLost(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = onDevice })
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("the device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	// Someone plays something else on it, mid-track: twice, it's lost.
	h.device.set(&DeviceState{Playing: true, Progress: 50 * time.Second, Track: &media.Track{ID: "spotify:other"}})
	h.wait("losing the device", func(p *Player) bool { return p.connect.lost && !p.s.Playing })
	// Playing again plays the track there again.
	h.p.Play()
	h.wait("playing it again", func(p *Player) bool { return !p.connect.lost && p.s.Playing })
}

func TestConnectFallsBack(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = onDevice })
	h.device.playErr = errors.New("no Spotify device is active.")
	h.p.PlayTracks([]media.Track{spotifyTrack("1"), spotifyTrack("2")}, 0)
	h.wait("a stream instead", func(p *Player) bool { return h.stream.lastLoad() == "load url:spotify:1 0s true" })
	h.p.Next()
	h.wait("the next one too", func(p *Player) bool { return h.stream.lastLoad() == "load url:spotify:2 0s true" })
	// Told once.
	n := h.notices()
	if len(n) != 1 || n[0].Message != "no Spotify device is active — playing it from YouTube Music instead" || !n[0].Settings {
		t.Errorf("notices %+v", n)
	}
}

// here plays Spotify tracks in MixMusic itself, and the others from their
// stream.
func here(t media.Track) (Source, error) {
	if t.Source == media.SourceSpotify {
		return Source{Mode: ModeSpotify, Track: t}, nil
	}
	return streamOf(t), nil
}

// TestSpotifyHereNowhere plays a Spotify track in MixMusic, where it
// can't: from YouTube Music.
func TestSpotifyHereNowhere(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("a stream instead", func(p *Player) bool { return h.stream.lastLoad() == "load url:spotify:1 0s true" })
	if n := h.notices(); len(n) != 1 || !strings.Contains(n[0].Message, "can't play inside MixMusic") {
		t.Errorf("notices %+v", n)
	}
}

// TestSpotifyHere plays a Spotify track on MixMusic's own device, then
// leaves it in the middle for a stream, which pauses it.
func TestSpotifyHere(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &fakeDevice{}
	h.p.SpotifyHere(own)
	h.p.PlayTracks([]media.Track{spotifyTrack("1"), track("b", "B")}, 0)
	h.wait("MixMusic's device playing", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	if !own.has("play spotify:1 0s") || h.device.has("play spotify:1 0s") {
		t.Errorf("MixMusic's device %v, Spotify Connect's %v", own.calls, h.device.calls)
	}
	h.p.Next()
	h.wait("b playing", playing("b"))
	h.wait("MixMusic's device paused", func(p *Player) bool { return own.has("pause") })
	if h.device.has("pause") {
		t.Errorf("Spotify Connect's device was paused: %v", h.device.calls)
	}
}

func TestSubscribe(t *testing.T) {
	h := newHarness(t, Saved{})
	var mu sync.Mutex
	var got []Event
	cancel := h.p.Subscribe(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e)
	})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	cancel()
	h.p.Pause()
	h.wait("pausing", func(p *Player) bool { return !p.s.Playing })
	mu.Lock()
	defer mu.Unlock()
	for _, want := range []Event{QueueChange, TrackChange, StateChange} {
		if !slices.Contains(got, want) {
			t.Errorf("no event %v in %v", want, got)
		}
	}
	if n := len(got); got[n-1] != StateChange {
		t.Errorf("events after cancel: %v", got)
	}
}
