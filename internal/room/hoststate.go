package room

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// startHold is how long after a start the guests are told of it, rather
// than of the host's track, while that doesn't play yet.
const startHold = 2 * time.Second

// run tells the guests what plays as it changes, at most every Changes,
// and every Ping besides, and forgets the guests gone longer than Keep,
// until stop.
func (s *Server) run(wake, stop <-chan struct{}) {
	tick := time.NewTicker(s.Timing.ping())
	defer tick.Stop()
	s.tell(true)
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			s.expire()
			s.tell(false)
			s.RulesChanged()
		case <-wake:
			s.tell(true)
			// What changes at once, as a seek and the play that follows,
			// goes in one state.
			select {
			case <-stop:
				return
			case <-time.After(s.Timing.changes()):
			}
		}
	}
}

// tell tells the guests here where the host's track is, and its queue
// when it changed, if queue.
func (s *Server) tell(queue bool) {
	st := s.Host.State()
	var q Queue
	var data []byte
	if queue {
		q = fitQueue(s.Host.Queue())
		data, _ = json.Marshal(q)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st = s.stateOf(st)
	newQueue := queue && !bytes.Equal(data, s.sentQueue)
	if newQueue {
		s.sentQueue = data
		s.queue = &q
	}
	for _, g := range s.guests {
		if g.conn == nil {
			continue
		}
		if newQueue {
			g.conn.send(message{Type: "queue", Queue: &q})
		}
		g.conn.send(message{Type: "state", State: &st})
	}
}

// queueMessage is the queue the guests were told last. s.mu is held.
func (s *Server) queueMessage() message {
	q := Queue{}
	if s.queue != nil {
		q = *s.queue
	}
	return message{Type: "queue", Queue: &q}
}

// stateOf is the state the guests are told, of what the host plays, st:
// the start of a track change while the host's track doesn't play yet, a
// moment. s.mu is held.
func (s *Server) stateOf(st State) State {
	if start := s.starting; start != nil {
		sounds := st.Entry == start.Entry && st.Playing && !st.Loading
		if !sounds && s.Clock.Now() < start.At+startHold {
			return *start
		}
		s.starting = nil
	}
	return st
}

// expire forgets the guests that dropped off longer than Keep ago, and the
// sources that tried codes a while ago.
func (s *Server) expire() {
	s.mu.Lock()
	s.adm.sweep(time.Now(), s.Timing.wait())
	gone := false
	for k, g := range s.guests {
		if g.conn == nil && !g.away.IsZero() && time.Since(g.away) > s.Timing.keep() {
			delete(s.guests, k)
			s.note(g.Name, Left)
			gone = true
		}
	}
	s.mu.Unlock()
	if gone {
		s.changed(true)
	}
}

// gate is a track change that waits for the guests to have the track
// ready: those here as it started, of which waiting are yet to tell.
type gate struct {
	seq     int64
	entry   string
	from    time.Duration
	waiting map[remote.Fingerprint]bool
	start   func(at time.Duration)
	timer   *time.Timer
}

// Prepare tells the guests here that the host's entry and its track plays
// next, from from, and calls start once each has it ready, or Gate passed,
// with the time of the host's clock it starts at, Lead ahead, which the
// guests start at too. With no guest here, start is called at once, with
// now. A track change that waits gives way to the next.
func (s *Server) Prepare(entry string, track remote.QueueTrack, from time.Duration, start func(at time.Duration)) {
	s.prepare(entry, track, from, s.Timing.gate(), start)
}

// PrepareFade is Prepare for a crossfade of fade, as the track that ends
// plays on: the guests are waited for half the fade at most, less Lead,
// so that it plays until they all start the next.
func (s *Server) PrepareFade(entry string, track remote.QueueTrack, from, fade time.Duration, start func(at time.Duration)) {
	s.prepare(entry, track, from, min(max(fade/2-s.Timing.lead(), 0), s.Timing.gate()), start)
}

// prepare is Prepare, waiting within at most for the guests.
func (s *Server) prepare(entry string, track remote.QueueTrack, from, within time.Duration, start func(at time.Duration)) {
	track = track.Clean()
	s.mu.Lock()
	if old := s.gate; old != nil {
		old.timer.Stop()
		s.gate = nil
	}
	s.seq++
	s.starting = nil
	waiting := map[remote.Fingerprint]bool{}
	for k, g := range s.guests {
		if g.conn != nil {
			waiting[k] = true
		}
	}
	if s.closed || len(waiting) == 0 {
		s.mu.Unlock()
		start(s.Clock.Now())
		return
	}
	g := &gate{seq: s.seq, entry: entry, from: from, waiting: waiting, start: start}
	s.gate = g
	g.timer = time.AfterFunc(within, func() { s.release(g) })
	for k := range waiting {
		s.guests[k].conn.send(message{Type: "prepare", Seq: g.seq, Entry: entry, Track: &track})
	}
	s.mu.Unlock()
}

// ready takes in that the guest of key has the track of the change seq
// ready, or can't play it, or left.
func (s *Server) ready(key remote.Fingerprint, seq int64) {
	s.mu.Lock()
	g := s.gate
	if g == nil || g.seq != seq {
		s.mu.Unlock()
		return
	}
	delete(g.waiting, key)
	done := len(g.waiting) == 0
	s.mu.Unlock()
	if done {
		s.release(g)
	}
}

// release starts the track change g, Lead ahead, for the host and its
// guests, unless another gave way to it.
func (s *Server) release(g *gate) {
	st := s.Host.State()
	s.mu.Lock()
	if s.gate != g {
		s.mu.Unlock()
		return
	}
	s.gate = nil
	g.timer.Stop()
	at := s.Clock.Now() + s.Timing.lead()
	st.Entry, st.Seq = g.entry, g.seq
	st.Playing, st.Starts, st.Loading = true, true, false
	st.Position, st.At = g.from, at
	s.starting = &st
	for _, gu := range s.guests {
		if gu.conn != nil {
			gu.conn.send(message{Type: "state", State: &st})
		}
	}
	s.mu.Unlock()
	g.start(at)
}

// Waiting reports whether a track change waits for the guests.
func (s *Server) Waiting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gate != nil
}
