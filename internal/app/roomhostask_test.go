package app

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomHostAskDeclined asks the guest's user before its computer hosts
// the room: declined, the room stays, and the host is told; a host that
// asks again at once is declined unasked; one the user doesn't answer is
// declined in time; and a host declined twice is declined unasked from
// then on.
func TestRoomHostAskDeclined(t *testing.T) {
	was := hostAskFor
	hostAskFor = 300 * time.Millisecond
	t.Cleanup(func() { hostAskFor = was })
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	deskNotes, lapNotes := roomNotes(t, desk.App), roomNotes(t, lap.App)
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	lapKey := lap.opts.Remote.Identity.Fingerprint().String()
	if err := desk.Rooms().MakeHost(lapKey); err != nil {
		t.Fatal(err)
	}
	toldNote(t, lapNotes, "Desk asks you to host the room")
	if name, ok := lap.Rooms().HostAsk(); !ok || name != "Desk" {
		t.Fatalf("asked by %q, %v", name, ok)
	}
	if _, hosts := lap.Rooms().Hosted(); hosts {
		t.Fatal("Lap hosts a room before its user said so")
	}
	lap.Rooms().AnswerHost(false)
	stays := "The room stays here: Lap doesn't host the room"
	toldNote(t, deskNotes, stays)
	// Again at once: declined unasked.
	until(t, "the move over", func() bool { _, moving := desk.Rooms().Moving(); return !moving })
	if err := desk.Rooms().MakeHost(lapKey); err != nil {
		t.Fatal(err)
	}
	until(t, "declined unasked", func() bool {
		return countNotes(deskNotes(), stays) == 2
	})
	if n := countNotes(lapNotes(), "Desk asks you to host the room"); n != 1 {
		t.Fatalf("Lap's user was asked %d times", n)
	}
	// Once a while passed, the host asks again; unanswered, it's declined.
	lap.room.mu.Lock()
	lap.room.guest.askedAt = time.Time{}
	lap.room.mu.Unlock()
	until(t, "the move over", func() bool { _, moving := desk.Rooms().Moving(); return !moving })
	if err := desk.Rooms().MakeHost(lapKey); err != nil {
		t.Fatal(err)
	}
	until(t, "declined in time", func() bool { return countNotes(deskNotes(), stays) == 3 })
	if _, ok := lap.Rooms().HostAsk(); ok {
		t.Fatal("the ask stays once declined in time")
	}
	if _, ok := desk.Rooms().Hosted(); !ok {
		t.Fatal("the room moved")
	}
	// Declined twice, the host asks no more for the rest of the stay, a
	// while after too.
	lap.room.mu.Lock()
	lap.room.guest.askedAt = time.Time{}
	lap.room.mu.Unlock()
	until(t, "the move over", func() bool { _, moving := desk.Rooms().Moving(); return !moving })
	if err := desk.Rooms().MakeHost(lapKey); err != nil {
		t.Fatal(err)
	}
	until(t, "declined unasked again", func() bool { return countNotes(deskNotes(), stays) == 4 })
	if n := countNotes(lapNotes(), "Desk asks you to host the room"); n != 2 {
		t.Fatalf("Lap's user was asked %d times", n)
	}
}

// countNotes counts note among notes.
func countNotes(notes []string, note string) int {
	n := 0
	for _, s := range notes {
		if s == note {
			n++
		}
	}
	return n
}
