// Package scrobble decides which plays count as scrobbles, by the rules
// of Last.fm that every scrobbling service of MixMusic follows, and sends
// them to a service with its now-playing updates, keeping those it has
// not taken yet in a queue.
package scrobble

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
	"time"
)

// Service is the part of a scrobbling service's API a Scrobbler calls,
// with the key of the user's session or token. Its errors tell what
// becomes of the plays: ErrKeyRefused, IsRejected or neither.
type Service interface {
	NowPlaying(ctx context.Context, key string, t Track) error
	Scrobble(ctx context.Context, key string, plays []Play) error
}

// Options are what tells services apart. The zero value is Last.fm's.
type Options struct {
	// Batch is the most plays one Scrobble call takes; 0 is 50.
	Batch int
}

const (
	// minLength is the length a track must exceed to be scrobbled.
	minLength = 30 * time.Second
	// maxListen is the listening time after which a track counts as a
	// scrobble, however long it is.
	maxListen = 4 * time.Minute
	// maxGap is the most time between two observations that counts as
	// listening, so that sleep or a stalled player do not.
	maxGap = 5 * time.Second
	// startWindow is how close to its beginning a track must be, after it
	// had played further, to be played again.
	startWindow = 5 * time.Second
	// maxQueue is how many scrobbles wait at most, the oldest dropped
	// first, so that a refused session cannot grow the saved queue
	// without end.
	maxQueue = 5000
	// defaultBatch is the most plays a Scrobble call takes, unless the
	// Options say otherwise: Last.fm's.
	defaultBatch = 50
)

// Scrobbler decides, from what the player reports, which plays count as
// scrobbles, and sends them to a service with now-playing updates. A track
// longer than 30 seconds counts once it has been listened to for half its
// duration or for four minutes, whichever comes first, and each play of it
// counts once.
//
// The app calls Observe about once a second and on every change of the
// player, and runs Run beside it, which calls Send whenever something
// waits to be sent. Scrobbles wait in a queue until the service accepts
// them; the app saves the queue with OnQueue and restores it with
// SetQueue. The methods of a Scrobbler are safe for concurrent use.
type Scrobbler struct {
	// OnQueue is called with the scrobbles waiting to be sent each time they
	// change, to save them for the next run. It is called while the
	// Scrobbler is locked, so it must not call the Scrobbler.
	OnQueue func([]Play)

	api   Service
	batch int

	mu         sync.Mutex
	session    string
	expired    bool
	cur        listen
	nowPlaying *Track
	queue      []Play
	// gen counts the calls of SetQueue, so that Send does not trim a queue
	// that was replaced while it sent the head of the former one.
	gen int
	// tryQueue is set when Send should send the queue: a scrobble joined
	// it, or the service answered. An error that keeps the plays clears
	// it.
	tryQueue bool
	sending  bool
	// size is how many plays the next Scrobble call takes, fewer than
	// batch while a rejected batch is split to find the play at fault.
	size int
	// wake has Run send: something waits to be sent.
	wake chan struct{}
	// after is time.After, which tests replace.
	after func(time.Duration) <-chan time.Time
}

// listen is the play of the track the player has loaded.
type listen struct {
	id       string
	track    Track
	playing  bool
	position time.Duration
	// rate is the speed it plays at, 1 its own.
	rate float64
	// seen is when the player was last observed.
	seen time.Time
	// start is when the play started, or zero until it plays.
	start time.Time
	// listened is how much of the track was heard.
	listened  time.Duration
	announced bool
	scrobbled bool
}

// New returns a Scrobbler that calls api for the user of key, the
// service's session key or token. With an empty key it scrobbles nothing
// until SetSession sets one.
func New(api Service, key string, o Options) *Scrobbler {
	batch := o.Batch
	if batch <= 0 {
		batch = defaultBatch
	}
	return &Scrobbler{api: api, batch: batch, size: batch, session: key, tryQueue: key != "", wake: make(chan struct{}, 1), after: time.After}
}

// SetSession changes the session key calls are made with, such as after the
// user connected again; an empty key stops scrobbling. Scrobbles in the
// queue go out with the new key on the next Send.
func (s *Scrobbler) SetSession(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session, s.expired, s.tryQueue = key, false, key != ""
	if key == "" {
		s.nowPlaying = nil
	}
	s.woken()
}

// Queue returns the scrobbles waiting to be sent, oldest first.
func (s *Scrobbler) Queue() []Play {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queue)
}

// Waiting returns how many scrobbles wait to be sent.
func (s *Scrobbler) Waiting() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queue)
}

// SetQueue replaces the scrobbles waiting to be sent, such as with the ones
// the last run saved. The next Send tries them.
func (s *Scrobbler) SetQueue(plays []Play) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = slices.Clone(plays)
	s.gen++
	s.size = s.batch
	s.tryQueue = true
	s.woken()
}

// Discard drops the scrobbles waiting to be sent, such as when the user
// chose not to send them to the account that took the place of theirs.
func (s *Scrobbler) Discard() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = nil
	s.gen++
	s.size = s.batch
	s.saveQueue()
}

// Expired reports whether the service refused the session key, so that
// plays wait in the queue until SetSession gives a new one.
func (s *Scrobbler) Expired() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.expired
}

