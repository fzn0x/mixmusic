package main

// Settings › Desktop › Listen with friends (#561): starting a room, or
// opening the one this computer is in, the user's name in rooms, and
// whether a guest plays at the host's volume.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

func init() { addSettings("desktop", 16, (*shell).roomSettings) }

// roomSettings start a room or open the one this computer is in, and keep
// the user's name in rooms and whether a guest follows the host's volume.
func (s *shell) roomSettings(c *ui.Context) {
	rs := s.a.Rooms()
	s.group(c, roomsTitle, func() {
		_, hosts := rs.Hosted()
		guest, isGuest := rs.Guest()
		switch {
		case hosts:
			s.row(c, "Your room is open", "Friends on this network join it with its code or its invite link.", func() {
				if s.button(c, "room-settings-open", "open", false).Label("Open your room").Clicked() {
					s.navigate(roomsPath)
				}
			}, "listen with friends room party together")
		case isGuest && guest.Step != app.RoomEnded:
			s.row(c, "In "+guest.Host+"'s room", "You hear what "+guest.Host+" plays, in step.", func() {
				if s.button(c, "room-settings-open", "open", false).Label("Open the room").Clicked() {
					s.navigate(roomsPath)
				}
			}, "listen with friends room party together")
		default:
			s.row(c, "Start a room", "Friends on this network hear what you play, in step, on their own computers: they join with the room's code or its invite link. Your audio is never sent.", func() {
				if s.button(c, "room-settings-start", "start a room", false).Disabled(s.rooms.starting).Clicked() {
					s.startRoom()
				}
			}, "listen with friends room party together host")
		}
		// Join a room…, and the rows of friends outside this network
		// (settings_roomsanywhere.go).
		s.settingsOf(c, roomRows)
		s.row(c, "Your name in rooms", "What the host and the other guests see you as: of at most 50 characters. Empty, it's this computer's name.", func() {
			in, _ := s.settingInput(c, "room-name", s.set.RoomName, func(name string) {
				s.set.RoomName = name
				s.save()
			})
			in.Placeholder(rs.Name()).Width(220).Label("Your name in rooms")
		}, "name guest room")
		s.row(c, "Follow the host's volume", "As a guest, play at the host's volume rather than your own.", func() {
			if s.switchOf(c.Key("room-follow-volume"), &s.set.RoomFollowVolume).Label("Follow the host's volume").Changed() {
				s.save()
			}
		}, "volume guest room host")
	})
}
