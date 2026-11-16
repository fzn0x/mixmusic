package app

import (
	"net/url"
	"strings"

	"github.com/fzn0x/mixmusic/internal/room"
)

// RoomPage is the page of the main window that shows rooms.
const RoomPage = "/room"

// followRoomLink opens an invite link, mixmusic://join/<code>#<key>.<secret>,
// which knocks at its room once show showed the main window, and reports
// whether u is one.
func (a *App) followRoomLink(u *url.URL, show func()) bool {
	if u.Host != "join" {
		return false
	}
	in, err := room.InviteOf(strings.Trim(u.Path, "/"), u.Fragment)
	show()
	if err != nil {
		a.notifyRooms(roomProblem("", err))
		return true
	}
	a.openInvite(in)
	a.navigateTo(RoomPage)
	return true
}
