package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// newSongRoomShell is newRoomShell, whose rooms find their guests' songs
// as they are, in place of the services.
func newSongRoomShell(t *testing.T, n *roomtest.Network, name string) *shell {
	t.Helper()
	s := newRemoteShellOf(t, remotetest.New(), name, func(o *app.Options) {
		o.Room = &app.RoomOptions{Discovery: n, Listen: roomtest.Listen, Timing: roomTiming,
			Song: func(_ context.Context, t media.Track) (media.Track, error) { return t, nil }}
	})
	t.Cleanup(func() {
		s.a.Rooms().End()
		s.a.Rooms().Leave()
	})
	return s
}

// quietRooms has s show no notifications, which the tests' windows can't.
func quietRooms(t *testing.T, s *shell) {
	t.Helper()
	if _, err := s.svc.Preferences.Update(func(set *settings.Settings) { set.Notifications = false }); err != nil {
		t.Fatal(err)
	}
}

// joinByLink has lap open the invite link of desk's room, and join it.
func joinByLink(t *testing.T, desk *shell, dt *ui.Tester, lap *shell, lt *ui.Tester) {
	t.Helper()
	hosted, ok := desk.a.Rooms().Hosted()
	if !ok {
		t.Fatal("no room hosted")
	}
	foundRoom(t, lap, "Desk")
	if err := lap.a.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilBoth(t, desk, dt, lap, lt, "the preview", func() bool { v, _ := lap.a.Rooms().Guest(); return v.Step == app.RoomPreview })
	lap.a.Rooms().Join()
}

// TestRoomRuleRows shows the host's rules in Settings › Desktop and on the
// room's page, and keeps them as they change.
func TestRoomRuleRows(t *testing.T) {
	n := roomtest.New()
	s := newRoomShell(t, n, "Desk")
	tt := tester(t, s)
	show(s, tt, "/settings/desktop")
	for _, text := range []string{"Songs from guests", "Guests can control playback", "Ask me before someone joins"} {
		if !tt.HasText(text) {
			t.Fatalf("Settings › Desktop doesn't show %q: %q", text, tt.Texts())
		}
	}
	click(t, s, tt, "add them")
	clickSwitch(t, s, tt, "Guests can control playback")
	clickSwitch(t, s, tt, "Ask me before someone joins")
	if got := s.svc.Preferences.Get(); got.RoomSongs != settings.RoomSongsAdd || !got.RoomGuestControl || !got.RoomAskJoin {
		t.Fatalf("the settings keep songs %q, control %v, ask %v", got.RoomSongs, got.RoomGuestControl, got.RoomAskJoin)
	}
	if err := s.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(s, tt, roomsPath)
	until(t, s, tt, "the rules on the room's page", func() bool {
		return tt.HasText("What guests can do") && tt.HasText("Songs from guests")
	})
}

// TestRoomSongFromTheMenu adds a song to the room from its menu on the
// guest, which waits on the host's page, whose user declines it: the
// guest is told.
func TestRoomSongFromTheMenu(t *testing.T) {
	n := roomtest.New()
	desk := newSongRoomShell(t, n, "Desk")
	lap := newRoomShell(t, n, "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	quietRooms(t, desk)
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinByLink(t, desk, dt, lap, lt)
	untilBoth(t, desk, dt, lap, lt, "the host's rules", func() bool { _, ok := lap.a.Rooms().RoomRules(); return ok })
	song := media.Track{ID: "yt:" + strings.Repeat("s", 11), Source: media.SourceYouTube, Title: "Room song", DurationMs: 200000,
		Artists: []media.Ref{{Name: "Band"}}}
	pl := media.Playlist{ID: "PLroom", Title: "For the room", Tracks: []media.Track{song}}
	lap.loads["playlist:"+pl.ID] = &loaded[media.Playlist]{done: true, value: pl}
	show(lap, lt, playlistPath(pl.ID))
	until(t, lap, lt, "the playlist", func() bool { return lt.HasText("Room song") })
	if items := menuOf(t, lt, "Room song"); !slices.Contains(items, "Ask to add to the room") {
		t.Fatalf("the song's menu is %q", items)
	}
	if err := lt.RightClick("Room song"); err != nil {
		t.Fatal(err)
	}
	if err := lt.ChooseMenuItem("Ask to add to the room"); err != nil {
		t.Fatal(err)
	}
	lap.drain()
	show(desk, dt, roomsPath)
	untilBoth(t, desk, dt, lap, lt, "the song on the host's page", func() bool {
		return dt.HasText("Room song · Band") && dt.HasText("From Lap")
	})
	click(t, desk, dt, "Decline Room song")
	untilBoth(t, desk, dt, lap, lt, "the song gone", func() bool { return !dt.HasText("From Lap") && len(desk.a.Rooms().Requests()) == 0 })
}

// TestRoomLetInFromThePage has the guest's page tell it waits for the host,
// whose page lets it in.
func TestRoomLetInFromThePage(t *testing.T) {
	n := roomtest.New()
	desk := newRoomShell(t, n, "Desk")
	lap := newRoomShell(t, n, "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	quietRooms(t, desk)
	if _, err := desk.svc.Preferences.Update(func(s *settings.Settings) { s.RoomAskJoin, s.RoomGuestControl = true, true }); err != nil {
		t.Fatal(err)
	}
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	show(lap, lt, roomsPath)
	show(desk, dt, roomsPath)
	joinByLink(t, desk, dt, lap, lt)
	untilBoth(t, desk, dt, lap, lt, "the knock", func() bool {
		return lt.HasText("Waiting for Desk to let you in…") && dt.HasText("Asking to join") &&
			len(desk.a.Rooms().Knocks()) == 1 && dt.HasText(desk.a.Rooms().Knocks()[0].Key+" · on this network")
	})
	click(t, desk, dt, "Let Lap in")
	untilBoth(t, desk, dt, lap, lt, "in the room", func() bool { return lt.HasText("In Desk's room") })
	// The host lets guests control its player: the guest's page has the
	// controls.
	untilBoth(t, desk, dt, lap, lt, "the controls", func() bool { return lt.HasText("next") && lt.HasText("previous") })
}

// TestRoomMakeHostFromThePage moves the room to the guest from the host's
// page: the guest hosts it, and the old host is in it.
func TestRoomMakeHostFromThePage(t *testing.T) {
	n := roomtest.New()
	desk := newRoomShell(t, n, "Desk")
	lap := newRoomShell(t, n, "Lap")
	dt, lt := tester(t, desk), tester(t, lap)
	quietRooms(t, lap)
	if err := desk.a.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinByLink(t, desk, dt, lap, lt)
	show(desk, dt, roomsPath)
	show(lap, lt, roomsPath)
	untilBoth(t, desk, dt, lap, lt, "Lap on the host's page", func() bool { return dt.HasText("Lap") && lt.HasText("In Desk's room") })
	click(t, desk, dt, "Make Lap the host")
	untilBoth(t, desk, dt, lap, lt, "the ask", func() bool {
		return lt.HasText("Desk asks you to host the room: this computer would play the room's queue, in place of yours, and the others would join it here.") &&
			dt.HasText("Waiting for Lap to host the room…")
	})
	click(t, lap, lt, "host the room")
	untilBoth(t, desk, dt, lap, lt, "the room moved", func() bool {
		_, hosts := lap.a.Rooms().Hosted()
		return hosts && lt.HasText("copy invite link") && dt.HasText("In Lap's room")
	})
}
