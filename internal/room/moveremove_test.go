package room

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// keptConn is a connection that its closing keeps open, so that whatever
// the server sends it still comes, whatever closed it before.
type keptConn struct{ net.Conn }

func (keptConn) Close() error { return nil }

// pipeConn is a server's connection to a guest of key over a pipe, whose
// messages come on the channel, by their types: one the server closes
// still takes them, until the test ends.
func pipeConn(t *testing.T, key remote.Fingerprint) (*conn, <-chan string) {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	types := make(chan string, 16)
	go func() {
		f := remote.NewFrames(b)
		for {
			var m message
			if f.Read(&m) != nil {
				return
			}
			types <- m.Type
		}
	}()
	return &conn{raw: keptConn{a}, f: newFrames(a, maxRate), key: key, out: make(chan message, outQueue), gone: make(chan struct{})}, types
}

// TestRemoveDuringAMoveSeatsNoOne removes a guest as the room's move
// commits, again and again: either the move commits first, and the guest
// is told the room moved, or the remove ends the move. A move never
// commits once the guest is gone, which would seat it in the new room.
func TestRemoveDuringAMoveSeatsNoOne(t *testing.T) {
	key := func() remote.Fingerprint { id, _ := remote.NewIdentity(); return id.Fingerprint() }
	ana, bo := key(), key()
	moves, ends := 0, 0
	for range 200 {
		to, _ := pipeConn(t, ana)
		gone, told := pipeConn(t, bo)
		s := &Server{Name: "Desk", guests: map[remote.Fingerprint]*guest{
			ana: {Member: Member{Key: ana, Name: "Ana", Here: true}, conn: to},
			bo:  {Member: Member{Key: bo, Name: "Bo", Here: true}, conn: gone},
		}}
		done := make(chan error, 1)
		mv := &move{to: ana, name: "Ana", conn: to, addr: "127.0.0.1:1", waiting: map[remote.Fingerprint]bool{}, self: true,
			timer: time.AfterFunc(time.Hour, func() {}), done: func(_ Move, err error) { done <- err }}
		s.asks.move = mv
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); s.Remove(bo) }()
		go func() { defer wg.Done(); s.reached(mv, nil, true) }()
		wg.Wait()
		if err := <-done; err != nil {
			ends++
			continue
		}
		moves++
		for heard := false; !heard; {
			select {
			case typ := <-told:
				heard = typ == "moved"
			case <-time.After(2 * time.Second):
				t.Fatal("the room moved once Bo was gone, with Bo's seat")
			}
		}
	}
	t.Logf("moved %d times, ended %d times", moves, ends)
}
