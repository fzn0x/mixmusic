package room

import (
	"io"
	"net"
	"testing"
)

// forward forwards the connections it takes to addr, as a router forwards
// a port of its internet address, which may be another, to a computer of
// its network. It returns where it takes them.
func forward(t *testing.T, addr string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			in, err := ln.Accept()
			if err != nil {
				return
			}
			out, err := net.Dial("tcp", addr)
			if err != nil {
				in.Close()
				continue
			}
			t.Cleanup(func() { in.Close(); out.Close() })
			go func() { io.Copy(out, in); out.Close() }()
			go func() { io.Copy(in, out); in.Close() }()
		}
	}()
	return ln.Addr().String()
}

// farHostAsApp is hostAsApp, of a guest whose room is reached from
// outside its network at the router's port that forward forwards to it.
func farHostAsApp(t *testing.T, g *Guest, s *Session) (<-chan *Server, <-chan string) {
	t.Helper()
	opened, at := make(chan *Server, 1), make(chan string, 1)
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
					s.Hosting(0)
					continue
				}
				go srv.Serve(ln)
				t.Cleanup(func() { srv.Close() })
				router := forward(t, ln.Addr().String())
				at <- router
				s.Hosting(ln.Addr().(*net.TCPAddr).Port, "not an address", router)
				opened <- srv
			}
		}
	}()
	return opened, at
}

// TestMakeHostFarAway moves the room to a guest of another network, as
// all its computers are: the host and the other guest reach the room it
// opened, and come back to it, at the port its router forwards, not at
// the one it listens on.
func TestMakeHostFarAway(t *testing.T) {
	r := nearRoom(t, func(string) bool { return false }, nil)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa, sb := r.join(ana), r.join(bo)
	_, router := farHostAsApp(t, ana, sa)
	bch := events(sb)
	reachAsApp(t, bo, sb, false)
	got := makeHost(t, r, ana)
	if got.err != nil {
		t.Fatal(got.err)
	}
	at := <-router
	if got.m.Addr != at {
		t.Fatalf("moved to %s, not at the router's %s", got.m.Addr, at)
	}
	if e := next(t, bch, Reach); e.Addr != at {
		t.Fatalf("Bo tried %s, not the router's %s", e.Addr, at)
	}
	waitFor(t, "Bo told the room moved", func() bool { m, ok := sb.Moved(); return ok && m.Addr == at })
}

// TestMakeHostNearbyKeepsItsPort moves the room to a guest of the host's
// network, which tells where its router forwards too: those of the network
// reach it at the port it listens on.
func TestMakeHostNearbyKeepsItsPort(t *testing.T) {
	r := newTestRoom(t)
	ana, bo := newGuest(t, "Ana"), newGuest(t, "Bo")
	sa, sb := r.join(ana), r.join(bo)
	opened, router := farHostAsApp(t, ana, sa)
	bch := events(sb)
	reachAsApp(t, bo, sb, false)
	got := makeHost(t, r, ana)
	if got.err != nil {
		t.Fatal(got.err)
	}
	srv := <-opened
	srv.mu.Lock()
	listens := srv.ln.Addr().String()
	srv.mu.Unlock()
	if at := <-router; got.m.Addr != listens || got.m.Addr == at {
		t.Fatalf("moved to %s, not where the room listens, %s", got.m.Addr, listens)
	}
	if e := next(t, bch, Reach); e.Addr != listens {
		t.Fatalf("Bo tried %s, not %s", e.Addr, listens)
	}
}

// TestMoveAddrForEach gives each the address it reaches the room a move
// goes to at: the address the host reached the guest at, while both are
// of the host's networks, and its router's otherwise.
func TestMoveAddrForEach(t *testing.T) {
	for _, tc := range []struct {
		target, near bool
		far          []string
		want         string
	}{
		{true, true, []string{"203.0.113.7:5000"}, "192.168.1.5:4000"},
		{true, false, []string{"203.0.113.7:5000"}, "203.0.113.7:5000"},
		{false, true, []string{"203.0.113.7:5000"}, "203.0.113.7:5000"},
		{false, false, nil, "192.168.1.5:4000"},
	} {
		mv := &move{conn: &conn{near: tc.target}, addr: "192.168.1.5:4000", far: tc.far}
		if got := mv.addrFor(tc.near); got != tc.want {
			t.Errorf("guest it moves to near %v, guest near %v: %s, want %s", tc.target, tc.near, got, tc.want)
		}
	}
	if got := farAddrs([]string{"bad", "203.0.113.7:5000", "203.0.113.7:5000", "[2001:db8::5]:5000", "a.example:1", "b.example:2", "c.example:3"}); len(got) != MaxAddrs || got[0] != "203.0.113.7:5000" || got[1] != "[2001:db8::5]:5000" {
		t.Fatalf("the addresses kept: %q", got)
	}
}
