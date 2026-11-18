package remote

import (
	"context"
	"errors"
)

// Bring is what the computer that controls asks for: what plays on the
// one controlled, which hands it over (FeatureBring). ID tells the
// hand-over that answers it apart from another's.
type Bring struct {
	ID int64 `json:"id"`
}

// ErrNothingThere: nothing plays on the computer controlled, which hands
// nothing over.
var ErrNothingThere = errors.New("nothing plays on that computer")

// Bring asks the computer controlled for what plays there, to play it on
// this one: its queue around the track that plays, cut as a hand-over is,
// where that track is, whether it plays, and how the queue goes on. The
// caller plays it in place of what plays here, and tells how it took it
// with Took, which pauses that computer, unless it kept nothing. A
// computer whose welcome doesn't tell FeatureBring can't: ErrOlder.
// ErrNothingThere tells that nothing plays there.
func (s *Session) Bring(ctx context.Context) (HandOver, error) {
	if !s.Can(FeatureBring) {
		return HandOver{}, ErrOlder
	}
	s.handing.Lock()
	defer s.handing.Unlock()
	answer := make(chan HandOver, 1)
	s.mu.Lock()
	s.handed++
	id := s.handed
	s.brought = answer
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.brought == answer {
			s.brought = nil
		}
		s.mu.Unlock()
	}()
	select {
	case <-s.done:
		return HandOver{}, s.Err()
	default:
	}
	if err := s.w.send(message{Type: "bring", Bring: &Bring{ID: id}}, s.timing.silence()); err != nil {
		return HandOver{}, err
	}
	for {
		select {
		case h := <-answer:
			if h.ID != id {
				// The answer to a bring that gave up waiting.
				continue
			}
			if len(h.Queue) == 0 {
				return HandOver{}, ErrNothingThere
			}
			return h, nil
		case <-s.done:
			return HandOver{}, s.Err()
		case <-ctx.Done():
			return HandOver{}, ctx.Err()
		}
	}
}

// Took tells the computer controlled how this one took the hand-over h,
// which Bring returned: that computer pauses once it took a track or more.
func (s *Session) Took(h HandOver, t Taken) error {
	t.ID = h.ID
	select {
	case <-s.done:
		return s.Err()
	default:
	}
	return s.w.send(message{Type: "took", Taken: &t}, s.timing.silence())
}

// broughtOver hands what the computer controlled handed over to the bring
// that waits for it.
func (s *Session) broughtOver(h HandOver) {
	s.mu.Lock()
	answer := s.brought
	s.mu.Unlock()
	if answer == nil {
		return
	}
	select {
	case answer <- h:
	default:
	}
}

// give answers the bring b of the computer at the other end of c, one
// that may control this one: what plays here, as the host hands it over,
// or nothing. It returns the ID of the hand-over, which that computer
// tells how it took (given).
func (s *Server) give(c *serverConn, b Bring) (int64, error) {
	h, ok := s.Host.Give(c.peer)
	if !ok {
		h = HandOver{}
	}
	h = fitHandOver(cleanHandOver(h))
	h.ID = b.ID
	return h.ID, c.w.send(message{Type: "brought", HandOver: &h}, s.Timing.silence())
}

// given hears how the computer at the other end of c took the hand-over
// of the ID gave, which it asked for: the host pauses once it took a track
// or more.
func (s *Server) given(c *serverConn, gave int64, t Taken) {
	if gave == 0 || t.ID != gave || t.Kept <= 0 {
		return
	}
	t.Kept, t.Left = max(t.Kept, 0), max(t.Left, 0)
	s.Host.Given(c.peer, t)
}
