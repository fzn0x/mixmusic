package app

// What every scrobbling service's connection shares: its scrobbler, the
// loop that sends what it has waiting, and what the windows hear of it.

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// scrobbling is a scrobbling service's connection: its scrobbler, the
// name it scrobbles as, and whether the service refused the key, which
// a.mu guards.
type scrobbling struct {
	scrobbler *scrobble.Scrobbler
	name      string
	expired   bool
	// service names the service in the log.
	service string
	sends   sendLoop
	waits   waitingTold
}

// sendLoop runs a scrobbler's Run beside the app, from the first play the
// app tells it of until stop.
type sendLoop struct {
	mu      sync.Mutex
	stopped bool
	cancel  context.CancelFunc
	done    chan struct{}
}

// start runs run in a goroutine of its own, once, unless the loop
// stopped.
func (l *sendLoop) start(run func(ctx context.Context)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped || l.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	l.cancel, l.done = cancel, done
	go func() {
		defer close(done)
		run(ctx)
	}()
}

// stop ends the loop and waits for it, so that it writes nothing after;
// it doesn't start again. Its caller holds no lock the loop takes.
func (l *sendLoop) stop() {
	l.mu.Lock()
	l.stopped = true
	cancel, done := l.cancel, l.done
	l.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

// waitingTold decides when the windows hear how many plays wait: at each
// change while sends fail or the service refuses the key, and once none
// wait any more. Plays that go out at once, as they do while the service
// answers, tell nothing.
type waitingTold struct {
	mu      sync.Mutex
	failing bool
	told    int
}

// changed takes in that n plays wait, and reports whether the windows
// should hear it.
func (w *waitingTold) changed(n int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n == w.told || !w.failing && n > 0 {
		return false
	}
	w.told = n
	return true
}

// fail takes in whether the last Send failed and kept the plays.
func (w *waitingTold) fail(failing bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.failing = failing
}

// sendScrobbles has sc's scrobbler send what it has waiting, beside the
// app, from now until sc stops: as plays count, when the network comes
// back, and after a failure, on a timer.
func (a *App) sendScrobbles(sc *scrobbling) {
	sc.sends.start(func(ctx context.Context) {
		if a.online != nil {
			cancel := a.online.OnChange(func(ok bool) {
				if ok {
					sc.scrobbler.Retry()
				}
			})
			defer cancel()
		}
		sc.scrobbler.Run(ctx, func(err error) { a.scrobblesSent(sc, err) })
	})
}

// scrobblesSent takes in what a Send of sc returned.
func (a *App) scrobblesSent(sc *scrobbling, err error) {
	refused := errors.Is(err, scrobble.ErrKeyRefused)
	switch {
	case refused:
		a.mu.Lock()
		told := sc.expired
		sc.expired = true
		a.mu.Unlock()
		if !told {
			notify()
		}
	case scrobble.IsRejected(err):
		log.Printf("mixmusic: %s refused a scrobble: %v", sc.service, err)
	}
	sc.waits.fail(refused || err != nil && !scrobble.IsRejected(err))
	a.tellWaiting(sc)
}

// tellWaiting tells the windows how many plays of sc wait, when they
// should hear it.
func (a *App) tellWaiting(sc *scrobbling) {
	if sc.waits.changed(sc.scrobbler.Waiting()) {
		notify()
	}
}

// userAgent is how MixMusic introduces itself to the services it asks of
// lyrics, parts to skip or scrobbles: with its version and its address.
func (a *App) userAgent() string {
	return "MixMusic/" + a.opts.Version + " (https://github.com/fzn0x/mixmusic)"
}

// stopScrobbling stops the scrobblers' sends, as the app closes.
func (a *App) stopScrobbling() {
	a.mu.Lock()
	l := a.lastfm
	a.mu.Unlock()
	if l != nil {
		l.sends.stop()
	}
}
