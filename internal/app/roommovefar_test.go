package app

import (
	"slices"
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestRoomOpenedForAMoveTellsItsReach opens a room for the room this
// computer is a guest of to move to: while friends outside may join, it
// tells the host where they reach it, the address typed for invites with
// the room's port; while they may not, nowhere.
func TestRoomOpenedForAMoveTellsItsReach(t *testing.T) {
	lap := newRoomApp(t, roomtest.New(), "Lap")
	for _, anywhere := range []bool{true, false} {
		setRooms(t, lap.App, func(s *settings.Settings) { s.RoomAnywhere, s.RoomAddress = anywhere, "198.51.100.9" })
		h, port, err := lap.openRoom(&room.Seats{})
		if err != nil {
			t.Fatal(err)
		}
		got := lap.reachAddrs(moveReachWait)
		h.close()
		var want []string
		if anywhere {
			want = []string{"198.51.100.9:" + strconv.Itoa(port)}
		}
		if !slices.Equal(got, want) {
			t.Errorf("with friends outside joining %v, it tells %q, want %q", anywhere, got, want)
		}
	}
}
