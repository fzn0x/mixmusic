package room

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

// closedAddr is an address of 127.0.0.1 that nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// TestDialFirstTakesTheOneThatAnswers dials an address that refuses, one
// that never answers and one that does: the one that does connects, soon.
func TestDialFirstTakesTheOneThatAnswers(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	silent := "192.0.2.1:9"
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == silent {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	start := time.Now()
	c, err := DialFirst(context.Background(), dial, []string{closedAddr(t), silent, ln.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.RemoteAddr().String() != ln.Addr().String() {
		t.Fatalf("connected to %v", c.RemoteAddr())
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v", d)
	}
}

// TestDialFirstTellsWhatItTried dials addresses that all refuse, or never
// answer: it tells them.
func TestDialFirstTellsWhatItTried(t *testing.T) {
	addrs := []string{closedAddr(t), closedAddr(t)}
	_, err := DialFirst(context.Background(), nil, addrs)
	var ue *UnreachableError
	if !errors.As(err, &ue) || len(ue.Addrs) != 2 {
		t.Fatalf("DialFirst: %v", err)
	}
	silent := func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = DialFirst(ctx, silent, []string{"192.0.2.1:9"})
	if !errors.As(err, &ue) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DialFirst of an address that never answers: %v", err)
	}
}

// TestKnockOverAddresses knocks at a room by the addresses of its link,
// the first of which nothing answers at.
func TestKnockOverAddresses(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Far")
	addrs := []string{closedAddr(t), r.addr}
	g.Dial = func(ctx context.Context, _, _ string) (net.Conn, error) { return DialFirst(ctx, nil, addrs) }
	s, err := g.Knock(context.Background(), addrs[0], &r.key, r.server.Secret, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Join(context.Background()); err != nil {
		t.Fatal(err)
	}
}