// Observe tells the Scrobbler what the player does at now: id identifies the
// loaded track and is empty when none is, t describes it, playing reports
// whether it plays, position is how far into the track it is, and rate is
// the speed it plays at, where 1, or 0, is its own. The time between two
// observations while the track plays counts as listening, up to 5
// seconds, to as much of the track as it played at the rate of the first:
// at 2×, a second plays two of the track. The track plays again when the
// position goes back near 0 after it had advanced, or after another one.
// Observe reports whether Send has something to send, which wakes Run.
func (s *Scrobbler) Observe(now time.Time, id string, t Track, playing bool, position time.Duration, rate float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !(rate > 0) || math.IsInf(rate, 0) {
		rate = 1
	}
	c := &s.cur
	if c.id != "" && c.playing {
		c.listened += time.Duration(float64(min(max(now.Sub(c.seen), 0), maxGap)) * c.rate)
		s.count()
	}
	again := id == c.id && position < startWindow && c.position-position > startWindow
	if id != c.id || again {
		s.cur = listen{id: id}
	}
	if id == "" {
		return s.woken()
	}
	c.track, c.playing, c.position, c.rate, c.seen = t, playing, position, rate, now
	if playing && c.start.IsZero() {
		// When it would have started, playing at the rate it does.
		c.start = now.Add(-time.Duration(float64(position) / rate))
	}
	if playing && !c.announced && t.complete() && s.active() {
		c.announced = true
		s.nowPlaying = &t
	}
	s.count()
	return s.woken()
}

// count queues the current play as a scrobble once it has been listened to
// long enough.
func (s *Scrobbler) count() {
	c := &s.cur
	if c.scrobbled || !c.track.complete() || c.track.Duration <= minLength {
		return
	}
	if c.listened < min(c.track.Duration/2, maxListen) {
		return
	}
	c.scrobbled = true
	if s.session == "" {
		return
	}
	// A refused session still queues: the plays go out once a new session
	// is set.
	s.queue = append(s.queue, Play{Track: c.track, Start: c.start})
	if len(s.queue) > maxQueue {
		s.queue = slices.Delete(s.queue, 0, len(s.queue)-maxQueue)
	}
	s.tryQueue = s.active()
	s.saveQueue()
}

// Send makes the calls the Scrobbler has waiting: the now-playing update of
// the track that started last, then the queue, oldest first and up to the
// Options' Batch of scrobbles per call, until the service has them all or a
// call fails.
//
// A failed now-playing update is not made again. Scrobbles that fail with
// an error that keeps them, as the network's, stay in the queue and are
// sent again after the next call the service answers: a now-playing
// update, or the scrobble of a new play, which joins the queue behind
// them. A batch the service rejects, as IsRejected tells, is split in
// halves and sent again, until the play at fault goes alone: that one is
// dropped, and Send returns the error. When the service refuses the key, Send returns an error that is
// ErrKeyRefused, once, and the Scrobbler does not send anything until
// SetSession gives it a new key; plays go on joining the queue meanwhile.
//
// Send returns at once while another Send runs.
func (s *Scrobbler) Send(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sending {
		return nil
	}
	s.sending = true
	defer func() { s.sending = false }()
	for s.active() {
		key := s.session
		if t := s.nowPlaying; t != nil {
			s.nowPlaying = nil
			if err := s.unlocked(func() error { return s.api.NowPlaying(ctx, key, *t) }); err != nil {
				return s.failed(key, err)
			}
			s.tryQueue = true
			continue
		}
		if !s.tryQueue || len(s.queue) == 0 {
			s.tryQueue = false
			return nil
		}
		batch := slices.Clone(s.queue[:min(len(s.queue), s.size)])
		gen := s.gen
		err := s.unlocked(func() error { return s.api.Scrobble(ctx, key, batch) })
		switch {
		case err != nil && !IsRejected(err):
			return s.failed(key, err)
		case gen != s.gen:
			continue // the queue was replaced meanwhile: its head goes next
		case err != nil && len(batch) > 1:
			// One play of the batch or more is at fault: half as many go
			// at once, until the one at fault goes alone.
			s.size = len(batch) / 2
			continue
		}
		s.queue = slices.Delete(s.queue, 0, len(batch))
		if err != nil || len(s.queue) == 0 {
			s.size = s.batch
		}
		s.saveQueue()
		if err != nil {
			return err
		}
	}
	return nil
}

// unlocked calls f with the Scrobbler unlocked, so that Observe does not
// wait for the network.
func (s *Scrobbler) unlocked(f func() error) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return f()
}

// failed handles the error of a call made with key, and returns the error
// Send returns.
func (s *Scrobbler) failed(key string, err error) error {
	switch {
	case errors.Is(err, ErrKeyRefused):
		if key != s.session {
			return nil // the key changed during the call: the next Send tries the new one
		}
		s.expired = true
		s.nowPlaying = nil
	case !IsRejected(err):
		s.tryQueue = false
	}
	return err
}

// active reports whether the Scrobbler has a key the service accepts.
func (s *Scrobbler) active() bool {
	return s.session != "" && !s.expired
}

// pending reports whether Send has something to send.
func (s *Scrobbler) pending() bool {
	return s.active() && (s.nowPlaying != nil || s.tryQueue && len(s.queue) > 0)
}

// saveQueue hands the queue to OnQueue.
func (s *Scrobbler) saveQueue() {
	if s.OnQueue != nil {
		s.OnQueue(slices.Clone(s.queue))
	}
}
