package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestRoomSkipsRest rests a guest's skipping of the parts SponsorBlock
// marks while it follows its host, whose player skips what its own
// settings skip, and the guest follows by the seek: they come back as it
// leaves.
func TestRoomSkipsRest(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	setRules(t, lap.App, func(s *settings.Settings) { s.SkipParts.On = true })
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	if (playerServices{lap.App}).Settings().SkipParts.On {
		t.Fatal("the guest skips parts of the host's track by its own settings")
	}
	lap.Rooms().Leave()
	if !(playerServices{lap.App}).Settings().SkipParts.On {
		t.Fatal("the guest's skipping of parts didn't come back")
	}
}
