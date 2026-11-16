package room

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// TestSeatsLetInFirst has the seats of a room that moved here wait for the
// host's user, while its rules ask, as joins do: let in, a seat comes back,
// and later comes back unasked; turned away, it's told so, and its place
// goes.
func TestSeatsLetInFirst(t *testing.T) {
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	f := &fakeGuests{rules: Rules{Ask: true}}
	r := newTestRoomWith(t, func(s *Server) {
		s.Guests = f.guests()
		s.Seats = &Seats{Guests: []Member{{Key: ana.Identity.Fingerprint(), Name: "Ana"}, {Key: bo.Identity.Fingerprint(), Name: "Bo"}}}
	})
	r.server.Open()
	type back struct {
		s   *Session
		err error
	}
	rejoin := func(g *Guest) <-chan back {
		ch := make(chan back, 1)
		go func() {
			s, err := g.Rejoin(context.Background(), r.addr, r.key)
			ch <- back{s, err}
		}()
		return ch
	}
	names := func() []string {
		var out []string
		for _, m := range r.server.Members() {
			out = append(out, m.Name)
		}
		return out
	}
	a := rejoin(ana)
	waitFor(t, "Ana's knock", func() bool { k := r.server.Knocks(); return len(k) == 1 && k[0].Name == "Ana" })
	r.server.LetIn(r.server.Knocks()[0].ID, true)
	got := <-a
	if got.err != nil {
		t.Fatal(got.err)
	}
	waitFor(t, "Ana here", func() bool { return r.server.CanHost(ana.Identity.Fingerprint()) })
	got.s.Close()
	waitFor(t, "Ana away", func() bool { return !r.server.CanHost(ana.Identity.Fingerprint()) })
	again, err := ana.Rejoin(context.Background(), r.addr, r.key)
	if err != nil {
		t.Fatal("Ana coming back once let in:", err)
	}
	again.Close()
	b := rejoin(bo)
	waitFor(t, "Bo's knock", func() bool { return len(r.server.Knocks()) == 1 })
	r.server.LetIn(r.server.Knocks()[0].ID, false)
	if got := <-b; !errors.Is(got.err, ErrDeclined) {
		t.Fatalf("Bo turned away: %v", got.err)
	}
	waitFor(t, "Bo's place gone", func() bool { return !slices.Contains(names(), "Bo") })
}
