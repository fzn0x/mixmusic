package online

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	fresh = 50 * time.Millisecond
	backoff = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond}
	every = 30 * time.Millisecond
	slowAfter = time.Hour
}

// fakeProbe is a network that answers once up is set, counting the checks;
// gate, when set, holds each check until it closes.
type fakeProbe struct {
	up     atomic.Bool
	checks atomic.Int32
	gate   chan struct{}
}

func (f *fakeProbe) probe(ctx context.Context) error {
	f.checks.Add(1)
	if f.gate != nil {
		select {
		case <-f.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.up.Load() {
		return nil
	}
	return errors.New("no network")
}

// waitAsync runs w.Wait beside the test, and returns what it returned.
func waitAsync(w *Watch, ctx context.Context) <-chan error {
	done := make(chan error, 1)
	go func() { done <- w.Wait(ctx) }()
	return done
}

func within(t *testing.T, what string, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("%s: Wait still waits", what)
		return nil
	}
}

func TestWaitReturnsOnceTheNetworkAnswers(t *testing.T) {
	f := &fakeProbe{}
	f.up.Store(true)
	w := New(f.probe)
	if err := within(t, "online", waitAsync(w, context.Background())); err != nil || f.checks.Load() != 1 {
		t.Errorf("Wait = %v after %d checks", err, f.checks.Load())
	}
	if w.Offline() {
		t.Error("offline after an answer")
	}
}

func TestWaitChecksAgain(t *testing.T) {
	f := &fakeProbe{}
	w := New(f.probe)
	done := waitAsync(w, context.Background())
	// The backoff, then every: several checks while nothing answers.
	deadline := time.Now().Add(5 * time.Second)
	for f.checks.Load() < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("%d checks", f.checks.Load())
		}
		time.Sleep(time.Millisecond)
	}
	if !w.Offline() {
		t.Error("not offline after failed checks")
	}
	f.up.Store(true)
	if err := within(t, "back", done); err != nil {
		t.Errorf("Wait = %v", err)
	}
	if w.Offline() {
		t.Error("offline once back")
	}
}

func TestOfflineForAWhile(t *testing.T) {
	w := New((&fakeProbe{}).probe)
	if w.Offline() || w.Check(context.Background()) || !w.Offline() {
		t.Fatal("not offline after a failed check")
	}
	// An old answer isn't trusted.
	w.mu.Lock()
	w.at = w.at.Add(-offlineFor)
	w.mu.Unlock()
	if w.Offline() {
		t.Error("offline after an old check")
	}
}

func TestKickChecksAtOnce(t *testing.T) {
	f := &fakeProbe{}
	backoff, every := backoff, every
	t.Cleanup(func() { setWaits(backoff, every) })
	setWaits(nil, time.Hour)
	w := New(f.probe)
	done := waitAsync(w, context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for f.checks.Load() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("no check")
		}
		time.Sleep(time.Millisecond)
	}
	f.up.Store(true)
	w.Kick()
	if err := within(t, "kicked", done); err != nil || f.checks.Load() != 2 {
		t.Errorf("Wait = %v after %d checks", err, f.checks.Load())
	}
}

func setWaits(b []time.Duration, e time.Duration) { backoff, every = b, e }

func TestSeenEndsAFailure(t *testing.T) {
	f := &fakeProbe{}
	backoff, every := backoff, every
	t.Cleanup(func() { setWaits(backoff, every) })
	setWaits(nil, time.Hour)
	w := New(f.probe)
	done := waitAsync(w, context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for !w.Offline() {
		if time.Now().After(deadline) {
			t.Fatal("not offline")
		}
		time.Sleep(time.Millisecond)
	}
	// Audio came: the network answers, though the probe doesn't.
	w.Seen()
	if w.Offline() {
		t.Error("offline after audio came")
	}
	if err := within(t, "seen", done); err != nil {
		t.Errorf("Wait = %v", err)
	}
}

func TestChecksAreShared(t *testing.T) {
	f := &fakeProbe{gate: make(chan struct{})}
	f.up.Store(true)
	w := New(f.probe)
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !w.Check(context.Background()) {
				t.Error("Check = false")
			}
		}()
	}
	deadline := time.Now().Add(5 * time.Second)
	for f.checks.Load() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("no check")
		}
		time.Sleep(time.Millisecond)
	}
	close(f.gate)
	wg.Wait()
	if n := f.checks.Load(); n != 1 {
		t.Errorf("%d checks for 5 callers", n)
	}
	// An answer younger than fresh stands for the next.
	if !w.Check(context.Background()) || f.checks.Load() != 1 {
		t.Errorf("%d checks after a fresh answer", f.checks.Load())
	}
	// A Kick asks again.
	w.Kick()
	if !w.Check(context.Background()) || f.checks.Load() != 2 {
		t.Errorf("%d checks after a kick", f.checks.Load())
	}
}

func TestWaitEndsWithItsContext(t *testing.T) {
	f := &fakeProbe{}
	w := New(f.probe)
	ctx, cancel := context.WithCancel(context.Background())
	done := waitAsync(w, ctx)
	cancel()
	if err := within(t, "canceled", done); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait = %v", err)
	}
}

// TestOnChange tells when the network went and came back, once each, as
// checks and audio tell it.
func TestOnChange(t *testing.T) {
	f := &fakeProbe{}
	f.up.Store(true)
	w := New(f.probe)
	var mu sync.Mutex
	var got []bool
	cancel := w.OnChange(func(ok bool) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, ok)
	})
	told := func() []bool {
		mu.Lock()
		defer mu.Unlock()
		return append([]bool(nil), got...)
	}
	ctx := context.Background()
	w.Check(ctx) // up, as at first: nothing to tell
	f.up.Store(false)
	w.Kick()
	w.Check(ctx)
	w.Kick()
	w.Check(ctx) // still gone
	w.Seen()     // audio came
	if g := told(); len(g) != 2 || g[0] || !g[1] {
		t.Errorf("told %v, want [false true]", g)
	}
	cancel()
	w.Kick()
	w.Check(ctx)
	if g := told(); len(g) != 2 {
		t.Errorf("told %v after cancel", g)
	}
}
