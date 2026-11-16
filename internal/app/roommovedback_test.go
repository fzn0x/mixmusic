package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomOldHostComesBackWhereTheRoomMoved moves the room to a guest
// whose room the old host finds only at an address that refuses, as its
// address on a VPN: the old host comes back where the room moved, as it
// reached the new host.
func TestRoomOldHostComesBackWhereTheRoomMoved(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	desk.GoPlayer().PlayTracks(tracksOf("a", "b"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	onVPN(t, desk, n, false)
	if err := desk.Rooms().MakeHost(lap.opts.Remote.Identity.Fingerprint().String()); err != nil {
		t.Fatal(err)
	}
	answerHost(t, lap, true)
	untilGuest(t, desk.App, "the old host in Lap's room", func(v GuestView) bool { return v.Step == RoomIn && v.Host == "Lap" })
	if f, ok := desk.foundRoom(lap.opts.Remote.Identity.Fingerprint().ID(), false); !ok || len(f.Others) != 0 {
		t.Fatalf("Desk finds Lap's room at %+v", f)
	}
}
