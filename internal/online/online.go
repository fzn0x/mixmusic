// Package online tells whether the network answers: whether YouTube or
// Spotify do, as a check over HTTPS tells, or as audio arriving shows. The
// system's own word, which counts a network behind a captive portal as
// online, only has it check again.
package online

import (
	"context"
	"slices"
	"sync"
	"time"
)

// These are the times a Watch keeps; tests shorten them.
var (
	// fresh is how long a check's answer stands for the checks after it.
	fresh = 2 * time.Second
	// backoff are the waits between the checks of a Wait, in turn, then
	// every; after slowAfter, slow.
	backoff   = []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second}
	every     = 15 * time.Second
	slowAfter = 5 * time.Minute
	slow      = time.Minute
	// checkTimeout bounds a check.
	checkTimeout = 10 * time.Second
	// offlineFor is how long Offline trusts a check that failed: a Wait
	// checks more often than that.
	offlineFor = 2 * time.Minute
)

// Watch tells whether the network answers, checking with its probe when
// asked, as rarely as it can. Its methods are safe for concurrent use.
type Watch struct {
	probe func(ctx context.Context) error

	mu sync.Mutex
	// ok is the last answer, from a check or audio that came, at at, zero
	// before the first; stale is set once it no longer stands for a check.
	ok    bool
	at    time.Time
	stale bool
	// seen is when audio last came.
	seen time.Time
	// check is the check under way, which callers share.
	check *check
	// kick is closed, and replaced, when a Wait should check at once.
	kick chan struct{}
	// told are told when the answer turns.
	told []*func(ok bool)
}

// check is a check of the network, and its answer once done closes.
type check struct {
	done chan struct{}
	ok   bool
}

// New returns a watch that checks with probe, which returns nil when the
// network answers.
func New(probe func(ctx context.Context) error) *Watch {
	return &Watch{probe: probe, kick: make(chan struct{})}
}

// Check reports whether the network answers: from an answer younger than
// fresh, or a check, which concurrent callers share. It is false once ctx
// ends.
func (w *Watch) Check(ctx context.Context) bool { return w.checkSince(ctx, time.Now().Add(-fresh)) }

// checkSince checks unless an answer that came after since stands.
func (w *Watch) checkSince(ctx context.Context, since time.Time) bool {
	w.mu.Lock()
	if !w.at.IsZero() && !w.stale && w.at.After(since) {
		ok := w.ok
		w.mu.Unlock()
		return ok
	}
	c := w.check
	if c == nil {
		c = &check{done: make(chan struct{})}
		w.check = c
		go w.run(c)
	}
	w.mu.Unlock()
	select {
	case <-c.done:
		return c.ok
	case <-ctx.Done():
		return false
	}
}

func (w *Watch) run(c *check) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	err := w.probe(ctx)
	cancel()
	w.mu.Lock()
	// Audio that came meanwhile answers too.
	c.ok = err == nil || w.seen.After(started)
	w.check = nil
	turned := w.at.IsZero() && !c.ok || !w.at.IsZero() && w.ok != c.ok
	w.ok, w.at, w.stale = c.ok, time.Now(), false
	told := w.told
	w.mu.Unlock()
	close(c.done)
	if turned {
		for _, fn := range told {
			(*fn)(c.ok)
		}
	}
}

// OnChange has fn told when the answer turns, from any goroutine: that the
// network went, or came back. The first answer is told when it's that the
// network doesn't answer.
func (w *Watch) OnChange(fn func(ok bool)) (cancel func()) {
	f := &fn
	w.mu.Lock()
	w.told = append(slices.Clone(w.told), f)
	w.mu.Unlock()
	return func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.told = slices.DeleteFunc(slices.Clone(w.told), func(g *func(bool)) bool { return g == f })
	}
}

// Offline reports whether the last answer, within offlineFor, was that the
// network doesn't answer, without checking.
func (w *Watch) Offline() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return !w.at.IsZero() && !w.ok && time.Since(w.at) < offlineFor
}

// Seen takes in audio that came from YouTube: the network answers.
func (w *Watch) Seen() {
	w.mu.Lock()
	now := time.Now()
	back := !w.at.IsZero() && !w.ok
	w.ok, w.at, w.seen, w.stale = true, now, now, false
	told := w.told
	if back {
		w.wake()
	}
	w.mu.Unlock()
	if back {
		for _, fn := range told {
			(*fn)(true)
		}
	}
}

// Kick has the next check ask the network, and a Wait check at once: the
// system told that the network changed, or the computer woke.
func (w *Watch) Kick() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.stale = true
	w.wake()
}

// wake has the Waits check at once. Its caller holds mu.
func (w *Watch) wake() {
	close(w.kick)
	w.kick = make(chan struct{})
}

// Wait returns once the network answers, checking at once, then after
// each of backoff, and every while it doesn't, or slow after slowAfter; a
// Kick checks at once. It returns ctx's error once ctx ends.
func (w *Watch) Wait(ctx context.Context) error {
	start := time.Now()
	// The first check may take the answer of one just before; the later
	// ones an answer that came since the one before, as audio that came.
	since := start.Add(-fresh)
	for i := 0; ; i++ {
		w.mu.Lock()
		kick := w.kick
		w.mu.Unlock()
		if w.checkSince(ctx, since) {
			return nil
		}
		since = time.Now()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		d := every
		switch {
		case time.Since(start) >= slowAfter:
			d = slow
		case i < len(backoff):
			d = backoff[i]
		}
		t := time.NewTimer(d)
		select {
		case <-t.C:
		case <-kick:
		case <-ctx.Done():
		}
		t.Stop()
	}
}
