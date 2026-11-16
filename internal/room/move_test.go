package room

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// hostAsApp has the guest g, in the session s, do what its app does as the
// host asks it to host: it opens a room of its own, which keeps the seats,
// and tells where; or closes it, as the room stays where it is. The rooms
// it opens come on the channel.
func hostAsApp(t *testing.T, g *Guest, s *Session) <-chan *Server {
	t.Helper()
	opened := make(chan *Server, 1)
	ch := events(s)
	go func() {
		var srv *Server
		for {
			select {
			case <-s.Done():
				return
			case e := <-ch:
				switch e.Kind {
				case HostAsked:
					clock := NewSystemClock()
					seats := e.Seats
					srv = &Server{Identity: g.Identity, Name: g.Name, Code: "XYZ789", Secret: []byte("0123456789abcdef"), Host: &fakeHost{clock: clock},
						Clock: clock, Timing: testTiming, Seats: &seats}
					ln, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						s.Hosting(0)
						continue
					}
					go srv.Serve(ln)
					t.Cleanup(func() { srv.Close() })
					s.Hosting(ln.Addr().(*net.TCPAddr).Port)
					opened <- srv
				case Unhosted:
					if srv != nil {
						srv.Close()
					}
				}
			}
		}
	}()
	return opened
}

// reachAsApp has the guest g, in the session s, tell whether it reaches
// the room the host moves to, as its app does: it tries it, and says it
// does unless cant.
func reachAsApp(t *testing.T, g *Guest, s *Session, cant bool) {
	t.Helper()
	ch := events(s)
	go func() {
		for {
			select {
			case <-s.Done():
				return
			case e := <-ch:
				if e.Kind == Reach {
					err := g.Reach(context.Background(), e.Addr, e.Key)
					s.Reached(err == nil && !cant)
				}
			}
		}
	}()
}

type moved struct {
	m   Move
	err error
}

// makeHost moves the room r to the guest g, and waits for how it went.
func makeHost(t *testing.T, r *testRoom, g *Guest) moved {
	t.Helper()
	done := make(chan moved, 1)
	if err := r.server.MakeHost(g.Identity.Fingerprint(), func(m Move, err error) { done <- moved{m, err} }); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		return got
	case <-time.After(15 * time.Second):
		t.Fatal("the move never ended")
	}
	return moved{}
}

// TestMakeHostMoves moves the room to a guest that the other reaches: both
// are told, and come back to it by their keys, the old host too; the key
// the room blocked stays out.
func TestMakeHostMoves(t *testing.T) {
	r := newTestRoom(t)
	ana, bo, cy := newGuest(t, "Ana"), newGuest(t, "Bo"), newGuest(t, "Cy")
	sa, sb := r.join(ana), r.join(bo)
	r.join(cy)
	r.server.Block(cy.Identity.Fingerprint())
	if !r.server.CanHost(ana.Identity.Fingerprint()) {
		t.Fatal("Ana can't host")
	}
	opened := hostAsApp(t, ana, sa)
	reachAsApp(t, bo, sb, false)
	got := makeHost(t, r, ana)
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.m.Key != ana.Identity.Fingerprint() || got.m.Name != "Ana" || !strings.HasPrefix(got.m.Addr, "127.0.0.1:") {
		t.Fatalf("moved %+v", got.m)
	}
	srv := <-opened
	// Until the room that moved there opens, none comes back to it.
	if _, err := bo.Rejoin(context.Background(), got.m.Addr, got.m.Key); !errors.Is(err, ErrRoomMoves) {
		t.Fatalf("Bo coming back before the room opened: %v", err)
	}
	srv.Open()
	for _, s := range []*Session{sa, sb} {
		select {
		case <-s.Done():
		case <-time.After(5 * time.Second):
			t.Fatal("a guest's session stays after the move")
		}
		if mv, ok := s.Moved(); !errors.Is(s.Err(), ErrMoved) || !ok || mv != got.m {
			t.Fatalf("the guest was told %v, %+v", s.Err(), mv)
		}
	}
	back, err := bo.Rejoin(context.Background(), got.m.Addr, got.m.Key)
	if err != nil {
		t.Fatal("Bo coming back:", err)
	}
	back.Close()
	old := &Guest{Identity: r.server.Identity, Name: "Desk", Clock: NewSystemClock(), Timing: testTiming}
	again, err := old.Rejoin(context.Background(), got.m.Addr, got.m.Key)
	if err != nil {
		t.Fatal("the old host coming back:", err)
	}
	again.Close()
	if _, err := cy.Knock(context.Background(), got.m.Addr, &got.m.Key, srv.Secret, true); !errors.Is(err, ErrBlocked) {
		t.Fatalf("the blocked key: %v", err)
	}
}

