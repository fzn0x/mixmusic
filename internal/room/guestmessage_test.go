package room

import (
	"strings"
	"testing"
	"time"
)

// TestGuestMessagesBounded drops a guest whose message, once it joined, is
// longer than a guest's may be, from its length alone; it reads one of a
// guest's size.
func TestGuestMessagesBounded(t *testing.T) {
	r := newTestRoom(t)
	s := r.join(newGuest(t, "Ana"))
	waitFor(t, "here", func() bool { m := r.server.Members(); return len(m) == 1 && m[0].Here })
	// A ping of a long name, a message of about 100 KiB. The host ends the
	// connection as it reads the length, so the send may not end well.
	_ = s.f.Send(message{Type: "ping", Sent: s.Now(), Name: strings.Repeat("x", 100<<10)}, time.Second)
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the guest stays after a message too long")
	}
	waitFor(t, "away", func() bool { m := r.server.Members(); return len(m) == 1 && !m[0].Here })
	// One of 32 KiB is read: the leave after it is too.
	bo := r.join(newGuest(t, "Bo"))
	if err := bo.f.Send(message{Type: "ping", Sent: bo.Now(), Name: strings.Repeat("x", 32<<10)}, time.Second); err != nil {
		t.Fatal(err)
	}
	bo.Leave()
	waitFor(t, "Bo gone", func() bool { m := r.server.Members(); return len(m) == 1 && m[0].Name == "Ana" })
}
