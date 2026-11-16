package room

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestKnockTakesSmallMessagesFirst knocks at something that takes TLS as a
// room does, then says a message of a megabyte is coming: the guest takes
// no message that long before the host proved the code, and stops at once.
func TestKnockTakesSmallMessagesFirst(t *testing.T) {
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		defer raw.Close()
		tc := tls.Server(raw, id.ServerTLS(alpn))
		if tc.Handshake() != nil {
			return
		}
		var hello message
		if remote.NewFrames(tc).Receive(&hello, 5*time.Second) != nil {
			return
		}
		tc.Write([]byte{0, 0x10, 0, 0})
		// Nothing more, until the guest goes.
		io.Copy(io.Discard, tc)
	}()
	g := newGuest(t, "Careful")
	g.Timing.Silence = 8 * time.Second
	start := time.Now()
	if _, err := g.Knock(context.Background(), ln.Addr().String(), nil, []byte("ABC123"), false); err == nil {
		t.Fatal("knocked")
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("the guest waited %v for a message too long to take", d)
	}
}
