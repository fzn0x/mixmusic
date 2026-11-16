package main

// Listen with friends (#561), in the main window: Start a room, and the
// rooms on the network, in the devices menu of the cast button; the page
// of the room this computer hosts or is a guest of, and of a room found
// (roompage.go); and Settings › Desktop › Listen with friends
// (settings_rooms.go). internal/app's room*.go runs them.

import (
	"net/url"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/locales"
)

// roomView is what the window keeps of rooms from frame to frame.
type roomView struct {
	// code is what the user typed of a room's code, and link the invite
	// link they pasted.
	code, link string
	// addr and addrCode are the address and the code of a room the user
	// joins at an address (rooms_anywhere.go).
	addr, addrCode string
	// starting is set while a room opens.
	starting bool
}

// roomsPath is the rooms' page, which deep links to a room open too.
const roomsPath = app.RoomPage

// roomPath is the page of the room of the host id, found on the network.
func roomPath(id string) string { return roomsPath + "/" + url.PathEscape(id) }

// roomsShown reports whether the devices menu shows rooms: while this
// computer hosts one or is a guest of one, or one is on the network.
func (s *shell) roomsShown() bool {
	rs := s.a.Rooms()
	if _, ok := rs.Hosted(); ok {
		return true
	}
	if _, ok := rs.Guest(); ok {
		return true
	}
	return len(rs.Found()) > 0
}

// roomItems lists the rooms in the devices menu, after the cast devices:
// the room this computer hosts or is a guest of, which opens its page, or
// Start a room, and the rooms on the network, by their hosts' names.
func (s *shell) roomItems(m *ui.Menu) {
	if !s.roomsShown() {
		return
	}
	rs := s.a.Rooms()
	m.Separator()
	hosted, hosts := rs.Hosted()
	guest, isGuest := rs.Guest()
	in := isGuest && guest.Step != app.RoomEnded
	switch {
	case hosts:
		label := locales.RoomMenuYourRoom()
		if n := len(hosted.Guests); n > 0 {
			label += " · " + listening(n)
		}
		if m.Item(label).Chosen() {
			s.navigate(roomsPath)
		}
	case in:
		if m.Item(locales.RoomPageInRoom(guest.Host)).Chosen() {
			s.navigate(roomsPath)
		}
	default:
		if m.Item(locales.RoomSettingsStartRoom()).Disabled(s.rooms.starting).Chosen() {
			s.startRoom()
		}
	}
	for _, f := range rs.Found() {
		if in && f.ID == guest.ID {
			continue
		}
		if m.Item(locales.RoomPageNameRoom(f.Name)).Disabled(hosts || in).Chosen() {
			s.navigate(roomPath(f.ID))
		}
	}
}

// listening tells how many guests listen.
func listening(n int) string {
	return locales.RoomMenuListening(n)
}

// startRoom opens a room beside the main thread, which may read the key,
// and shows its page.
func (s *shell) startRoom() {
	s.rooms.starting = true
	go func() {
		err := s.a.Rooms().Start()
		s.update(func() {
			s.rooms.starting = false
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			// Listening, MixMusic may have been let in or kept out, by
			// Windows Firewall's question: it's read again.
			s.firewall.reread()
			s.navigate(roomsPath)
		})
	}()
}

// newRoomCode gives the room a new code and invite link beside the main
// thread, and tells why it can't.
func (s *shell) newRoomCode() {
	go func() {
		err := s.a.Rooms().NewCode()
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.notes = append(s.notes, notice(locales.RoomMenuNewCode()))
		})
	}()
}

// followRooms redraws the window as rooms change, and tells what they
// say, as a guest that joined.
func (s *shell) followRooms() func() {
	return s.a.OnRooms(func(note string) {
		s.update(func() {
			if note != "" {
				s.notes = append(s.notes, notice(note))
			}
		})
	})
}
