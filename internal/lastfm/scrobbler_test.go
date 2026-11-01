package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// fakeAPI records the calls of a Scrobbler and fails them as told.
type fakeAPI struct {
	mu sync.Mutex
	// calls lists the calls made, as "nowPlaying <key> <title>" and
	// "scrobble <key> <number of plays>".
	calls []string
	// scrobbled lists the batches of the scrobble calls that succeeded.
	scrobbled [][]Play
	// errs are the errors of the next calls, of either kind; nil succeeds.
	errs []error
}

func (f *fakeAPI) next() error {
	if len(f.errs) == 0 {
		return nil
	}
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

func (f *fakeAPI) NowPlaying(_ context.Context, key string, t Track) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "nowPlaying "+key+" "+t.Title)
	return f.next()
}

func (f *fakeAPI) Scrobble(_ context.Context, key string, plays []Play) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf("scrobble %s %d", key, len(plays)))
	err := f.next()
	if err == nil {
		f.scrobbled = append(f.scrobbled, plays)
	}
	return err
}

func (f *fakeAPI) wantCalls(t *testing.T, want ...string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if !reflect.DeepEqual(f.calls, want) {
		t.Errorf("calls = %q, want %q", f.calls, want)
	}
}

var t0 = time.Date(2026, 10, 19, 20, 0, 0, 0, time.UTC)

func song(title string, d time.Duration) Track {
	return Track{Artist: "Artist", Title: title, Album: "Album", Duration: d}
}

// player drives a Scrobbler as the app does: it observes the player every
// second, and at once when it changes.
type player struct {
	s       *Scrobbler
	now     time.Time
	id      string
	track   Track
	playing bool
	pos     time.Duration
	// rate is the speed it plays at.
	rate float64
}

func newPlayer(s *Scrobbler) *player { return &player{s: s, now: t0, rate: 1} }

func (p *player) observe() bool { return p.s.Observe(p.now, p.id, p.track, p.playing, p.pos, p.rate) }

// load plays id from its beginning.
func (p *player) load(id string, t Track) bool {
	p.id, p.track, p.playing, p.pos = id, t, true, 0
	return p.observe()
}

func (p *player) pause() bool  { p.playing = false; return p.observe() }
func (p *player) resume() bool { p.playing = true; return p.observe() }

func (p *player) seek(pos time.Duration) bool { p.pos = pos; return p.observe() }

// run lets d pass, observing the player every second.
func (p *player) run(d time.Duration) {
	for range d / time.Second {
		p.now = p.now.Add(time.Second)
		if p.playing {
			p.pos += time.Duration(float64(time.Second) * p.rate)
		}
		p.observe()
	}
}

// wantQueue checks the titles and start times of the queued scrobbles.
func wantQueue(t *testing.T, s *Scrobbler, want ...Play) {
	t.Helper()
	if got := s.Queue(); !samePlays(got, want) {
		t.Errorf("queue = %v, want %v", got, want)
	}
}

func samePlays(a, b []Play) bool {
	return len(a) == len(b) && func() bool {
		for i := range a {
			if a[i].Track != b[i].Track || !a[i].Start.Equal(b[i].Start) {
				return false
			}
		}
		return true
	}()
}

func TestScrobblerCountsHalfTheTrack(t *testing.T) {
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	a := song("A", 3*time.Minute)
	p.load("a", a)
	p.run(89 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{a, t0})
	p.run(90 * time.Second)
	wantQueue(t, s, Play{a, t0}) // a play counts once
}

func TestScrobblerCountsFourMinutes(t *testing.T) {
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	long := song("Long", 10*time.Minute)
	p.load("long", long)
	p.run(239 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{long, t0})
}

func TestScrobblerSkipsShortTracks(t *testing.T) {
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	p.load("short", song("Short", 30*time.Second))
	p.run(30 * time.Second)
	wantQueue(t, s)
	b := song("B", 31*time.Second)
	p.load("b", b)
	start := p.now
	p.run(15 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{b, start})
	p.load("unknown", song("Unknown length", 0))
	p.run(5 * time.Minute)
	wantQueue(t, s, Play{b, start})
}

func TestScrobblerDoesNotCountPauses(t *testing.T) {
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	a := song("A", 3*time.Minute)
	p.load("a", a)
	p.run(60 * time.Second)
	p.pause()
	p.run(10 * time.Minute)
	p.resume()
	p.run(29 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{a, t0})
}

