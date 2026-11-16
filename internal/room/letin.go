package room

// Ask me before someone joins (#734): while the host's rules say so, a
// computer that proved the code or the link, and asks to join, waits for
// the host's user to let it in, AskFor at most; maxKnocks wait at most.
// Guests that come back by their keys aren't asked about.

import (
	"errors"
	"net/netip"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

func init() {
	hostTells["asking"] = func(s *Session, m message) {
		s.mu.Lock()
		s.asks.asking = true
		s.mu.Unlock()
		s.tell(Event{Kind: Asking})
	}
}

// AskFor is how long a computer waits for the host's user to let it in
// before it is turned away.
const AskFor = 2 * time.Minute

// maxKnocks bounds the computers that wait for the host's user to let them
// in, and maxKnocksFrom those of a source; knockTellEvery is how often at
// most the host's user is told a source knocks.
const (
	maxKnocks      = 20
	maxKnocksFrom  = 2
	knockTellEvery = time.Minute
)

// knocksHere reports whether the source src (sourceOf) is this computer
// itself, of a loopback address: its own user, whose knocks aren't counted
// by source.
func knocksHere(src string) bool {
	ip, err := netip.ParseAddr(src)
	return err == nil && ip.IsLoopback()
}

// Why the host turns a computer away that asks to join: reasonLeft is
// that it went meanwhile.
const (
	reasonDeclined = "declined"
	reasonManyAsk  = "many asking"
	reasonLeft     = "left"
)

var (
	// ErrDeclined: the host's user didn't let this computer in.
	ErrDeclined = errors.New("the host didn't let this computer in")
	// ErrManyAsk: so many wait to join that the host takes no more for
	// now.
	ErrManyAsk = errors.New("so many wait to join that the room takes no more for now")
)

// Knock is a computer that proved the code or the link and asks to join,
// which waits for the host's user to let it in.
type Knock struct {
	ID   int64
	Name string
	At   time.Time
	// Key is the start of the computer's key's ID, which tells two of the
	// same name apart, and Local whether it knocks from the host's own
	// networks, not from the internet.
	Key   string
	Local bool
	// answer takes the host's user's answer; src is the source it knocks
	// from (sourceOf), and conn its connection.
	answer chan bool
	src    string
	conn   *conn
}

// letIn has the host's user let in c, of the computer named name, which
// proved the code or the link and asks to join, when the rules ask them;
// it answers c's pings meanwhile, from msgs. It returns why not, or "".
func (s *Server) letIn(c *conn, name string, msgs <-chan message) string {
	if !s.rules().Ask {
		return ""
	}
	s.mu.Lock()
	k, tell := s.knock(c.key, c.src, name, time.Now())
	if k != nil {
		k.conn = c
	}
	s.mu.Unlock()
	if k == nil {
		return reasonManyAsk
	}
	defer func() {
		s.mu.Lock()
		s.asks.knocks = slices.DeleteFunc(s.asks.knocks, func(o *Knock) bool { return o == k })
		s.mu.Unlock()
		s.changed(false)
	}()
	t := s.Timing
	if c.f.Send(message{Type: "asking"}, t.silence()) != nil {
		return reasonDeclined
	}
	if gs := s.Guests; tell && gs != nil && gs.Asks != nil {
		gs.Asks(name + " asks to join your room")
	}
	s.changed(false)
	wait := time.NewTimer(AskFor)
	defer wait.Stop()
	for {
		select {
		case m, ok := <-msgs:
			if !ok {
				return reasonLeft
			}
			if m.Type == "ping" && c.f.Send(message{Type: "pong", Sent: m.Sent, HostAt: s.Clock.Now()}, t.silence()) != nil {
				return reasonDeclined
			}
		case yes := <-k.answer:
			if yes {
				return ""
			}
			return reasonDeclined
		case <-wait.C:
			return reasonDeclined
		}
	}
}

// letInSeat has the host's user let in c, of the computer named name,
// which comes back by its key to a seat of the room that moved here
// (move.go), when the rules ask them, as a join is; a seat turned away
// goes, and joins again only by the code or the link. It returns why not, or "", and
// whether it asked, which reads c's messages from then on.
func (s *Server) letInSeat(c *conn, name string) (string, bool) {
	s.mu.Lock()
	g := s.guests[c.key]
	seated := s.asks.seated[c.key] && g != nil && g.conn == nil && s.refusal(c.key) == ""
	s.mu.Unlock()
	if !seated {
		return "", false
	}
	if !s.rules().Ask {
		s.unseat(c.key, false)
		return "", false
	}
	c.f.Limit(guestMessage)
	reason := s.letIn(c, name, c.messages(s.Timing.silence()))
	if reason != reasonLeft {
		s.unseat(c.key, reason != "")
	}
	return reason, true
}

// unseat takes the seat of key as let in, or turned away, when gone: its
// place goes then.
func (s *Server) unseat(key remote.Fingerprint, gone bool) {
	s.mu.Lock()
	delete(s.asks.seated, key)
	g := s.guests[key]
	gone = gone && g != nil && g.conn == nil
	if gone {
		delete(s.guests, key)
		s.note(g.Name, Removed)
	}
	s.mu.Unlock()
	if gone {
		s.changed(true)
	}
}

// letIn waits, on a session of a computer that comes back to the seat of
// a room that moved, for the host's user to let it in, AskFor at most.
func (s *Session) letIn() (*Session, error) {
	s.mu.Lock()
	s.asks.asking = true
	s.mu.Unlock()
	s.start()
	wait := time.NewTimer(AskFor + s.timing.silence())
	defer wait.Stop()
	select {
	case <-s.joinedCh:
		return s, nil
	case <-s.done:
		return nil, s.Err()
	case <-wait.C:
		s.Close()
		return nil, ErrDeclined
	}
}

// knock keeps the knock of the computer of key, named name, from the source
// src, at now, and reports whether to tell the host's user of it: of a
// source's first knock, and of none of it again for a while, however it
// knocks. It keeps none past maxKnocks, or maxKnocksFrom of src, unless
// src is this computer itself. s.mu is held.
func (s *Server) knock(key remote.Fingerprint, src, name string, now time.Time) (*Knock, bool) {
	from := 0
	for _, o := range s.asks.knocks {
		if o.src == src {
			from++
		}
	}
	if len(s.asks.knocks) >= maxKnocks || from >= maxKnocksFrom && !knocksHere(src) {
		return nil, false
	}
	s.asks.next++
	k := &Knock{ID: s.asks.next, Name: name, At: now, Key: key.ID()[:8], Local: localSource(src), answer: make(chan bool, 1), src: src}
	s.asks.knocks = append(s.asks.knocks, k)
	tell := from == 0 && now.Sub(s.asks.knockTold[src]) >= knockTellEvery
	if tell {
		if s.asks.knockTold == nil || len(s.asks.knockTold) >= maxSources {
			s.asks.knockTold = map[string]time.Time{}
		}
		s.asks.knockTold[src] = now
	}
	return k, tell
}

// knocking reports whether c waits for the host's user to let it in: its
// preview's place, and its time, are its knock's then (previews.go). s.mu
// is held.
func (s *Server) knocking(c *conn) bool {
	for _, k := range s.asks.knocks {
		if k.conn == c {
			return true
		}
	}
	return false
}

// Knocks returns the computers that wait for the host's user to let them
// in, the oldest first.
func (s *Server) Knocks() []Knock {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Knock, len(s.asks.knocks))
	for i, k := range s.asks.knocks {
		out[i] = Knock{ID: k.ID, Name: k.Name, At: k.At, Key: k.Key, Local: k.Local}
	}
	return out
}

// LetIn lets in the computer of the knock id, or turns it away.
func (s *Server) LetIn(id int64, yes bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.asks.knocks {
		if k.ID == id {
			select {
			case k.answer <- yes:
			default:
			}
		}
	}
}

// Asking: the host asks its user to let this computer in (Asking).
const Asking EventKind = 67

// Asking reports whether the host asks its user to let this computer in,
// as it joins.
func (s *Session) Asking() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.asks.asking && !s.joined
}
