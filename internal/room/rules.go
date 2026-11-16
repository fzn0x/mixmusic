package room

// The host's rules for its guests (#734): what becomes of the songs they
// add (songs.go), whether they may control its player (control.go), and
// whether its user lets in each computer that joins (letin.go). The host
// checks each ask of a guest as it comes, by the rules of that moment:
// whose it is, how often it asks, and how much.

import (
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// Rules are what the host lets its guests do, as its settings choose.
type Rules struct {
	// Songs is what becomes of the songs guests add: SongsAdd, SongsAsk,
	// or SongsOff, as anything else is.
	Songs string `json:"songs,omitempty"`
	// Control lets the guests play, pause, and move to the next or the
	// last track of the host's queue.
	Control bool `json:"control,omitempty"`
	// Ask has the host's user let in each computer that joins, by the code
	// or the link: guests that come back by their keys aren't asked about.
	// The guests aren't told.
	Ask bool `json:"-"`
}

func (r Rules) clean() Rules {
	if r.Songs != SongsAdd && r.Songs != SongsAsk {
		r.Songs = SongsOff
	}
	return r
}

// Guests is what the host's app does as its guests ask, and the rules it
// asks by. Its funcs are called without the server's lock, and mustn't
// wait long.
type Guests struct {
	// Rules are the host's rules now, which each ask is checked against.
	Rules func() Rules
	// Playable reports whether the host can play t, a song a guest adds.
	Playable func(t remote.QueueTrack) bool
	// Add adds t, which from added, to the host's queue.
	Add func(from Member, t remote.QueueTrack) error
	// Control has the host's player do what from asks: Play, Pause, Next
	// or Previous.
	Control func(from Member, do string) error
	// Asks tells the host's user of what waits for their answer, in
	// note's words.
	Asks func(note string)
}

// asks are what the guests asked and what waits for the host's user, as
// the files of each keep them. Server.mu guards them.
type asks struct {
	next     int64
	requests []*Request
	knocks   []*Knock
	limits   map[remote.Fingerprint]*limits
	sent     *Rules
}

// rules are the host's rules now: with no Guests, guests add no songs,
// control nothing, and join without asking.
func (s *Server) rules() Rules {
	if g := s.Guests; g != nil && g.Rules != nil {
		return g.Rules().clean()
	}
	return Rules{}.clean()
}

// RulesChanged tells the guests the host's rules, once they changed.
func (s *Server) RulesChanged() {
	r := s.rules()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.asks.sent != nil && *s.asks.sent == r {
		return
	}
	s.asks.sent = &r
	for _, g := range s.guests {
		if g.conn != nil {
			g.conn.send(message{Type: "rules", Rules: &r})
		}
	}
}

// tellRules tells c, a guest let in, the host's rules. s.mu is held.
func (s *Server) tellRules(c *conn) {
	r := s.rules()
	c.send(message{Type: "rules", Rules: &r})
}

// guestOf is the guest whose connection c is now, or nil. s.mu is held.
func (s *Server) guestOf(c *conn) *guest {
	if g := s.guests[c.key]; g != nil && g.conn == c {
		return g
	}
	return nil
}

// guestAsks take in what a guest of the room asks, by its message's type,
// beside its pings and its readiness: the files of each add theirs.
var guestAsks = map[string]func(s *Server, c *conn, m message){}

// fromGuest takes in what the guest of c asks.
func (s *Server) fromGuest(c *conn, m message) {
	if fn := guestAsks[m.Type]; fn != nil {
		fn(s, c, m)
	}
}

// hostTells take in what the host tells a guest beside what it plays, by
// its message's type: the files of each add theirs.
var hostTells = map[string]func(s *Session, m message){
	"rules": func(s *Session, m message) {
		if m.Rules == nil {
			return
		}
		r := m.Rules.clean()
		s.mu.Lock()
		s.asks.rules, s.asks.hasRules = r, true
		s.mu.Unlock()
		s.tell(Event{Kind: RulesChanged})
	},
}

// fromHost takes in what the host tells beside what it plays.
func (s *Session) fromHost(m message) {
	if fn := hostTells[m.Type]; fn != nil {
		fn(s, m)
	}
}

// RulesChanged: the host told what it lets its guests do (Rules). The
// kinds of events of what the host tells beside what it plays come from
// 64 on, apart from the session's own.
const RulesChanged EventKind = 64

// sessionAsks are what a session keeps of what the host tells beside what
// it plays, as the files of each keep it. Session.mu guards them.
type sessionAsks struct {
	rules    Rules
	hasRules bool
	seq      int64
	songs    map[int64]remote.QueueTrack
	asking   bool
}

// Rules returns what the host lets its guests do, and whether it told.
func (s *Session) Rules() (Rules, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asks.rules, s.asks.hasRules
}

// limits are what a guest may still ask.
type limits struct {
	songs, controls bucket
}

// bucket lets size asks through at once, then one every every.
type bucket struct {
	tokens float64
	at     time.Time
}

// take reports whether an ask may go through at now, and counts it.
func (b *bucket) take(now time.Time, size int, every time.Duration) bool {
	if b.at.IsZero() {
		b.tokens = float64(size)
	} else {
		b.tokens = min(float64(size), b.tokens+float64(now.Sub(b.at))/float64(every))
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// limitsOf are what the guest of key may still ask, forgetting those of
// computers the room no longer keeps once many are kept. s.mu is held.
func (s *Server) limitsOf(key remote.Fingerprint) *limits {
	a := &s.asks
	if len(a.limits) > 2*MaxGuests {
		for k := range a.limits {
			if s.guests[k] == nil {
				delete(a.limits, k)
			}
		}
	}
	if a.limits == nil {
		a.limits = map[remote.Fingerprint]*limits{}
	}
	l := a.limits[key]
	if l == nil {
		l = &limits{}
		a.limits[key] = l
	}
	return l
}