func TestScrobblerCapsGaps(t *testing.T) {
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	a := song("A", 3*time.Minute)
	p.load("a", a)
	// The computer sleeps for an hour while the track plays: 5 seconds
	// count.
	p.now = p.now.Add(time.Hour)
	p.observe()
	// The clock goes back: nothing counts.
	p.now = p.now.Add(-time.Hour)
	p.observe()
	p.run(84 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{a, t0})
}

func TestScrobblerStartsOverWithAnotherTrack(t *testing.T) {
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	a := song("A", 3*time.Minute)
	p.load("a", a)
	p.run(60 * time.Second)
	p.load("b", song("B", 3*time.Minute))
	p.run(10 * time.Second)
	p.load("a", a)
	start := p.now
	p.run(60 * time.Second)
	wantQueue(t, s)
	p.run(30 * time.Second)
	wantQueue(t, s, Play{a, start})
}

func TestScrobblerCountsReplays(t *testing.T) {
	api := &fakeAPI{}
	s := NewScrobbler(api, "SK")
	p := newPlayer(s)
	a := song("A", 3*time.Minute)
	ctx := context.Background()
	p.load("a", a)
	p.run(3 * time.Minute)
	wantQueue(t, s, Play{a, t0})
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	// The track repeats: it is announced again and counts again.
	again := p.now
	if !p.seek(0) {
		t.Error("a replay has nothing to send")
	}
	p.run(89 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{a, again})
	// Seeking back within the track is no replay.
	p.seek(time.Minute)
	p.run(2 * time.Minute)
	wantQueue(t, s, Play{a, again})
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	api.wantCalls(t, "nowPlaying SK A", "scrobble SK 1", "nowPlaying SK A", "scrobble SK 1")
}

func TestScrobblerAnnouncesNowPlayingOnce(t *testing.T) {
	api := &fakeAPI{}
	s := NewScrobbler(api, "SK")
	p := newPlayer(s)
	ctx := context.Background()
	p.id, p.track = "a", song("A", 3*time.Minute)
	if p.observe() {
		t.Error("a paused track has something to send")
	}
	if !p.resume() {
		t.Error("a track that starts has nothing to send")
	}
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	p.run(10 * time.Second)
	p.pause()
	p.run(10 * time.Second)
	if p.resume() {
		t.Error("a track that resumes has something to send")
	}
	p.run(10 * time.Second)
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	p.load("b", song("B", 3*time.Minute))
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	api.wantCalls(t, "nowPlaying SK A", "nowPlaying SK B")
}

func TestScrobblerRetriesTheQueue(t *testing.T) {
	api := &fakeAPI{}
	s := NewScrobbler(api, "SK")
	var saved []byte
	s.OnQueue = func(q []Play) {
		var err error
		if saved, err = json.Marshal(q); err != nil {
			t.Error(err)
		}
	}
	ctx := context.Background()

	// The last run left 60 scrobbles, saved as JSON.
	var last []Play
	for i := range 60 {
		last = append(last, Play{song(fmt.Sprint("Old ", i), 3*time.Minute), t0.Add(time.Duration(i-60) * 3 * time.Minute)})
	}
	data, err := json.Marshal(last)
	if err != nil {
		t.Fatal(err)
	}
	var restored []Play
	if err := json.Unmarshal(data, &restored); err != nil || !samePlays(restored, last) {
		t.Fatalf("restored %v, %v", restored, err)
	}
	s.SetQueue(restored)

	p := newPlayer(s)
	a := song("A", 3*time.Minute)
	p.load("a", a)
	p.run(90 * time.Second)
	queue := append(last, Play{a, t0})
	wantQueue(t, s, queue...)
	var got []Play
	if err := json.Unmarshal(saved, &got); err != nil || !samePlays(got, queue) {
		t.Errorf("saved %d plays, %v", len(got), err)
	}

	// Last.fm fails the first batch for a moment: everything stays queued.
	api.errs = []error{nil, &APIError{Code: 16, Message: "There was a temporary error processing your request. Please try again", Status: 200}}
	if err := s.Send(ctx); !IsTemporary(err) {
		t.Fatalf("err = %v", err)
	}
	wantQueue(t, s, queue...)
	// Nothing is sent again before the next contact.
	p.run(30 * time.Second)
	if p.observe() {
		t.Error("something to send after a temporary error")
	}
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	api.wantCalls(t, "nowPlaying SK A", "scrobble SK 50")

	// The next track's now-playing update goes through, and the queue
	// follows in order.
	if !p.load("b", song("B", 3*time.Minute)) {
		t.Error("a new track has nothing to send")
	}
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	api.wantCalls(t, "nowPlaying SK A", "scrobble SK 50", "nowPlaying SK B", "scrobble SK 50", "scrobble SK 11")
	if len(api.scrobbled) != 2 || !samePlays(api.scrobbled[0], queue[:50]) || !samePlays(api.scrobbled[1], queue[50:]) {
		t.Errorf("scrobbled %v", api.scrobbled)
	}
	wantQueue(t, s)
	if err := json.Unmarshal(saved, &got); err != nil || len(got) != 0 {
		t.Errorf("saved %d plays, %v", len(got), err)
	}
}

