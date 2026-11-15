package app

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomNewCode gives the room a new code and link: a guest in it stays,
// the old link knocks in vain, and the new one opens the room.
func TestRoomNewCode(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	other := newRoomApp(t, n, "Other")
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	old, _ := desk.Rooms().Hosted()
	joinRoom(t, lap, desk)
	if err := desk.Rooms().NewCode(); err != nil {
		t.Fatal(err)
	}
	now, _ := desk.Rooms().Hosted()
	if now.Code == old.Code || now.Link == old.Link || !strings.HasPrefix(now.Link, "mixmusic://join/"+now.Code+"#") {
		t.Fatalf("the code %s and link %s, after %s and %s", now.Code, now.Link, old.Code, old.Link)
	}
	other.browseRooms()
	if err := other.Rooms().OpenLink(old.Link); err != nil {
		t.Fatal(err)
	}
	v := untilGuest(t, other.App, "the old link refused", func(v GuestView) bool { return v.Step == RoomEnded })
	if !strings.Contains(v.Problem, "isn't the code") {
		t.Fatalf("problem %q", v.Problem)
	}
	if err := other.Rooms().OpenLink(now.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, other.App, "the new link's preview", func(v GuestView) bool { return v.Step == RoomPreview })
	if g, _ := lap.Rooms().Guest(); g.Step != RoomIn {
		t.Fatalf("the guest in the room shows %+v", g)
	}
}
