package room

import (
	"context"
	"errors"
	"net"
	"testing"
)

// hostWhenAsked has the guest g, in the session s, open a room when its
// host asks it to host, as its app does, and say where once release closes.
func hostWhenAsked(t *testing.T, g *Guest, s *Session, release <-chan struct{}) {
	t.Helper()
	ch := events(s)
	go func() {
		for {
			select {
			case <-s.Done():
				return
			case e := <-ch:
				if e.Kind != HostAsked {
					continue
				}
				clock := NewSystemClock()
				seats := e.Seats
				srv := &Server{Identity: g.Identity, Name: g.Name, Code: "XYZ789", Secret: []byte("0123456789abcdef"), Host: &fakeHost{clock: clock},
					Clock: clock, Timing: testTiming, Seats: &seats}
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					return
				}
				go srv.Serve(ln)
				t.Cleanup(func() { srv.Close() })
				<-release
				s.Hosting(ln.Addr().(*net.TCPAddr).Port)
			}
		}
	}()
}

// reachWhenAsked has the guest g, in the session s, try the room the host
// moves to, and say whether it reaches it once release closes.
func reachWhenAsked(t *testing.T, g *Guest, s *Session, release <-chan struct{}) {
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
					<-release
					s.Reached(err == nil)
				}
			}
		}
	}()
}

// TestBlockStopsAMove keeps the room where it is when the host blocks the
// guest it moves to once that one told where its room listens: no guest
// is told the room moved.
func TestBlockStopsAMove(t *testing.T) {
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa, sb := r.join(ana), r.join(bo)
	now := make(chan struct{})
	close(now)
	hostWhenAsked(t, ana, sa, now)
	later := make(chan struct{})
	reachWhenAsked(t, bo, sb, later)
	done := make(chan error, 1)
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), func(_ Move, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	// Ana told where; Bo tries it, and waits to answer.
	waitFor(t, "Ana's room told", func() bool {
		r.server.mu.Lock()
		defer r.server.mu.Unlock()
		return r.server.asks.move != nil && r.server.asks.move.addr != ""
	})
	r.server.Block(ana.Identity.Fingerprint())
	close(later)
	if err := <-done; !errors.Is(err, errDropped) {
		t.Fatalf("moved: %v", err)
	}
	// The move ended before it told anyone, as it tells only once it
	// moved, which it can no longer do.
	if _, moving := r.server.Moving(); moving {
		t.Fatal("still moving")
	}
	select {
	case <-sb.Done():
		t.Fatalf("Bo left the room: %v", sb.Err())
	default:
	}
}

// TestRemoveStopsAMove keeps the room where it is when the host removes a
// guest before the guest it moves to told where its room listens, which
// would keep that one's place.
func TestRemoveStopsAMove(t *testing.T) {
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa := r.join(ana)
	r.join(bo)
	release := make(chan struct{})
	hostWhenAsked(t, ana, sa, release)
	done := make(chan error, 1)
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), func(_ Move, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	r.server.Remove(bo.Identity.Fingerprint())
	if err := <-done; !errors.Is(err, errDropped) {
		t.Fatalf("moved: %v", err)
	}
	close(release)
	if _, moving := r.server.Moving(); moving {
		t.Fatal("still moving")
	}
}

// TestMoveAfterRemoveSeatsNoOneRemoved starts a move once a guest was
// removed: the seats it tells are of the guests the room has then.
func TestMoveAfterRemoveSeatsNoOneRemoved(t *testing.T) {
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa := r.join(ana)
	r.join(bo)
	ch := events(sa)
	r.server.Remove(bo.Identity.Fingerprint())
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), nil); err != nil {
		t.Fatal(err)
	}
	e := next(t, ch, HostAsked)
	for _, m := range e.Seats.Guests {
		if m.Key == bo.Identity.Fingerprint() {
			t.Fatal("the guest removed has a seat")
		}
	}
}

// TestLeaveStopsAMove keeps the room where it is when a guest leaves as it
// moves, whose seat the room it moves to would keep.
func TestLeaveStopsAMove(t *testing.T) {
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa, sb := r.join(ana), r.join(bo)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	hostWhenAsked(t, ana, sa, release)
	done := make(chan error, 1)
	if err := r.server.MakeHost(ana.Identity.Fingerprint(), func(_ Move, err error) { done <- err }); err != nil {
		t.Fatal(err)
	}
	sb.Leave()
	if err := <-done; !errors.Is(err, errLeft) {
		t.Fatalf("moved: %v", err)
	}
}
