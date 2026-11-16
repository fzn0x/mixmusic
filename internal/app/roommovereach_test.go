package app

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestReachGate lets a guest try the room its host moves to once each
// MoveFor at most.
func TestReachGate(t *testing.T) {
	var g reachGate
	now := time.Now()
	if !g.take(now) {
		t.Fatal("the first try refused")
	}
	if g.take(now.Add(room.MoveFor / 2)) {
		t.Fatal("a second try within MoveFor")
	}
	if !g.take(now.Add(room.MoveFor)) {
		t.Fatal("a try once MoveFor passed refused")
	}
}

// TestReachFailsLate tells the host a guest doesn't reach the room it
// moves to only reachFailsAfter after it asked, though nothing listens
// there, which the guest knows at once.
func TestReachFailsLate(t *testing.T) {
	was := reachFailsAfter
	reachFailsAfter = 300 * time.Millisecond
	t.Cleanup(func() { reachFailsAfter = was })
	lap := newRoomApp(t, roomtest.New(), "Lap")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // refused at once
	other, _ := remote.NewIdentity()
	asked := time.Now()
	got := make(chan bool, 1)
	lap.reach(func(ok bool) error { got <- ok; return nil }, other.Fingerprint(), addr)
	if ok := <-got; ok {
		t.Fatal("reached a room where nothing listens")
	}
	if waited := time.Since(asked); waited < reachFailsAfter {
		t.Fatalf("told after %v", waited)
	}
}

// TestReachFailsAtOneMoment tells the host a guest doesn't reach the room
// it moves to at reachFailsAfter, where something takes the connection and
// never answers, as where nothing listens: a no tells neither apart.
func TestReachFailsAtOneMoment(t *testing.T) {
	was := reachFailsAfter
	reachFailsAfter = 300 * time.Millisecond
	t.Cleanup(func() { reachFailsAfter = was })
	lap := newRoomApp(t, roomtest.New(), "Lap")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var held []net.Conn // taken, never answered
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range held {
			c.Close()
		}
	})
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			held = append(held, c)
			mu.Unlock()
		}
	}()
	other, _ := remote.NewIdentity()
	asked := time.Now()
	got := make(chan bool, 1)
	lap.reach(func(ok bool) error { got <- ok; return nil }, other.Fingerprint(), ln.Addr().String())
	if ok := <-got; ok {
		t.Fatal("reached a room that never answered")
	}
	if waited := time.Since(asked); waited < reachFailsAfter || waited > reachFailsAfter+time.Second {
		t.Fatalf("told after %v", waited)
	}
}
