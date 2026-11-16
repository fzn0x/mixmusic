package main

// Settings › Desktop › Listen with friends, for friends outside this
// network (#562): Port for rooms, Address for invites, and Join a room….
// The rows of the group register from their features' files.

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/locales"
)

// roomRows are the rows of Settings › Desktop › Listen with friends that
// register from their features' files.
const roomRows = "desktop › Listen with friends"

func init() { addSettings(roomRows, 10, (*shell).joinRoomRow) }

// joinRoomRow opens the rooms' page, which takes an invite link, or an
// address and a code.
func (s *shell) joinRoomRow(c *ui.Context) {
	rs := s.a.Rooms()
	if _, hosts := rs.Hosted(); hosts {
		return
	}
	s.row(c, locales.RoomSettingsJoinTitle(), locales.RoomSettingsJoinDesc(), func() {
		if s.button(c, "room-settings-join", locales.RoomSettingsJoinButton(), false).Clicked() {
			s.navigate(roomsPath)
		}
	}, "listen with friends room join link address code")
}

func init() { addSettings(roomRows, 20, (*shell).roomPortRow) }

// roomPortRow keeps the port rooms listen on, which a port forwarded by
// hand to this computer needs.
func (s *shell) roomPortRow(c *ui.Context) {
	saved := ""
	if s.set.RoomPort != 0 {
		saved = strconv.Itoa(s.set.RoomPort)
	}
	s.row(c, locales.RoomSettingsPortTitle(), locales.RoomSettingsPortDesc(), func() {
		in, _ := s.settingInput(c, "room-port", saved, func(v string) {
			p, err := strconv.Atoi(v)
			if v != "" && (err != nil || p < 1024 || p > 65535) {
				s.notes = append(s.notes, notice(locales.RoomSettingsPortInvalid()))
				return
			}
			s.set.RoomPort = p
			s.save()
		})
		in.Placeholder(locales.RoomSettingsPortPlaceholder()).Width(160).Label(locales.RoomSettingsPortLabel())
	}, "port forward router listen with friends room internet")
}

func init() { addSettings(roomRows, 30, (*shell).roomAddressRow) }

// roomAddressRow keeps the address invites carry in place of those
// MixMusic finds.
func (s *shell) roomAddressRow(c *ui.Context) {
	s.row(c, locales.RoomSettingsAddressTitle(), locales.RoomSettingsAddressDesc(), func() {
		in, _ := s.settingInput(c, "room-address", s.set.RoomAddress, func(v string) {
			if _, ok := room.InviteAddr(v, 1024); v != "" && !ok {
				s.notes = append(s.notes, notice(locales.RoomSettingsAddressInvalid()))
				return
			}
			s.set.RoomAddress = v
			s.save()
			s.a.Rooms().ReachAgain()
		})
		in.Placeholder(locales.RoomSettingsAddressPlaceholder()).Width(220).Label(locales.RoomSettingsAddressLabel())
	}, "address vpn tailscale invite internet router listen with friends room")
}
