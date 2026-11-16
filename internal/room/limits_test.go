package room

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestConnectionsOfASourceAreBounded joins 32 guests from one address, as
// many as a source may hold: the next can't connect, until one leaves.
func TestConnectionsOfASourceAreBounded(t *testing.T) {
	r := newTestRoom(t)
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
