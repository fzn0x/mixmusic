package main

// Settings › Desktop › Listen with friends (#561): starting a room, or
// opening the one this computer is in, the user's name in rooms, and
// whether a guest plays at the host's volume.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("desktop", 16, (*shell).roomSettings) }

// roomSettings start a room or open the one this computer is in, and keep
// the user's name in rooms and whether a guest follows the host's volume.
func (s *shell) roomSettings(c *ui.Context) {
	rs := s.a.Rooms()
	s.group(c, roomsTitle(), func() {
		_, hosts := rs.Hosted()
		guest, isGuest := rs.Guest()
		switch {
		case hosts:
			s.row(c, locales.RoomSettingsYourRoomOpen(), locales.RoomSettingsJoinIt(), func() {
				if s.button(c, "room-settings-open", locales.DesktopSettingsOpen(), false).Label(locales.RoomSettingsOpenYourRoom()).Clicked() {
					s.navigate(roomsPath)
				}
			}, "listen with friends room party together")
		case isGuest && guest.Step != app.RoomEnded:
			s.row(c, locales.RoomPageInRoom(guest.Host), locales.RoomSettingsHearPlays(guest.Host), func() {
				if s.button(c, "room-settings-open", locales.DesktopSettingsOpen(), false).Label(locales.RoomSettingsOpenTheRoom()).Clicked() {
					s.navigate(roomsPath)
				}
			}, "listen with friends room party together")
		default:
			s.row(c, locales.RoomSettingsStartRoom(), locales.RoomSettingsStartDesc(), func() {
				if s.button(c, "room-settings-start", locales.RoomPageStartRoom(), false).Disabled(s.rooms.starting).Clicked() {
					s.startRoom()
				}
			}, "listen with friends room party together host")
		}
		// Join a room…, and the rows of friends outside this network
		// (settings_roomsanywhere.go).
		s.settingsOf(c, roomRows)
		s.row(c, locales.RoomSettingsYourName(), locales.RoomSettingsNameDesc(), func() {
			in, _ := s.settingInput(c, "room-name", s.set.RoomName, func(name string) {
				s.set.RoomName = name
				s.save()
			})
			in.Placeholder(rs.Name()).Width(220).Label(locales.RoomSettingsYourName())
		}, "name guest room")
		// What the guests of the user's room can do (roomrules.go).
		s.roomRuleRows(c)
		s.row(c, locales.RoomSettingsFollowVolume(), locales.RoomSettingsFollowDesc(), func() {
			if s.switchOf(c.Key("room-follow-volume"), &s.set.RoomFollowVolume).Label(locales.RoomSettingsFollowVolume()).Changed() {
				s.save()
			}
		}, "volume guest room host")
	})
}
