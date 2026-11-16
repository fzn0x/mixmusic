package room

// Songs from guests (#734): a guest adds a song, which the host adds to
// its queue at once, or once its user accepts it, or takes none of, as its
// rules say. A guest adds songsPerMinute at most, and has MaxWaiting wait
// for the host's user at most.

import (
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

func init() {
	guestAsks["add"] = (*Server).addSong
	hostTells["added"] = (*Session).added
}

// What becomes of the songs guests add (Rules.Songs).
const (
	// SongsAdd: the host adds them to its queue at once.
	SongsAdd = "add"
	// SongsAsk: each waits for the host's user to accept it, MaxWaiting
	// of a guest at most.
	SongsAsk = "ask"
	// SongsOff: the host takes none.
	SongsOff = "off"
)

// The bounds of the songs a guest adds.
const (
	// MaxWaiting bounds the songs of a guest that wait for the host's
	// user to accept them.
	MaxWaiting = 10
	// songsPerMinute bounds the songs a guest adds, taken or not: ten at
	// once, then one each six seconds.
	songsPerMinute = 10
)

// What the activity of a room tells of a guest that added a song, whose
// title the activity's Song tells, or asked to.
const (
	Added      = "added"
	AskedToAdd = "asked to add"
)

// What the host tells of a song a guest added, in added messages: it
// waits for the host's user (SongWaits), or was refused, for a reason.
// Added at once, it tells none.
const (
	SongWaits    = "asked"
	SongDeclined = "declined"
	SongsAreOff  = "songs off"
	TooMany      = "too many"
	CantPlay     = "can't play"
)

// Request is a song a guest asked the host to add, which waits for the
// host's user to accept it.
type Request struct {
	ID    int64
	From  Member
	Track remote.QueueTrack
	At    time.Time
	// seq is the guest's number of it, which the answer tells.
	seq int64
}

// addSong takes in a song the guest of c adds: the host adds it, or asks
// its user first, or takes none, as its rules say.
func (s *Server) addSong(c *conn, m message) {
	if m.Track == nil {
		return
	}
	t := m.Track.Clean()
	rules := s.rules()
	answer := func(reason string) { c.send(message{Type: "added", Seq: m.Seq, Reason: reason}) }
	s.mu.Lock()
	g := s.guestOf(c)
	if g == nil {
		s.mu.Unlock()
		return
	}
	from := g.Member
	waiting := 0
	for _, r := range s.asks.requests {
		if r.From.Key == c.key {
			waiting++
		}
	}
	gs := s.Guests
	reason := ""
	switch {
	case rules.Songs == SongsOff || gs == nil || gs.Add == nil:
		reason = SongsAreOff
	case rules.Songs == SongsAsk && waiting >= MaxWaiting:
		reason = TooMany
	case !s.limitsOf(c.key).songs.take(time.Now(), songsPerMinute, time.Minute/songsPerMinute):
		reason = TooMany
	}
	s.mu.Unlock()
	if reason == "" && (t.Title == "" || gs.Playable != nil && !gs.Playable(t)) {
		reason = CantPlay
	}
	if reason != "" {
		answer(reason)
		return
	}
	if rules.Songs == SongsAsk {
		s.mu.Lock()
		s.asks.next++
		s.asks.requests = append(s.asks.requests, &Request{ID: s.asks.next, From: from, Track: t, At: time.Now(), seq: m.Seq})
		s.noteSong(from.Name, AskedToAdd, t.Title)
		s.mu.Unlock()
		answer(SongWaits)
		if gs.Asks != nil {
			gs.Asks(from.Name + " asks to add " + t.Title)
		}
		s.changed(false)
		return
	}
	if err := gs.Add(from, t); err != nil {
		answer(CantPlay)
		return
	}
	s.mu.Lock()
	s.noteSong(from.Name, Added, t.Title)
	s.mu.Unlock()
	answer("")
	s.changed(false)
}

// noteSong keeps in the activity that the guest named name did what of
// the song title. s.mu is held.
func (s *Server) noteSong(name, what, title string) {
	s.note(name, what)
	s.activity[len(s.activity)-1].Song = title
}

// Requests returns the songs that wait for the host's user to accept
// them, the oldest first: those of the guests the room keeps.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asks.requests = slices.DeleteFunc(s.asks.requests, func(r *Request) bool { return s.guests[r.From.Key] == nil })
	out := make([]Request, len(s.asks.requests))
	for i, r := range s.asks.requests {
		out[i] = *r
	}
	return out
}

// answerRequest takes the request id off those that wait, and returns it
// and the connection its guest has now, if any.
func (s *Server) answerRequest(id int64) (*Request, *conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.asks.requests, func(r *Request) bool { return r.ID == id })
	if i < 0 {
		return nil, nil
	}
	r := s.asks.requests[i]
	s.asks.requests = slices.Delete(s.asks.requests, i, i+1)
	var c *conn
	if g := s.guests[r.From.Key]; g != nil {
		c = g.conn
	}
	return r, c
}

// Accept adds the song of the request id to the host's queue, and tells
// its guest.
func (s *Server) Accept(id int64) error {
	r, c := s.answerRequest(id)
	if r == nil || s.Guests == nil || s.Guests.Add == nil {
		return nil
	}
	err := s.Guests.Add(r.From, r.Track)
	reason := ""
	if err != nil {
		reason = CantPlay
	} else {
		s.mu.Lock()
		s.noteSong(r.From.Name, Added, r.Track.Title)
		s.mu.Unlock()
	}
	if c != nil {
		c.send(message{Type: "added", Seq: r.seq, Reason: reason})
	}
	s.changed(false)
	return err
}

// Decline declines the song of the request id, and tells its guest.
func (s *Server) Decline(id int64) {
	r, c := s.answerRequest(id)
	if r == nil {
		return
	}
	if c != nil {
		c.send(message{Type: "added", Seq: r.seq, Reason: SongDeclined})
	}
	s.changed(false)
}

// SongAnswered: the host told what became of the song Seq this guest
// added, Track: added at once (Reason ""), waiting for the host's user
// (SongWaits), or refused (Reason).
const SongAnswered EventKind = 65

// AddSong asks the host to add t to its queue, and returns the number of
// the ask, which SongAnswered tells of.
func (s *Session) AddSong(t remote.QueueTrack) (int64, error) {
	t = t.Clean()
	s.mu.Lock()
	s.asks.seq++
	seq := s.asks.seq
	if s.asks.songs == nil {
		s.asks.songs = map[int64]remote.QueueTrack{}
	}
	// Those whose answers never came go, of the oldest.
	for k := range s.asks.songs {
		if k <= seq-2*MaxWaiting {
			delete(s.asks.songs, k)
		}
	}
	s.asks.songs[seq] = t
	s.mu.Unlock()
	return seq, s.f.Send(message{Type: "add", Seq: seq, Track: &t}, s.timing.silence())
}

// added takes in what became of a song this guest added.
func (s *Session) added(m message) {
	s.mu.Lock()
	t, ok := s.asks.songs[m.Seq]
	if ok && m.Reason != SongWaits {
		delete(s.asks.songs, m.Seq)
	}
	s.mu.Unlock()
	if ok {
		s.tell(Event{Kind: SongAnswered, Seq: m.Seq, Track: t, Reason: cleanText(m.Reason, 32)})
	}
}
