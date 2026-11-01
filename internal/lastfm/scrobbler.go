package lastfm

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
	"time"
)

// API is the part of the Last.fm API a Scrobbler calls. *Client implements
// it; tests use a fake.
type API interface {
	NowPlaying(ctx context.Context, sessionKey string, t Track) error
	Scrobble(ctx context.Context, sessionKey string, plays []Play) error
}

var _ API = (*Client)(nil)

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
)

// Scrobbler decides, from what the player reports, which plays count as
// scrobbles, and sends them to Last.fm with now-playing updates. A track
// longer than 30 seconds counts once it has been listened to for half its
// duration or for four minutes, whichever comes first, and each play of it
// counts once.
//
// The app calls Observe about once a second and on every change of the
// player, and Send when Observe reports something to send. Scrobbles wait in
// a queue until Last.fm accepts them; the app saves the queue with OnQueue
// and restores it with SetQueue. The methods of a Scrobbler are safe for
// concurrent use.
type Scrobbler struct {
	// OnQueue is called with the scrobbles waiting to be sent each time they
	// change, to save them for the next run. It is called while the
	// Scrobbler is locked, so it must not call the Scrobbler.
	OnQueue func([]Play)

	api API

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
	// it, or Last.fm answered. A temporary error clears it.
	tryQueue bool
	sending  bool
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

// NewScrobbler returns a Scrobbler that calls api for the user of
// sessionKey. With an empty key it scrobbles nothing until SetSession sets
// one.
func NewScrobbler(api API, sessionKey string) *Scrobbler {
	return &Scrobbler{api: api, session: sessionKey, tryQueue: sessionKey != ""}
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
}

// Queue returns the scrobbles waiting to be sent, oldest first.
func (s *Scrobbler) Queue() []Play {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queue)
}

// SetQueue replaces the scrobbles waiting to be sent, such as with the ones
// the last run saved. The next Send tries them.
func (s *Scrobbler) SetQueue(plays []Play) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = slices.Clone(plays)
	s.gen++
	s.tryQueue = true
}

// Observe tells the Scrobbler what the player does at now: id identifies the
// loaded track and is empty when none is, t describes it, playing reports
// whether it plays, position is how far into the track it is, and rate is
// the speed it plays at, where 1, or 0, is its own. The time between two
// observations while the track plays counts as listening, up to 5
// seconds, to as much of the track as it played at the rate of the first:
// at 2×, a second plays two of the track. The track plays again when the
// position goes back near 0 after it had advanced, or after another one.
// Observe reports whether Send has something to send.
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
		return s.pending()
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
	return s.pending()
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
	if !s.active() {
		return
	}
	s.queue = append(s.queue, Play{Track: c.track, Start: c.start})
	s.tryQueue = true
	s.saveQueue()
}

// Send makes the calls the Scrobbler has waiting: the now-playing update of
// the track that started last, then the queue, oldest first and up to 50
// scrobbles per call, until Last.fm has them all or a call fails.
//
// A failed now-playing update is not made again. Scrobbles that fail with a
// temporary error, as IsTemporary tells, stay in the queue and are sent
// again after the next call Last.fm answers: a now-playing update, or the
// scrobble of a new play, which joins the queue behind them. Scrobbles that
// Last.fm rejects for another reason are dropped. When Last.fm rejects the
// session key, Send returns ErrSessionExpired, once, and the Scrobbler
// neither counts nor sends anything until SetSession gives it a new key.
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
		batch := slices.Clone(s.queue[:min(len(s.queue), maxBatch)])
		gen := s.gen
		err := s.unlocked(func() error { return s.api.Scrobble(ctx, key, batch) })
		if errors.Is(err, ErrSessionExpired) || IsTemporary(err) {
			return s.failed(key, err)
		}
		if gen == s.gen {
			s.queue = slices.Delete(s.queue, 0, len(batch))
			s.saveQueue()
		}
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
	case errors.Is(err, ErrSessionExpired):
		if key != s.session {
			return nil // the key changed during the call: the next Send tries the new one
		}
		s.expired = true
		s.nowPlaying = nil
	case IsTemporary(err):
		s.tryQueue = false
	}
	return err
}

// active reports whether the Scrobbler has a session key Last.fm accepts.
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
