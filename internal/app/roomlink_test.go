package app

import (
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomLinkOpensTheRoom opens an invite link as the system hands it
// over: it knocks at its room, and shows the room's page.
func TestRoomLinkOpensTheRoom(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.Rooms().Hosted()
	var shown atomic.Value
	t.Cleanup(lap.OnNavigate(func(target string) { shown.Store(target) }))
	shown2 := false
	if !lap.followRoomLink(mustURL(t, hosted.Link), func() { shown2 = true }) || !shown2 {
		t.Fatal("the invite link isn't one")
	}
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	if got, _ := shown.Load().(string); got != RoomPage {
		t.Fatalf("shows %q", got)
	}
	if lap.followRoomLink(mustURL(t, "mixmusic://album/x"), func() {}) {
		t.Fatal("an album's link was taken for an invite")
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