func TestScrobblerDropsRejectedScrobbles(t *testing.T) {
	api := &fakeAPI{errs: []error{nil, &APIError{Code: 6, Message: "Invalid parameters - Your request is missing a required parameter", Status: 400}}}
	s := NewScrobbler(api, "SK")
	p := newPlayer(s)
	p.load("a", song("A", 3*time.Minute))
	p.run(90 * time.Second)
	var apiErr *APIError
	if err := s.Send(context.Background()); !errors.As(err, &apiErr) || apiErr.Code != 6 {
		t.Fatalf("err = %v", err)
	}
	wantQueue(t, s)
}

func TestScrobblerStopsWhenTheSessionExpires(t *testing.T) {
	api := &fakeAPI{errs: []error{nil, ErrSessionExpired}}
	s := NewScrobbler(api, "SK")
	p := newPlayer(s)
	ctx := context.Background()
	a := song("A", 3*time.Minute)
	p.load("a", a)
	p.run(90 * time.Second)
	if err := s.Send(ctx); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v", err)
	}
	wantQueue(t, s, Play{a, t0}) // kept for the next session

	// It tells once, then neither counts nor sends.
	if p.load("b", song("B", 3*time.Minute)) {
		t.Error("something to send with an expired session")
	}
	p.run(90 * time.Second)
	if err := s.Send(ctx); err != nil {
		t.Errorf("second Send: %v", err)
	}
	wantQueue(t, s, Play{a, t0})
	api.wantCalls(t, "nowPlaying SK A", "scrobble SK 1")

	// The user connects again: the queue goes out with the new key, and
	// the track playing is announced.
	s.SetSession("SK2")
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	if !p.observe() {
		t.Error("the playing track has nothing to send")
	}
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	api.wantCalls(t, "nowPlaying SK A", "scrobble SK 1", "scrobble SK2 1", "nowPlaying SK2 B")
	wantQueue(t, s)
}

// TestScrobblerCountsAtSpeed counts how much of the track was heard: half
// of a song of 4 minutes is heard in a minute at 2×, and in four at 0.5×.
func TestScrobblerCountsAtSpeed(t *testing.T) {
	a := song("A", 4*time.Minute)
	for _, c := range []struct {
		rate  float64
		after time.Duration
	}{{2, time.Minute}, {3, 40 * time.Second}, {0.5, 4 * time.Minute}} {
		s := NewScrobbler(&fakeAPI{}, "SK")
		p := newPlayer(s)
		p.rate = c.rate
		p.load("a", a)
		p.run(c.after - time.Second)
		wantQueue(t, s)
		p.run(time.Second)
		wantQueue(t, s, Play{a, t0})
	}
	// Played from a minute in at 2×, it started half a minute before.
	s := NewScrobbler(&fakeAPI{}, "SK")
	p := newPlayer(s)
	p.rate = 2
	p.id, p.track, p.playing, p.pos = "a", a, true, time.Minute
	p.observe()
	p.run(time.Minute)
	wantQueue(t, s, Play{a, t0.Add(-30 * time.Second)})
	// Half a minute at 1×, then at 3×: what each played counts.
	s = NewScrobbler(&fakeAPI{}, "SK")
	p = newPlayer(s)
	p.load("a", a)
	p.run(30 * time.Second)
	p.rate = 3
	p.observe()
	p.run(29 * time.Second)
	wantQueue(t, s)
	p.run(time.Second)
	wantQueue(t, s, Play{a, t0})
}
