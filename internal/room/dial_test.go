package room

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
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

// otherRoom listens as a room does, with another key: what a stale
// address, or another's, answers.
func otherRoom(t *testing.T) string {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", id.ServerTLS(alpn))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				c.(*tls.Conn).Handshake()
				io.Copy(io.Discard, c)
			}()
		}
	}()
	return ln.Addr().String()
}

// silentAddr takes connections and never answers TLS, as an address no
// router forwards, but that answers TCP.
func silentAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				io.Copy(io.Discard, c)
			}()
		}
	}()
	return ln.Addr().String()
}

// TestKnockAtTheAddressThatIsTheHost knocks with the link at a room whose
// first addresses refuse, take TCP and never answer, and answer as another
// key, before its own: the guest joins the host, soon.
func TestKnockAtTheAddressThatIsTheHost(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Far")
	addrs := []string{closedAddr(t), silentAddr(t), otherRoom(t), r.addr}
	g.Elsewhere = func() []string { return addrs[1:] }
	start := time.Now()
	s, err := g.Knock(context.Background(), addrs[0], &r.key, r.server.Secret, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Host() != r.key {
		t.Fatal("joined another host")
	}
	if err := s.Join(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v", d)
	}
}

// TestKnockByCodeGoesWhereItIsTold knocks by the code, with no key to pin:
// the addresses elsewhere aren't tried, as any host would pass.
func TestKnockByCodeGoesWhereItIsTold(t *testing.T) {
	r := newTestRoom(t)
	g := newGuest(t, "Near")
	tried := false
	g.Elsewhere = func() []string { tried = true; return []string{otherRoom(t)} }
	s, err := g.Knock(context.Background(), r.addr, nil, []byte("ABC123"), false)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if tried {
		t.Fatal("a knock by the code tried the addresses elsewhere")
	}
}

// TestUnreachableTellsWhatItTried knocks where no address is the host's:
// refused, another key's, or silent until the wait runs out. It tells
// them all, and another key's answer as ErrNotSame.
func TestUnreachableTellsWhatItTried(t *testing.T) {
	key := remote.Fingerprint{1}
	g := newGuest(t, "Lost")
	g.Timing.Silence = 500 * time.Millisecond
	addrs := []string{closedAddr(t), otherRoom(t), silentAddr(t)}
	g.Elsewhere = func() []string { return addrs[1:] }
	_, err := g.Rejoin(context.Background(), addrs[0], key)
	var ue *UnreachableError
	if !errors.As(err, &ue) || len(ue.Addrs) != 3 || !errors.Is(err, ErrNotSame) {
		t.Fatalf("Rejoin: %v", err)
	}
}
