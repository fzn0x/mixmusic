package app

import (
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestRoomLinkToThisComputerIsRefused opens, from another network, a link
// whose address is this computer's own, as a made-up one may be: the
// guest never connects there, and says why. Typed by the user, the same
// address is joined as it is.
func TestRoomLinkToThisComputerIsRefused(t *testing.T) {
	desk := newRoomApp(t, roomtest.New(), "Desk")
	lap := newRoomApp(t, roomtest.New(), "Lap")
	lap.opts.Room.DialFar = nil
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	setRooms(t, desk.App, func(s *settings.Settings) { s.RoomAddress = "127.0.0.1" })
	desk.Rooms().SetAnywhere(true)
	untilReach(t, desk.App, "the address typed", func(v RoomReach) bool { return len(v.Addrs) == 1 })
	hosted, _ := desk.Rooms().Hosted()
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	v := untilGuest(t, lap.App, "the link refused", func(v GuestView) bool { return v.Step == RoomEnded })
	if v.Problem != roomFarProblem {
		t.Fatalf("the guest shows %q", v.Problem)
	}
	if h, _ := desk.Rooms().Hosted(); len(h.Guests) != 0 {
		t.Fatalf("the host has %+v", h.Guests)
	}
	lap.Rooms().KnockAt("127.0.0.1:"+strconv.Itoa(listenPort(desk)), hosted.Code)
	untilGuest(t, lap.App, "the preview at the address typed", func(v GuestView) bool { return v.Step == RoomPreview && v.Host == "Desk" })
}
