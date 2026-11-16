package room

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// TestLeaveReachesABusyHost has a guest leave while the host is busy with
// what it asked before, and its own messages wait to be read: closed at
// once, with them unread, its connection was reset, which may drop the
// leave the host had yet to read, so that the host kept its place, as of a
// guest that dropped off. Its leave reaches the host each time.
func TestLeaveReachesABusyHost(t *testing.T) {
	var slow atomic.Bool
	r := newTestRoomWith(t, func(s *Server) {
		// A guest that dropped off stays a member a minute: a leave alone
		// takes it out in time.
		s.Timing.Keep = time.Minute
		s.Guests = &Guests{Rules: func() Rules {
			if slow.Load() {
				time.Sleep(100 * time.Millisecond)
			}
			return Rules{Songs: SongsOff}
		}}
	})
	song := remote.QueueTrack{Source: "youtube", ID: "yt:aaaaaaaaaaa", Title: "A song"}
	for i := range 5 {
		s := r.join(newGuest(t, "Ed"))
		// The guest reads slowly, so that the host's messages wait for it.
		s.Watch(func(Event) { time.Sleep(20 * time.Millisecond) })
		waitFor(t, "the guest here", func() bool { m := r.server.Members(); return len(m) == 1 && m[0].Here })
		// The host takes its time over the first song, and the second waits
		// to be taken in, so that the leave waits to be read.
		slow.Store(true)
		for range 2 {
			if _, err := s.AddSong(song); err != nil {
				t.Fatal(err)
			}
		}
		s.Leave()
		waitFor(t, "the host told the guest left", func() bool { return len(r.server.Members()) == 0 })
		slow.Store(false)
		if a := r.server.Activity(); a[len(a)-1].What != Left {
			t.Fatalf("try %d: the activity ends with %+v", i, a[len(a)-1])
		}
	}
}
