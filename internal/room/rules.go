package room

// The host's rules for its guests (#734): what becomes of the songs they
// add (songs.go), whether they may control its player (control.go), and
// whether its user lets in each computer that joins (letin.go). The host
// checks each ask of a guest as it comes, by the rules of that moment:
// whose it is, how often it asks, and how much.

import (
	"context"
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
	// Look is the song t a guest adds as the host finds it, by its ID and
	// its kind alone, on its service or among its own files, or why it
	// finds none: the guest's word on its title, artists, covers and
	// whether it's explicit is taken for nothing. It may ask the network.
	Look func(ctx context.Context, t remote.QueueTrack) (remote.QueueTrack, error)
	// Add adds t, as Look found it, which from added, to the host's queue,
	// or tells why it doesn't, as its settings hide it.
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
	next         int64
	requests     []*Request
	knocks       []*Knock
	knockTold    map[string]time.Time
	limits       map[remote.Fingerprint]*limits
	sourceLimits map[string]*limits
	looking      map[remote.Fingerprint]int
	looks        int
	lookRate     bucket
	sent         *Rules
	move         *move
	moved        bool
	open         bool
	seated       map[remote.Fingerprint]bool
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

// tellRules tells c, a guest let in, the host's rules. s.mu isn't held, as
// Guests' funcs are called without it.
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
	moved    *Move
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
	if !b.has(now, size, every) {
		return false
	}
	b.tokens--
	return true
}

// has reports whether an ask may go through at now, without counting it.
func (b *bucket) has(now time.Time, size int, every time.Duration) bool {
	if b.at.IsZero() {
		b.tokens = float64(size)
	} else {
		b.tokens = min(float64(size), b.tokens+float64(now.Sub(b.at))/float64(every))
	}
	b.at = now
	return b.tokens >= 1
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

// takes reports whether the guest of c may ask once more, by the bucket
// of limits each, which take, of size at once and one each every: its
// key's, and its source's (sourceOf), of twice as many, so that a computer
// that makes new keys asks no more. s.mu is held.
func (s *Server) takes(c *conn, bucketOf func(*limits) *bucket, size int, every time.Duration) bool {
	now := time.Now()
	key, src := bucketOf(s.limitsOf(c.key)), bucketOf(s.limitsFrom(c.src))
	// Either refuses without the other's counting it.
	if !key.has(now, size, every) || !src.has(now, 2*size, every/2) {
		return false
	}
	return key.take(now, size, every) && src.take(now, 2*size, every/2)
}

// limitsFrom are what the guests of the source src may still ask, of
// sources the room has guests of. s.mu is held.
func (s *Server) limitsFrom(src string) *limits {
	a := &s.asks
	if len(a.sourceLimits) > 2*MaxGuests {
		here := map[string]bool{}
		for _, g := range s.guests {
			if g.conn != nil {
				here[g.conn.src] = true
			}
		}
		for src := range a.sourceLimits {
			if !here[src] {
				delete(a.sourceLimits, src)
			}
		}
	}
	if a.sourceLimits == nil {
		a.sourceLimits = map[string]*limits{}
	}
	l := a.sourceLimits[src]
	if l == nil {
		l = &limits{}
		a.sourceLimits[src] = l
	}
	return l
}
