package stream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

const (
	// restFirst is how long the direct route rests once YouTube's player
	// API refused pauseAfter streams in a row, the last of them with a new
	// visitor ID too: each rest after, while the refusals go on, lasts
	// twice the one before, up to restMax.
	restFirst = 2 * time.Minute
	restMax   = 30 * time.Minute
	// slowTimeout bounds a try of the direct route after one that didn't
	// answer in its time, so that a route that stopped answering holds
	// each track less before yt-dlp finds it.
	slowTimeout = 2 * time.Second
)

// restState is how the direct route rests, under its mutex.
type restState struct {
	// refused counts the refusals in a row, and the tries that didn't
	// answer in time, since the route last found a stream.
	refused int
	// until is when the rest ends, and last how long it was: 0 once the
	// route found a stream since.
	until time.Time
	last  time.Duration
	// slow is set while the last try didn't answer in time.
	slow bool
}

// Stream returns the audio stream of a video in quality q, once the media
// server answered for it. After pauseAfter refusals in a row, the route
// asks once more with a new visitor ID, then rests: restFirst, and twice
// as long each time it is refused again after a rest, up to restMax,
// until it finds a stream. A try that doesn't answer in time counts as a
// refusal, and the next one waits slowTimeout at most.
func (d *Direct) Stream(ctx context.Context, videoID string, q Quality) (Stream, error) {
	d.mu.Lock()
	resting := d.now().Before(d.rest.until)
	slow := d.rest.slow
	d.mu.Unlock()
	if resting {
		return Stream{}, errDirectPaused
	}
	if slow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, slowTimeout)
		defer cancel()
	}
	s, err := d.stream(ctx, videoID, q)
	if d.count(ctx, err, false) {
		// YouTube may refuse what it gave the visitor ID, rather than the
		// route: once more, with a new one, before resting.
		d.forgetVisitor()
		s, err = d.stream(ctx, videoID, q)
		d.count(ctx, err, true)
	}
	return s, err
}

// count takes in how a try of finding a stream went, retried when it was
// the one asked again with a new visitor ID, and tells whether to ask
// again so. The refusal that makes pauseAfter in a row since a stream was
// last found, and each one after, rest the route once asked again; a try
// that ran out of its time rests it at once, as a new visitor ID wouldn't
// make an answer come.
func (d *Direct) count(ctx context.Context, err error, retried bool) (retry bool) {
	late := err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
	d.mu.Lock()
	defer d.mu.Unlock()
	r := &d.rest
	r.slow = late
	switch {
	case err == nil:
		*r = restState{}
		return false
	case !late && !refusal(err):
		return false
	}
	if !retried {
		r.refused++
	}
	switch {
	case r.refused < pauseAfter:
		return false
	case !late && !retried:
		return true
	}
	now := d.now()
	if now.Before(r.until) {
		return false // another stream rests it already
	}
	r.last = min(max(2*r.last, restFirst), restMax)
	r.until = now.Add(r.last)
	// The try after a rest has its whole time.
	r.slow = false
	why := fmt.Sprintf("refused %d streams in a row, the last with a new visitor ID too", r.refused)
	if late {
		why = fmt.Sprintf("refused or didn't answer %d streams in a row, the last not in time", r.refused)
	}
	log.Printf("mixmusic: YouTube's player API %s; yt-dlp alone finds them for %d minutes", why, int(r.last/time.Minute))
	return false
}

// forgetVisitor drops the route's visitor ID, so that its next request asks
// YouTube for a new one.
func (d *Direct) forgetVisitor() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.visitor = ""
}
