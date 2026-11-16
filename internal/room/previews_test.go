package room

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

// knockAndWait knocks at r with its link, and leaves the session in the
// preview, as a user who looks and never joins.
func knockAndWait(t *testing.T, r *testRoom) *Session {
	t.Helper()
	s, err := newGuest(t, "Looker").Knock(context.Background(), r.addr, &r.key, r.server.Secret, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// previewsHeld is how many computers wait to join r.
func previewsHeld(r *testRoom) int {
	r.server.mu.Lock()
	defer r.server.mu.Unlock()
	return len(r.server.previews)
}

// TestPreviewsOfASourceAreBounded knocks 6 times with the link from one
// address, and joins none: the room holds 4, the two oldest are told the
// preview ended, and a guest that dropped off still comes back.
func TestPreviewsOfASourceAreBounded(t *testing.T) {
	r := newTestRoomWith(t, func(s *Server) {
		s.caps = &caps{unproven: maxUnproven, unprovenFrom: maxUnprovenFrom, tries: Tries, roomTries: RoomTries}
	})
	back := newGuest(t, "Back")
	r.join(back).Close()
	waitFor(t, "the guest away", func() bool { m := r.server.Members(); return len(m) == 1 && !m[0].Here })
	var all []*Session
	for range maxPreviewsFrom + 2 {
		all = append(all, knockAndWait(t, r))
	}
	for i, s := range all[:2] {
		waitFor(t, fmt.Sprintf("preview %d ended", i), func() bool {
			select {
			case <-s.Done():
				return errors.Is(s.Err(), ErrPreviewEnded)
			default:
				return false
			}
		})
	}
	if n := previewsHeld(r); n != maxPreviewsFrom {
		t.Fatalf("%d previews held, want %d", n, maxPreviewsFrom)
	}
	again, err := back.Rejoin(context.Background(), r.addr, r.key)
	if err != nil {
		t.Fatalf("a guest that comes back, with the previews full: %v", err)
	}
	again.Close()
	// The newest joins.
	if err := all[len(all)-1].Join(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// TestPreviewEndsAfterItsTime knocks with the link and never joins: the
// preview ends after Timing.Preview.
func TestPreviewEndsAfterItsTime(t *testing.T) {
	r := newTestRoomWith(t, func(s *Server) { s.Timing.Preview = 300 * time.Millisecond })
	s := knockAndWait(t, r)
	select {
	case <-s.Done():
		if !errors.Is(s.Err(), ErrPreviewEnded) {
			t.Fatalf("the preview ended with %v", s.Err())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the preview lasted")
	}
	if n := previewsHeld(r); n != 0 {
		t.Fatalf("%d previews held", n)
	}
	// One that joins in time stays past it: it is watched for twice the
	// preview's time.
	in := r.join(newGuest(t, "Quick"))
	deadline := time.Now().Add(600 * time.Millisecond)
	for time.Now().Before(deadline) {
		select {
		case <-in.Done():
			t.Fatalf("the guest that joined ended: %v", in.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// previewConn is a connection among the previews of the source src, whose
// other end reads what it is sent.
func previewConn(t *testing.T, src string) *conn {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := b.Read(buf); err != nil {
				return
			}
		}
	}()
	return &conn{raw: a, f: newFrames(a, maxRate), src: src, out: make(chan message, 1), gone: make(chan struct{})}
}

// TestFullPreviewsMakeRoomFromTheMostCrowdedSource fills the previews from
// 9 sources that each hold a few, beside one guest's: their new previews
// take each other's places, never the guest's, and the room holds 32.
func TestFullPreviewsMakeRoomFromTheMostCrowdedSource(t *testing.T) {
	s := &Server{Timing: Timing{Preview: time.Minute}, previews: map[*conn]int{}}
	s.adm.caps = (*caps)(nil).orDefault()
	guest := previewConn(t, "198.51.100.7")
	s.mu.Lock()
	s.previews[guest] = 0
	s.holdPreview(guest)
	s.mu.Unlock()
	for i := range 200 {
		c := previewConn(t, fmt.Sprintf("203.0.113.%d", i%9))
		s.mu.Lock()
		s.previews[c] = 0
		s.holdPreview(c)
		_, kept := s.previews[guest]
		n := len(s.previews)
		s.mu.Unlock()
		if !kept {
			t.Fatalf("preview %d took the guest's place", i)
		}
		if n > maxPreviews {
			t.Fatalf("%d previews", n)
		}
	}
}

// TestKnocksKeepTheirPreviews keeps the preview of a computer that waits
// for the host's user to let it in, however many previews of its source,
// or of others, come after it.
func TestKnocksKeepTheirPreviews(t *testing.T) {
	s := &Server{Timing: Timing{Preview: time.Minute}, previews: map[*conn]int{}}
	s.adm.caps = (*caps)(nil).orDefault()
	knocker := previewConn(t, "203.0.113.1")
	s.mu.Lock()
	s.previews[knocker] = 0
	s.holdPreview(knocker)
	s.asks.knocks = append(s.asks.knocks, &Knock{ID: 1, conn: knocker, src: knocker.src})
	s.mu.Unlock()
	for i := range 200 {
		src := "203.0.113.1"
		if i%2 == 1 {
			src = fmt.Sprintf("198.51.100.%d", i%7)
		}
		c := previewConn(t, src)
		s.mu.Lock()
		s.previews[c] = 0
		s.holdPreview(c)
		_, kept := s.previews[knocker]
		s.mu.Unlock()
		if !kept {
			t.Fatalf("preview %d took the place of one that knocks", i)
		}
	}
}
