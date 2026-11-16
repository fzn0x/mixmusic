package room

// Ask me before someone joins (#734): while the host's rules say so, a
// computer that proved the code or the link, and asks to join, waits for
// the host's user to let it in, AskFor at most; maxKnocks wait at most.
// Guests that come back by their keys aren't asked about.

import (
	"errors"
	"slices"
	"time"
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
// in.
const maxKnocks = 20

// Why the host turns a computer away that asks to join.
const (
	reasonDeclined = "declined"
	reasonManyAsk  = "many asking"
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
	// answer takes the host's user's answer.
	answer chan bool
}

// letIn has the host's user let in c, of the computer named name, which
// proved the code or the link and asks to join, when the rules ask them;
// it answers c's pings meanwhile, from msgs. It returns why not, or "".
func (s *Server) letIn(c *conn, name string, msgs <-chan message) string {
	if !s.rules().Ask {
		return ""
	}
	s.mu.Lock()
	if len(s.asks.knocks) >= maxKnocks {
		s.mu.Unlock()
		return reasonManyAsk
	}
	s.asks.next++
	k := &Knock{ID: s.asks.next, Name: name, At: time.Now(), answer: make(chan bool, 1)}
	s.asks.knocks = append(s.asks.knocks, k)
	s.mu.Unlock()
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
	if gs := s.Guests; gs != nil && gs.Asks != nil {
		gs.Asks(name + " asks to join your room")
	}
	s.changed(false)
	wait := time.NewTimer(AskFor)
	defer wait.Stop()
	for {
		select {
		case m, ok := <-msgs:
			if !ok {
				return reasonDeclined
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

// Knocks returns the computers that wait for the host's user to let them
// in, the oldest first.
func (s *Server) Knocks() []Knock {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Knock, len(s.asks.knocks))
	for i, k := range s.asks.knocks {
		out[i] = Knock{ID: k.ID, Name: k.Name, At: k.At}
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
