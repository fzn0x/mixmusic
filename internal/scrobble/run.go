package scrobble

import (
	"context"
	"errors"
	"time"
)

const (
	// firstRetry is how long Run waits to send the queue again after a
	// Send that failed, twice as long after each failure in a row, up to
	// lastRetry.
	firstRetry = 15 * time.Second
	lastRetry  = 5 * time.Minute
	// sendTimeout bounds each Send of Run.
	sendTimeout = 30 * time.Second
)

// RetryAfter returns how long the service asked to wait before the next
// call, as the RetryAfter method of an error of err's chain says, or 0
// when it asked nothing.
func RetryAfter(err error) time.Duration {
	var w interface{ RetryAfter() time.Duration }
	if errors.As(err, &w) {
		return w.RetryAfter()
	}
	return 0
}

// Retry has the queue sent again at once, as when the network came back,
// unless the service refused the key.
func (s *Scrobbler) Retry() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tryQueue = true
	s.woken()
}

// woken reports whether Send has something to send, and then wakes Run.
// Its caller holds mu.
func (s *Scrobbler) woken() bool {
	if !s.pending() {
		return false
	}
	select {
	case s.wake <- struct{}{}:
	default: // Run is woken already
	}
	return true
}

// Run sends what the Scrobbler has waiting until ctx ends, giving each
// Send 30 seconds: at once when Observe, SetSession, SetQueue or Retry has
// something to send. After a Send that failed with an error that keeps the
// plays, it sends the queue again after the wait the service asked for,
// as RetryAfter tells, or else after 15 seconds, twice as long after each
// failure in a row, up to every 5 minutes; a play the service rejected has
// the rest sent at once. told, unless nil, hears what each Send returned,
// nil when nothing failed.
func (s *Scrobbler) Run(ctx context.Context, told func(error)) {
	var retry <-chan time.Time
	var wait time.Duration
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
		case <-retry:
			retry = nil
			s.mu.Lock()
			s.tryQueue = true
			s.mu.Unlock()
		}
		sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
		err := s.Send(sendCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		switch {
		case err == nil:
			// A Send with nothing to try leaves the wait as it was.
			if len(s.Queue()) == 0 {
				wait, retry = 0, nil
			}
		case errors.Is(err, ErrKeyRefused):
			// Nothing goes until SetSession, which wakes Run.
			wait, retry = 0, nil
		case IsRejected(err):
			s.mu.Lock()
			s.woken()
			s.mu.Unlock()
		default:
			if d := RetryAfter(err); d > 0 {
				retry = s.after(d)
				break
			}
			wait = nextWait(wait)
			retry = s.after(wait)
		}
		if told != nil {
			told(err)
		}
	}
}

// nextWait is the wait after a Send that failed, when the last waited
// wait: 0 is none.
func nextWait(wait time.Duration) time.Duration {
	return min(max(2*wait, firstRetry), lastRetry)
}
