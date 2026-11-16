package room

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

// TestConnectionsOfASourceAreBounded joins 32 guests from one address, as
// many as a source may hold: the next can't connect, until one leaves.
func TestConnectionsOfASourceAreBounded(t *testing.T) {
	// The guests open their connections faster than a source may.
	r := newTestRoomWith(t, func(s *Server) {
		s.caps = &caps{unproven: maxUnproven, unprovenFrom: maxUnprovenFrom, tries: Tries, roomTries: RoomTries}
	})
	var in []*Session
	for range maxConnsFrom {
		in = append(in, r.join(newGuest(t, "Guest")))
	}
	waitFor(t, "every guest here", func() bool { return r.server.Listeners() == maxConnsFrom })
	if _, err := newGuest(t, "One more").Knock(context.Background(), r.addr, nil, []byte("ABC123"), false); err == nil {
		t.Fatal("a guest past the source's connections knocked")
	}
	in[0].Leave()
	waitFor(t, "the guest gone", func() bool { return r.server.Listeners() == maxConnsFrom-1 })
	r.join(newGuest(t, "Another"))
}

// TestLimitListenerBoundsAll holds as many connections as it takes in
// all, of any sources, and closes the next as it comes.
func TestLimitListenerBoundsAll(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln := &limitListener{Listener: raw, most: 100, all: 3, from: map[string]int{}}
	defer ln.Close()
	accepted := make(chan net.Conn, 10)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()
	var clients []net.Conn
	for range 4 {
		c, err := net.Dial("tcp", raw.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		clients = append(clients, c)
	}
	var held []net.Conn
	for range 3 {
		held = append(held, <-accepted)
	}
	// The fourth is closed by the listener: a read ends at once.
	clients[3].SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := clients[3].Read(make([]byte, 1)); err == nil {
		t.Fatal("the fourth connection reads")
	}
	select {
	case c := <-accepted:
		t.Fatalf("a fourth connection held: %v", c.RemoteAddr())
	default:
	}
	held[0].Close()
	c, err := net.Dial("tcp", raw.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	select {
	case <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("no connection held after one closed")
	}
}

// TestFramesReadAtTheirRate sends a connection's messages faster than the
// room reads them: past the burst, they're read at the rate, and all of
// them are.
func TestFramesReadAtTheirRate(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	const rate, sent = 50, 100
	out := newFrames(a, rate)
	in := newFrames(b, rate)
	go func() {
		for i := range sent {
			if out.Send(message{Type: "ping", Seq: int64(i)}, 5*time.Second) != nil {
				return
			}
		}
	}()
	start := time.Now()
	for i := range sent {
		var m message
		if err := in.Receive(&m, 5*time.Second); err != nil || m.Seq != int64(i) {
			t.Fatalf("message %d: %+v, %v", i, m, err)
		}
		if i == rate-1 {
			if d := time.Since(start); d > 500*time.Millisecond {
				t.Fatalf("the burst of %d took %v", rate, d)
			}
		}
	}
	// 50 at once, then 50 at 50 a second: a second at least.
	if d := time.Since(start); d < 900*time.Millisecond {
		t.Fatalf("%d messages read in %v, at %d a second", sent, d, rate)
	}
}

// TestSourceOpensAFewASecond opens connections from one source faster
// than it may: past the burst, they're closed as they come, before TLS,
// and the source opens more a moment later; another source opens its own
// meanwhile.
func TestSourceOpensAFewASecond(t *testing.T) {
	l := &limitListener{most: 100, all: 100, rate: acceptRate, from: map[string]int{}, buckets: map[string]*bucket{}}
	now := time.Now()
	opened := 0
	for range 3 * acceptBurst {
		if l.opens("203.0.113.7", now) {
			opened++
		}
	}
	if opened != acceptBurst {
		t.Fatalf("%d opened at once, want %d", opened, acceptBurst)
	}
	if !l.opens("198.51.100.1", now) {
		t.Fatal("another source can't open one")
	}
	if !l.opens("203.0.113.7", now.Add(time.Second)) {
		t.Fatal("the source can't open one a second later")
	}
	n := 0
	for later := now.Add(time.Second); n < 100 && l.opens("203.0.113.7", later); n++ {
	}
	if n > acceptRate {
		t.Fatalf("%d more in the same second", n)
	}
	// The buckets stay bounded, those of sources long quiet going first.
	for i := range maxBuckets + 10 {
		l.opens(fmt.Sprintf("10.%d.%d.1", i/256, i%256), now.Add(time.Duration(i)*time.Millisecond))
	}
	if len(l.buckets) > maxBuckets {
		t.Fatalf("%d buckets", len(l.buckets))
	}
}

// TestRoomClosesAConnectionPastTheRate knocks at a room from one source
// faster than it may open connections: those past the burst can't even
// take TLS.
func TestRoomClosesAConnectionPastTheRate(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Hasty")
	failed := 0
	for range acceptBurst + 4 {
		s, err := g.Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
		if err != nil {
			failed++
			continue
		}
		s.Close()
	}
	if failed < 2 {
		t.Fatalf("%d of %d knocks at once failed", failed, acceptBurst+4)
	}
}

// TestWrongCodesFromOutsideLockNoCodeOnTheNetwork has six sources of the
// internet try 5 wrong codes each, as many as the room takes a minute: the
// room takes no more codes from the internet, but still from its own
// network, and the other way round.
func TestWrongCodesFromOutsideLockNoCodeOnTheNetwork(t *testing.T) {
	a := newAdmission()
	now := time.Now()
	for i := range RoomTries / Tries {
		src := fmt.Sprintf("203.0.113.%d", i)
		for range Tries {
			if got := a.tryCode(src, now, time.Minute); got != tryRuns {
				t.Fatalf("%s: %v", src, got)
			}
			a.endTry(src, true, now, time.Minute)
		}
	}
	if got := a.tryCode("198.51.100.9", now, time.Minute); got != tryLocked {
		t.Fatalf("another source of the internet: %v, want locked", got)
	}
	for _, src := range []string{"192.168.1.20", "fd00:1:2:300::/56", "fe80::/56", "127.0.0.1"} {
		if got := a.tryCode(src, now, time.Minute); got != tryRuns {
			t.Fatalf("%s of the network: %v, want it to run", src, got)
		}
		a.endTry(src, false, now, time.Minute)
	}
}

// TestLocalSource tells the sources of the room's own network from those
// of the internet, VPNs' shared range among them.
func TestLocalSource(t *testing.T) {
	for src, want := range map[string]bool{
		"192.168.1.20": true, "10.1.2.3": true, "172.16.5.4": true, "127.0.0.1": true, "169.254.3.4": true,
		"fd00:1:2:300::/56": true, "fe80::/56": true,
		"203.0.113.7": false, "100.101.102.103": false, "2001:db8:1:200::/56": false, "not an address": false,
	} {
		if got := localSource(src); got != want {
			t.Errorf("localSource(%q) = %v", src, got)
		}
	}
}
