package room

import (
	"context"
	"errors"
	"testing"
)

// knockByLink knocks at the room by its invite link.
func (r *testRoom) knockByLink(g *Guest) *Session {
	r.t.Helper()
	s, err := g.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
	if err != nil {
		r.t.Fatalf("%s knocking: %v", g.Name, err)
	}
	r.t.Cleanup(s.Close)
	return s
}

// joining joins s on a goroutine, and returns what Join returns.
func joining(s *Session) <-chan error {
	done := make(chan error, 1)
	go func() { done <- s.Join(context.Background()) }()
	return done
}

// TestLetInFirst has a computer that proved the code and joins wait for
// the host's user, who is told; let in, it joins; turned away, it's told
// so. A guest that comes back by its key isn't asked about.
func TestLetInFirst(t *testing.T) {
	r, f := newRuledRoom(t, Rules{Ask: true})
	ana := newGuest(t, "Ana")
	s, err := ana.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	joined := joining(s)
	waitFor(t, "the knock", func() bool { k := r.server.Knocks(); return len(k) == 1 && k[0].Name == "Ana" && s.Asking() })
	if len(r.server.Members()) != 0 {
		t.Fatal("in before the host's user let it in")
	}
	f.mu.Lock()
	notes := append([]string(nil), f.notes...)
	f.mu.Unlock()
	if len(notes) != 1 || notes[0] != "Ana asks to join your room" {
		t.Fatalf("the host's user was told %q", notes)
	}
	r.server.LetIn(r.server.Knocks()[0].ID, true)
	if err := <-joined; err != nil {
		t.Fatal(err)
	}
	if s.Asking() || len(r.server.Knocks()) != 0 || len(r.server.Members()) != 1 {
		t.Fatal("let in, the knock stays")
	}
	waitFor(t, "the rules", func() bool { _, ok := s.Rules(); return ok })
	if got, _ := s.Rules(); got.Ask {
		t.Fatal("the guests are told the host asks")
	}
	// Turned away.
	bo := r.knockByLink(newGuest(t, "Bo"))
	joined = joining(bo)
	waitFor(t, "Bo's knock", func() bool { return len(r.server.Knocks()) == 1 })
	r.server.LetIn(r.server.Knocks()[0].ID, false)
	if err := <-joined; !errors.Is(err, ErrDeclined) {
		t.Fatalf("turned away: %v", err)
	}
	// Ana comes back by her key, unasked.
	s.Close()
	waitFor(t, "away", func() bool { m := r.server.Members(); return len(m) == 1 && !m[0].Here })
	back, err := ana.Rejoin(context.Background(), r.addr, r.key)
	if err != nil {
		t.Fatal(err)
	}
	back.Close()
}

// TestLetInBounded turns away a computer once maxKnocks wait.
func TestLetInBounded(t *testing.T) {
	f := &fakeGuests{rules: Rules{Ask: true}}
	// All come from 127.0.0.1, at once: the source may open and hold as
	// many as the room.
	r := newTestRoomWith(t, func(s *Server) {
		s.Guests = f.guests()
		s.caps = &caps{unproven: maxUnproven, unprovenFrom: maxUnproven, tries: Tries, roomTries: RoomTries}
	})
	for i := range maxKnocks {
		joining(r.knockByLink(newGuest(t, "Guest")))
		waitFor(t, "the knock", func() bool { return len(r.server.Knocks()) == i+1 })
	}
	if err := <-joining(r.knockByLink(newGuest(t, "One more"))); !errors.Is(err, ErrManyAsk) {
		t.Fatalf("one knock too many: %v", err)
	}
}
