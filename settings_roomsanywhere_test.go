package main

import (
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/room/roomtest"
)

// TestRoomSettingsForFriendsOutside shows Join a room…, Port for rooms and
// Address for invites in Settings › Desktop › Listen with friends, and
// keeps a port typed.
func TestRoomSettingsForFriendsOutside(t *testing.T) {
	s := newRoomShell(t, roomtest.New(), "Desk")
	tt := tester(t, s)
	show(s, tt, "/settings/desktop")
	until(t, s, tt, "the rows", func() bool {
		return tt.HasText("Join a room…") && tt.HasText("Port for rooms") && tt.HasText("Address for invites")
	})
	click(t, s, tt, "The port rooms listen on")
	tt.Type("41234")
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	until(t, s, tt, "the port kept", func() bool { return s.svc.Preferences.Get().RoomPort == 41234 })
	if got := strconv.Itoa(s.svc.Preferences.Get().RoomPort); got != "41234" {
		t.Fatalf("port %s", got)
	}
}
