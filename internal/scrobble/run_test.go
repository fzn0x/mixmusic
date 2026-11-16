package scrobble

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// waitAPI fails its scrobble calls with the errors it is given, one each,
// and takes them once it has none left.
type waitAPI struct {
	mu   sync.Mutex
	errs []error
	sent int
	keys []string
}

func (f *waitAPI) NowPlaying(context.Context, string, Track) error { return nil }

func (f *waitAPI) Scrobble(_ context.Context, key string, plays []Play) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, key)
	if len(f.errs) > 0 {
		err := f.errs[0]
		f.errs = f.errs[1:]
		return err
	}
	f.sent += len(plays)
	return nil
}

func (f *waitAPI) sentPlays() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent
}

// slowDown is a 429 that asks to wait.
type slowDown time.Duration

func (d slowDown) Error() string             { return "too many requests" }
func (d slowDown) RetryAfter() time.Duration { return time.Duration(d) }

// timer is a wait Run asked for, which the test ends.
type timer struct {
	d    time.Duration
	fire chan time.Time
}

// runner runs a Scrobbler whose waits the test ends, and hears what each
// Send returned.
type runner struct {
	s      *Scrobbler
	timers chan timer
	told   chan error
}

func startRun(t *testing.T, s *Scrobbler) *runner {
	t.Helper()
	r := &runner{s: s, timers: make(chan timer, 10), told: make(chan error, 10)}
	s.after = func(d time.Duration) <-chan time.Time {
		fire := make(chan time.Time, 1)
		r.timers <- timer{d, fire}
		return fire
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, func(err error) { r.told <- err })
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return r
}

// sent waits for the next Send to end, and returns what it returned.
func (r *runner) sent(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.told:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("no Send")
		return nil
	}
}

// waits waits for Run to ask to wait, and checks how long.
func (r *runner) waits(t *testing.T, want time.Duration) timer {
	t.Helper()
	select {
	case tm := <-r.timers:
		if tm.d != want {
			t.Errorf("waits %v, want %v", tm.d, want)
		}
		return tm
	case <-time.After(5 * time.Second):
		t.Fatalf("no wait of %v", want)
		return timer{}
	}
}

// noWait checks that Run asked for no wait.
func (r *runner) noWait(t *testing.T) {
	t.Helper()
	select {
	case tm := <-r.timers:
		t.Errorf("waits %v", tm.d)
	default:
	}
}

func queued(n int) []Play {
	plays := make([]Play, n)
	for i := range plays {
		plays[i] = Play{song("Song", 3*time.Minute), t0.Add(time.Duration(i) * time.Minute)}
	}
	return plays
}

// TestRunTriesAgainLater sends the queue again 15 seconds after a Send
// that failed, then twice as long each time it fails again, and at once
// when told to.
func TestRunTriesAgainLater(t *testing.T) {
	api := &waitAPI{errs: []error{errBusy, errBusy, errBusy, errBusy}}
	s := New(api, "SK", Options{})
	r := startRun(t, s)
	s.SetQueue(queued(2))
	if err := r.sent(t); !errors.Is(err, errBusy) {
		t.Fatalf("err = %v", err)
	}
	tm := r.waits(t, 15*time.Second)
	tm.fire <- time.Now()
	r.sent(t)
	tm = r.waits(t, 30*time.Second)
	tm.fire <- time.Now()
	r.sent(t)
	r.waits(t, time.Minute)
	// The network came back: the queue goes at once, and fails again.
	s.Retry()
	r.sent(t)
	tm = r.waits(t, 2*time.Minute)
	tm.fire <- time.Now()
	if err := r.sent(t); err != nil {
		t.Fatal(err)
	}
	if api.sentPlays() != 2 || len(s.Queue()) != 0 {
		t.Errorf("sent %d, %d waiting", api.sentPlays(), len(s.Queue()))
	}
	r.noWait(t)
}

// TestRunWaitsAsAsked waits as long as the service asked, without
// doubling the wait of the failures before.
func TestRunWaitsAsAsked(t *testing.T) {
	api := &waitAPI{errs: []error{errBusy, slowDown(7 * time.Second), errBusy}}
	s := New(api, "SK", Options{})
	r := startRun(t, s)
	s.SetQueue(queued(1))
	r.sent(t)
	r.waits(t, 15*time.Second).fire <- time.Now()
	r.sent(t)
	r.waits(t, 7*time.Second).fire <- time.Now()
	r.sent(t)
	r.waits(t, 30*time.Second).fire <- time.Now()
	if err := r.sent(t); err != nil || api.sentPlays() != 1 {
		t.Errorf("err = %v, sent %d", err, api.sentPlays())
	}
}

// TestRunWaitsForANewKey sends nothing after the key is refused, without
// a timer, until SetSession gives another.
func TestRunWaitsForANewKey(t *testing.T) {
	api := &waitAPI{errs: []error{ErrKeyRefused}}
	s := New(api, "old", Options{})
	r := startRun(t, s)
	s.SetQueue(queued(3))
	if err := r.sent(t); !errors.Is(err, ErrKeyRefused) {
		t.Fatalf("err = %v", err)
	}
	r.noWait(t)
	s.Retry() // the network coming back changes nothing
	s.SetSession("new")
	for len(s.Queue()) != 0 {
		r.sent(t)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.sent != 3 || api.keys[len(api.keys)-1] != "new" {
		t.Errorf("sent %d, with %q", api.sent, api.keys)
	}
}

// TestRunGoesOnAfterARejectedPlay sends the rest at once once the play at
// fault is dropped.
func TestRunGoesOnAfterARejectedPlay(t *testing.T) {
	api := &pickyAPI{refuse: "Bad"}
	s := New(api, "SK", Options{})
	r := startRun(t, s)
	plays := queued(4)
	plays[1].Title = "Bad"
	s.SetQueue(plays)
	if err := r.sent(t); !IsRejected(err) {
		t.Fatalf("err = %v", err)
	}
	if err := r.sent(t); err != nil {
		t.Fatal(err)
	}
	if len(s.Queue()) != 0 {
		t.Errorf("%d waiting", len(s.Queue()))
	}
	r.noWait(t)
}

// TestNextWait doubles the wait from 15 seconds, up to 5 minutes.
func TestNextWait(t *testing.T) {
	var got []time.Duration
	for w := time.Duration(0); len(got) < 7; {
		w = nextWait(w)
		got = append(got, w)
	}
	want := []time.Duration{15 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("waits %v, want %v", got, want)
			break
		}
	}
}