// TestMakeHostStays keeps the room where it is when a guest can't reach
// the one it would move to: the host is told who, and the guest that
// opened it closes it.
func TestMakeHostStays(t *testing.T) {
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa, sb := r.join(ana), r.join(bo)
	opened := hostAsApp(t, ana, sa)
	reachAsApp(t, bo, sb, true)
	got := makeHost(t, r, ana)
	if got.err == nil || got.err.Error() != "Bo can't reach Ana's computer" {
		t.Fatalf("moved %+v, %v", got.m, got.err)
	}
	srv := <-opened
	waitFor(t, "Ana's room closed", func() bool {
		srv.mu.Lock()
		defer srv.mu.Unlock()
		return srv.closed
	})
	if _, moving := r.server.Moving(); moving {
		t.Fatal("still moving")
	}
	select {
	case <-sb.Done():
		t.Fatal("Bo left a room that stayed")
	default:
	}
}

// TestMakeHostInTime keeps the room where it is when the guest doesn't
// answer in time; and refuses a move to a guest away, or one while another
// is under way.
func TestMakeHostInTime(t *testing.T) {
	was := answerFor
	answerFor = 300 * time.Millisecond
	t.Cleanup(func() { answerFor = was })
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	r.join(ana)
	sb := r.join(bo)
	done := make(chan error, 1)
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), func(_ Move, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	if err := r.server.MakeHost(bo.Identity.Fingerprint(), nil); !errors.Is(err, ErrMoving) {
		t.Fatalf("a second move: %v", err)
	}
	if err := <-done; err == nil || err.Error() != "Ana didn't answer in time" {
		t.Fatalf("moved: %v", err)
	}
	sb.Close()
	waitFor(t, "Bo away", func() bool { return !r.server.CanHost(bo.Identity.Fingerprint()) })
	if err := r.server.MakeHost(bo.Identity.Fingerprint(), nil); !errors.Is(err, ErrCantHost) {
		t.Fatalf("a move to a guest away: %v", err)
	}
}

// TestMakeHostReachedInTime gives the guest asked to host longer than the
// others to reach its room, and keeps the room where it is when they don't
// in time.
func TestMakeHostReachedInTime(t *testing.T) {
	wasAnswer, wasMove := answerFor, moveFor
	answerFor, moveFor = 5*time.Second, 300*time.Millisecond
	t.Cleanup(func() { answerFor, moveFor = wasAnswer, wasMove })
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa, sb := r.join(ana), r.join(bo)
	// Ana answers after more than moveFor; Bo never says it reaches.
	later := make(chan struct{})
	hostWhenAsked(t, ana, sa, later)
	reachWhenAsked(t, bo, sb, make(chan struct{}))
	done := make(chan error, 1)
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), func(_ Move, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	time.AfterFunc(2*moveFor, func() { close(later) })
	if err := <-done; err == nil || err.Error() != "not everyone reached Ana's computer in time" {
		t.Fatalf("moved: %v", err)
	}
}

// TestSeatsOfAHost takes the seats a host tells as keys that are keys,
// once each, MaxGuests and one more at most.
func TestSeatsOfAHost(t *testing.T) {
	id, _ := remote.NewIdentity()
	key := id.Fingerprint().String()
	seats := []Seat{{Key: key, Name: " Ana\x07"}, {Key: key, Name: "Again"}, {Key: "not a key", Name: "Bo"}}
	for range MaxGuests + 5 {
		other, _ := remote.NewIdentity()
		seats = append(seats, Seat{Key: other.Fingerprint().String(), Name: "Guest"})
	}
	got := membersOfSeats(seats, MaxGuests+1)
	if len(got) != MaxGuests-1 || got[0].Key != id.Fingerprint() || got[0].Name != "Ana" {
		t.Fatalf("%d seats, the first %+v", len(got), got[0])
	}
}

// TestBlockedSeatsBeyondTheGuests keeps more blocked keys than a room
// takes guests, as a host blocks those who left too.
func TestBlockedSeatsBeyondTheGuests(t *testing.T) {
	var seats []Seat
	for range maxBlocked + 10 {
		id, _ := remote.NewIdentity()
		seats = append(seats, Seat{Key: id.Fingerprint().String(), Name: "Blocked"})
	}
	if got := membersOfSeats(seats, maxBlocked); len(got) != maxBlocked {
		t.Fatalf("%d blocked kept", len(got))
	}
	// Their message, with every seat, takes what the protocol takes.
	long := strings.Repeat("\U0001F600", MaxName)
	all := make([]Seat, maxBlocked)
	for i := range all {
		all[i] = Seat{Key: seats[i].Key, Name: long}
	}
	data, err := json.Marshal(message{Type: "host", Seats: all[:MaxGuests+1], Blocked: all})
	if err != nil || len(data) > remote.MaxMessage {
		t.Fatalf("the host message takes %d bytes: %v", len(data), err)
	}
}

// TestReachGoesNowhereElse reaches the room a host moves to at its
// address alone: not where Elsewhere tells the room the guest is in is.
func TestReachGoesNowhereElse(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Ana")
	var mu sync.Mutex
	var dialed []string
	g.Elsewhere = func() []string { return []string{"127.0.0.1:1"} }
	g.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, addr)
		mu.Unlock()
		if addr == r.addr {
			// Slower than the stagger, after which another address would
			// be tried too.
			time.Sleep(2 * dialStagger)
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	if err := g.Reach(context.Background(), r.addr, r.key); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(dialed) != 1 || dialed[0] != r.addr {
		t.Fatalf("dialed %q", dialed)
	}
}
